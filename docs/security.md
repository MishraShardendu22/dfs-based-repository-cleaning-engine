# Security Considerations

## Overview

GitHub-Cleaner-Go performs privileged operations across multiple security domains: GitHub API access, SSH-based repository cloning, filesystem mutation, arbitrary package installation, automated Git operations, and Prometheus metrics exposure. Each domain introduces specific security considerations that operators must understand before deployment.

---

## Domain 1: SSH Key Usage

### Mechanism

Repository cloning uses SSH URLs:

```go
repoURL := "git@github.com:MishraShardendu22/" + repo + ".git"
cmd := exec.Command("git", "clone", repoURL, repoPath)
```

The tool inherits the SSH credentials of the executing user. It relies on an active SSH agent with registered keys.

### Risks

| Risk | Severity | Description |
|---|---|---|
| Key exposure | High | SSH private keys loaded in agent are accessible to any process running under the same user |
| Key misuse | Medium | The tool clones all discoverable repositories, including private ones the key has access to |
| Agent forwarding | Medium | If run in an SSH session with agent forwarding, remote hosts gain key access |

### Mitigations

- Run the tool in an isolated environment (container, VM, dedicated workstation)
- Use a read-only deploy key for cloning operations
- Do not run in SSH sessions with agent forwarding enabled
- Verify SSH agent configuration before execution:
  ```bash
  ssh-add -l  # List loaded keys
  ```

---

## Domain 2: Git Execution

### Mechanism

Git is executed as a subprocess for cloning and committing:

```go
exec.Command("git", "clone", repoURL, repoPath)
exec.Command("sh", "-c", "git cm 'auto: cleanup ui and build'")
```

### Risks

| Risk | Severity | Description |
|---|---|---|
| Malicious gitconfig | High | A repository's `.git/config` can contain malicious hooks or aliases |
| Git hook execution | High | `clone` and `commit` operations can trigger hooks from untrusted repositories |
| Alias dependency | Medium | The tool assumes `cm` is an alias for `commit -am` — undefined behavior if misconfigured |
| Command injection | Medium | Repository names are interpolated into shell commands without sanitization |

### Repo Name Injection Analysis

The repository name is obtained from the GitHub API and directly interpolated:

```go
repoURL := "git@github.com:MishraShardendu22/" + repo + ".git"
```

If a repository name contained shell special characters or path traversal sequences, the behavior would be:

- `git clone` receives the name as an argument (not shell-interpreted), so injection is limited
- `os.RemoveAll(absRepoPath)` uses an absolute path — path traversal is mitigated

**Mitigations**:
- Validate repository names against a strict pattern (`^[A-Za-z0-9_.-]+$`)
- Use `filepath.Clean` to sanitize filesystem paths
- Avoid `sh -c` for git commands where possible

---

## Domain 3: Repository Trust

### Risks

| Risk | Severity | Description |
|---|---|---|
| Malicious repository content | High | The tool downloads and executes code from every repository under the target account |
| Malicious package.json | Medium | `react` and `react-dom` detection is substring-based — a file could be crafted to trigger on non-React projects |
| Malicious import paths | Low | Regex analysis reads all source files — crafted content could cause ReDoS (though Go's RE2 engine is linear-time) |

### Mitigations

- Only run against repositories you own or trust
- Review the repository list before execution (currently hardcoded to a single user)
- Implement a repository allowlist/denylist

---

## Domain 4: Arbitrary Code Execution via npm

### Mechanism

```go
exec.Command("sh", "-c", "npm install --legacy-peer-deps && npm run build")
```

### THIS IS THE HIGHEST RISK OPERATION

`npm install` executes arbitrary code:
- **Preinstall scripts** — `preinstall` lifecycle hook in any dependency
- **Postinstall scripts** — `postinstall` lifecycle hook in any dependency
- **Build scripts** — `install` scripts in native modules (node-gyp compilation)
- **Package.json scripts** — The repository's own `build` script

### Risk Classification

| Risk | Severity | Description |
|---|---|---|
| Malicious dependency install | Critical | npm packages execute arbitrary code during installation |
| Supply chain attack | Critical | Compromised dependency can execute code, exfiltrate data, install malware |
| Build script injection | High | The repository's `build` script runs with the tool's privileges |
| Network access during install | Medium | npm installs make outbound network connections to package registries |
| Disk space exhaustion | Low | `node_modules` can consume significant disk space across repositories |

### Mitigations

1. **Sandboxing** — Run npm operations in a container or VM with restricted network access
   ```bash
   docker run --rm -v $(pwd):/workspace node:18 sh -c "cd /workspace && npm install"
   ```

2. **Network restriction** — Use `npm config set registry` to a private registry, or block outbound network during install

3. **Read-only mode** — If build validation is not required, disable npm execution

4. **Time-bounded execution** — Use `timeout` to limit npm execution duration:
   ```go
   exec.Command("timeout", "300", "npm install --legacy-peer-deps")
   ```

5. **Audit trail** — Log all packages installed during execution

---

## Domain 5: Filesystem Deletion

### Mechanism

```go
os.RemoveAll(path)
```

### Risks

| Risk | Severity | Description |
|---|---|---|
| Data loss | Critical | Deleted files are not sent to trash — they are permanently removed |
| Path traversal | High | If `uiDir` resolves outside the repository, system files could be deleted |
| Race condition | Medium | Files created between analysis and deletion are not detected |

### Mitigations

- The tool currently operates only within cloned repositories (`_Repos/`), limiting scope
- `os.RemoveAll` is bounded by the repository's directory structure
- No symlink following — dangerous if a symlink in `components/ui` points outside the repo

---

## Domain 6: Prometheus Metrics Exposure

### Mechanism

```go
http.Handle("/metrics", promhttp.Handler())
slog.Info("metrics server starting on :2112")
if err := http.ListenAndServe(":2112", nil); err != nil {
    slog.Error("metrics server failed", "error", err)
}
```

### Risks

| Risk | Severity | Description |
|---|---|---|
| Information disclosure | Low | Metrics endpoint exposes repository count, processing durations, and failure counters |
| Unauthenticated endpoint | Medium | The `/metrics` endpoint is open to any client that can reach port 2112 |
| Debug information | Low | Metric labels may reveal repository names or processing characteristics |

### Mitigations

- Bind to localhost only if remote access is not needed (currently binds to all interfaces)
- Use a reverse proxy with authentication for production deployments
- Restrict network access to port 2112 via firewall rules

---

## Safe Execution Recommendations

### Development/Testing

```bash
# 1. Run in a disposable environment
docker run -it --rm golang:1.24 bash

# 2. Use a test GitHub account with forked repositories
# (Modify the username in main.go to a test account)

# 3. Add a dry-run flag (not yet implemented)
# See roadmap.md for planned features
```

### Production

```bash
# 1. Run in isolated container with read-only filesystem overlay
docker run \
  --rm \
  --network none \
  --read-only \
  -v /tmp/cleanup:/tmp/cleanup:rw \
  github-cleaner

# 2. Use dedicated SSH deploy key with minimal permissions
# Key should have: Contents > Read-only access to target repos

# 3. Set resource limits
ulimit -f 1000000  # Limit file size
ulimit -n 1024     # Limit open files
ulimit -u 100      # Limit processes
```

---

## Sandboxing Recommendations

### Option 1: Docker Container

```dockerfile
FROM golang:1.24-alpine

RUN apk add --no-cache git openssh curl npm

COPY . /app
WORKDIR /app
RUN go build -o /usr/local/bin/github-cleaner .

# Create non-root user
RUN adduser -D cleaner
USER cleaner

ENTRYPOINT ["github-cleaner"]
```

### Option 2: Firejail (Linux)

```bash
firejail \
  --net=none \
  --private=/tmp/cleanup-home \
  --seccomp \
  --noroot \
  ./github-cleaner
```

### Option 3: nsenter / User Namespaces

```bash
unshare --user --map-root-user --mount-proc --fork --pid \
  /bin/bash -c './github-cleaner'
```

## Security Checklist

- [ ] Review repository list before execution
- [ ] Verify SSH keys are read-only with minimal scope
- [ ] Run in isolated environment (container/VM)
- [ ] Block outbound network during npm install if possible
- [ ] Set resource limits on the process
- [ ] Review npm package audit before widespread use
- [ ] Test on a single repository before batch processing
- [ ] Have a backup strategy for affected repositories
- [ ] Monitor disk space during execution
- [ ] Check for symlinks in `components/ui` directories
- [ ] Secure Prometheus metrics endpoint (port 2112)