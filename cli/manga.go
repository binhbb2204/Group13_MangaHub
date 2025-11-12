package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var (
	searchGenre  string
	searchStatus string
	searchLimit  int
	rankingLimit int
)

var mangaCmd = &cobra.Command{
	Use:   "manga",
	Short: "Manga management commands",
	Long:  `Search and manage manga information.`,
}

var mangaSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for manga",
	Long:  `Search for manga by title using MyAnimeList API with optional genre and status filters.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		if len(strings.TrimSpace(query)) < 3 {
			fmt.Printf("\nSearching for \"%s\"...\n", query)
			fmt.Println("\n✗ Search query too short")
			fmt.Println("\nRequirements:")
			fmt.Println("  - Search query must be at least 3 characters")
			fmt.Println("\nSuggestions:")
			fmt.Println("  - Use more specific search terms")
			fmt.Println("  - Try full manga title instead of abbreviations")
			return nil
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		   requestLimit := searchLimit
		   if requestLimit > 100 || requestLimit <= 0 {
			   requestLimit = 100
		   }
		   searchURL := fmt.Sprintf("%s/manga/search?q=%s&limit=%d", serverURL, url.QueryEscape(query), requestLimit)

		res, err := http.Get(searchURL)
		if err != nil {
			printError("Search failed: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)

			if strings.Contains(errRes["error"], "at least 3 characters") {
				fmt.Printf("\nSearching for \"%s\"...\n", query)
				fmt.Println("\n✗ Search query too short")
				fmt.Println("\nRequirements:")
				fmt.Println("  - Search query must be at least 3 characters")
				fmt.Println("\nSuggestions:")
				fmt.Println("  - Use more specific search terms")
				fmt.Println("  - Try full manga title instead of abbreviations")
				return nil
			}

			printError(fmt.Sprintf("Search failed: %s", errRes["error"]))
			return fmt.Errorf("search failed")
		}

		var result struct {
			Mangas []struct {
				ID            string   `json:"id"`
				Title         string   `json:"title"`
				Author        string   `json:"author"`
				Genres        []string `json:"genres"`
				Status        string   `json:"status"`
				TotalChapters int      `json:"total_chapters"`
				Description   string   `json:"description"`
			} `json:"mangas"`
			Count int `json:"count"`
		}
		json.Unmarshal(body, &result)

		// Apply client-side filters
		filteredMangas := result.Mangas
		if searchGenre != "" || searchStatus != "" {
			filteredMangas = []struct {
				ID            string   `json:"id"`
				Title         string   `json:"title"`
				Author        string   `json:"author"`
				Genres        []string `json:"genres"`
				Status        string   `json:"status"`
				TotalChapters int      `json:"total_chapters"`
				Description   string   `json:"description"`
			}{}

			for _, manga := range result.Mangas {
				//Check genre filter
				genreMatch := searchGenre == ""
				if searchGenre != "" {
					for _, g := range manga.Genres {
						if strings.EqualFold(g, searchGenre) {
							genreMatch = true
							break
						}
					}
				}

				//Check status filter
				statusMatch := searchStatus == ""
				if searchStatus != "" {
					if strings.EqualFold(manga.Status, searchStatus) ||
						(strings.EqualFold(searchStatus, "completed") && strings.EqualFold(manga.Status, "finished")) {
						statusMatch = true
					}
				}

				if genreMatch && statusMatch {
					filteredMangas = append(filteredMangas, manga)
					if len(filteredMangas) >= searchLimit {
						break
					}
				}
			}
		} else if len(filteredMangas) > searchLimit {
			filteredMangas = filteredMangas[:searchLimit]
		}

		if len(filteredMangas) == 0 {
			fmt.Printf("\nSearching for \"%s\"", query)
			if searchGenre != "" || searchStatus != "" {
				fmt.Printf(" (filters:")
				if searchGenre != "" {
					fmt.Printf(" genre=%s", searchGenre)
				}
				if searchStatus != "" {
					fmt.Printf(" status=%s", searchStatus)
				}
				fmt.Printf(")")
			}
			fmt.Println("...")
			fmt.Println("\nNo manga found matching your search criteria.")
			fmt.Println("\nSuggestions:")
			fmt.Println("  - Check spelling and try again")
			fmt.Println("  - Use broader search terms")
			if searchGenre != "" || searchStatus != "" {
				fmt.Println("  - Try removing filters")
			}
			fmt.Println("  - Try different keywords")
			fmt.Println("  - Browse by searching popular titles")
			return nil
		}

		//Print formatted table output
		fmt.Printf("\nSearching for \"%s\"", query)
		if searchGenre != "" || searchStatus != "" {
			fmt.Printf(" (filters:")
			if searchGenre != "" {
				fmt.Printf(" genre=%s", searchGenre)
			}
			if searchStatus != "" {
				fmt.Printf(" status=%s", searchStatus)
			}
			fmt.Printf(")")
		}
		fmt.Println("...")
		fmt.Printf("\nFound %d results:\n\n", len(filteredMangas))

		//Print table header (match ranking table width)
		fmt.Println("┌─────────────────────┬──────────────────────┬──────────────────────┬──────────┬─────────────┐")
		fmt.Println("│ ID                  │ Title                │ Author               │ Status   │ Chapters    │")
		fmt.Println("├─────────────────────┼──────────────────────┼──────────────────────┼──────────┼─────────────┤")

		//Print manga rows
		for _, manga := range filteredMangas {
			chaptersStr := fmt.Sprintf("%d", manga.TotalChapters)
			if manga.TotalChapters == 0 {
				chaptersStr = "Ongoing"
			}

			fmt.Printf("│ %-19s │ %-20s │ %-20s │ %-8s │ %-11s │\n",
				truncateString(manga.ID, 19),
				truncateString(manga.Title, 20),
				truncateString(manga.Author, 20),
				truncateString(manga.Status, 8),
				chaptersStr)
		}

		fmt.Println("└─────────────────────┴──────────────────────┴──────────────────────┴──────────┴─────────────┘")
		fmt.Println("\nUse 'mangahub manga info <id>' to view details")
		fmt.Println("Use 'mangahub library add --manga-id <id>' to add to your library")

		return nil
	},
}

var mangaInfoCmd = &cobra.Command{
	Use:   "info [manga-id]",
	Short: "Get detailed information about a manga",
	Long:  `Get detailed information about a manga by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID := args[0]

		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		// Support both MAL numeric id and human-friendly slug (e.g., "one-piece")
		displayID := mangaID
		resolvedID := mangaID
		// If not purely numeric, try to resolve by searching title
		isNumeric := true
		for _, ch := range mangaID {
			if ch < '0' || ch > '9' {
				isNumeric = false
				break
			}
		}
		if !isNumeric {
			q := strings.ReplaceAll(mangaID, "-", " ")
			searchURL := fmt.Sprintf("%s/manga/search?q=%s&limit=1", serverURL, url.QueryEscape(q))
			searchRes, err := http.Get(searchURL)
			if err == nil {
				defer searchRes.Body.Close()
				if searchRes.StatusCode == http.StatusOK {
					var sr struct {
						Mangas []struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"mangas"`
						Count int `json:"count"`
					}
					b, _ := io.ReadAll(searchRes.Body)
					_ = json.Unmarshal(b, &sr)
					if sr.Count > 0 && len(sr.Mangas) > 0 {
						resolvedID = sr.Mangas[0].ID
					}
				}
			}
		}

		res, err := http.Get(fmt.Sprintf("%s/manga/info/%s", serverURL, resolvedID))
		if err != nil {
			printError("Failed to get manga info: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode == http.StatusNotFound {
			printError(fmt.Sprintf("Manga not found: %s", mangaID))
			fmt.Println("\nTry searching for manga:")
			fmt.Println("  mangahub manga search \"manga title\"")
			return fmt.Errorf("manga not found")
		}

		if res.StatusCode != http.StatusOK {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to get manga info: %s", errRes["error"]))
			return fmt.Errorf("failed to get manga info")
		}

		var manga struct {
			ID            string   `json:"id"`
			Title         string   `json:"title"`
			Author        string   `json:"author"`
			Genres        []string `json:"genres"`
			Status        string   `json:"status"`
			TotalChapters int      `json:"total_chapters"`
			Description   string   `json:"description"`
			CoverURL      string   `json:"cover_url"`
		}
		json.Unmarshal(body, &manga)

		// Title box
		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────────────┐")
		title := strings.ToUpper(manga.Title)
		boxWidth := 69
		lp := (boxWidth - len(title)) / 2
		rp := boxWidth - len(title) - lp
		if lp < 0 {
			lp = 0
		}
		if rp < 0 {
			rp = 0
		}
		fmt.Printf("│%s%s%s│\n", strings.Repeat(" ", lp), title, strings.Repeat(" ", rp))
		fmt.Println("└─────────────────────────────────────────────────────────────────────┘")

		// Basic Information
		fmt.Println("Basic Information:")
		fmt.Printf("ID: %s\n", displayID)
		altTitle := manga.Title
		fmt.Printf("Title: %s\n", altTitle)
		artist := manga.Author
		if artist == "" {
			artist = "-"
		}
		author := manga.Author
		if author == "" {
			author = "-"
		}
		fmt.Printf("Author: %s\n", author)
		fmt.Printf("Artist: %s\n", artist)
		if len(manga.Genres) > 0 {
			fmt.Printf("Genres: %s\n", strings.Join(manga.Genres, ", "))
		} else {
			fmt.Println("Genres: -")
		}
		status := manga.Status
		if status == "" {
			status = "-"
		}
		fmt.Printf("Status: %s\n", status)
		fmt.Println("Year: -")

		// Progress
		fmt.Println("Progress:")
		if manga.TotalChapters > 0 {
			fmt.Printf("Total Chapters: %d\n", manga.TotalChapters)
		} else {
			fmt.Println("Total Chapters: Ongoing")
		}
		fmt.Println("Total Volumes: -")
		fmt.Println("Serialization: -")
		fmt.Println("Publisher: -")
		fmt.Println("Your Status: -")
		fmt.Println("Current Chapter: -")
		fmt.Println("Last Updated: -")
		fmt.Println("Started Reading: -")
		fmt.Println("Personal Rating: -")

		// Description
		if manga.Description != "" {
			fmt.Println("Description:")
			fmt.Println(wrapText(manga.Description, 80))
		}

		// External Links
		fmt.Println("External Links:")
		if manga.ID != "" {
			fmt.Printf("MyAnimeList: https://myanimelist.net/manga/%s\n", manga.ID)
		}
		fmt.Printf("MangaDex (search): https://mangadex.org/titles?q=%s\n", url.QueryEscape(manga.Title))

		// Actions
		fmt.Println("Actions:")
		fmt.Printf("Update Progress: mangahub progress update --manga-id %s --chapter <num>\n", displayID)
		fmt.Printf("Rate/Review: mangahub library update --manga-id %s --rating <1-10>\n", displayID)
		fmt.Printf("Remove: mangahub library remove --manga-id %s\n", displayID)
		fmt.Println()

		return nil
	},
}

var mangaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available manga",
	Long:  `List all manga in the database with optional filtering.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		// Build URL with filters
		listURL := fmt.Sprintf("%s/manga/all", serverURL)

		res, err := http.Get(listURL)
		if err != nil {
			printError("Failed to list manga: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to list manga: %s", errRes["error"]))
			return fmt.Errorf("failed to list manga")
		}

		var result struct {
			Mangas []struct {
				ID            string `json:"id"`
				Title         string `json:"title"`
				Author        string `json:"author"`
				Status        string `json:"status"`
				TotalChapters int    `json:"total_chapters"`
			} `json:"mangas"`
			Count int `json:"count"`
		}
		json.Unmarshal(body, &result)

		if result.Count == 0 {
			fmt.Println("\nNo manga found in the database.")
			fmt.Println("\nThe database is empty. Manga can be added by administrators.")
			return nil
		}

		fmt.Printf("\nTotal manga available: %d\n\n", result.Count)

		for i, manga := range result.Mangas {
			fmt.Printf("%3d. %-40s [%s]\n", i+1,
				truncateString(manga.Title, 40),
				manga.ID)
			fmt.Printf("     Author: %-20s Status: %-15s Chapters: %d\n",
				manga.Author, manga.Status, manga.TotalChapters)
		}

		fmt.Println("\nUse 'mangahub manga info <id>' to view details")

		return nil
	},
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		if i > 0 {
			if lineLen+len(word)+1 > width {
				result.WriteString("\n")
				lineLen = 0
			} else {
				result.WriteString(" ")
				lineLen++
			}
		}
		result.WriteString(word)
		lineLen += len(word)
	}

	return result.String()
}

var mangaFeaturedCmd = &cobra.Command{
	Use:   "featured",
	Short: "Show featured manga for homepage",
	Long:  `Display top ranked, most popular, and most favorited manga from MyAnimeList.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		res, err := http.Get(fmt.Sprintf("%s/manga/featured", serverURL))
		if err != nil {
			printError("Failed to fetch featured manga: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to fetch featured manga: %s", errRes["error"]))
			return fmt.Errorf("failed to fetch featured manga")
		}

		var result struct {
			Sections []struct {
				Label  string `json:"label"`
				Mangas []struct {
					ID          int    `json:"id"`
					Title       string `json:"title"`
					Status      string `json:"status"`
					NumChapters int    `json:"num_chapters"`
					Authors     []struct {
						Node struct {
							Name      string `json:"name"`
							FirstName string `json:"first_name"`
							LastName  string `json:"last_name"`
						} `json:"node"`
					} `json:"authors"`
				} `json:"mangas"`
			} `json:"sections"`
		}
		json.Unmarshal(body, &result)

		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println("  📚 FEATURED MANGA FOR HOMEPAGE")
		fmt.Println(strings.Repeat("=", 80))

		for _, section := range result.Sections {
			if len(section.Mangas) == 0 {
				continue
			}

			fmt.Println()
			fmt.Println("┌─────────────────────────────────────────────────────────────────────┐")
			boxWidth := 69
			leftPadding := (boxWidth - len(section.Label)) / 2
			rightPadding := boxWidth - len(section.Label) - leftPadding
			fmt.Printf("│%s%s%s│\n", strings.Repeat(" ", leftPadding), section.Label, strings.Repeat(" ", rightPadding))
			fmt.Println("└─────────────────────────────────────────────────────────────────────┘")

			fmt.Println("┌─────────────────────┬──────────────────────┬──────────────────────┬──────────┬─────────────┐")
			fmt.Println("│ ID                  │ Title                │ Author               │ Status   │ Chapters    │")
			fmt.Println("├─────────────────────┼──────────────────────┼──────────────────────┼──────────┼─────────────┤")

			for _, manga := range section.Mangas {
				title := manga.Title
				if len(title) > 20 {
					title = title[:17] + "..."
				}

				author := "?"
				if len(manga.Authors) > 0 {
					name := manga.Authors[0].Node.Name
					if name == "" {
						name = strings.TrimSpace(manga.Authors[0].Node.FirstName + " " + manga.Authors[0].Node.LastName)
					}
					if name != "" {
						if len(name) > 20 {
							name = name[:17] + "..."
						}
						author = name
					}
				}

				status := manga.Status
				if status == "" {
					status = "?"
				}
				if len(status) > 8 {
					status = status[:5] + "..."
				}

				chapters := "?"
				if manga.NumChapters > 0 {
					chapters = fmt.Sprintf("%d", manga.NumChapters)
				}

				fmt.Printf("│ %-19d │ %-20s │ %-20s │ %-8s │ %-11s │\n",
					manga.ID, title, author, status, chapters)
			}

			fmt.Println("└─────────────────────┴──────────────────────┴──────────────────────┴──────────┴─────────────┘")
		}

		fmt.Println("\nUse 'mangahub manga info <id>' to view details")
		fmt.Println("Use 'mangahub library add --manga-id <id>' to add to your library")
		fmt.Println()

		return nil
	},
}

var mangaRankingCmd = &cobra.Command{
	Use:   "ranking [type]",
	Short: "Show manga ranking by type",
	Long:  `Display manga ranking from MyAnimeList. Available types: all, bypopularity, favorite.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rankingType := "all"
		if len(args) > 0 {
			rankingType = args[0]
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		limit := rankingLimit
		if limit <= 0 || limit > 100 {
			limit = 100
		}
		rankingURL := fmt.Sprintf("%s/manga/ranking?type=%s&limit=%d", serverURL, rankingType, limit)

		res, err := http.Get(rankingURL)
		if err != nil {
			printError("Failed to fetch ranking: Server connection error")
			fmt.Println("Check server status: mangahub server status")
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)

		if res.StatusCode != http.StatusOK {
			var errRes map[string]string
			json.Unmarshal(body, &errRes)
			printError(fmt.Sprintf("Failed to fetch ranking: %s", errRes["error"]))
			return fmt.Errorf("failed to fetch ranking")
		}

		var result struct {
			Mangas []struct {
				ID          int    `json:"id"`
				Title       string `json:"title"`
				Status      string `json:"status"`
				NumChapters int    `json:"num_chapters"`
				Authors     []struct {
					Node struct {
						FirstName string `json:"first_name"`
						LastName  string `json:"last_name"`
					} `json:"node"`
				} `json:"authors"`
			} `json:"mangas"`
			Count int    `json:"count"`
			Type  string `json:"type"`
		}
		json.Unmarshal(body, &result)

		if result.Count == 0 {
			fmt.Printf("\nNo manga found for ranking type: %s\n", rankingType)
			fmt.Println("\nAvailable ranking types:")
			fmt.Println("  - all: Top ranked manga")
			fmt.Println("  - bypopularity: Most popular manga")
			fmt.Println("  - favorite: Most favorited manga")
			return nil
		}

		typeLabel := map[string]string{
			"all":          "Top Ranked Manga",
			"bypopularity": "Most Popular Manga",
			"favorite":     "Most Favorited Manga",
		}

		label := typeLabel[result.Type]
		if label == "" {
			label = "Manga Ranking"
		}

		fmt.Println()
		fmt.Println("┌─────────────────────────────────────────────────────────────────────┐")
		boxWidth := 69
		leftPadding := (boxWidth - len(label)) / 2
		rightPadding := boxWidth - len(label) - leftPadding
		fmt.Printf("│%s%s%s│\n", strings.Repeat(" ", leftPadding), label, strings.Repeat(" ", rightPadding))
		fmt.Println("└─────────────────────────────────────────────────────────────────────┘")
		fmt.Printf("Found %d results\n\n", result.Count)

		fmt.Println("┌─────────────────────┬──────────────────────┬──────────────────────┬──────────┬─────────────┐")
		fmt.Println("│ ID                  │ Title                │ Author               │ Status   │ Chapters    │")
		fmt.Println("├─────────────────────┼──────────────────────┼──────────────────────┼──────────┼─────────────┤")

		for _, manga := range result.Mangas {
			title := manga.Title
			if len(title) > 20 {
				title = title[:17] + "..."
			}

			author := ""
			if len(manga.Authors) > 0 {
				first := manga.Authors[0].Node.FirstName
				last := manga.Authors[0].Node.LastName
				fullName := strings.TrimSpace(first + " " + last)
				if fullName != "" {
					if len(fullName) > 20 {
						fullName = fullName[:17] + "..."
					}
					author = fullName
				}
			}

			status := manga.Status
			if status == "" {
				status = "?"
			}
			if len(status) > 8 {
				status = status[:5] + "..."
			}

			chapters := "?"
			if manga.NumChapters > 0 {
				chapters = fmt.Sprintf("%d", manga.NumChapters)
			}

			   fmt.Printf("│ %-19d │ %-20s │ %-20s │ %-8s │ %-11s │\n",
				   manga.ID, title, author, status, chapters)
		}

		fmt.Println("└─────────────────────┴──────────────────────┴──────────────────────┴──────────┴─────────────┘")
		fmt.Println("\nUse 'mangahub manga info <id>' to view details")
		fmt.Println("Use 'mangahub library add --manga-id <id>' to add to your library")
		fmt.Println()

		return nil
	},
}

func init() {
	mangaSearchCmd.Flags().StringVar(&searchGenre, "genre", "", "Filter by genre (e.g., Action, Romance, Comedy)")
	mangaSearchCmd.Flags().StringVar(&searchStatus, "status", "", "Filter by status (ongoing, completed, finished)")
	mangaSearchCmd.Flags().IntVar(&searchLimit, "limit", 100, "Maximum number of results (max 100)")

	mangaCmd.AddCommand(mangaSearchCmd)
	mangaCmd.AddCommand(mangaInfoCmd)
	mangaCmd.AddCommand(mangaListCmd)
	mangaCmd.AddCommand(mangaFeaturedCmd)
	mangaCmd.AddCommand(mangaRankingCmd)

	// Flags for ranking command
	mangaRankingCmd.Flags().IntVar(&rankingLimit, "limit", 100, "Maximum number of results (max 100)")
}
