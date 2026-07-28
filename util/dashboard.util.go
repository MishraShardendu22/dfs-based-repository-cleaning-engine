package util

import (
	"fmt"
	"net/http"
)

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>GitHub Cleaner — Systems Telemetry & Metrics Dashboard</title>
  <meta name="description" content="Production-grade Go systems telemetry and repository cleaner metrics dashboard.">
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap" rel="stylesheet">
  <style>
    :root {
      color-scheme: dark;
      --bg-base: #09090b;
      --bg-raised: #121318;
      --bg-overlay: #1c1d24;
      --border-default: #27272a;
      --border-subtle: #1f1f23;
      --border-focus: #6366f1;
      --text-primary: #f4f4f5;
      --text-secondary: #a1a1aa;
      --text-muted: #71717a;
      --accent: #6366f1;
      --accent-muted: rgba(99, 102, 241, 0.15);
      --success: #10b981;
      --success-bg: rgba(16, 185, 129, 0.12);
      --warning: #f59e0b;
      --danger: #ef4444;
      --font-sans: 'Inter', system-ui, sans-serif;
      --font-mono: 'JetBrains Mono', monospace;
      --radius-sm: 6px;
      --radius-md: 10px;
      --radius-lg: 14px;
    }
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg-base);
      color: var(--text-primary);
      font-family: var(--font-sans);
      font-size: 14px;
      line-height: 1.6;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
    }
    .header {
      background: var(--bg-raised);
      border-bottom: 1px solid var(--border-default);
      position: sticky;
      top: 0;
      z-index: 100;
    }
    .header-inner {
      max-width: 1400px;
      margin: 0 auto;
      padding: 16px 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .brand-logo {
      width: 32px;
      height: 32px;
      background: linear-gradient(135deg, #10b981, #6366f1);
      border-radius: var(--radius-sm);
      display: grid;
      place-items: center;
      font-weight: 700;
      color: #fff;
    }
    .brand-title { font-size: 16px; font-weight: 700; }
    .nav-btn {
      background: var(--bg-overlay);
      color: var(--text-secondary);
      border: 1px solid var(--border-default);
      padding: 6px 14px;
      border-radius: var(--radius-sm);
      text-decoration: none;
      font-size: 13px;
      font-weight: 500;
      transition: all 0.15s;
    }
    .nav-btn:hover { color: var(--text-primary); border-color: var(--text-muted); }
    .main {
      max-width: 1400px;
      margin: 0 auto;
      width: 100%;
      padding: 32px 24px;
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 28px;
    }
    .grid-stats {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
      gap: 16px;
    }
    .stat-card {
      background: var(--bg-raised);
      border: 1px solid var(--border-default);
      border-radius: var(--radius-md);
      padding: 20px;
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    .stat-label {
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.06em;
      color: var(--text-muted);
    }
    .stat-value {
      font-family: var(--font-mono);
      font-size: 28px;
      font-weight: 700;
      color: var(--text-primary);
    }
    .card-panel {
      background: var(--bg-raised);
      border: 1px solid var(--border-default);
      border-radius: var(--radius-lg);
      padding: 24px;
    }
    .panel-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 20px;
    }
    .panel-title { font-size: 16px; font-weight: 600; }
    .metrics-raw {
      width: 100%;
      height: 320px;
      background: var(--bg-base);
      border: 1px solid var(--border-default);
      border-radius: var(--radius-sm);
      padding: 16px;
      color: #34d399;
      font-family: var(--font-mono);
      font-size: 12px;
      overflow: auto;
      white-space: pre-wrap;
    }
  </style>
</head>
<body>
  <header class="header">
    <div class="header-inner">
      <div class="brand">
        <div class="brand-logo">⚡</div>
        <span class="brand-title">GitHub Cleaner Telemetry</span>
      </div>
      <a href="/metrics" class="nav-btn">Raw Prometheus Metrics</a>
    </div>
  </header>

  <main class="main">
    <div class="grid-stats">
      <div class="stat-card">
        <div class="stat-label">Active Worker Goroutines</div>
        <div id="val-workers" class="stat-value">10</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">React Repos Processed</div>
        <div id="val-repos" class="stat-value">--</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Unused UI Files Deleted</div>
        <div id="val-deleted" class="stat-value">--</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Build Failures</div>
        <div id="val-failures" class="stat-value">0</div>
      </div>
    </div>

    <div class="card-panel">
      <div class="panel-header">
        <h2 class="panel-title">Live Telemetry Inspector (/metrics)</h2>
        <span id="poll-status" style="font-size:12px; color:var(--text-muted); font-family:var(--font-mono);">Polling active</span>
      </div>
      <pre id="metrics-output" class="metrics-raw">Fetching live telemetry data from Go runtime...</pre>
    </div>
  </main>

  <script>
    async function fetchMetrics() {
      try {
        const res = await fetch('/metrics');
        const text = await res.text();
        document.getElementById('metrics-output').textContent = text;
        
        // Parse simple metrics
        const workersMatch = text.match(/active_workers\s+(\d+)/);
        if (workersMatch) document.getElementById('val-workers').textContent = workersMatch[1];
        
        const reposMatch = text.match(/react_repos_total\s+(\d+)/);
        if (reposMatch) document.getElementById('val-repos').textContent = reposMatch[1];

        const deletedMatch = text.match(/files_deleted_total\s+(\d+)/);
        if (deletedMatch) document.getElementById('val-deleted').textContent = deletedMatch[1];

        const failuresMatch = text.match(/build_failures_total\s+(\d+)/);
        if (failuresMatch) document.getElementById('val-failures').textContent = failuresMatch[1];

        document.getElementById('poll-status').textContent = 'Last sync: ' + new Date().toLocaleTimeString();
      } catch (err) {
        document.getElementById('poll-status').textContent = 'Sync error: ' + err.message;
      }
    }

    fetchMetrics();
    setInterval(fetchMetrics, 3000);
  </script>
</body>
</html>`

func ServeDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}
