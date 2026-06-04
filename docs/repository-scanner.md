# Repository Scanner

## Overview

The repository scanner subsystem handles **autonomous discovery, cloning, traversal, and lifecycle management** of GitHub repositories. It is the first stage of the cleanup pipeline and establishes the execution context for all downstream analysis.

## Component Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Repository Scanner                         │
├──────────────────┬───────────────────┬───────────────────────┤
│  Discovery Layer │  Clone Layer      │  Traversal Layer      │
├──────────────────┼───────────────────┼───────────────────────┤
│  HTTP Client     │  Git Subprocess   │  Depth-First Walker   │
│  JSON Decoder    │  Context Switch   │  Project Detector     │
│  Error Boundary  │  Deferred Cleanup │  Delegation Router    │
└──────────────────┴───────────────────┴───────────────────────┘
```

## Discovery Layer

### Mechanism

```go
func getRepos(url string) []string {
    resp, err := http.Get(url)
    // ...
    var repos []Repo
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

Only the `name` field is deserialized. All other fields in the response are discarded.

### Rate Limiting

| Authentication | Rate Limit | Per-Window Cost |
|---|---|---|
| Unauthenticated | 60 requests/hour | 1 request per execution |
| Authenticated (token) | 5,000 requests/hour | Requires code modification |

The current implementation uses `http.Get` without authentication headers. For accounts with more than 60 repositories, pagination would be required, which is not implemented.

### Error Handling

`getRepos` uses `log.Fatal` for all error conditions:
- HTTP request failure → immediate termination
- JSON decode failure → immediate termination

This is a hard failure boundary. If repository discovery fails, no cleanup occurs.

## Clone Layer

### Mechanism

```go
func Clone(repo string) {
    repoURL := "git@github.com:MishraShardendu22/" + repo + ".git"
    cmd := exec.Command("git", "clone", repoURL)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()

    if err := os.Chdir(repo); err != nil {
        os.RemoveAll(repo)
        return
    }
    defer func() {
        os.Chdir("..")
        os.RemoveAll(repo)
    }()

    Cleaner()
}
```

**Clone lifecycle**:

1. SSH URL constructed: `git@github.com:{username}/{repo}.git`
2. `git clone` executed as subprocess with stdout/stderr passthrough
3. Working directory changed to cloned repository root
4. `Cleaner()` invoked to begin analysis
5. On return: directory restored to parent, clone deleted

### SSH Authentication

The clone operation depends entirely on the user's SSH configuration:
- SSH agent must be running
- SSH key must be registered with GitHub
- Key must have access to the target repositories

**Failure modes**:
- No SSH agent → `git clone` fails silently
- No registered key → authentication failure
- Repository doesn't exist → 404 error from GitHub
- Network connectivity issues → transport error

Since `cmd.Run()` return value is not checked, clone failures are silent. `os.Chdir` returning an error is the only clone-layer failure that is handled.

### Context Switch

`os.Chdir(repo)` changes the process's working directory. This is significant because Go's `filepath.WalkDir` and `os.ReadFile` use relative paths if none are provided. All subsequent operations occur relative to the cloned repository root.

### Deferred Cleanup

```go
defer func() {
    os.Chdir("..")
    os.RemoveAll(repo)
}()
```

Two operations are deferred:
1. `os.Chdir("..")` — returns to the original working directory
2. `os.RemoveAll(repo)` — recursively deletes the cloned repository

This ensures cleanup even if a panic occurs during analysis. However, if `Cleaner()` calls `os.Exit` (which it doesn't), deferred functions would not execute.

## Traversal Layer

### Mechanism

```go
func DeepSearchAndClean(folder string) {
    files := Folder(folder, false)
    dirs := Folder(folder, true)

    if Contains(files, "package.json") {
        CleanThis(folder)
        return
    }

    for _, d := range dirs {
        DeepSearchAndClean(filepath.Join(folder, d))
    }
}
```

### Traversal Algorithm

The traversal is a **depth-first, recursive directory walk** with the following properties:

| Property | Value |
|---|---|
| Algorithm | DFS (recursive) |
| Stop condition | `package.json` found OR no subdirectories |
| Node evaluation | File + directory listing |
| Branching | All subdirectories traversed |
| Cycle detection | None (filesystem prevents cycles via symlinks not followed) |
| Max depth | Limited by Go call stack (~1GB default, effectively unlimited) |

### Node Evaluation

At each directory node, two lists are computed:
1. `files` — non-directory entries
2. `dirs` — directory entries

The `Folder()` function:

```go
func Folder(root string, wantDir bool) []string {
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
| Repository count | Linear O(R) — each repo cloned and processed sequentially |
| Repository size | Clone time dominates — large repos with long git history increase execution time |
| Directory depth | Linear O(D) — deeper trees require more recursion |
| File count | Linear O(N) for listing + O(F) for file walk during import analysis |
| npm install | Highly variable — dependency resolution can take minutes per repo |

## Failure Recovery

The scanner has no checkpointing or progress persistence. If the process is interrupted:
1. Currently cloning repo — orphaned directory on disk
2. Currently analyzing repo — incomplete cleanup, but deferred cleanup runs on function return
3. Process killed (SIGKILL) — deferred cleanup does not execute, orphaned clones remain

## Scanner Configuration

All scanner parameters are hardcoded:

| Parameter | Value | Hardcoded Location |
|---|---|---|
| GitHub username | `MishraShardendu22` | `getRepos()` URL string |
| API endpoint | `api.github.com` | `getRepos()` URL string |
| Max repos | `100` | `per_page=100` query parameter |
| Clone protocol | SSH | `git@github.com:` URL prefix |
| Clone destination | Current working directory | Implicit |
| Traversal strategy | DFS (recursive) | Algorithm choice |
| Stop condition | `package.json` presence | `Contains(files, "package.json")` |
| Directory filter | None | Absence of filter logic |