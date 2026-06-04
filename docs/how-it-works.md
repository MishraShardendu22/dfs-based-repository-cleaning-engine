# How It Works

## Operational Model

GitHub-Cleaner-Go operates as an **autonomous repository maintenance agent**. Unlike interactive cleanup tools that require manual input per operation, this engine processes repositories in batch, making independent decisions about dead component elimination based on static analysis of the source graph.

The system does not maintain state between executions. Each run is an atomic operation that:

1. Discovers repositories from GitHub
2. Processes up to 5 repositories concurrently via a goroutine pool
3. Destroys all local state (cloned repos in `_Repos/`) upon completion

## Cleanup Pipeline (Detailed)

### Phase 1: Repository Enumeration

```
GET https://api.github.com/users/{username}/repos?per_page=100
       │
       ▼
Response: [{ name: "repo-1" }, { name: "repo-2" }, ...]
       │
       ▼
Extracted: ["repo-1", "repo-2", ...]
```

The GitHub API is queried without authentication. This imposes a rate limit of 60 requests per hour for unauthenticated requests.

### Phase 2: Concurrent Worker Dispatch

```
For each repo name:
   │
   ├── wg.Add(1)
   ├── go func(repo) {
   │     limit <- struct{}{}          // Acquire semaphore slot
   │     defer func() { <-limit }()    // Release slot
   │     defer wg.Done()
   │     
   │     util.MetricsRegistry.ActiveWorkers.Inc()
   │     defer util.MetricsRegistry.ActiveWorkers.Dec()
   │     
   │     CloneAndClean(repo)
   │   }(repo)
   │
wg.Wait()   // Wait for all workers to complete
```

A buffered channel of capacity 5 acts as a semaphore, ensuring no more than 5 goroutines run simultaneously.

### Phase 3: Clone & Cleanup

```
CloneAndClean(repo):
   │
   ├── Construct SSH URL: git@github.com:{user}/{repo}.git
   ├── Execute: git clone {url} _Repos/{repo}
   ├── On failure: log error, increment metric, return
   ├── Resolve absolute path: filepath.Abs("_Repos/{repo}")
   ├── Run: Cleaner(absRepoPath, repo, repoStart)
   └── Defer: os.RemoveAll(absRepoPath)
```

The working directory is never changed — all operations use absolute paths, enabling safe concurrent processing.

### Phase 4: Recursive Project Discovery

```
Cleaner(absRepoPath, repo, repoStart):
  └── DeepSearchAndClean(absRepoPath, repo, repoStart)

DeepSearchAndClean("/repo-root", repo, repoStart):
   │
   ├── Segregator(currFolder, false) → files
   ├── Segregator(currFolder, true)  → directories
   ├── Contains(files, "package.json")?
   │   ├── Yes → CleanThis(folder, repo, repoStart) ← React project found
   │   └── No  → Recurse into each subdirectory
   │               │
   │               └── DeepSearchAndClean("/repo-root/src", repo, repoStart)
   │                     ├── List files → ["App.tsx", ...]
   │                     └── No package.json → recurse...
```

This recursion terminates at two points:
1. A directory containing `package.json` — triggers the cleanup pipeline
2. A directory with no subdirectories — leaf node, nothing to process

### Phase 5: React Detection

```
Read file: package.json
   │
   ├── strings.Contains(raw, "react")?
   │   ├── Yes → Continue
   │   └── No  → Return (not a React project)
   │
   ├── strings.Contains(raw, "react-dom")?
   │   ├── Yes → React project confirmed
   │   └── No  → Return (not a React project)
```

The detection is intentionally permissive — it performs substring matching on the raw file content rather than structured JSON parsing of `dependencies` or `devDependencies`. This means comments or package-lock remnants containing "react" could produce false positives.

### Phase 6: UI Directory Discovery

```
FindUIDir(root)
   │
   └── For each directory entry:
         ├── d.IsDir()?
         ├── filepath.Base(path) == "ui"?
         └── filepath.Base(filepath.Dir(path)) == "components"?
               │
               ├── All true → Return path: /root/src/components/ui
               └── Any false → Continue walking
```

The walker scans every directory in the tree. The first (and only the first) `components/ui` match is used.

### Phase 7: Source Graph Analysis

```
For each file in root (recursive):
   │
   ├── Extension in [.ts, .tsx, .js, .jsx]?
   │   ├── Yes → Read file content
   │   └── No  → Skip
   │
   ├── Apply regex: [./@"]components/ui/([A-Za-z0-9_-]+)
   │
   └── For each match:
         └── used[strings.ToLower(match)] = true
```

The regex captures the component name after the `components/ui/` prefix. The pattern supports:

| Prefix | Example | Matches |
|---|---|---|
| `./` | `./components/ui/Button` | Relative imports |
| `@/` | `@/components/ui/Card` | Path alias imports |
| `"` | `"components/ui/Modal"` | Bare specifier imports |
| None | `components/ui/Button` | Direct references |

### Phase 8: Dead Component Elimination

```
For each entry in components/ui/:
   │
   ├── Extract name without extension: "Button.tsx" → "Button"
   ├── Lowercase: "Button" → "button"
   │
   ├── used["button"] == true?
   │   ├── No  → os.RemoveAll(entry) → DELETED
   │   └── Yes → Keep
   │
   └── FilesDeletedTotal.Add(deletedCount)  ← Prometheus metric
```

Components are deleted using `os.RemoveAll`, which handles both files and directories. This means component subdirectories (e.g., `Button/index.tsx`) are also removed.

### Phase 9: Build Verification

```
LogBuildStart(logger, repo)
build := exec.Command("sh", "-c", "npm install --legacy-peer-deps && npm run build")
build.Dir = filesAndFolder
build.Stdout = os.Stdout
build.Stderr = os.Stderr
buildErr := build.Run()

buildStatus := "success"
if buildErr != nil {
    buildStatus = "failed"
    MetricsRegistry.BuildFailuresTotal.Inc()
}
LogBuildEnd(logger, repo, buildDur, buildStatus, buildErr)
MetricsRegistry.BuildDurationSeconds.Observe(buildDur.Seconds())
```

Build output is piped to stdout/stderr for visibility. Build errors are captured, logged, and metered, but build failures do not halt the pipeline or revert the deletions.

### Phase 10: Git Automation

```
LogGitCommitStart(logger, repo)
git := exec.Command("sh", "-c", "git cm 'auto: cleanup ui and build'")
git.Dir = filesAndFolder
git.Stdout = os.Stdout
git.Stderr = os.Stderr
gitErr := git.Run()

if gitErr != nil {
    LogGitCommitEnd(logger, repo, gitDur, gitErr)
    MetricsRegistry.GitCommitFailuresTotal.Inc()
} else {
    LogGitCommitEnd(logger, repo, gitDur, nil)
}
```

This stages all changes and commits them. The `-a` flag automatically stages all modified and deleted files. No push is performed — changes remain local. Git errors are logged and metered.

### Phase 11: Repository Cleanup

```
defer func() {
    os.RemoveAll(absRepoPath)    // Delete cloned repository
}()
```

Executed when `CloneAndClean()` returns, regardless of success or failure of the cleanup pipeline.

### Phase 12: Execution Summary

```
logger.Info("execution_summary",
    slog.Int("total_repos", totalRepos),
)
```

After all workers complete, a summary log entry is emitted with the total number of repositories processed.

## Mermaid: Complete System Workflow

```mermaid
flowchart LR
    subgraph Input
        A[GitHub Account]
    end

    subgraph Orchestration
        B[API Discovery]
        C[Concurrent Worker Pool]
        D[SSH Clone]
    end

    subgraph Analysis
        E[Recursive Traversal]
        F[React Detection]
        G[UI Directory Locator]
        H[Import Regex Scan]
        I[Usage Map Build]
    end

    subgraph Execution
        J[Dead Component Deletion]
        K[Build Validation]
        L[Git Commit]
    end

    subgraph Cleanup
        M[Repository Removal]
    end

    A --> B --> C --> D --> E
    E --> F --> G --> H --> I
    I --> J --> K --> L --> M
```

## Mermaid: Cleanup Pipeline

```mermaid
flowchart TD
    subgraph "Cleanup Pipeline per Repository"
        S[Start CleanThis] --> D1{Has package.json?}
        D1 -->|No| R1[Recurse Deeper]
        D1 -->|Yes| D2{Has react + react-dom?}
        D2 -->|No| R2[Skip - Not React]
        D2 -->|Yes| D3{Has components/ui?}
        D3 -->|No| R3[Skip - No UI Dir]
        D3 -->|Yes| W[Walk All Source Files]
        W --> RX[Apply Import Regex]
        RX --> UM[Build Usage Map]
        UM --> DEL[Delete Unused Components]
        DEL --> BUILD[npm install + build]
        BUILD --> GIT[git commit]
        GIT --> DONE[Done]
    end
```

## Mermaid: Repository Traversal Flow

```mermaid
flowchart TD
    subgraph "Recursive Traversal (DeepSearchAndClean)"
        ENTER[Enter Directory] --> LIST[Segregator: Files & Dirs]
        LIST --> CHECK{Has package.json?}
        CHECK -->|Yes| CLEAN[Enter CleanThis Pipeline]
        CHECK -->|No| RECURSE[For Each Subdirectory]
        RECURSE --> ENTER
        CLEAN --> RETURN[Return to Parent]
        RETURN --> NEXT{More Dirs?}
        NEXT -->|Yes| RECURSE
        NEXT -->|No| EXIT[Exit Node]
    end
```

## Mermaid: Component Analysis Flow

```mermaid
flowchart TD
    subgraph "Import Resolution & Dead Component Detection"
        UI[components/ui/ Directory] --> LIST_UI[List All Entries]

        subgraph SourceScan[Source File Scan]
            SRC[Source Files .ts/.tsx/.js/.jsx] --> READ[Read Content]
            READ --> REGEX[Apply Import Regex]
            REGEX --> MATCH{Capture Component Name?}
            MATCH -->|Yes| ADD_USED[Add to Used Set]
            MATCH -->|No| NEXT_FILE[Next File]
        end

        LIST_UI --> FOR_EACH[For Each Entry]
        FOR_EACH --> STEM[Extract Stem + Lowercase]
        STEM --> CHECK_USED{In Used Set?}
        CHECK_USED -->|No| DELETE[os.RemoveAll]
        CHECK_USED -->|Yes| KEEP[Preserve File]
        DELETE --> REPORT[Log Deletion]
        KEEP --> NEXT_ENTRY[Next Entry]
        NEXT_ENTRY --> FOR_EACH
    end