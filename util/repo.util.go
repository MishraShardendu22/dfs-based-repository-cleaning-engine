package util

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/MishraShardendu22/model"
)

func GetAllRepos(url string) []string {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var repos []model.Repo
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		log.Fatal(err)
	}

	var names []string
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names
}
