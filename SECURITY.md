# Security Policy

## Supported Versions

| Version | Supported |
|---|---|
| Current (HEAD) | ✅ Active development |
| Prior releases | ❌ Not supported |

Security updates are applied to the latest commit on the `main` branch. There are no versioned releases at this time.

---

## Security Domains

GitHub-Cleaner-Go operates across five security domains, each with distinct risk profiles. See [docs/security.md](docs/security.md) for detailed analysis.

### 1. SSH Key Usage

- **Risk**: The tool inherits the executing user's SSH credentials for repository cloning
- **Exposure**: All repositories the SSH key has access to may be cloned and processed
- **Recommendation**: Use a dedicated read-only deploy key with minimal repository scope

### 2. Git Execution

- **Risk**: Repository hooks and `.git/config` can execute arbitrary commands
- **Exposure**: Git clone and commit operations may trigger malicious hooks
- **Recommendation**: Only run against trusted repositories; disable hook execution with `GIT_DISABLE_HOOKS=1`

### 3. Repository Trust

- **Risk**: Repository content is downloaded and analyzed without trust verification
- **Exposure**: Malicious `package.json` files, import paths, or build scripts may affect execution
- **Recommendation**: Only process repositories you own; implement allowlist filtering

### 4. Arbitrary Code Execution via npm

- **Critical Risk**: `npm install` executes lifecycle scripts (preinstall, postinstall, build)
- **Exposure**: A compromised dependency or malicious `package.json` can execute arbitrary code with the tool's privileges
- **Recommendation**: Run in an isolated container; block outbound network; disable npm lifecycle scripts with `--ignore-scripts`

### 5. Filesystem Deletion

- **Risk**: `os.RemoveAll` permanently deletes files without trash/recovery
- **Exposure**: Incorrect path resolution could delete unintended files
- **Recommendation**: Test with dry-run mode (not yet implemented); ensure `components/ui` path is within the cloned repository

---

## Reporting a Vulnerability

If you discover a security vulnerability in GitHub-Cleaner-Go, please report it privately.

**Do not** open a public GitHub Issue for security vulnerabilities.

### Reporting Process

1. **Contact**: Open a [security advisory](https://github.com/MishraShardendu22/GitHub-Cleaner-Go/security/advisories/new) or email the maintainer directly
2. **Response**: The maintainer will acknowledge receipt within 48 hours
3. **Assessment**: A fix will be developed and tested within 7 days for critical vulnerabilities
4. **Disclosure**: A public advisory will be published after the fix is deployed

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if available)
- Affected versions or code paths

---

## Security Best Practices for Operators

### Before Running

```bash
# 1. Use a dedicated, minimal-permission SSH key
ssh-keygen -t ed25519 -C "github-cleaner-deploy-key" -f ~/.ssh/github-cleaner
# Add to GitHub: Settings > Deploy Keys > Read-only access to target repos

# 2. Run in a disposable container
docker run --rm -it \
  -v $HOME/.ssh:/root/.ssh:ro \
  -v $(pwd):/workspace \
  golang:1.24 \
  bash

# 3. Disable Git hooks
export GIT_DISABLE_HOOKS=1

# 4. Limit npm execution
export npm_config_ignore_scripts=true  # If build validation not needed
```

### During Execution

- Monitor console output for unexpected behavior
- Watch for excessive disk I/O or network connections
- Set a timeout for the overall process

### After Execution

- Verify that no orphaned clones remain on disk
- Review Git commit history for unexpected changes
- Check for deleted files that should have been preserved

---

## Threat Model

| Threat | Attack Vector | Impact | Likelihood | Mitigation |
|---|---|---|---|---|
| SSH key theft | Process memory inspection | Access to all accessible repos | Low | Isolated execution |
| Malicious repository | Crafted package.json | Arbitrary code execution | Medium | Repository trust validation |
| npm supply chain | Compromised dependency | Remote code execution | Low-Medium | Network isolation |
| Path traversal | Repository with `..` in name | Deletion outside repo | Low | Name validation |
| ReDoS | Crafted import path | Denial of service | Low | Go RE2 engine (linear-time) |
| Hook execution | Malicious `.git/config` | Command execution | Medium | Disable hooks |

---

## Dependency Security

### Go Standard Library

The tool uses only Go standard library packages. There are no third-party Go dependencies. This eliminates supply chain risk from Go module dependencies.

### External Tool Dependencies

| Tool | Source | Risk |
|---|---|---|
| `git` | System package | Standard |
| `ssh` | System package | Standard |
| `npm` | Node.js distribution | High (executes arbitrary code) |
| `node` | Node.js distribution | Medium (runtime only) |
| `curl` | System package | Low (used only by Language.go) |

### npm Dependency Chain

When `npm install` is executed, the following dependencies are resolved:
- All packages listed in `package.json` `dependencies` and `devDependencies`
- Transitive dependencies of every listed package
- Lifecycle scripts at each level

This chain is the primary attack surface. See [docs/security.md](docs/security.md) for detailed risk analysis and mitigation strategies.

---

## Security Roadmap

See [docs/roadmap.md](docs/roadmap.md) for planned security improvements, including:

- Dry-run mode (Phase 1)
- Repository name validation (Phase 1)
- GitHub token support with authenticated API (Phase 1)
- Container-based sandboxing recommendations (Phase 1)
- Dependency audit integration (Phase 2)
- Signed commits (Phase 3)
- Workload identity integration (Phase 3)