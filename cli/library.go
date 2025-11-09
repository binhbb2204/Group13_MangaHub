package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var (
	mangaID      string
	mangaStatus  string
	favoriteFlag bool
)

var libraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Manage your manga library",
	Long:  `Add manga to library, view your library, and manage your collection.`,
}

var libraryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add manga to your library",
	Long:  `Add a manga to your personal library with reading status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if mangaID == "" {
			return fmt.Errorf("manga ID is required (--manga-id)")
		}
		if mangaStatus == "" {
			mangaStatus = "plan_to_read"
		}

		// Validate status
		validStatuses := map[string]bool{
			"reading":      true,
			"completed":    true,
			"on_hold":      true,
			"dropped":      true,
			"plan_to_read": true,
		}
		if !validStatuses[mangaStatus] {
			return fmt.Errorf("invalid status: %s (use: reading, completed, on_hold, dropped, plan_to_read)", mangaStatus)
		}

		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printError("Not logged in")
			fmt.Println("Run: mangahub auth login --username <username>")
			return fmt.Errorf("authentication required")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		reqBody := map[string]interface{}{
			"manga_id":    mangaID,
			"status":      mangaStatus,
			"is_favorite": favoriteFlag,
		}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", serverURL+"/users/library", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			printError("Failed to add to library: Server connection error")
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			var errResp map[string]string
			json.Unmarshal(body, &errResp)
			printError(fmt.Sprintf("Failed to add to library: %s", errResp["error"]))
			return fmt.Errorf("failed to add to library")
		}

		printSuccess("Manga added to library!")
		fmt.Printf("Manga ID: %s\n", mangaID)
		fmt.Printf("Status: %s\n", mangaStatus)
		if favoriteFlag {
			fmt.Println("Marked as favorite: Yes")
		}
		fmt.Println("\nNext steps:")
		fmt.Println("  View library: mangahub library list")
		fmt.Println("  Update progress: mangahub progress update --manga-id", mangaID, "--chapter <chapter>")

		return nil
	},
}

var libraryListCmd = &cobra.Command{
	Use:   "list",
	Short: "View your manga library",
	Long:  `View all manga in your personal library.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			fmt.Println("Run: mangahub init")
			return err
		}

		if cfg.User.Token == "" {
			printError("Not logged in")
			fmt.Println("Run: mangahub auth login --username <username>")
			return fmt.Errorf("authentication required")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		req, _ := http.NewRequest("GET", serverURL+"/users/library", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			printError("Failed to get library: Server connection error")
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			var errResp map[string]string
			json.Unmarshal(body, &errResp)
			printError(fmt.Sprintf("Failed to get library: %s", errResp["error"]))
			return fmt.Errorf("failed to get library")
		}

		var library []struct {
			MangaID    string `json:"manga_id"`
			Title      string `json:"title"`
			Status     string `json:"status"`
			IsFavorite bool   `json:"is_favorite"`
		}
		json.Unmarshal(body, &library)

		if len(library) == 0 {
			fmt.Println("Your library is empty")
			fmt.Println("\nAdd manga to library:")
			fmt.Println("  mangahub manga search \"one piece\"")
			fmt.Println("  mangahub library add --manga-id <manga-id> --status reading")
			return nil
		}

		fmt.Printf("Your Library (%d manga):\n\n", len(library))
		for i, item := range library {
			fmt.Printf("%d. %s\n", i+1, item.Title)
			fmt.Printf("   ID: %s\n", item.MangaID)
			fmt.Printf("   Status: %s\n", item.Status)
			if item.IsFavorite {
				fmt.Println("   ⭐ Favorite")
			}
			fmt.Println()
		}

		return nil
	},
}

func init() {
	libraryAddCmd.Flags().StringVar(&mangaID, "manga-id", "", "Manga ID to add")
	libraryAddCmd.Flags().StringVar(&mangaStatus, "status", "plan_to_read", "Reading status (reading, completed, on_hold, dropped, plan_to_read)")
	libraryAddCmd.Flags().BoolVar(&favoriteFlag, "favorite", false, "Mark as favorite")
	libraryAddCmd.MarkFlagRequired("manga-id")

	libraryCmd.AddCommand(libraryAddCmd)
	libraryCmd.AddCommand(libraryListCmd)
}
