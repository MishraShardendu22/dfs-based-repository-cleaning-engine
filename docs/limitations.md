# Limitations

## Current Technical Limitations

### 1. Regex-Based Static Analysis

The import analysis uses regular expressions rather than a full Abstract Syntax Tree (AST) parser. This introduces several categories of false negatives:

**Unresolvable import patterns:**

```typescript
// Dynamic imports with computed paths
const Comp = dynamic(() => import(`./components/ui/${componentName}`))

// Re-exports through barrel files
export { Button } from './components/ui/Button'

// Re-exports with alias
export { default as Button } from './ui/Button'

// Dynamic require
const Button = require('./components/ui/Button').default

// Template literal paths
import(`@/components/ui/${name}`).then(m => m.default)

// Indirect imports through wrapper modules
// file: src/shared/ui.ts
export { Button } from '@/components/ui/Button'
// file: src/App.tsx
import { Button } from '@/shared/ui'  // Button usage is not detected
```

**Impact**: Components that are only imported through indirect paths or dynamic references will be incorrectly identified as unused and deleted.

### 2. Case-Insensitive Comparison

```go
used[strings.ToLower(m[1])] = true
// ...
base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
```

Both the import reference and filesystem entry are lowercased for comparison. This means:

- `Button.tsx` imported as `button` → correctly matched
- `BUTTON.tsx` imported as `Button` → correctly matched
- `button.tsx` imported as `Button` → correctly matched

However, on case-sensitive filesystems (Linux), deleting `button.tsx` when the import references `Button` would cause the build to fail. The tool would still delete because it matched on lowercase.

### 3. Single UI Directory Assumption

`findUIDir()` returns the first `components/ui` match and short-circuits:

```go
if filepath.Base(filepath.Dir(path)) == "components" {
    uiDir = path
    found = true
    return filepath.SkipDir
}
```

**Issues**:
- Projects with multiple UI directories are only partially analyzed
- Nested projects within monorepos may have their own `components/ui` that is missed
- Some shadcn/ui installations use `@/components/ui` without a physical `components/ui` path

### 4. No Dry-Run Mode

The tool has no preview capability. Every execution performs destructive operations:

- Files are permanently deleted (not moved to trash)
- Git commits are created without confirmation
- No summary of planned deletions is provided before execution

### 5. Hardcoded Configuration

| Constraint | Impact |
|---|---|
| Single GitHub username | Cannot process multiple accounts or organizations |
| SSH-only cloning | No HTTPS fallback for environments without SSH |
| Static commit message | No contextual information in commit history |
| `--legacy-peer-deps` | May not work with all npm projects |
| No repository filtering | Processes all repositories, including archived or empty ones |
| No branch management | Commits directly to current branch (typically main/master) |

### 6. Build Validation Is Non-Blocking

```go
build.Run()  // return value discarded
```

Build failures do not:
- Halt processing for the current repository
- Revert deletions
- Prevent the git commit
- Log structured error information

This makes build validation an **observability feature** rather than a **safety mechanism**.

### 7. Git Alias Dependency

The commit command assumes a user-configured Git alias:

```go
exec.Command("sh", "-c", "git cm 'auto: cleanup ui and build'")
```

If the `cm` alias is not configured, the command fails silently and no commit is created. The standard command would be `git commit -am`.

### 8. No Pagination Support

The GitHub API query uses `per_page=100` but does not implement page iteration:

```go
url := "https://api.github.com/users/" + username + "/repos?per_page=100"
```

For accounts with more than 100 repositories, only the first page of results is processed. Remaining repositories are never discovered.

### 9. Unauthenticated API Requests

```go
resp, err := http.Get(url)
```

No authentication token is sent. This means:
- 60 requests/hour rate limit (vs 5,000 with authentication)
- Private repositories are not listed
- The Language.go module handles authentication differently (via curl with token)

### 10. Silent Error Handling

Multiple operations discard errors silently:

| Operation | Error Handling | Consequence |
|---|---|---|
| `os.ReadDir` in `Folder()` | Error ignored | Traversal continues with empty lists |
| `filepath.WalkDir` callback | Error returned nil | Walk continues past problematic files |
| `os.ReadFile` in import scan | Return nil | File is skipped without warning |
| `os.ReadFile("package.json")` | Return | Directory is silently skipped |
| `cmd.Run()` for clone | Return value not checked | Clone failures are invisible |
| `cmd.Run()` for build | Return value not checked | Build failures are invisible |
| `cmd.Run()` for git commit | Return value not checked | Commit failures are invisible |
| `os.ReadDir(uiDir)` | Print error + return | Cleanup stops without alerting |

## Environmental Limitations

### Operating System Compatibility

- **Linux**: Full support
- **macOS**: Partial support (filesystem behavior differs for case sensitivity)
- **Windows**: Untested (path separator differences likely cause issues)

### Network Dependencies

| Dependency | Purpose | Failure Mode |
|---|---|---|
| GitHub API | Repository discovery | No repos discovered |
| SSH (port 22) | Repository cloning | Clone failures |
| npm registry | Dependency installation | npm install failures |

### Resource Constraints

| Resource | Minimum | Practical |
|---|---|---|
| Disk space | ~100MB per repository | 10GB+ recommended for batch processing |
| Memory | ~50MB | 256MB+ recommended |
| Network bandwidth | 1Mbps | 10Mbps+ for reasonable clone times |
| API rate limit | 60 req/hr (unauthenticated) | 5000 req/hr (authenticated) |

## Design Limitations

### Single-Threaded Execution

Repositories are processed sequentially. No goroutine-based parallelism is implemented. This was a deliberate design choice to:

- Simplify error handling
- Avoid resource contention during git operations
- Ensure deterministic output ordering

However, it means total execution time scales linearly with repository count.

### No State Persistence

The tool maintains no state between runs:
- No tracking of previously cleaned repositories
- No cache of analysis results
- No logging of historical operations (beyond console output)
- No checkpointing for resumption after interruption

### No Rollback Mechanism

Once files are deleted and committed, there is no automated undo:
- Relies on Git's reflog for recovery
- Requires manual `git revert` or `git checkout` operations
- No safety net for incorrect deletions

## Intended Scope

The tool is designed for a specific use case: **cleaning unused shadcn/ui components from React projects**. It is not a general-purpose dead-code elimination tool and should not be expected to work with:

- Vue.js, Angular, Svelte, or other frameworks
- Non-UI component directories
- CSS-only or utility-only projects
- Projects using path aliases that don't match the expected pattern
- Backend or non-browser JavaScript projects
- TypeScript projects with complex paths configuration