package util

import (
	"os"
	"path/filepath"
)

// DFS recursively searches components/ui
func FindUIDir(root string) (string, bool) {
	var uiDir string

	found := false
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if found || err != nil || !d.IsDir() || filepath.Base(path) != "ui" {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "components" {
			uiDir = path
			found = true
			return filepath.SkipDir
		}
		return nil
	})
	return uiDir, found
}
