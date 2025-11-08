package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// getServerPort reads the port from .env file or returns default
func getServerPort() string {
	// Try to load .env file (ignore errors if it doesn't exist)
	godotenv.Load()

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080" // default port
	}
	return port
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage MangaHub server",
	Long:  `Start, stop, or check status of the MangaHub server.`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MangaHub server",
	Long:  `Start the MangaHub HTTP API server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting MangaHub server...")

		projectDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		apiServerPath := filepath.Join(projectDir, "cmd", "api-server", "main.go")
		if _, err := os.Stat(apiServerPath); os.IsNotExist(err) {
			return fmt.Errorf("API server not found at %s", apiServerPath)
		}

		goCmd := exec.Command("go", "run", apiServerPath)
		goCmd.Stdout = os.Stdout
		goCmd.Stderr = os.Stderr
		goCmd.Dir = projectDir

		if err := goCmd.Start(); err != nil {
			return fmt.Errorf("failed to start server: %w", err)
		}

		port := getServerPort()
		printSuccess("Server started successfully!")
		fmt.Printf("Server is running on http://localhost:%s\n", port)
		fmt.Println("Press Ctrl+C to stop the server")

		return goCmd.Wait()
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check server status",
	Long:  `Check if the MangaHub server is running.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking server status...")

		port := getServerPort()
		healthURL := fmt.Sprintf("http://localhost:%s/health", port)

		var checkCmd *exec.Cmd
		if runtime.GOOS == "windows" {
			checkCmd = exec.Command("powershell", "-Command",
				fmt.Sprintf("(Invoke-WebRequest -Uri %s -UseBasicParsing -TimeoutSec 2).StatusCode", healthURL))
		} else {
			checkCmd = exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", healthURL)
		}

		output, err := checkCmd.Output()
		if err != nil || string(output) != "200" {
			printError("Server is not running")
			fmt.Println("Start the server with: mangahub server start")
			return nil
		}

		printSuccess("Server is running")
		fmt.Printf("Server URL: http://localhost:%s\n", port)
		return nil
	},
}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStatusCmd)
}
