package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Manage logs",
	Long:  `View, search, and manage MangaHub CLI logs.`,
}

var logsErrorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Show error logs",
	Long:  `Display error messages from the log files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		logDir := cfg.Logging.Path
		files, err := os.ReadDir(logDir)
		if err != nil {
			return fmt.Errorf("failed to read log directory: %w", err)
		}

		fmt.Println("Error Logs:")
		fmt.Println("-----------")

		found := false
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
				path := filepath.Join(logDir, file.Name())
				f, err := os.Open(path)
				if err != nil {
					continue
				}
				defer f.Close()

				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "level=error") {
						fmt.Printf("[%s] %s\n", file.Name(), line)
						found = true
					}
				}
			}
		}

		if !found {
			fmt.Println("No errors found in logs.")
		}

		return nil
	},
}

var logsSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search logs",
	Long:  `Search for a specific string in the log files.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.ToLower(args[0])
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		logDir := cfg.Logging.Path
		files, err := os.ReadDir(logDir)
		if err != nil {
			return fmt.Errorf("failed to read log directory: %w", err)
		}

		fmt.Printf("Searching for \"%s\" in logs...\n", query)
		fmt.Println("-----------------------------------")

		found := false
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
				path := filepath.Join(logDir, file.Name())
				f, err := os.Open(path)
				if err != nil {
					continue
				}
				defer f.Close()

				scanner := bufio.NewScanner(f)
				lineNum := 0
				for scanner.Scan() {
					lineNum++
					line := scanner.Text()
					if strings.Contains(strings.ToLower(line), query) {
						fmt.Printf("[%s:%d] %s\n", file.Name(), lineNum, line)
						found = true
					}
				}
			}
		}

		if !found {
			fmt.Println("No matches found.")
		}

		return nil
	},
}

var logsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean old logs",
	Long:  `Delete all log files in the log directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		logDir := cfg.Logging.Path
		files, err := os.ReadDir(logDir)
		if err != nil {
			return fmt.Errorf("failed to read log directory: %w", err)
		}

		count := 0
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
				path := filepath.Join(logDir, file.Name())
				if err := os.Remove(path); err == nil {
					count++
				}
			}
		}

		printSuccess(fmt.Sprintf("Deleted %d log files", count))
		return nil
	},
}

var logsRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate logs",
	Long:  `Archive current logs and start fresh.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		logDir := cfg.Logging.Path
		files, err := os.ReadDir(logDir)
		if err != nil {
			return fmt.Errorf("failed to read log directory: %w", err)
		}

		timestamp := time.Now().Format("20060102-150405")
		count := 0

		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") && !strings.Contains(file.Name(), "archive") {
				oldPath := filepath.Join(logDir, file.Name())
				newPath := filepath.Join(logDir, fmt.Sprintf("%s.archive.%s.log", strings.TrimSuffix(file.Name(), ".log"), timestamp))

				if err := os.Rename(oldPath, newPath); err == nil {
					count++
				}
			}
		}

		printSuccess(fmt.Sprintf("Rotated %d log files", count))
		return nil
	},
}

func init() {
	logsCmd.AddCommand(logsErrorsCmd)
	logsCmd.AddCommand(logsSearchCmd)
	logsCmd.AddCommand(logsCleanCmd)
	logsCmd.AddCommand(logsRotateCmd)
}
