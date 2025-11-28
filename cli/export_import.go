package cli

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportOutput string
	importFormat string
	importInput  string
	batchStatus  string
	batchFile    string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data",
	Long:  `Export your library or other data to a file.`,
}

var exportLibraryCmd = &cobra.Command{
	Use:   "library",
	Short: "Export library",
	Long:  `Export your manga library to JSON or CSV format.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		if cfg.User.Token == "" {
			return fmt.Errorf("authentication required")
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		// Fetch library
		req, _ := http.NewRequest("GET", serverURL+"/users/library", nil)
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to fetch library: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)

		var library []struct {
			MangaID    string `json:"manga_id"`
			Title      string `json:"title"`
			Status     string `json:"status"`
			IsFavorite bool   `json:"is_favorite"`
		}
		json.Unmarshal(body, &library)

		// Format output
		var outputData []byte
		switch strings.ToLower(exportFormat) {
		case "json":
			outputData, _ = json.MarshalIndent(library, "", "  ")
		case "csv":
			var buf bytes.Buffer
			w := csv.NewWriter(&buf)
			w.Write([]string{"MangaID", "Title", "Status", "IsFavorite"})
			for _, item := range library {
				w.Write([]string{item.MangaID, item.Title, item.Status, fmt.Sprintf("%v", item.IsFavorite)})
			}
			w.Flush()
			outputData = buf.Bytes()
		default:
			return fmt.Errorf("unsupported format: %s", exportFormat)
		}

		// Write to file or stdout
		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, outputData, 0644); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			printSuccess(fmt.Sprintf("Library exported to %s", exportOutput))
		} else {
			fmt.Println(string(outputData))
		}

		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import data",
	Long:  `Import data from external sources (e.g., MyAnimeList export).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if importInput == "" {
			return fmt.Errorf("input file is required (--input)")
		}

		data, err := os.ReadFile(importInput)
		if err != nil {
			return fmt.Errorf("failed to read input file: %w", err)
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		var entries []map[string]interface{}

		switch strings.ToLower(importFormat) {
		case "mal":
			// Simple XML parsing for MAL export
			type Manga struct {
				ID     string `xml:"manga_mangadb_id"`
				Title  string `xml:"manga_title"`
				Status string `xml:"my_status"`
			}
			type MyAnimeList struct {
				Manga []Manga `xml:"manga"`
			}
			var mal MyAnimeList
			if err := xml.Unmarshal(data, &mal); err != nil {
				return fmt.Errorf("failed to parse MAL XML: %w", err)
			}

			for _, m := range mal.Manga {
				status := "plan_to_read"
				switch m.Status {
				case "Reading":
					status = "reading"
				case "Completed":
					status = "completed"
				case "On-Hold":
					status = "on_hold"
				case "Dropped":
					status = "dropped"
				}

				entries = append(entries, map[string]interface{}{
					"manga_id": m.ID, // Note: This assumes ID mapping matches, which might not be true for real MAL IDs vs internal IDs
					"status":   status,
				})
			}
		default:
			return fmt.Errorf("unsupported format: %s", importFormat)
		}

		// Send batch import
		jsonData, _ := json.Marshal(map[string]interface{}{"entries": entries})
		req, _ := http.NewRequest("POST", serverURL+"/users/library/import", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to import: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("import failed with status: %d", resp.StatusCode)
		}

		printSuccess(fmt.Sprintf("Imported %d entries successfully", len(entries)))
		return nil
	},
}

var libraryBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update",
	Short: "Batch update library",
	Long:  `Update status for multiple manga from a file list.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if batchFile == "" {
			return fmt.Errorf("file is required (--file)")
		}

		file, err := os.Open(batchFile)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer file.Close()

		var mangaIDs []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				mangaIDs = append(mangaIDs, line)
			}
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		serverURL, err := config.GetServerURL()
		if err != nil {
			return err
		}

		// Construct batch update payload
		payload := map[string]interface{}{
			"manga_ids": mangaIDs,
			"status":    batchStatus,
		}
		jsonData, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", serverURL+"/users/library/batch", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.User.Token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to batch update: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("batch update failed with status: %d", resp.StatusCode)
		}

		printSuccess(fmt.Sprintf("Updated %d manga to status '%s'", len(mangaIDs), batchStatus))
		return nil
	},
}

func init() {
	exportLibraryCmd.Flags().StringVar(&exportFormat, "format", "json", "Output format (json, csv)")
	exportLibraryCmd.Flags().StringVar(&exportOutput, "output", "", "Output file path")
	exportCmd.AddCommand(exportLibraryCmd)

	importCmd.Flags().StringVar(&importFormat, "format", "mal", "Input format (mal)")
	importCmd.Flags().StringVar(&importInput, "input", "", "Input file path")
	importCmd.MarkFlagRequired("input")

	libraryBatchUpdateCmd.Flags().StringVar(&batchStatus, "status", "plan_to_read", "New status for all manga")
	libraryBatchUpdateCmd.Flags().StringVar(&batchFile, "file", "", "File containing manga IDs (one per line)")
	libraryBatchUpdateCmd.MarkFlagRequired("file")

	// Note: libraryBatchUpdateCmd needs to be added to libraryCmd in root.go or library.go
	// Since we are in a separate file, we'll export it or add it in init() if libraryCmd was accessible.
	// However, libraryCmd is in library.go. We will register this in root.go or we need to export libraryCmd.
	// For now, we'll assume we register these top-level commands in root.go
}
