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
        if searchGenre != "" || searchStatus != "" {
            requestLimit = searchLimit * 3
        }
        if requestLimit > 100 {
            requestLimit = 100 //MAL API max
        }

        searchURL := fmt.Sprintf("%s/manga/search?q=%s", serverURL, url.QueryEscape(query))
        
        if requestLimit > 0 {
            searchURL += fmt.Sprintf("&limit=%d", requestLimit)
        }

        res, err := http.Get(searchURL)
        if err != nil {
            printError("Search failed: Server connection error")
            fmt.Println("Check server status: mangahub server status")
            return err
        }
        defer res.Body.Close()

        body, _ := io.ReadAll(res.Body)

        if res.StatusCode != http.StatusOK{
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

        var result struct{
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

        //Print table header
        fmt.Println("┌─────────────────────┬──────────────────────┬───────────┬──────────┬──────────────┐")
        fmt.Println("│ ID                  │ Title                │ Author    │ Status   │ Chapters     │")
        fmt.Println("├─────────────────────┼──────────────────────┼───────────┼──────────┼──────────────┤")

        //Print manga rows
        for _, manga := range filteredMangas {
            chaptersStr := fmt.Sprintf("%d", manga.TotalChapters)
            if manga.TotalChapters == 0 {
                chaptersStr = "Ongoing"
            }

            fmt.Printf("│ %-19s │ %-20s │ %-9s │ %-8s │ %-12s │\n",
                truncateString(manga.ID, 19),
                truncateString(manga.Title, 20),
                truncateString(manga.Author, 9),
                truncateString(manga.Status, 8),
                chaptersStr)
        }

        fmt.Println("└─────────────────────┴──────────────────────┴───────────┴──────────┴──────────────┘")
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

        res, err := http.Get(fmt.Sprintf("%s/manga/info/%s", serverURL, mangaID))
        if err != nil{
            printError("Failed to get manga info: Server connection error")
            fmt.Println("Check server status: mangahub server status")
            return err
        }
        defer res.Body.Close()

        body, _ := io.ReadAll(res.Body)

        if res.StatusCode == http.StatusNotFound{
            printError(fmt.Sprintf("Manga not found: %s", mangaID))
            fmt.Println("\nTry searching for manga:")
            fmt.Println("  mangahub manga search \"manga title\"")
            return fmt.Errorf("manga not found")
        }

        if res.StatusCode != http.StatusOK{
            var errRes map[string]string
            json.Unmarshal(body, &errRes)
            printError(fmt.Sprintf("Failed to get manga info: %s", errRes["error"]))
            return fmt.Errorf("failed to get manga info")
        }

        var manga struct{
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

        fmt.Println("\n" + strings.Repeat("=", 60))
        fmt.Printf("  %s\n", manga.Title)
        fmt.Println(strings.Repeat("=", 60))
        fmt.Printf("\nID:             %s\n", manga.ID)
        fmt.Printf("Author:         %s\n", manga.Author)
        fmt.Printf("Status:         %s\n", manga.Status)
        
        if manga.TotalChapters > 0{
            fmt.Printf("Total Chapters: %d\n", manga.TotalChapters)
        } else{
            fmt.Printf("Total Chapters: Ongoing\n")
        }
        
        if len(manga.Genres) > 0{
            fmt.Printf("Genres:         %s\n", strings.Join(manga.Genres, ", "))
        }
        
        if manga.CoverURL != ""{
            fmt.Printf("Cover URL:      %s\n", manga.CoverURL)
        }
        
        if manga.Description != ""{
            fmt.Printf("\nDescription:\n%s\n", wrapText(manga.Description, 60))
        }
        
        fmt.Println("\n" + strings.Repeat("-", 60))
        fmt.Println("\nActions:")
        fmt.Printf("  Add to library:     mangahub library add --manga-id %s --status reading\n", manga.ID)
        fmt.Printf("  Update progress:    mangahub progress update --manga-id %s --chapter <num>\n", manga.ID)
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
        if err != nil{
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

        var result struct{
            Mangas []struct{
                ID            string   `json:"id"`
                Title         string   `json:"title"`
                Author        string   `json:"author"`
                Status        string   `json:"status"`
                TotalChapters int      `json:"total_chapters"`
            } `json:"mangas"`
            Count int `json:"count"`
        }
        json.Unmarshal(body, &result)

        if result.Count == 0{
            fmt.Println("\nNo manga found in the database.")
            fmt.Println("\nThe database is empty. Manga can be added by administrators.")
            return nil
        }

        fmt.Printf("\nTotal manga available: %d\n\n", result.Count)

        for i, manga := range result.Mangas{
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

// Helper function to truncate strings
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}

// Helper function to wrap text
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

func init() {
    mangaSearchCmd.Flags().StringVar(&searchGenre, "genre", "", "Filter by genre (e.g., Action, Romance, Comedy)")
    mangaSearchCmd.Flags().StringVar(&searchStatus, "status", "", "Filter by status (ongoing, completed, finished)")
    mangaSearchCmd.Flags().IntVar(&searchLimit, "limit", 20, "Maximum number of results")

    mangaCmd.AddCommand(mangaSearchCmd)
    mangaCmd.AddCommand(mangaInfoCmd)
    mangaCmd.AddCommand(mangaListCmd)
}