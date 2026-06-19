# DFS based repository cleaninge engine

Autonomous Repository Maintenance and Dead Component Elimination Engine.

GitHub-Cleaner-Go is a production-grade automation tool that systematically traverses GitHub repositories, performs static import analysis on React codebases, identifies and removes unused UI components, validates builds post-cleanup, and commits the results without human intervention.

Built for developers managing large React ecosystems where component bloat accumulates across repositories. The tool functions as an autonomous maintenance agent, reducing technical debt through programmatic dead-code elimination.

## Latest Check
- Runtime: 16m 1.22s
- Concurrency: 10 workers
- Repositories scanned: 148

---

## Architecture

```
+------------------------------------------------------------------------------------------+
|                              GitHub-Cleaner-Go Engine                                     |
+------------------+---------------------------+-------------------------------------------+
|  Repository      |  Static Analysis           |  Build and Commit                          |
|  Orchestration   |  Pipeline                  |  Pipeline                                 |
+------------------+---------------------------+-------------------------------------------+
|  - GitHub API    |  - Regex Import Scan       |  - npm install                             |
|  - SSH Clone     |  - Source Graph Build      |  - Production Build                        |
|  - DFS Traversal |  - Dead-Component ID       |  - Git Commit                              |
|  - Concurrent    |  - File Deletion           |  - Local Cleanup                           |
|    (5 workers)   |                           |                                           |
+------------------+---------------------------+-------------------------------------------+
|                        Observability Layer                                              |
|              Prometheus Metrics (:2112) + Structured JSON Logging                        |
+------------------------------------------------------------------------------------------+
```

The system operates as a three-stage pipeline: **repository orchestration**, **static analysis and dead-component elimination**, and **build verification and commit**. Repositories are processed concurrently (up to 5 at a time) via a goroutine pool with channel-based rate limiting.

---

## Core Features

- **Autonomous Repository Discovery** - Fetches all repositories from a GitHub account via the REST API (up to 100 repos per request).
- **Concurrent Processing** - Processes up to 5 repositories simultaneously using goroutines with channel-based semaphore limiting.
- **Recursive Filesystem Traversal** - Walks directory trees (DFS) to locate React projects with `components/ui` directory structures.
- **Regex-Based Static Import Analysis** - Scans `.ts`, `.tsx`, `.js`, `.jsx` files for import statements referencing `components/ui/*` components.
- **Dead Component Elimination** - Compares used components against filesystem entries; removes orphaned components.
- **Build Validation** - Executes `npm install --legacy-peer-deps && npm run build` to verify post-cleanup integrity.
- **Prometheus Metrics** - Exposes real-time metrics including active workers, repos processed, clone/build durations, files deleted, and failure counters.
- **Structured JSON Logging** - All operations logged with `slog` in JSON format with consistent attribute structure.
- **Git Automation** - Automatically commits cleanup changes with standardized commit messages.
- **Ephemeral Repository Lifecycle** - Clones, processes, and destroys local repository copies leaving no residual artifacts.

---

## Technical Workflow

### Stage 1: Repository Orchestration

1. **GitHub API Discovery** - Issues `GET /users/{username}/repos?per_page=100` to enumerate all repositories (unauthenticated, 60 req/hr limit).
2. **Concurrent Clone Pool** - Up to 5 repositories cloned simultaneously via goroutine pool with channel-based capacity control.
3. **SSH Clone** - Clones each repository via `git@github.com:{username}/{repo}.git`.
4. **Absolute Path Resolution** - Uses `filepath.Abs` to resolve the cloned repository root (no `os.Chdir` state mutation).
5. **Recursive Scanner Invocation** - Initiates `DeepSearchAndClean()` on the repository root.

### Stage 2: Static Analysis and Cleanup Pipeline

1. **Filesystem Enumeration** - Lists files and directories at current level via `Segregator()` (separates files from directories).
2. **React Project Detection** - Checks for `package.json` containing both `react` and `react-dom` dependencies.
3. **UI Directory Discovery** - DFS walk via `FindUIDir()` to locate `components/ui` directories.
4. **Source Graph Analysis** - Walks every `.ts/.tsx/.js/.jsx` file, extracting import paths matching the pattern:
   - `[./@"]components/ui/([A-Za-z0-9_-]+)`
5. **Usage Mapping** - Builds a lowercase-normalized set of used component names.
6. **Dead Component Elimination** - Iterates over `components/ui` entries, deleting any file whose base name (without extension) has zero import references.
7. **Build Verification** - Runs the project build to confirm no regressions were introduced; build exit code is captured and logged.

### Stage 3: Commit and Cleanup

1. **Git Commit** - Stages and commits all changes with message: `auto: cleanup ui and build` (requires `git cm` alias for `commit -am`).
2. **Repository Destruction** - Recursively removes the cloned repository via deferred `os.RemoveAll` on absolute path.

---

## Example Execution Flow

```
flowchart TD
    A[GitHub API: Fetch Repos] --> B[Concurrent Pool: Up to 5 Workers]
    B --> C[Clone Repo via SSH]
    C --> D[Resolve Absolute Path]
    D --> E{Has package.json?}
    E -->|Yes| F{Is React Project?}
    E -->|No| G[Recurse into Subdirectories]
    F -->|Yes| H[Find components/ui]
    F -->|No| G
    H --> I{Found UI Dir?}
    I -->|Yes| J[Scan All Source Files for Imports]
    I -->|No| G
    J --> K[Build Used-Component Set]
    K --> L[Delete Unused Components]
    L --> M[npm install + npm run build]
    M --> N[Git Commit]
    N --> O[Remove Local Clone]
    G --> P[Traverse Next Directory]
    P --> E
    O --> Q{More Repos?}
    Q -->|Yes| C
    Q -->|No| R[Done]
```

---

## Repository Traversal

The traversal is implemented as a **depth-first recursive directory walk** (`DeepSearchAndClean`). At each node:

1. The directory is enumerated for files and subdirectories using `Segregator()`.
2. If a `package.json` is present, the node is treated as a potential project root and passed to `CleanThis`.
3. If no `package.json` exists, the function recurses into each subdirectory.

This design allows the system to handle monorepos, nested projects, and repositories with complex directory structures. The traversal terminates at leaf directories with no further subdirectories or upon discovering a valid React project.

---

## Static Import Analysis

The static analysis subsystem uses **regex-based import scanning** rather than full AST parsing. The regular expression:

```
[./@"]components/ui/([A-Za-z0-9_-]+)
```

Matches import statements in the following common patterns:

| Import Pattern     | Example                              |
|--------------------|--------------------------------------|
| Relative import    | `./components/ui/Button`             |
| Absolute import    | `@/components/ui/Card`               |
| String import      | `"components/ui/Modal"`              |
| Named import       | `components/ui/Button`               |

The analysis builds a **usage map** (boolean, presence-based) by lowercasing the captured component name. Files are then compared against this map; any file in `components/ui` whose stem does not appear in the usage set is considered dead and scheduled for deletion.

**Known Limitation**: Dynamic imports using template literals or computed strings are not resolved. The analysis also does not handle re-exports or barrel files.

---

## Build Validation

Post-cleanup, the system executes:

```bash
npm install --legacy-peer-deps && npm run build
```

This serves dual purposes:
1. **Integrity Check** - Verifies that no deleted component was actually required at build time (catches false positives).
2. **Dependency Resolution** - Ensures the project is in a buildable state after modifications.

Build results are logged with duration and status. Build failures are tracked via Prometheus metrics but do not halt the pipeline.

---

## Observability

### Prometheus Metrics

Metrics are exposed via an HTTP server on **`:2112/metrics`**:

| Metric                            | Type      | Description                       |
|-----------------------------------|-----------|-----------------------------------|
| `repos_processed_total`           | Counter   | Total repos processed             |
| `react_repos_total`               | Counter   | React repos found                 |
| `files_deleted_total`             | Counter   | Total files deleted               |
| `clone_failures_total`            | Counter   | Clone failures                    |
| `build_failures_total`            | Counter   | Build failures                    |
| `cleanup_failures_total`          | Counter   | Cleanup failures                  |
| `git_commit_failures_total`       | Counter   | Git commit failures               |
| `active_workers`                  | Gauge     | Current active goroutines         |
| `repo_processing_duration_seconds`| Histogram | Per-repo processing time          |
| `clone_duration_seconds`          | Histogram | Clone operation duration          |
| `build_duration_seconds`          | Histogram | Build operation duration          |

### Grafana Dashboard

A full monitoring stack is available via Docker Compose:

```bash
make start      # Start Prometheus + Grafana + App
make grafana    # Open Grafana at http://localhost:3000
make prometheus # Open Prometheus at http://localhost:9090
make metrics    # Curl raw metrics endpoint
```

---

## Installation

### Prerequisites

| Dependency     | Version | Purpose                                              |
|----------------|---------|------------------------------------------------------|
| Go             | 1.24.4+ | Compilation and runtime                              |
| Git            | 2.x+    | Repository cloning and automation                    |
| SSH Agent      | Any     | Authentication for repository cloning                |
| npm / Node.js  | Any     | React project build validation                       |
| Docker (opt.)  | Any     | Prometheus/Grafana monitoring stack                  |

### Build and Run

```bash
# Clone the repository
git clone git@github.com:MishraShardendu22/GitHub-Cleaner-Go.git
cd GitHub-Cleaner-Go

# Build the binary
go build -o github-cleaner .

# Run
./github-cleaner
```

### Quick Start with Make

```bash
make run           # Run cleanup engine
make metrics       # View Prometheus metrics
make start         # Start monitoring stack + app
make clean         # Stop services and remove _Repos
```

---

## Usage

```bash
go run main.go
```

Fetches repositories from the configured GitHub account, clones each one, performs dead component analysis, deletes unused files, validates builds, commits changes, and cleans up.

---

## Project Structure

```
GitHub-Cleaner-Go/
 main.go                    # Core cleanup engine and repository orchestration
 go.mod                     # Go module definition
 Makefile                   # Build, run, and monitoring targets
 docker-compose.yml         # Prometheus + Grafana stack
 README.md                  # This file
 CONTRIBUTING.md            # Contributor guidelines
 SECURITY.md                # Security considerations
 .gitignore                 # Git exclusion rules
 model/
  metric.model.go           # Prometheus metrics struct definition
  repo.model.go             # GitHub API repo response model
 util/
  contains.util.go          # Slice containment check
  find-ui-directory.util.go # DFS components/ui locator
  logger.util.go            # Structured JSON logging helpers
  metrics.util.go           # Prometheus metric registrations
  repo.util.go              # GitHub API repository fetcher
  segregator.util.go        # File/directory splitter
 prometheus/
  prometheus.yml            # Prometheus scrape configuration
 docs/
  architecture.md           # System architecture documentation
  how-it-works.md           # Detailed operational explanation
  cleanup-engine.md         # Cleanup pipeline specification
  repository-scanner.md     # Scanner implementation details
  build-validation.md       # Build verification methodology
  security.md               # Security model and risks
  limitations.md            # Known limitations
  roadmap.md                # Future development plans
```

---

## Technical Limitations

- **Regex-based analysis** - Cannot resolve dynamic imports, computed paths, or re-exports. May produce false negatives for obfuscated import patterns.
- **React-only scope** - Currently limited to React projects with `components/ui` structures. No support for Vue, Angular, or other frameworks.
- **SSH-only authentication** - Requires configured SSH keys for repository cloning. No HTTPS fallback.
- **Single-user mode** - Hardcoded to a single GitHub username. No multi-account or organization support.
- **No dry-run mode** - Operations are destructive by design. No preview capability for what would be deleted.
- **Git alias dependency** - Requires `git cm` alias for `git commit -am`.
- **No API pagination** - Only fetches the first 100 repos from the GitHub API.
- **No directory exclusion** - Traverses `.git`, `node_modules`, and hidden directories.

---

## Safety Warnings

> **Destructive Operations**
> This tool deletes files and makes Git commits automatically. It is strongly recommended to:
> - Test on a fork or backup repository first.
> - Review the codebase to understand deletion criteria.
> - Ensure all important work is committed and pushed before running.
> - Use a feature branch if possible (modify the commit step to push to a branch).

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed contributor guidelines, including setup instructions, code style expectations, and pull request process.

---

## License

This project is licensed under the **MIT License**. See the LICENSE file for details.

The MIT License was chosen for this project because:
- It permits commercial and private use without restrictions.
- It allows other developers to integrate the cleanup engine into their own tooling.
- It is the most widely understood and accepted license in the open-source ecosystem.
- It imposes no copyleft obligations, which is appropriate for an automation tool that may be embedded into CI/CD pipelines.
- It provides appropriate disclaimer of liability (critical for a tool that performs destructive filesystem operations).

---

## Author

**MishraShardendu22**

- GitHub: [@MishraShardendu22](https://github.com/MishraShardendu22)
- Project: [Repository-cleaning-engine](https://github.com/MishraShardendu22/Repository-cleaning-engine)

---

## Related Links

- [Go Programming Language](https://go.dev/)
- [GitHub REST API Documentation](https://docs.github.com/en/rest)
- [React Documentation](https://react.dev/)
- [Prometheus Documentation](https://prometheus.io/docs/)
