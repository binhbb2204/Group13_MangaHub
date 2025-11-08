package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Author struct {
	Node struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"node"`
}

type Manga struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	NumChapters int       `json:"num_chapters"`
	Authors     []Author  `json:"authors"`
}

type MangaList struct {
	Data []struct {
		Node Manga `json:"node"`
	} `json:"data"`
}

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	clientID := os.Getenv("MAL_CLIENT_ID")
	if clientID == "" {
		log.Fatal("Missing MAL_CLIENT_ID in .env")
	}

	// Search term (change or pass via CLI)
	query := "attack on titan"
	fmt.Printf("Searching for %q...\n\n", query)

	// Build API request
	apiURL := "https://api.myanimelist.net/v2/manga"
	params := url.Values{}
	params.Add("q", query)
	params.Add("limit", "17")
	params.Add("fields", "id,title,authors,status,num_chapters")
	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("X-MAL-Client-ID", clientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("MAL API returned status: %v", resp.Status)
	}

	var result MangaList
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d results:\n\n", len(result.Data))

	// Header
	fmt.Println("┌─────────────────────┬──────────────────────┬──────────────────────┬──────────┬─────────────┐")
	fmt.Println("│ ID                  │ Title                │ Author               │ Status   │ Chapters    │")
	fmt.Println("├─────────────────────┼──────────────────────┼──────────────────────┼──────────┼─────────────┤")

	// Rows
	for _, item := range result.Data {
		m := item.Node

		title := m.Title
		if len(title) > 20 {
			title = title[:20] + "..."
		}

		author := "?"
		if len(m.Authors) > 0 {
			first := m.Authors[0].Node.FirstName
			last := m.Authors[0].Node.LastName
			fullName := strings.TrimSpace(first + " " + last)
			if len(fullName) > 20 {
				fullName = fullName[:20] + "..."
			}
			author = fullName
		}

		status := m.Status
		if len(status) > 7 {
			status = status[:7] + "..."
		}
		if status == "" {
			status = "?"
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
