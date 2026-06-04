# Architecture

## System Overview

GitHub-Cleaner-Go implements a three-stage pipeline architecture: **Repository Orchestration**, **Static Analysis Engine**, and **Build Verification & Commit Pipeline**. Each stage is a discrete processing phase with well-defined inputs, outputs, and failure boundaries.

```
┌────────────────────────────────────────────────────────────────────┐
│                      GitHub-Cleaner-Go Engine                      │
│                                                                    │
│  ┌─────────────────┐   ┌──────────────────┐   ┌────────────────┐  │
│  │  Stage 1         │   │  Stage 2         │   │  Stage 3        │  │
│  │  Repository      │──>│  Static Analysis │──>│  Build & Commit │  │
│  │  Orchestration   │   │  & Cleanup       │   │  Pipeline       │  │
│  └─────────────────┘   └──────────────────┘   └────────────────┘  │
│         │                       │                       │          │
│         ▼                       ▼                       ▼          │
│  ┌─────────────┐       ┌──────────────┐       ┌──────────────┐    │
│  │ GitHub API   │       │ Import       │       │ npm install  │    │
│  │ HTTP Client  │       │ Regex Engine │       │ Build Runner │    │
│  ├─────────────┤       ├──────────────┤       ├──────────────┤    │
│  │ SSH Git     │       │ Filesystem   │       │ Git Client   │    │
│  │ Client      │       │ Walker       │       │ (Commit)     │    │
│  ├─────────────┤       ├──────────────┤       ├──────────────┤    │
│  │ Recursive   │       │ Deletion     │       │ Local Cleanup│    │
│  │ Traverser   │       │ Executor     │       │ (Remove Clone)│    │
│  └─────────────┘       └──────────────┘       └──────────────┘    │
└────────────────────────────────────────────────────────────────────┘
```

## Stage 1: Repository Orchestration

**Entry Point**: `main()` → `getRepos()` → `Clone()` → `Cleaner()` → `DeepSearchAndClean()`

### Component: GitHub API Client (`getRepos`)

- Makes a single `GET` request to `https://api.github.com/users/{username}/repos?per_page=100`
- JSON-deserializes response into a slice of `Repo` structs
- Extracts repository names into a `[]string` slice
- **Failure mode**: `log.Fatal` on HTTP or decode errors — entire process halts

### Component: Clone Engine (`Clone`)

- Constructs SSH remote URL: `git@github.com:{username}/{repo}.git`
- Executes `git clone` as an external subprocess
- Changes working directory to cloned repository root via `os.Chdir`
- Registers deferred cleanup: `os.Chdir("..")` + `os.RemoveAll(repo)`
- **Failure mode**: If `os.Chdir` fails, the repository is force-deleted and processing continues to the next repo

### Component: Recursive Traverser (`DeepSearchAndClean`)

- Employs depth-first directory traversal
- At each node, checks for `package.json` presence
- If found, delegates to `CleanThis` and terminates that branch
- If not found, recurses into all child directories

## Stage 2: Static Analysis & Cleanup

**Entry Point**: `CleanThis()` → `findUIDir()` → filesystem walk → regex analysis → deletion

### Component: React Project Detector

- Reads `package.json` from the target directory
- Performs substring matching for `"react"` and `"react-dom"` in the raw file content
- **Failure mode**: Returns early if either dependency is missing

### Component: UI Directory Locator (`findUIDir`)

- Walks the entire directory tree using `filepath.WalkDir`
- Matches directories named `ui` whose parent is named `components`
- Returns the matched path immediately upon first discovery (short-circuit)
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
- Sets working directory to the project root
- Uses `os.Stdout` and `os.Stderr` passthrough for transparent logging
- **Failure mode**: Build errors are suppressed (return value not checked)

### Component: Git Client

- Executes `git cm 'auto: cleanup ui and build'` via shell
- Assumes `cm` is a configured Git alias for `commit -am`
- **Failure mode**: Git errors are suppressed (return value not checked)

### Component: Cleanup Executor

- Deferred call in `Clone()` performs:
  1. `os.Chdir("..")` — returns to parent directory
  2. `os.RemoveAll(repo)` — recursively deletes cloned repository

## Module Architecture: Language.go

The `Language` module operates independently from the cleanup engine:

- Fetches non-fork repositories via GitHub API with authentication token
- Queries `/repos/{owner}/{repo}/languages` per repository
- Aggregates byte counts into a cumulative map
- Computes and prints percentage distributions

## Dependency Graph

```
main.go
  ├── encoding/json       ← GitHub API response parsing
  ├── net/http            ← API client
  ├── os/exec             ← Git & npm subprocess execution
  ├── path/filepath       ← Filesystem path manipulation
  ├── regexp              ← Import statement pattern matching
  └── strings             ← package.json dependency detection

Language.go
  ├── encoding/json       ← API response parsing
  ├── os/exec             ← curl subprocess execution
  └── (stdlib)            ← No external dependencies
```

## Key Design Decisions

1. **Subprocess execution over libraries** — Git and npm operations are delegated to shell commands rather than Go libraries, avoiding dependency management overhead for version-sensitive tools.

2. **Regex over AST** — The import analysis uses regular expressions rather than TypeScript/JavaScript AST parsing to maintain zero external dependencies and compile-time simplicity.

3. **Synchronous sequential processing** — Repositories are processed one at a time to avoid resource contention and simplify error handling. No goroutine-based parallelism is employed.

4. **Deferred cleanup** — Repository clone cleanup is guaranteed via Go's `defer` mechanism, ensuring no disk space leakage even on panics.

5. **Silent error handling** — Build and Git errors are not propagated upstream. The pipeline continues to subsequent repositories when a single operation fails.