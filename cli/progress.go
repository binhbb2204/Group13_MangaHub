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
	progressMangaID string
	chapter         int
	volume          int
)

var progressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Manage reading progress",
	Long:  `Update and view your reading progress for manga.`,
}

var progressUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update reading progress",
	Long:  `Update your current reading progress for a manga.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if progressMangaID == "" {
			return fmt.Errorf("manga ID is required (--manga-id)")
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
			"manga_id":        progressMangaID,
			"current_chapter": chapter,
		}
		if volume > 0 {
			reqBody["current_volume"] = volume
		}
		jsonData, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", serverURL+"/users/progress", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			printError("Failed to update progress: Server connection error")
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			var errResp map[string]string
			json.Unmarshal(body, &errResp)
			printError(fmt.Sprintf("Failed to update progress: %s", errResp["error"]))
			return fmt.Errorf("failed to update progress")
		}

		printSuccess("Reading progress updated!")
		fmt.Printf("Manga ID: %s\n", progressMangaID)
		if volume > 0 {
			fmt.Printf("Progress: Volume %d, Chapter %d\n", volume, chapter)
		} else {
			fmt.Printf("Progress: Chapter %d\n", chapter)
		}
		fmt.Println("\nView your library:")
		fmt.Println("  mangahub library list")

		return nil
	},
}

var progressViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View reading progress",
	Long:  `View your reading progress for a specific manga.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if progressMangaID == "" {
			return fmt.Errorf("manga ID is required (--manga-id)")
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

		url := fmt.Sprintf("%s/users/progress/%s", serverURL, progressMangaID)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			printError("Failed to get progress: Server connection error")
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			var errResp map[string]string
			json.Unmarshal(body, &errResp)
			printError(fmt.Sprintf("Failed to get progress: %s", errResp["error"]))
			return fmt.Errorf("failed to get progress")
		}

		var progress struct {
			MangaID        string `json:"manga_id"`
			CurrentChapter int    `json:"current_chapter"`
			CurrentVolume  int    `json:"current_volume"`
			LastReadAt     string `json:"last_read_at"`
		}
		json.Unmarshal(body, &progress)

		fmt.Printf("Reading Progress for %s:\n", progressMangaID)
		if progress.CurrentVolume > 0 {
			fmt.Printf("  Volume: %d\n", progress.CurrentVolume)
		}
		fmt.Printf("  Chapter: %d\n", progress.CurrentChapter)
		if progress.LastReadAt != "" {
			fmt.Printf("  Last read: %s\n", progress.LastReadAt)
		}

		return nil
	},
}

func init() {
	progressUpdateCmd.Flags().StringVar(&progressMangaID, "manga-id", "", "Manga ID")
	progressUpdateCmd.Flags().IntVar(&chapter, "chapter", 0, "Current chapter number")
	progressUpdateCmd.Flags().IntVar(&volume, "volume", 0, "Current volume number (optional)")
	progressUpdateCmd.MarkFlagRequired("manga-id")
	progressUpdateCmd.MarkFlagRequired("chapter")

	progressViewCmd.Flags().StringVar(&progressMangaID, "manga-id", "", "Manga ID")
	progressViewCmd.MarkFlagRequired("manga-id")

	progressCmd.AddCommand(progressUpdateCmd)
	progressCmd.AddCommand(progressViewCmd)
}
