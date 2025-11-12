package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// --- Structs ---

type Author struct {
	Node struct {
		Name      string `json:"name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"node"`
}

type Manga struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	NumChapters int      `json:"num_chapters"`
	Authors     []Author `json:"authors"`
}

type MangaList struct {
	Data []struct {
		Node Manga `json:"node"`
	} `json:"data"`
}

// --- Fetch Manga Ranking ---

func fetchRanking(clientID, rankingType string, limit int) ([]Manga, error) {
	apiURL := "https://api.myanimelist.net/v2/manga/ranking"
	params := url.Values{}
	params.Add("ranking_type", rankingType)
	params.Add("limit", fmt.Sprintf("%d", limit))
	params.Add("fields", "id,title,authors,status,num_chapters")

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MAL-Client-ID", clientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MAL API returned status: %v", resp.Status)
	}

	var result MangaList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var mangas []Manga
	for _, item := range result.Data {
		mangas = append(mangas, item.Node)
	}
	return mangas, nil
}

// --- Print Manga Table ---

func printTable(title string, mangas []Manga) {
	fmt.Printf("\n📚 %s\n", title)
	fmt.Println("┌─────────────────────┬──────────────────────┬──────────────────────┬──────────┬─────────────┐")
	fmt.Println("│ ID                  │ Title                │ Author               │ Status   │ Chapters    │")
	fmt.Println("├─────────────────────┼──────────────────────┼──────────────────────┼──────────┼─────────────┤")

	for _, m := range mangas {
		title := m.Title
		if len(title) > 20 {
			title = title[:20] + "..."
		}

		author := "?"
		if len(m.Authors) > 0 {
			name := m.Authors[0].Node.Name
			if name == "" {
				first := m.Authors[0].Node.FirstName
				last := m.Authors[0].Node.LastName
				name = strings.TrimSpace(first + " " + last)
			}
			if len(name) > 20 {
				name = name[:20] + "..."
			}
			if name != "" {
				author = name
			}
		}

		status := m.Status
		if status == "" {
			status = "?"
		}
		if len(status) > 7 {
			status = status[:7] + "..."
		}

		chapters := "?"
		if m.NumChapters > 0 {
			chapters = fmt.Sprintf("%d", m.NumChapters)
		}

		fmt.Printf("│ %-19d │ %-20s │ %-20s │ %-8s │ %-11s │\n",
			m.ID, title, author, status, chapters)
	}

	fmt.Println("└─────────────────────┴──────────────────────┴──────────────────────┴──────────┴─────────────┘")
}

// --- Main Program ---

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	clientID := os.Getenv("MAL_CLIENT_ID")
	if clientID == "" {
		log.Fatal("Missing MAL_CLIENT_ID in .env")
	}

	fmt.Println("Fetching featured manga for homepage...\n")

	sections := []struct {
		label string
		key   string
	}{
		{"🏆 Top Ranked Manga", "all"},
		{"🔥 Most Popular Manga", "bypopularity"},
		{"❤️ Most Favorited Manga", "favorite"},
	}

	var wg sync.WaitGroup
	results := make([][]Manga, len(sections))

	for i, s := range sections {
		wg.Add(1)
		go func(i int, s struct {
			label string
			key   string
		}) {
			defer wg.Done()
			mangas, err := fetchRanking(clientID, s.key, 10)
			if err != nil {
				log.Printf("Error fetching %s: %v", s.label, err)
				return
			}
			results[i] = mangas
		}(i, s)
	}

	wg.Wait()

	// Print all sections
	for i, s := range sections {
		if len(results[i]) > 0 {
			printTable(s.label, results[i])
		}
	}
}
