package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MishraShardendu22/util"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var totalRepos int
var wg sync.WaitGroup
var logger *slog.Logger


func main() {
	logger = util.NewLogger()

	// Start metrics HTTP server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		slog.Info("metrics server starting on :2112")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	username := "MishraShardendu22"
	url := "https://api.github.com/users/" + username + "/repos?per_page=100"

	repos := util.GetAllRepos(url)
	totalRepos = len(repos)

	// max 5 goroutines at a time
	limit := make(chan struct{}, 5)

	for _, repo := range repos {
		wg.Add(1)

		go func(r string) {
			defer wg.Done()

			// add 1 to channel to increase the current capacity
			limit <- struct{}{}

			// remove 1 from channel to decrease capacity so it does not overflow
			defer func() {
				<-limit
			}()

			util.MetricsRegistry.ActiveWorkers.Inc()
			defer util.MetricsRegistry.ActiveWorkers.Dec()

			// clone and clean parallely
			CloneAndClean(r)

		}(repo)
	}

	wg.Wait()

	logger.Info("execution_summary",
		slog.String("operation", "summary"),
		slog.Int("total_repos", totalRepos),
	)

	logger.Info("All repos processed. Keeping metrics server alive. Press Ctrl+C to exit.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logger.Info("Exiting.")
}

func CloneAndClean(repo string) {
	repoStart := time.Now()
	repoPath := filepath.Join("_Repos", repo)

	// clone using ssh - standard
	repoURL := "git@github.com-project:MishraShardendu22/" + repo + ".git"

	util.LogCloneStart(logger, repo)
	cloneStart := time.Now()

	cmd := exec.Command(
		"git",
		"clone",
		repoURL,
		repoPath,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	cloneDur := time.Since(cloneStart)

	if err != nil {
		util.LogCloneEnd(logger, repo, cloneDur, err)
		util.MetricsRegistry.CloneFailuresTotal.Inc()
		util.LogFailure(logger, repo, "clone", err)
		return
	}
	util.LogCloneEnd(logger, repo, cloneDur, nil)
	util.MetricsRegistry.CloneDurationSeconds.Observe(cloneDur.Seconds())

	// which repo is currently being cleaned
	repoPath = filepath.Join("_Repos", repo)
	absRepoPath, err := filepath.Abs(repoPath)

	if err != nil {
		logger.Error("failed_get_absolute_path",
			slog.String("repo", repo),
			slog.String("error", err.Error()),
		)
		os.RemoveAll(repoPath)
		return
	}

	defer func() {
		os.RemoveAll(absRepoPath)
	}()

	Cleaner(absRepoPath, repo, repoStart)
}

// Basically a middle function to initiate timers and improve logging
func Cleaner(staringRepo string, repo string, repoStart time.Time) {
	logger.Info("cleaner_start",
		slog.String("repo", repo),
		slog.String("path", staringRepo),
	)

	util.LogCleanupStart(logger, repo)
	cleanupStart := time.Now()

	func() {
		defer func() {
			cleanupDur := time.Since(cleanupStart)
			util.LogCleanupEnd(logger, repo, cleanupDur, 0, nil)
		}()
		DeepSearchAndClean(staringRepo, repo, repoStart)
	}()
}

// DFS call to clean the directory recursively
func DeepSearchAndClean(currFolder string, repo string, repoStart time.Time) {
	// note all the files and folders basically 
	dirs := util.Segregator(currFolder, true)
	files := util.Segregator(currFolder, false)

	if util.Contains(files, "package.json") {
		CleanThis(currFolder, repo, repoStart)
	}

	for _, d := range dirs {
		DeepSearchAndClean(filepath.Join(currFolder, d), repo, repoStart)
	}
}

func CleanThis(filesAndFolder string, repo string, repoStart time.Time) {
	content, err := os.ReadFile(filepath.Join(filesAndFolder, "package.json"))
	if err != nil {
		return
	}

	pkg := string(content)
	if !strings.Contains(pkg, "react") || !strings.Contains(pkg, "react-dom") {
		return
	}

	util.MetricsRegistry.ReactReposTotal.Inc()

	uiDir, ok := util.FindUIDir(filesAndFolder)
	if !ok {
		return
	}

	used := map[string]bool{}
	exts := map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true}

	filepath.WalkDir(filesAndFolder, func(path string, d os.DirEntry, err error) error {

		// if the path has error or is a directory skip
		if err != nil || d.IsDir() {
			return nil
		}

		// binary / no extension file skipped
		if !exts[filepath.Ext(path)] {
			return nil
		}

		// read the ts, tsx, js, jsx file
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		
		// mark all the used components as true
		for _, m := range regexp.MustCompile(`[./@"]components/ui/([A-Za-z0-9_-]+)`).FindAllStringSubmatch(string(data), -1) {
			used[strings.ToLower(m[1])] = true
		}

		return nil
	})

	
	entries, err := os.ReadDir(uiDir)
	if err != nil {
		logger.Error("failed_read_ui_dir",
			slog.String("repo", repo),
			slog.String("error", err.Error()),
		)
		util.MetricsRegistry.CleanupFailuresTotal.Inc()
		return
	}

	deletedCount := 0
	for _, entry := range entries {
		name := entry.Name()
		base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if !used[base] {
			path := filepath.Join(uiDir, name)
			os.RemoveAll(path)
			deletedCount++
		}
	}

	util.MetricsRegistry.FilesDeletedTotal.Add(float64(deletedCount))

	util.LogBuildStart(logger, repo)
	buildStart := time.Now()

	pmInstall := "npm install --legacy-peer-deps"
	pmBuild := "npm run build"
	folderFiles := util.Segregator(filesAndFolder, false)
	if util.Contains(folderFiles, "yarn.lock") {
		pmInstall = "yarn install"
		pmBuild = "yarn build"
	} else if util.Contains(folderFiles, "pnpm-lock.yaml") {
		pmInstall = "pnpm install"
		pmBuild = "pnpm run build"
	} else if util.Contains(folderFiles, "bun.lockb") {
		pmInstall = "bun install"
		pmBuild = "bun run build"
	}

	buildCmdStr := fmt.Sprintf("%s && %s", pmInstall, pmBuild)
	build := exec.Command("sh", "-c", buildCmdStr)
	build.Dir = filesAndFolder
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	buildErr := build.Run()
	buildDur := time.Since(buildStart)

	buildStatus := "success"
	if buildErr != nil {
		buildStatus = "failed"
		util.MetricsRegistry.BuildFailuresTotal.Inc()
	}
	util.LogBuildEnd(logger, repo, buildDur, buildStatus, buildErr)
	util.MetricsRegistry.BuildDurationSeconds.Observe(buildDur.Seconds())

	if deletedCount > 0 && buildStatus == "success" {
		util.LogGitCommitStart(logger, repo)
		gitStart := time.Now()
		git := exec.Command("sh", "-c", "git add . && git cm 'auto: cleanup ui and build'")
		git.Dir = filesAndFolder
		git.Stdout = os.Stdout
		git.Stderr = os.Stderr
		gitErr := git.Run()
		gitDur := time.Since(gitStart)

		if gitErr != nil {
			util.LogGitCommitEnd(logger, repo, gitDur, gitErr)
			util.MetricsRegistry.GitCommitFailuresTotal.Inc()
		} else {
			util.LogGitCommitEnd(logger, repo, gitDur, nil)
		}
	}

	totalDur := time.Since(repoStart)
	util.MetricsRegistry.RepoProcessingDuration.Observe(totalDur.Seconds())
	util.MetricsRegistry.ReposProcessedTotal.Inc()
	util.LogRepoComplete(logger, repo, totalDur, deletedCount, buildStatus)
}