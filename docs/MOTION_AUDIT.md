# Motion Audit: GitHub-Cleaner-Go (Systems Telemetry)

## 1. Existing Motion Grep Analysis
* **Grep Hits**: 1 file (`util/dashboard.util.go`) with CSS `transition: all 0.15s`.
* **Missing Animations**: Prometheus stat metric card count-up, live `/metrics` sync indicator pulse, raw metrics inspector expand/collapse transition.

## 2. High-Value Targeted Additions
* **Library / Strategy**: Lightweight CSS transitions + Vanilla JS metric counter animator (no external bundle overhead).
* **Target Interactions**:
  1. Stat cards entry stagger on page load (`@keyframes cardEntrance`).
  2. Prometheus metric numbers animated count-up / smooth transition on poll update.
  3. Live sync pulse indicator light (`@keyframes pulseGlow`).
  4. Table/Metric list hover depth & elevation response.
* **Accessibility**: Respect `prefers-reduced-motion: reduce`.
