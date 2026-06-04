# Build Validation

## Overview

Build validation is the **regression detection mechanism** within the cleanup pipeline. After dead components are eliminated, the build step confirms that the remaining codebase compiles and bundles successfully. Build failures indicate that the static analysis may have misidentified a used component as dead.

## Implementation

```go
build := exec.Command("sh", "-c", "npm install --legacy-peer-deps && npm run build")
build.Dir = folder
build.Stdout = os.Stdout
build.Stderr = os.Stderr
build.Run()
```

The build validation executes two chained commands via a shell:

1. `npm install --legacy-peer-deps` — Dependency installation with legacy peer dependency resolution
2. `npm run build` — Execution of the project's build script

## Command Breakdown

### `npm install --legacy-peer-deps`

| Flag | Purpose |
|---|---|
| `--legacy-peer-deps` | Bypasses strict peer dependency validation in npm v7+ |

This flag is necessary because React projects often have peer dependency conflicts that would otherwise block installation. The legacy mode uses npm v6-style peer dependency resolution (auto-installing without strict validation).

**Risks**:
- May install incompatible dependency versions
- Skips dependency conflict warnings
- Can produce non-deterministic `node_modules` trees

### `npm run build`

Executes the `build` script defined in `package.json`:

```json
{
  "scripts": {
    "build": "react-scripts build"
  }
}
```

Common build tools include:

| Tool | Build Script | Output |
|---|---|---|
| Create React App | `react-scripts build` | `build/` |
| Next.js | `next build` | `.next/` |
| Vite | `vite build` | `dist/` |
| Remix | `remix build` | `build/` |
| Gatsby | `gatsby build` | `public/` |

The build step is framework-agnostic — it simply invokes whatever is configured as the project's build script.

## Validation Semantics

### Success Path

When the build succeeds (exit code 0), the cleanup is implicitly validated:

```
> react-scripts build
Creating an optimized production build...
Successfully compiled.

Files:
  build/static/js/main.abc123.js
  build/static/css/main.def456.css
```

A successful build confirms:
- No deleted component was imported by remaining code
- All import paths in the source graph are resolvable
- The bundle can be produced without errors

### Failure Path

When the build fails (non-zero exit code), the error is visible in console but does not halt processing:

```
> react-scripts build
Failed to compile.

./src/App.tsx
Module not found: Can't resolve './components/ui/DeletedButton'
```

**Current behavior**: The build error is logged but ignored. The Git commit proceeds regardless, creating a commit that introduces a build failure.

## Architectural Weaknesses

### Non-Blocking Failure Mode

The most significant architectural issue is that `build.Run()` return value is discarded:

```go
build.Run()  // error not captured
```

This means:
- Build failures do not roll back deletions
- Build failures do not halt the pipeline
- Build failures do not trigger any alerting
- The Git commit includes broken code

### No Pre-Build Validation

The pipeline does not:
- Run a build before deletion to establish a baseline
- Compare pre and post-cleanup build outputs
- Validate that the build command exists before execution
- Check for required configuration files (e.g., `tsconfig.json`, `.babelrc`)

### No Dependency Caching

Each repository runs `npm install` from scratch. There is no:
- Global npm cache utilization (beyond npm's built-in cache)
- Shared `node_modules` between repositories
- Package lockfile validation

## Recommended Improvements

### Pre-Build Baseline

```go
// Establish baseline
baselineBuild := exec.Command("sh", "-c", "npm run build")
baselineBuild.Dir = folder
if baselineBuild.Run() != nil {
    fmt.Println("Pre-cleanup build failed, skipping")
    return
}

// Perform cleanup
// ...

// Post-cleanup validation
postBuild := exec.Command("sh", "-c", "npm run build")
if postBuild.Run() != nil {
    fmt.Println("Post-cleanup build failed, reverting")
    // Rollback logic
}
```

### Rollback Strategy

```go
type CleanupTransaction struct {
    DeletedFiles []string
    RepoRoot     string
}

func (t *CleanupTransaction) Rollback() {
    // Restore deleted files from git
    exec.Command("sh", "-c", "git checkout -- .")
}
```

### Failure Classification

| Build Error Type | Cause | Recommended Action |
|---|---|---|
| Module not found | False positive in static analysis | Rollback, log component name |
| TypeScript error | Type-only import deleted | Rollback, add type-aware analysis |
| Syntax error | Pre-existing issue | Log as pre-existing, skip |
| Missing build script | Not a buildable project | Log as configuration issue, skip |

## Validation Matrix

| Scenario | Build Result | Pipeline Action | Risk |
|---|---|---|---|
| Component unused → deleted | Success | Commit cleanup | Low |
| Component used → missed by regex → deleted | Failure | Commit broken code | High |
| Component used → correctly kept | Success | No change | None |
| Pre-existing build failure | Failure | Commit with deletions (build still broken) | Medium |
| npm install fails | Failure | Commit with deletions (no build) | High |

## Current Status

Build validation is implemented but **non-functional as a safety mechanism** due to the discarded error return. The build output provides observability but does not influence pipeline behavior.