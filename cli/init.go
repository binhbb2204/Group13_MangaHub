package cli

import (
	"fmt"
	"path/filepath"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize MangaHub CLI configuration",
	Long:  `Initialize MangaHub configuration and create necessary directories.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := config.GetConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}

		fmt.Println("Initializing MangaHub...")
		fmt.Printf("Config directory: %s\n", configDir)

		if err := config.Init(); err != nil {
			return fmt.Errorf("failed to initialize config: %w", err)
		}

		printSuccess("Configuration initialized successfully!")
		fmt.Println("\nCreated:")
		fmt.Printf("  - %s\n", filepath.Join(configDir, "config.yaml"))
		fmt.Printf("  - %s\n", filepath.Join(configDir, "data.db"))
		fmt.Printf("  - %s\n", filepath.Join(configDir, "logs/"))
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Start the server: mangahub server start")
		fmt.Println("  2. Register an account: mangahub auth register --username <username> --email <email>")

		return nil
	},
}
