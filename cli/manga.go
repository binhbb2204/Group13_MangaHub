package cli

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"

    "github.com/binhbb2204/Manga-Hub-Group13/cli/config"
    "github.com/spf13/cobra"
)

var mangaCmd = &cobra.Command{
    Use:   "manga",
    Short: "Manga management commands",
    Long:  `Search and manage manga information.`,
}

var mangaSearchCmd = &cobra.Command{
    Use:   "search [query]",
    Short: "Search for manga",
    Long:  `Search for manga by title, author, or genre.`,
    Args:  cobra.MinimumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        query := args[0]

        serverURL, err := config.GetServerURL()
        if err != nil {
            printError("Configuration not initialized")
            fmt.Println("Run: mangahub init")
            return err
        }

        //Build URL with query parameter
        searchURL := fmt.Sprintf("%s/manga?q=%s", serverURL, url.QueryEscape(query))

        resp, err := http.Get(searchURL)
        if err != nil {
            printError("Search failed: Server connection error")
            fmt.Println("Check server status: mangahub server status")
            return err
        }
        defer resp.Body.Close()

        body, _ := io.ReadAll(resp.Body)

        if resp.StatusCode != http.StatusOK {
            var errResp map[string]string
            json.Unmarshal(body, &errResp)
            printError(fmt.Sprintf("Search failed: %s", errResp["error"]))
            return fmt.Errorf("search failed")
        }

        var mangaList []struct {
            ID          string   `json:"id"`
            Title       string   `json:"title"`
            Author      string   `json:"author"`
            Genres      []string `json:"genres"`
            Status      string   `json:"status"`
            Description string   `json:"description"`
        }
        json.Unmarshal(body, &mangaList)

        if len(mangaList) == 0 {
            fmt.Printf("No manga found for query: %s\n", query)
            return nil
        }

        fmt.Printf("Found %d manga(s):\n\n", len(mangaList))
        for i, manga := range mangaList {
            fmt.Printf("%d. %s\n", i+1, manga.Title)
            fmt.Printf("   ID: %s\n", manga.ID)
            fmt.Printf("   Author: %s\n", manga.Author)
            fmt.Printf("   Genres: %v\n", manga.Genres)
            fmt.Printf("   Status: %s\n", manga.Status)
            if manga.Description != "" {
                desc := manga.Description
                if len(desc) > 100 {
                    desc = desc[:100] + "..."
                }
                fmt.Printf("   Description: %s\n", desc)
            }
            fmt.Println()
        }

        fmt.Println("To add to library:")
        fmt.Println("  mangahub library add --manga-id <manga-id> --status reading")

        return nil
    },
}

func init() {
    mangaCmd.AddCommand(mangaSearchCmd)
}