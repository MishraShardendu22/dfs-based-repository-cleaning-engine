# Repository Scanner

## Overview

The repository scanner subsystem handles **autonomous discovery, concurrent cloning, traversal, and lifecycle management** of GitHub repositories. It is the first stage of the cleanup pipeline and establishes the execution context for all downstream analysis.

## Component Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                        Repository Scanner                             │
├──────────────────┬────────────────────┬─────────────────────────────┤
│  Discovery Layer  │  Clone Layer       │  Traversal Layer            │
├──────────────────┼────────────────────┼─────────────────────────────┤
│  HTTP Client      │  Git Subprocess    │  Depth-First Walker         │
│  JSON Decoder     │  Absolute Path     │  Project Detector           │
│  Error Boundary   │  Deferred Cleanup  │  Delegation Router          │
├──────────────────┴────────────────────┴─────────────────────────────┤
│  Concurrency Layer                                                  │
│  Channel Semaphore (limit chan struct{}) | sync.WaitGroup           │
└──────────────────────────────────────────────────────────────────────┘
```

## Concurrency Layer

The scanner processes up to 5 repositories concurrently using a channel-based semaphore pattern:

```go
limit := make(chan struct{}, 5)

for _, repo := range repos {
    wg.Add(1)

    go func(r string) {
        defer wg.Done()
        limit <- struct{}{}          // Acquire slot
        defer func() { <-limit }()   // Release slot

        util.MetricsRegistry.ActiveWorkers.Inc()
        defer util.MetricsRegistry.ActiveWorkers.Dec()

        CloneAndClean(r)
    }(repo)
}

wg.Wait()
```

**Properties**:
- **Bounded parallelism**: Max 5 concurrent goroutines
- **Fair scheduling**: Repositories are dispatched in order as slots become available
- **Graceful shutdown**: `sync.WaitGroup` ensures all workers complete before `main()` exits
- **Observability**: `active_workers` Prometheus gauge tracks real-time concurrency

## Discovery Layer

### Mechanism

```go
func GetAllRepos(url string) []string {
    resp, err := http.Get(url)
    // ...
    var repos []model.Repo
    json.NewDecoder(resp.Body).Decode(&repos)
    // ...
    for _, r := range repos {
        names = append(names, r.Name)
    }
    return names
}
```

The discovery layer makes a single HTTP GET request to the GitHub REST API:

```
GET https://api.github.com/users/{username}/repos?per_page=100
```

**Response structure**:

```json
[
  {
    "name": "repository-name",
    "full_name": "username/repository-name",
    "private": false,
    "fork": false,
    ...
  },
  ...
]
```

Only the `name` field is deserialized via the `model.Repo` struct. All other fields in the response are discarded.

### Rate Limiting

| Authentication | Rate Limit | Per-Window Cost |
|---|---|---|
| Unauthenticated | 60 requests/hour | 1 request per execution |
| Authenticated (token) | 5,000 requests/hour | Requires code modification |

The current implementation uses `http.Get` without authentication headers. For accounts with more than 60 repositories, pagination would be required, which is not implemented.

### Error Handling

`GetAllRepos` uses `log.Fatal` for all error conditions:
- HTTP request failure → immediate termination
- JSON decode failure → immediate termination

This is a hard failure boundary. If repository discovery fails, no cleanup occurs.

## Clone Layer

### Mechanism

```go
func CloneAndClean(repo string) {
    repoPath := filepath.Join("_Repos", repo)
    repoURL := "git@github.com:MishraShardendu22/" + repo + ".git"

    util.LogCloneStart(logger, repo)
    cloneStart := time.Now()

    cmd := exec.Command("git", "clone", repoURL, repoPath)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    err := cmd.Run()
    cloneDur := time.Since(cloneStart)

    if err != nil {
        util.LogCloneEnd(logger, repo, cloneDur, err)
        util.MetricsRegistry.CloneFailuresTotal.Inc()
        util.LogFailure(logger, repo, "clone", err)
        return
    }
    util.LogCloneEnd(logger, repo, cloneDur, nil)
    util.MetricsRegistry.CloneDurationSeconds.Observe(cloneDur.Seconds())

    // Resolve absolute path for isolated processing
    repoPath = filepath.Join("_Repos", repo)
    absRepoPath, err := filepath.Abs(repoPath)
    // ...
    defer func() {
        os.RemoveAll(absRepoPath)
    }()

    Cleaner(absRepoPath, repo, repoStart)
}
```

**Clone lifecycle**:

1. Clone destination: `_Repos/{repo-name}` (subdirectory under working dir)
2. SSH URL constructed: `git@github.com:{username}/{repo}.git`
3. `git clone` executed as subprocess with stdout/stderr passthrough
4. Clone duration measured and logged
5. On failure: metrics incremented, return (no further processing)
6. On success: absolute path resolved via `filepath.Abs`
7. `Cleaner()` invoked with absolute path
8. On return: cloned repository deleted via `os.RemoveAll`

### SSH Authentication

The clone operation depends entirely on the user's SSH configuration:
- SSH agent must be running
- SSH key must be registered with GitHub
- Key must have access to the target repositories

**Failure modes**:
- No SSH agent → `git clone` fails
- No registered key → authentication failure
- Repository doesn't exist → 404 error from GitHub
- Network connectivity issues → transport error

Clone failures are handled gracefully: the error is logged, `CloneFailuresTotal` metric is incremented, and processing continues to the next repository.

### Path Isolation

Unlike the earlier design that used `os.Chdir` (which mutates global process state), the current implementation uses:
- **Clone destination**: `_Repos/{repo}` subdirectory
- **Absolute path resolution**: `filepath.Abs(repoPath)` for safe concurrent access
- **Build directory**: `build.Dir = filesAndFolder` for npm subprocess

This allows multiple repositories to be processed concurrently without race conditions on the working directory.

### Deferred Cleanup

```go
defer func() {
    os.RemoveAll(absRepoPath)
}()
```

This ensures cleanup even if a panic occurs during analysis. GitHub clone authentication relies entirely on the user's SSH configuration.

## Traversal Layer

### Mechanism

```go
func DeepSearchAndClean(currFolder string, repo string, repoStart time.Time) {
    dirs := util.Segregator(currFolder, true)    // Get directories
    files := util.Segregator(currFolder, false)   // Get files

    if util.Contains(files, "package.json") {
        CleanThis(currFolder, repo, repoStart)
        return
    }

    for _, d := range dirs {
        DeepSearchAndClean(filepath.Join(currFolder, d), repo, repoStart)
    }
}
```

### Traversal Algorithm

The traversal is a **depth-first, recursive directory walk** with the following properties:

| Property | Value |
|---|---|
| Algorithm | DFS (recursive) |
| Stop condition | `package.json` found OR no subdirectories |
| Node evaluation | File + directory separation via `Segregator()` |
| Branching | All subdirectories traversed |
| Cycle detection | None (filesystem prevents cycles via symlinks not followed) |
| Max depth | Limited by Go call stack (~1GB default, effectively unlimited) |

### Node Evaluation

At each directory node, two lists are computed using `Segregator()`:

```go
func Segregator(root string, wantDir bool) []string {
    entries, _ := os.ReadDir(root)
    var items []string
    for _, e := range entries {
        if wantDir && e.IsDir() {
            items = append(items, e.Name())
        }
        if !wantDir && !e.IsDir() {
            items = append(items, e.Name())
        }
    }
    return items
}
```

**Note**: Errors from `os.ReadDir` are silently ignored (underscore assignment).

### Stop Condition

The traversal stops descending when `package.json` is found in the current directory. The directory is then passed to `CleanThis()` for full cleanup pipeline processing.

This design assumes that `package.json` always marks a project root. In monorepo structures where multiple `package.json` files exist at nested levels, the outermost match is processed, and the traversal of that branch terminates. Nested `package.json` files are never reached.

### Monorepo Handling

```
monorepo/
├── package.json        ← Detected first, traversal stops here
├── packages/
│   ├── app/
│   │   └── package.json     ← Never reached
│   └── web/
│       └── package.json     ← Never reached
```

In this case, only the root-level `components/ui` is analyzed. Nested workspace packages are ignored.

### Directory Exclusion

The scanner does **not** exclude any directories:
- `.git` directories are traversed (though they won't contain `package.json`)
- `node_modules` directories are traversed (wasteful but not harmful)
- Hidden directories (`.` prefixed) are traversed

This increases traversal time but does not affect correctness, as `node_modules` and `.git` rarely contain `components/ui` directories.

## Performance Characteristics

| Factor | Impact |
|---|---|
| Repository count | Linear O(R) — processed concurrently up to 5 at a time |
| Repository size | Clone time dominates — large repos with long git history increase execution time |
| Directory depth | Linear O(D) — deeper trees require more recursion |
| File count | Linear O(N) for listing + O(F) for file walk during import analysis |
| npm install | Highly variable — dependency resolution can take minutes per repo |

## Failure Recovery

The scanner has no checkpointing or progress persistence. If the process is interrupted:
1. Currently cloning repo — orphaned `_Repos/` directory may remain on disk
2. Currently analyzing repo — incomplete cleanup, but deferred cleanup runs on function return
3. Process killed (SIGKILL) — deferred cleanup does not execute, orphaned clones remain in `_Repos/`

## Scanner Configuration

All scanner parameters are hardcoded:

| Parameter | Value | Hardcoded Location |
|---|---|---|
| GitHub username | `MishraShardendu22` | `GetAllRepos()` URL string |
| API endpoint | `api.github.com` | `GetAllRepos()` URL string |
| Max repos | `100` | `per_page=100` query parameter |
| Clone protocol | SSH | `git@github.com:` URL prefix |
| Clone destination | `_Repos/{repo}` | `filepath.Join("_Repos", repo)` |
| Concurrency limit | `5` | `make(chan struct{}, 5)` |
| Traversal strategy | DFS (recursive) | Algorithm choice |
| Stop condition | `package.json` presence | `util.Contains(files, "package.json")` |
| Directory filter | None | Absence of filter logic |