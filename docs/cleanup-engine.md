# Cleanup Engine

## Overview

The cleanup engine is the core subsystem responsible for **dead component detection and elimination**. It implements a regex-based static analysis approach to determine which UI components are referenced across the source graph and which can be safely removed.

## Engine Entry Point

```
CleanThis(filesAndFolder string, repo string, repoStart time.Time)
```

**Invariant**: The `filesAndFolder` parameter must point to a directory containing a `package.json` with both `react` and `react-dom` as dependencies.

## Detection Pipeline

### 1. Dependency Verification

```go
content, err := os.ReadFile(filepath.Join(filesAndFolder, "package.json"))
if err != nil {
    return
}
pkg := string(content)
if !strings.Contains(pkg, "react") || !strings.Contains(pkg, "react-dom") {
    return
}
```

The engine performs raw byte-level substring matching on `package.json`. This approach was chosen over `json.Unmarshal` to:
- Avoid importing the `dependencies` vs `devDependencies` distinction
- Eliminate concerns about JSON structure variation across package managers
- Keep the detection logic in a single conditional statement

**False positive risk**: Any string "react" or "react-dom" in the file (including in lockfiles or comments) triggers detection.

### 2. UI Directory Location

```go
func FindUIDir(root string) (string, bool) {
    filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
        if found || err != nil || !d.IsDir() || filepath.Base(path) != "ui" {
            return nil
        }
        if filepath.Base(filepath.Dir(path)) == "components" {
            uiDir = path
            found = true
            return filepath.SkipDir
        }
        return nil
    })
    return uiDir, found
}
```

The directory walker uses `filepath.WalkDir` with a short-circuit mechanism. Once the first matching `components/ui` directory is found, `filepath.SkipDir` terminates the walk. This means:

- Only the first `components/ui` match is used
- Multiple UI directories in the same project are ignored
- Nested `components/ui/components/ui` patterns do not cause issues

### 3. Static Import Analysis

```go
used := map[string]bool{}
exts := map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true}

filepath.WalkDir(filesAndFolder, func(path string, d os.DirEntry, err error) error {
    if err != nil || d.IsDir() { return nil }
    if !exts[filepath.Ext(path)] { return nil }

    data, err := os.ReadFile(path)
    if err != nil { return nil }

    for _, m := range regexp.MustCompile(`[./@"]components/ui/([A-Za-z0-9_-]+)`).FindAllStringSubmatch(string(data), -1) {
        used[strings.ToLower(m[1])] = true
    }
    return nil
})
```

**Analysis characteristics**:

| Property | Value |
|---|---|
| Scanner | `filepath.WalkDir` (recursive) |
| Regex engine | Go `regexp` (RE2, linear-time) |
| Match mode | `FindAllStringSubmatch` (global) |
| Case normalization | `strings.ToLower` on captured group |
| Memory complexity | O(U + S) where U = used components, S = source files |
| Time complexity | O(N × F × L) where N = files, F = matches per file, L = line length |

**Supported import patterns**:

```typescript
// Relative imports
import { Button } from './components/ui/Button'
import { Card } from "./components/ui/Card"

// Path alias imports
import { Modal } from '@/components/ui/Modal'
import { Dialog } from "@/components/ui/Dialog"

// Bare specifier imports (uncommon)
import { Badge } from 'components/ui/Badge'
import { Toast } from "components/ui/Toast"
```

**Unsupported patterns**:

```typescript
// Dynamic imports with template literals
const Component = dynamic(() => import(`./components/ui/${name}`))

// Computed paths
const path = './components/ui/Button'
const Component = await import(path)

// Re-exports
export { Button } from './components/ui/Button'

// Barrel file imports (index.ts re-exporting from submodules)
export * from './Button'
```

### 4. Dead Component Elimination

```go
entries, err := os.ReadDir(uiDir)
if err != nil {
    logger.Error("failed_read_ui_dir",
        slog.String("repo", repo),
        slog.String("error", err.Error()),
    )
    util.MetricsRegistry.CleanupFailuresTotal.Inc()
    return
}

deletedCount := 0
for _, entry := range entries {
    name := entry.Name()
    base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
    if !used[base] {
        path := filepath.Join(uiDir, name)
        os.RemoveAll(path)
        deletedCount++
    }
}

util.MetricsRegistry.FilesDeletedTotal.Add(float64(deletedCount))
```

**Deletion logic**:

1. Read all entries in the `components/ui` directory
2. On error: log the failure, increment `CleanupFailuresTotal` metric, return early
3. For each entry, strip the file extension to get the "stem"
4. Lowercase the stem for case-insensitive comparison
5. Check if the stem exists in the usage map
6. If absent, execute `os.RemoveAll` (handles both files and directories)
7. Track `deletedCount` and emit to Prometheus metric

**Edge cases handled**:

- Directory-based components: `Button/index.tsx` → stem "Button" → compared against used set
- Multiple file extensions: `Button.tsx`, `Button.module.css` → both produce stem "Button" → both deleted
- Mixed case: `Button.tsx` imported as `button` → lowercasing normalizes

**Edge cases NOT handled**:

- Components imported under aliases: `import { Btn as Button } from './ui/Btn'` → alias is the matched name
- Components with same stem different extensions: `Button.tsx` used, `Button.stories.tsx` deleted (if not imported elsewhere)
- Case mismatch in filesystem: Linux is case-sensitive, but Windows/macOS may not be

### 5. Build Validation

```go
util.LogBuildStart(logger, repo)
buildStart := time.Now()
build := exec.Command("sh", "-c", "npm install --legacy-peer-deps && npm run build")
build.Dir = filesAndFolder
build.Stdout = os.Stdout
build.Stderr = os.Stderr
buildErr := build.Run()
buildDur := time.Since(buildStart)

buildStatus := "success"
if buildErr != nil {
    buildStatus = "failed"
    util.MetricsRegistry.BuildFailuresTotal.Inc()
}
util.LogBuildEnd(logger, repo, buildDur, buildStatus, buildErr)
util.MetricsRegistry.BuildDurationSeconds.Observe(buildDur.Seconds())
```

The build step serves as a **regression detection mechanism**. If a used component was incorrectly identified as unused:

1. Build fails due to missing import
2. Error output is visible in console
3. Build failure is logged with structured attributes (`build_status`, `duration`, `error`)
4. `BuildFailuresTotal` metric is incremented
5. The deletion is NOT reverted
6. Processing continues to the next operation

**Note on error handling**: Unlike the earlier implementation that silently discarded the build error, the current code captures `buildErr` and uses it for logging and metrics. However, the build result still does not block the pipeline.

### 6. Git Commit

```go
util.LogGitCommitStart(logger, repo)
gitStart := time.Now()
git := exec.Command("sh", "-c", "git cm 'auto: cleanup ui and build'")
git.Dir = filesAndFolder
git.Stdout = os.Stdout
git.Stderr = os.Stderr
gitErr := git.Run()
gitDur := time.Since(gitStart)

if gitErr != nil {
    util.LogGitCommitEnd(logger, repo, gitDur, gitErr)
    util.MetricsRegistry.GitCommitFailuresTotal.Inc()
} else {
    util.LogGitCommitEnd(logger, repo, gitDur, nil)
}
```

**Commit behavior**:

- Assumes `git cm` is a user-configured alias for `git commit -am`
- The `-a` flag stages all tracked files that have been modified or deleted
- Commit message is static: `auto: cleanup ui and build`
- Git duration is measured and logged
- On failure: `GitCommitFailuresTotal` metric is incremented
- No push is performed
- No tagged release is created
- No branch management is performed

### 7. Final Logging & Metrics

```go
totalDur := time.Since(repoStart)
util.MetricsRegistry.RepoProcessingDuration.Observe(totalDur.Seconds())
util.MetricsRegistry.ReposProcessedTotal.Inc()
util.LogRepoComplete(logger, repo, totalDur, deletedCount, buildStatus)
```

Each repository's complete processing cycle is recorded:
- Total processing duration (repo start to finish) emitted as Prometheus histogram
- `ReposProcessedTotal` counter incremented
- Structured log with repo name, total duration, files deleted count, and build status

## Engine Safety Characteristics

| Aspect | Assessment |
|---|---|
| Deletion safety | No backup, no trash, no undo |
| Build verification | Present and measured, but non-blocking |
| Git safety | Local commit only, no push |
| Error recovery | Non-existent (failures are logged and metered) |
| Dry-run support | Not implemented |
| Rollback support | Not implemented |

## Engine Configuration Surface

The engine has **zero configuration parameters**. All behavior is hardcoded:

- GitHub username: hardcoded in `GetAllRepos()` URL
- Repository limit: `?per_page=100`
- Clone URL scheme: SSH only
- Concurrency limit: 5 goroutines
- Import regex pattern: hardcoded
- File extensions: `{".ts", ".tsx", ".js", ".jsx"}`
- npm flags: `--legacy-peer-deps`
- Commit message: `auto: cleanup ui and build`
- Git alias dependency: `cm`

This lack of configurability is a design constraint of the current implementation. Future versions should externalize these parameters. See [roadmap.md](roadmap.md).