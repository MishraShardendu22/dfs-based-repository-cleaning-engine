# Architecture

## System Overview

GitHub-Cleaner-Go implements a three-stage pipeline architecture: **Repository Orchestration**, **Static Analysis Engine**, and **Build Verification & Commit Pipeline**. Each stage is a discrete processing phase with well-defined inputs, outputs, and failure boundaries. Repositories are processed concurrently via a worker pool of up to 5 goroutines.

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                           GitHub-Cleaner-Go Engine                                │
│                                                                                    │
│  ┌─────────────────────┐   ┌──────────────────────┐   ┌────────────────────────┐  │
│  │  Stage 1             │   │  Stage 2             │   │  Stage 3               │  │
│  │  Repository          │──>│  Static Analysis     │──>│  Build & Commit        │  │
│  │  Orchestration       │   │  & Cleanup           │   │  Pipeline              │  │
│  └─────────────────────┘   └──────────────────────┘   └────────────────────────┘  │
│         │                           │                           │                  │
│         ▼                           ▼                           ▼                  │
│  ┌─────────────────┐       ┌──────────────────┐       ┌──────────────────┐        │
│  │ GitHub API       │       │ Import            │       │ npm install      │        │
│  │ HTTP Client      │       │ Regex Engine      │       │ Build Runner     │        │
│  ├─────────────────┤       ├──────────────────┤       ├──────────────────┤        │
│  │ SSH Git Clone   │       │ Filesystem        │       │ Git Client       │        │
│  │ (Concurrent)     │       │ Walker (DFS)      │       │ (Commit)         │        │
│  ├─────────────────┤       ├──────────────────┤       ├──────────────────┤        │
│  │ Worker Pool     │       │ Deletion          │       │ Local Cleanup    │        │
│  │ (5 goroutines)  │       │ Executor          │       │ (Remove Clone)   │        │
│  └─────────────────┘       └──────────────────┘       └──────────────────┘        │
│                                                                                    │
│  ┌──────────────────────────────────────────────────────────────────────────────┐ │
│  │                         Observability Layer                                   │ │
│  │  ┌──────────────┐     ┌───────────────────┐     ┌───────────────────────┐   │ │
│  │  │ Prometheus    │     │ Structured JSON   │     │ Prometheus + Grafana  │   │ │
│  │  │ Metrics :2112 │     │ Logging (slog)     │     │ (Docker Compose)      │   │ │
│  │  └──────────────┘     └───────────────────┘     └───────────────────────┘   │ │
│  └──────────────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────────────┘
```

## Stage 1: Repository Orchestration

**Entry Point**: `main()` → `util.GetAllRepos()` → concurrent goroutines calling `CloneAndClean()` → `Cleaner()` → `DeepSearchAndClean()`

### Component: GitHub API Client (`GetAllRepos`)

- Makes a single `GET` request to `https://api.github.com/users/{username}/repos?per_page=100`
- JSON-deserializes response into a slice of `model.Repo` structs
- Extracts repository names into a `[]string` slice
- **Failure mode**: `log.Fatal` on HTTP or decode errors — entire process halts

### Component: Concurrent Worker Pool

- Up to 5 goroutines process repositories simultaneously
- A buffered channel (`limit chan struct{}`) acts as a semaphore to cap concurrency
- Each worker sends a token into the channel before starting and releases it via `defer`
- Prometheus gauge `active_workers` tracks current concurrency level
- `sync.WaitGroup` ensures all workers complete before `main()` exits

### Component: Clone Engine (`CloneAndClean`)

- Constructs SSH remote URL: `git@github.com:{username}/{repo}.git`
- Executes `git clone` as an external subprocess
- Resolves absolute path via `filepath.Abs` (no `os.Chdir` state mutation)
- Registers deferred cleanup: `os.RemoveAll(absRepoPath)`
- **Failure mode**: If clone fails, error is logged, metrics increment, and processing continues to the next repo

### Component: Recursive Traverser (`DeepSearchAndClean`)

- Employs depth-first directory traversal
- At each node, uses `Segregator()` to separate files from directories
- If `package.json` is found, delegates to `CleanThis` and terminates that branch
- If not found, recurses into all child directories

## Stage 2: Static Analysis & Cleanup

**Entry Point**: `CleanThis()` → `FindUIDir()` → filesystem walk → regex analysis → deletion → build → git commit

### Component: React Project Detector

- Reads `package.json` from the target directory
- Performs substring matching for `"react"` and `"react-dom"` in the raw file content
- **Failure mode**: Returns early if either dependency is missing

### Component: UI Directory Locator (`FindUIDir`)

- Walks the entire directory tree using `filepath.WalkDir`
- Matches directories named `ui` whose parent is named `components`
- Returns the matched path immediately upon first discovery (short-circuit via `filepath.SkipDir`)
- **Failure mode**: Returns `false` if no match is found — cleanup is skipped

### Component: Import Regex Engine

- Walks all files with extensions `.ts`, `.tsx`, `.js`, `.jsx`
- Applies regex: `[./@"]components/ui/([A-Za-z0-9_-]+)`
- Captures component names and normalizes to lowercase
- Populates a `map[string]bool` of used components
- **Complexity**: O(n × m) where n = source files, m = matches per file

### Component: Deletion Executor

- Reads directory entries from the discovered `components/ui` path
- For each entry, strips the file extension and lowercases the stem
- If the stem is absent from the usage map, deletes the entry via `os.RemoveAll`
- **Note**: Supports both files and directories (handles component subdirectories)

## Stage 3: Build Verification & Commit

### Component: Build Runner

- Executes `npm install --legacy-peer-deps && npm run build` via shell
- Sets working directory to the project root via `build.Dir`
- Uses `os.Stdout` and `os.Stderr` passthrough for transparent logging
- Build duration is measured and recorded in Prometheus histogram
- **Failure mode**: Build errors are logged; `buildErr` is checked and increments `BuildFailuresTotal` counter, but does not halt the pipeline

### Component: Git Client

- Executes `git cm 'auto: cleanup ui and build'` via shell
- Assumes `cm` is a configured Git alias for `commit -am`
- Git duration is measured and logged
- **Failure mode**: Git errors are logged and increment `GitCommitFailuresTotal` counter

### Component: Cleanup Executor

- Deferred call in `CloneAndClean()` performs `os.RemoveAll(absRepoPath)` — recursively deletes cloned repository

## Dependency Graph

```
main.go
  ├── log/slog                ← Structured JSON logging
  ├── net/http                ← API client + metrics HTTP server
  ├── os/exec                 ← Git & npm subprocess execution
  ├── path/filepath           ← Filesystem path manipulation
  ├── regexp                  ← Import statement pattern matching
  ├── strings                 ← package.json dependency detection
  ├── sync                    ← WaitGroup + channel semaphore
  └── time                    ← Duration measurement

model/
  ├── metric.model.go         ← Prometheus metrics struct
  └── repo.model.go           ← GitHub API response model

util/
  ├── repo.util.go            ← GitHub API repository fetcher
  ├── segregator.util.go      ← File/directory splitter
  ├── contains.util.go        ← Slice containment check
  ├── find-ui-directory.util.go ← DFS components/ui locator
  ├── logger.util.go          ← Structured logging helpers
  └── metrics.util.go         ← Prometheus metric registrations
```

## Key Design Decisions

1. **Subprocess execution over libraries** — Git and npm operations are delegated to shell commands rather than Go libraries, avoiding dependency management overhead for version-sensitive tools.

2. **Regex over AST** — The import analysis uses regular expressions rather than TypeScript/JavaScript AST parsing to maintain minimal dependencies and compile-time simplicity.

3. **Concurrent processing with bounded parallelism** — Up to 5 repositories are processed concurrently using a channel-based semaphore (`chan struct{}`), providing controlled parallelism without unbounded resource consumption.

4. **Deferred cleanup** — Repository clone cleanup is guaranteed via Go's `defer` mechanism, ensuring no disk space leakage even on panics.

5. **Absolute path isolation** — Unlike `os.Chdir` which mutates global process state, the tool uses absolute paths (`filepath.Abs`) for all filesystem operations, allowing safe concurrent repository processing.

6. **Prometheus instrumentation** — All operations are instrumented with counters, gauges, and histograms for real-time observability of the cleanup pipeline.

7. **Structured JSON logging** — All log events use `slog` with consistent attribute keys (`repo`, `operation`, `duration`, `error`, etc.) for machine-parsable output.