package cli

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "System information",
	Long:  `Display system information and diagnostics.`,
}

var systemInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show system info",
	Long:  `Display detailed system information including OS, architecture, and server status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("System Information:")
		fmt.Println("-------------------")
		fmt.Printf("OS: %s\n", runtime.GOOS)
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("CPUs: %d\n", runtime.NumCPU())

		cfg, err := config.Load()
		if err != nil {
			fmt.Println("\nConfiguration: Not initialized")
		} else {
			fmt.Println("\nConfiguration:")
			fmt.Printf("  Config Path: %s\n", cfg.Logging.Path) // Using logging path as proxy for config location
			fmt.Printf("  Server Host: %s\n", cfg.Server.Host)
			fmt.Printf("  HTTP Port: %d\n", cfg.Server.HTTPPort)
		}

		fmt.Println("\nServer Connectivity:")
		serverURL, err := config.GetServerURL()
		if err == nil {
			client := http.Client{
				Timeout: 2 * time.Second,
			}
			resp, err := client.Get(serverURL + "/health")
			if err != nil {
				fmt.Printf("  Status: ✗ Unreachable (%s)\n", err.Error())
			} else {
				defer resp.Body.Close()
				if resp.StatusCode == 200 {
					fmt.Printf("  Status: ✓ Online (HTTP %d)\n", resp.StatusCode)
				} else {
					fmt.Printf("  Status: ⚠ Issues (HTTP %d)\n", resp.StatusCode)
				}
			}
		} else {
			fmt.Println("  Status: Unknown (Config error)")
		}

		return nil
	},
}

func init() {
	systemCmd.AddCommand(systemInfoCmd)
}
