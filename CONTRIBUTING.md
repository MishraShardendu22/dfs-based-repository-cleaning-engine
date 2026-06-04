# Contributing to GitHub-Cleaner-Go

## Table of Contents

- [Getting Started](#getting-started)
- [Local Development](#local-development)
- [Code Style](#code-style)
- [Pull Request Process](#pull-request-process)
- [Commit Conventions](#commit-conventions)
- [Testing](#testing)
- [Issue Reporting](#issue-reporting)

---

## Getting Started

### Prerequisites

- Go 1.24.4 or later
- Git 2.x+
- SSH key configured for GitHub access
- npm/Node.js (for build validation testing)

### Fork & Clone

```bash
# Fork the repository on GitHub, then:
git clone git@github.com:YOUR_USERNAME/GitHub-Cleaner-Go.git
cd GitHub-Cleaner-Go
git remote add upstream git@github.com:MishraShardendu22/GitHub-Cleaner-Go.git
```

### Verify Setup

```bash
go build -o /dev/null main.go Language.go
```

The build should complete without errors. If you encounter dependency issues, ensure your Go version is 1.24.4+.

---

## Local Development

### Development Workflow

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes**
   - Write code following the project's conventions
   - Update or add documentation as needed
   - Ensure existing functionality is not broken

3. **Run the tool against a test repository**
   ```bash
   # Modify the username in main.go to point to a test account
   # or create a local test directory structure:
   mkdir -p /tmp/test-repo/src/components/ui
   cd /tmp/test-repo
   npm init -y
   npm install react react-dom
   echo 'export const Button = () => <button />' > src/components/ui/Button.tsx
   echo 'export const Unused = () => <div />' > src/components/ui/Unused.tsx
   echo 'import { Button } from "./components/ui/Button"' > src/App.tsx
   cd -
   go run main.go
   ```

4. **Verify expected behavior**
   - Unused components are deleted
   - Used components are preserved
   - Build succeeds
   - Git commit is created

### Testing with a Safe Environment

```bash
# Create an isolated test directory
mkdir -p /tmp/cleanup-test && cd /tmp/cleanup-test

# Initialize a test Go module
go mod init test-cleanup

# Link or copy the source files
cp /path/to/GitHub-Cleaner-Go/*.go .

# Modify the username to a test GitHub account
# or use a mock repository list
```

---

## Code Style

### General Guidelines

- **Formatting**: Use `gofmt` (or `go fmt`) before committing. The project follows standard Go formatting conventions.
- **Imports**: Group imports into stdlib, external, and internal blocks. Use `goimports` for automatic management.
- **Naming**: Follow Go conventions:
  - `camelCase` for unexported identifiers
  - `PascalCase` for exported identifiers
  - `UPPER_CASE` for constants
- **Error handling**: Prefer returning errors over `log.Fatal`. New code should not introduce additional silent error handling.

### Code Style Checklist

- [ ] Code is formatted with `go fmt`
- [ ] Imports are properly organized
- [ ] Variables have meaningful names (avoid single-letter names except for loop indices)
- [ ] Functions are focused on a single responsibility
- [ ] Magic strings and numbers are replaced with named constants
- [ ] Error return values are checked (existing code that ignores errors should be noted)
- [ ] Comments explain "why" not "what" (code should be self-documenting for the "what")
- [ ] No `log.Fatal` outside of `main()` (library functions should return errors)

### Documentation Style

- Use Go-style comments (`//`) for exported functions, types, and constants
- Include examples in doc comments where appropriate
- Markdown files should follow the existing documentation style (engineering-focused, technical, not tutorial-style)

---

## Pull Request Process

### Before Submitting

1. Ensure your fork is up to date with upstream
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. Run a final verification
   ```bash
   go build ./...
   go vet ./...
   ```

3. Review your changes
   ```bash
   git diff main --stat  # Review changed files
   ```

### PR Requirements

- **Single purpose**: Each PR should address one concern (feature, bug fix, documentation)
- **Descriptive title**: Briefly describe the change
- **Detailed description**: Explain:
  - What the change does
  - Why it's needed
  - How it was tested
  - Any risks or side effects
- **Related issues**: Reference any related GitHub issues with `Closes #123` or `Relates to #123`
- **No unrelated changes**: Avoid formatting changes, whitespace fixes, or refactoring in feature PRs

### PR Template

```markdown
## Description
[Brief description of the change]

## Motivation
[Why this change is necessary or beneficial]

## Testing
[How the change was tested]

## Risks
[Potential risks, regressions, or side effects]

Closes #[issue_number]
```

### Review Process

1. Maintainer reviews within 5 business days
2. Address review feedback with additional commits
3. Squash commits before merging (the maintainer may do this)
4. PRs are merged into `main` via squash merge

---

## Commit Conventions

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types

| Type | Usage |
|---|---|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation changes |
| `refactor` | Code restructuring without functional changes |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `chore` | Build process, tooling, or dependency changes |
| `security` | Security-related changes |

### Scope

| Scope | Description |
|---|---|
| `engine` | Cleanup engine (`CleanThis`, `findUIDir`) |
| `scanner` | Repository scanner (`Clone`, `DeepSearchAndClean`) |
| `build` | Build validation |
| `lang` | Language statistics module |
| `docs` | Documentation |
| `config` | Configuration |
| `*` | Multiple or global changes |

### Examples

```
feat(engine): add dry-run mode for previewing deletions

Adds a --dry-run flag that reports which components would be
deleted without performing the actual deletion. Useful for
reviewing cleanup impact before execution.

Closes #42
```

```
fix(scanner): handle repository name with special characters

Sanitize repository names against the allowed character pattern
before constructing clone URLs and filesystem paths.

Fixes #37
```

```
docs(*): add security considerations for npm execution

Documents the risks of arbitrary code execution during
npm install and provides sandboxing recommendations.
```

---

## Testing

### Current State

The project currently has **no automated test suite**. Manual testing is the standard approach.

### Manual Testing Checklist

- [ ] Tool compiles without errors (`go build ./...`)
- [ ] `go vet` reports no issues
- [ ] Clone operation succeeds for public repositories
- [ ] React project with `components/ui` is correctly identified
- [ ] Used components are preserved
- [ ] Unused components are deleted
- [ ] Build validation executes (even if non-blocking)
- [ ] Git commit is created
- [ ] Repository clone is cleaned up after processing
- [ ] Non-React projects are skipped
- [ ] Directories without `package.json` are traversed correctly

### Testing Contributions

When contributing code, you are expected to:
1. Test your changes manually using the test methodology above
2. Describe your testing procedure in the PR description
3. Note any edge cases or scenarios that were not tested

### Future Test Infrastructure

Contributions toward an automated test suite are welcome. The ideal test infrastructure would include:

- **Unit tests**: Test individual functions (`findUIDir`, `Contains`, `Folder`)
- **Integration tests**: Test the full cleanup pipeline against a mock repository
- **Golden file tests**: Compare console output against expected output

---

## Issue Reporting

### Bug Reports

Include the following information:

- **Go version**: `go version`
- **Operating system**: Linux distribution and version
- **Repository details**: Public/private, approximate size, structure
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happens
- **Console output**: Full output from the run
- **Steps to reproduce**: Clear, numbered steps

### Feature Requests

- Describe the problem you're trying to solve
- Explain how the feature would work
- Note any alternatives you've considered
- Indicate if you're willing to implement the feature

---

## Getting Help

- Open a [GitHub Discussion](https://github.com/MishraShardendu22/GitHub-Cleaner-Go/discussions)
- Tag `@MishraShardendu22` in issues for maintainer attention

---

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.