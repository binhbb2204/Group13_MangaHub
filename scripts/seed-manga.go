package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("=== MangaHub Database Seeder ===")

	// Load environment
	godotenv.Load()

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/mangahub.db"
	}

	if err := database.InitDatabase(dbPath); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Check existing count
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	fmt.Printf("Current manga count: %d\n", count)

	if count >= 500 {
		fmt.Println("Database already has 500+ manga. Skipping seed.")
		return
	}

	// Fetch from MAL
	malSource := manga.NewMALSource()
	ctx := context.Background()

	var allManga []MangaData

	// Fetch diverse ranking types to populate frontend filters
	rankingTypes := []struct {
		Type  string
		Label string
	}{
		{"all", "top overall"},
		{"bypopularity", "popular"},
		{"favorite", "most favorited"},
		{"manga", "manga"},
		{"novels", "light novels"},
	}

	for _, rt := range rankingTypes {
		fmt.Printf("\nFetching 150 %s...\n", rt.Label)
		mangas, err := fetchRankingManga(malSource, ctx, rt.Type, 150)
		if err != nil {
			log.Printf("Warning: Failed to fetch %s: %v\n", rt.Label, err)
			continue
		}
		allManga = append(allManga, mangas...)
		fmt.Printf("✓ Fetched %d %s\n", len(mangas), rt.Label)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var uniqueManga []MangaData
	for _, m := range allManga {
		if !seen[m.ID] {
			seen[m.ID] = true
			uniqueManga = append(uniqueManga, m)
		}
	}

	fmt.Printf("\nTotal unique manga: %d\n", len(uniqueManga))

	// Insert into database
	fmt.Println("\nInserting into database...")
	inserted := 0
	skipped := 0

	for _, m := range uniqueManga {
		if insertManga(m) {
			inserted++
		} else {
			skipped++
		}

		if (inserted+skipped)%20 == 0 {
			fmt.Printf("Progress: %d/%d\n", inserted+skipped, len(uniqueManga))
		}
	}

	fmt.Printf("\n=== Seed Complete ===\n")
	fmt.Printf("Inserted: %d\n", inserted)
	fmt.Printf("Skipped (duplicates): %d\n", skipped)

	// Final count
	database.DB.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	fmt.Printf("Total manga in DB: %d\n", count)
}

// Manga data structure
type MangaData struct {
	ID          string
	Title       string
	Author      string
	Genres      []string
	Status      string
	Chapters    int
	Description string
	CoverURL    string
	MediaType   string
}

// Fetch ranking manga
func fetchRankingManga(source *manga.MALSource, ctx context.Context, rankingType string, limit int) ([]MangaData, error) {
	clientID := os.Getenv("MAL_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("MAL_CLIENT_ID not set")
	}

	var results []MangaData
	offset := 0

	for len(results) < limit {
		fetchLimit := 100
		if len(results)+fetchLimit > limit {
			fetchLimit = limit - len(results)
		}

		// Call MAL ranking API directly
		apiURL := fmt.Sprintf("https://api.myanimelist.net/v2/manga/ranking?ranking_type=%s&limit=%d&offset=%d&fields=id,title,main_picture,authors{first_name,last_name},status,num_chapters,synopsis,genres,media_type",
			rankingType, fetchLimit, offset)

		mangas, err := fetchFromMAL(clientID, apiURL)
		if err != nil {
			return results, err
		}

		if len(mangas) == 0 {
			break
		}

		results = append(results, mangas...)
		offset += len(mangas)

		if len(mangas) < fetchLimit {
			break
		}

		time.Sleep(200 * time.Millisecond)
	}

	return results, nil
}

// Fetch from MAL API
func fetchFromMAL(clientID, apiURL string) ([]MangaData, error) {
	httpReq, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-MAL-Client-ID", clientID)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MAL API error: %s", resp.Status)
	}

	var rankingResp struct {
		Data []struct {
			Node struct {
				ID          int    `json:"id"`
				Title       string `json:"title"`
				MainPicture *struct {
					Large  string `json:"large"`
					Medium string `json:"medium"`
				} `json:"main_picture"`
				Status      string `json:"status"`
				NumChapters int    `json:"num_chapters"`
				Synopsis    string `json:"synopsis"`
				MediaType   string `json:"media_type"`
				Genres      []struct {
					Name string `json:"name"`
				} `json:"genres"`
				Authors []struct {
					Node struct {
						FirstName string `json:"first_name"`
						LastName  string `json:"last_name"`
					} `json:"node"`
				} `json:"authors"`
			} `json:"node"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rankingResp); err != nil {
		return nil, err
	}

	var results []MangaData
	for _, item := range rankingResp.Data {
		node := item.Node

		coverURL := ""
		if node.MainPicture != nil {
			if node.MainPicture.Large != "" {
				coverURL = node.MainPicture.Large
			} else {
				coverURL = node.MainPicture.Medium
			}
		}

		var genres []string
		for _, g := range node.Genres {
			genres = append(genres, g.Name)
		}

		author := ""
		if len(node.Authors) > 0 {
			author = strings.TrimSpace(node.Authors[0].Node.FirstName + " " + node.Authors[0].Node.LastName)
		}

		status := strings.ToLower(node.Status)
		if status == "finished" {
			status = "completed"
		}

		mediaType := strings.ToLower(node.MediaType)
		if mediaType == "" {
			mediaType = "manga"
		}

		results = append(results, MangaData{
			ID:          fmt.Sprintf("%d", node.ID),
			Title:       node.Title,
			Author:      author,
			Genres:      genres,
			Status:      status,
			Chapters:    node.NumChapters,
			Description: node.Synopsis,
			CoverURL:    coverURL,
			MediaType:   mediaType,
		})
	}

	return results, nil
}

// Insert manga into database
func insertManga(m MangaData) bool {
	genresJSON, _ := json.Marshal(m.Genres)

	query := `INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, media_type) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := database.DB.Exec(
		query,
		m.ID,
		m.Title,
		m.Author,
		string(genresJSON),
		m.Status,
		m.Chapters,
		m.Description,
		m.CoverURL,
		m.MediaType,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return false
		}
		log.Printf("Insert error for %s: %v", m.Title, err)
		return false
	}

	return true
}
