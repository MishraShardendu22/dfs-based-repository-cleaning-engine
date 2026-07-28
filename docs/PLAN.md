# Phase 1 Plan: GitHub-Cleaner-Go

## 1. Design Intent
Create a dark-mode, engineering-grade Systems & Prometheus Telemetry Dashboard for `GitHub-Cleaner-Go`, served at `/` on port `:2112`. The aesthetic follows the canonical design system: dark void background (`#09090b`), raised card panels (`#121318`), indigo & emerald telemetry badges, real-time worker metrics counters, and repository processing status grid.

## 2. Primitives & Token System
* **Colors**:
  * Surface layers: `--bg-base` (`#09090b`), `--bg-raised` (`#121318`), `--bg-overlay` (`#1c1d24`).
  * Text: `--text-primary` (`#f4f4f5`), `--text-secondary` (`#a1a1aa`), `--text-muted` (`#71717a`).
  * Borders: `--border-default` (`#27272a`), `--border-focus` (`#6366f1`).
  * Status: Active (`#10b981`), Build Error (`#ef4444`), Worker (`#3b82f6`).
* **Typography**: Inter font for UI + Monospace font for Go goroutine metrics, repository names, and Prometheus gauge counters.

## 3. Concrete Component Work
* **Telemetry Overview Grid**: Metric tiles for Active Workers, Repos Processed, Files Deleted, Build Success Rate, and Total Execution Duration.
* **Live Prometheus Telemetry Feed**: Auto-polling `/metrics` parser displaying Prometheus metric gauges (`react_repos_total`, `files_deleted_total`, `build_failures_total`, `active_workers`).
* **Repository Processing Table**: High-density table displaying repository name, React status, UI directory state, build duration, and cleanup commit status.
* **HTTP Route Handler**: In `main.go`, bind `http.HandleFunc("/", ServeDashboard)` alongside `http.Handle("/metrics", promhttp.Handler())`.

## 4. Accessibility & SEO
* Semantic HTML5 layout (`<header>`, `<main>`, `<section>`, `<table>`, `<button>`).
* Dark mode contrast compliant against WCAG AA standards.
* Responsive grid supporting 320px mobile through 2560px ultrawide screens.

## 5. Verification Plan
* `go build`: Successful Go compilation.
* `go test ./...`: All Go unit and integration tests pass cleanly.
