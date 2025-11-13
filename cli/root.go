package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:   "mangahub-cli",
	Short: "MangaHub - Manga Library Management System",
	Long: `MangaHub is a CLI tool for managing your manga library.
It provides commands for authentication, manga search, library management,
and reading progress tracking.`,
	Version: "1.0.0",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-error output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(mangaCmd)
	rootCmd.AddCommand(libraryCmd)
	rootCmd.AddCommand(progressCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(notifyCmd)

}

func Execute() error {
	return rootCmd.Execute()
}

func printSuccess(message string) {
	if !quiet {
		fmt.Printf("✓ %s\n", message)
	}
}

func printError(message string) {
	fmt.Printf("✗ %s\n", message)
}

func printInfo(message string) {
	if !quiet {
		fmt.Println(message)
	}
}
