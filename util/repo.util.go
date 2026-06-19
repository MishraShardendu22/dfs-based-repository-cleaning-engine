package util

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/MishraShardendu22/model"
)

func GetAllRepos(baseURL string) []string {
	var names []string
	page := 1

	token := os.Getenv("GITHUB_TOKEN")

	for {
		url := fmt.Sprintf("%s&page=%d", baseURL, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatal(err)
		}

		if token != "" {
			req.Header.Set("Authorization", "token "+token)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}

		if resp.StatusCode != http.StatusOK {
			// If not OK (e.g. rate limit), log and return what we have
			log.Printf("GitHub API error: status %d", resp.StatusCode)
			resp.Body.Close()
			break
		}

		var repos []model.Repo
		err = json.NewDecoder(resp.Body).Decode(&repos)
		resp.Body.Close()
		
		if err != nil {
			log.Fatal(err)
		}

		if len(repos) == 0 {
			break
		}

		for _, r := range repos {
			names = append(names, r.Name)
		}
		page++
	}

	return names
}
