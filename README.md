# GitHub Cleaner - Go

An automation tool to intelligently clean GitHub repositories by detecting and removing unused **React UI components**.  
Built with **Go** for performance and reliability, this tool optimizes repository size, reduces bundle bloat, and enforces clean project structure across multiple repositories at once.

---

## Why This Project?

Every time I needed UI in a project, I ran:

```bash
npx shadcn@latest add --all
````

It was convenient, but it installed **every single component**, most of which I never used.
Over time, this bloated repository sizes, slowed down builds, and increased CI/CD costs.

So, I automated the cleanup.

---

## How It Works

The script executes the following steps:

1. **Clones** each public, non-fork repository from my GitHub account
2. **Detects** if it's a React project (`package.json`)
3. **Scans** `components/ui` for unused exports
4. **Deletes** unused files safely
5. **Builds** the project to ensure no errors
6. **Commits** the cleanup automatically
7. **Deletes** the local clone and moves to the next repository

This runs across **50+ repositories** in batch mode.

---

## Results

* **20%** of my entire GitHub codebase cleaned
* **0 build errors**
* All projects still fully functional
* Entire cleanup completed in **one shot** with full automation

I trusted the script enough to run it directly on my main account — no dry-run.
If it failed, I’d have been stuck manually fixing everything for a week.
It didn’t fail.

---

## Features

* **Repository Analysis** – Automated detection of React projects
* **Component Scanning** – Identifies unused UI components
* **Safe Removal** – Deletes only unused files, verified by successful builds
* **Batch Processing** – Handles multiple repositories in sequence
* **Automated Commits** – Clean, descriptive commit messages

---

## Tech Stack

* **Go** – Concurrency, performance, reliability
* **GitHub API** – Repository management and metadata
* **Git** – Automated commits and pushes
* **File System Ops** – Component scanning and cleanup
* **Regex & JSON Parsing** – Detecting usage patterns and dependencies

---

## Safety & Performance

* **Build Validation** – Ensures no project breaks after cleanup
* **Concurrency Control** – Runs repositories in batches (not too many at once)
* **Minimal Risk** – Best practices ensure safe deletions

---

## Demo & Resources

* **Source Code**: [GitHub Repository](https://github.com/MishraShardendu22/GitHub-Cleaner-Go)
* **Live Demo Post**: [LinkedIn](https://www.linkedin.com/posts/shardendumishra22_github-devtools-automation-activity-7347923255464235009-wUEK?utm_source=share&utm_medium=member_desktop&rcm=ACoAAEbGRlsB-5BiDEhn6IkCFFLR11MXpuLMukQ)
* **Video Demo**: [YouTube](https://youtu.be/WM4jTgIm4qg)

---

## Key Takeaway

This project proves:
**Smart scripting > repetitive manual effort.**

Automating mundane dev work saves time, reduces errors, and keeps projects lean.
