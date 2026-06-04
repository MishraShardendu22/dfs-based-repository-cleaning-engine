package util

import "os"

// Returns the files and the folder in a repo depending on the bool switch
// true - give folder
// fasle - give files

func Segregator(root string, wantDir bool) []string {
	entries, _ := os.ReadDir(root)
	var items []string
	for _, e := range entries {
		if wantDir && e.IsDir() {
			items = append(items, e.Name())
		}
		if !wantDir && !e.IsDir() {
			items = append(items, e.Name())
		}
	}
	return items
}
