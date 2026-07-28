# Phase 1 Plan: GitHub-Cleaner-Go (Motion Elevation)

## 1. Library Selection & Strategy
* **Strategy**: Pure CSS Keyframes + Vanilla JS animated number count-up.
* **Justification**: Lightweight, 0-dependency Go server-rendered HTML dashboard.

## 2. Targeted Interactions
* **Telemetry Counters**: Animated count-up transition when Prometheus metrics poll `/metrics`.
* **Sync Light**: Smooth pulse animation for live telemetry indicator.
* **Card Elevation**: Micro-hover scale and border highlight.
* **Accessibility**: Respect `prefers-reduced-motion: reduce`.
