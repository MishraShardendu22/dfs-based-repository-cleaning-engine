package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/MishraShardendu22/util"
)

var wg sync.WaitGroup

func main() {
	username := "MishraShardendu22"
	url := "https://api.github.com/users/" + username + "/repos?per_page=100"

	repos := util.GetAllRepos(url)

	// max 10 goroutines at a time
	limit := make(chan struct{}, 10)

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

			// clone and clean parallely
			CloneAndClean(r)

		}(repo)
	}

	wg.Wait()
}

func CloneAndClean(repo string) {
	// clone using ssh - standard
	repoURL := "git@github.com-project:MishraShardendu22/" + repo + ".git"
	cmd := exec.Command("git", "clone", repoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Clone failed:", repo, err)
		return
	}

	// which repo is currently being cleaned
	repoPath, err := filepath.Abs(repo)
	if err != nil {
		fmt.Println("Failed to get absolute path:", err)
		os.RemoveAll(repo)
		return
	}

	defer func() {
		os.RemoveAll(repoPath)
	}()

	Cleaner(repoPath)
}

func Cleaner(staringRepo string) {
	fmt.Println("Starting in:", staringRepo)
	DeepSearchAndClean(staringRepo)
}

// DFS call to clean the directory recursively
func DeepSearchAndClean(currFolder string) {
	var dirs []string
	var files []string

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		dirs = util.Segregator(currFolder, true)
	}()

	go func() {
		defer wg.Done()
		files = util.Segregator(currFolder, false)
	}()

	wg.Wait()

	if util.Contains(files, "package.json") {
		CleanThis(currFolder)
		return
	}

	for _, d := range dirs {
		DeepSearchAndClean(filepath.Join(currFolder, d))
	}
}

func CleanThis(filesAndFolder string) {
	content, err := os.ReadFile(filepath.Join(filesAndFolder, "package.json"))
	if err != nil {
		return
	}

	pkg := string(content)
	if !strings.Contains(pkg, "react") || !strings.Contains(pkg, "react-dom") {
		return
	}

	uiDir, ok := util.FindUIDir(filesAndFolder)
	if !ok {
		return
	}
	fmt.Println("Cleaning UI in:", uiDir)

	used := map[string]bool{}
	exts := map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true}

	filepath.WalkDir(filesAndFolder, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !exts[filepath.Ext(path)] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range regexp.MustCompile(`[./@"]components/ui/([A-Za-z0-9_-]+)`).FindAllStringSubmatch(string(data), -1) {
			used[strings.ToLower(m[1])] = true
		}
		return nil
	})

	entries, err := os.ReadDir(uiDir)
	if err != nil {
		fmt.Println("Failed to read ui directory:", err)
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		if !used[base] {
			path := filepath.Join(uiDir, name)
			fmt.Println("Deleting unused:", path)
			os.RemoveAll(path)
		}
	}

	build := exec.Command("sh", "-c", "npm install --legacy-peer-deps && npm run build")
	build.Dir = filesAndFolder
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	build.Run()

	git := exec.Command("sh", "-c", "git cm 'auto: cleanup ui and build'")
	git.Dir = filesAndFolder
	git.Stdout = os.Stdout
	git.Stderr = os.Stderr
	git.Run()
}