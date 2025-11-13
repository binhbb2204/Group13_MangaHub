package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/spf13/cobra"
)

var (
	eventTypes []string
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "UDP notification commands",
	Long:  `Manage UDP notifications for chapter releases and library updates.`,
}

var notifySubscribeCmd = &cobra.Command{
	Use:   "subscribe",
	Short: "Subscribe to chapter release notifications",
	Long:  `Establish a UDP connection to receive real-time notifications for chapter releases and library updates.`,
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

		if !cfg.Notifications.Enabled {
			printError("Notifications are disabled in configuration")
			fmt.Println("Run: mangahub notify preferences --enable")
			return fmt.Errorf("notifications disabled")
		}

		types := eventTypes
		if len(types) == 0 {
			types = []string{"progress_update", "library_update"}
		}

		serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.UDPPort))
		fmt.Printf("Connecting to UDP notification server at %s...\n", serverAddr)

		conn, err := net.Dial("udp", serverAddr)
		if err != nil {
			printError(fmt.Sprintf("Failed to connect: %s", err.Error()))
			return err
		}
		defer conn.Close()

		registerMsg := udp.CreateRegisterMessage(cfg.User.Token)
		if _, err := conn.Write(registerMsg); err != nil {
			printError("Failed to register")
			return err
		}

		buffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			printError("Failed to receive registration response")
			return err
		}

		var response udp.Message
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			printError("Invalid response from server")
			return err
		}

		if response.Type == "error" {
			printError("Registration failed")
			return fmt.Errorf("server error")
		}

		subscribeMsg := udp.CreateSubscribeMessage(types)
		if _, err := conn.Write(subscribeMsg); err != nil {
			printError("Failed to subscribe")
			return err
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err = conn.Read(buffer)
		if err != nil {
			printError("Failed to receive subscription response")
			return err
		}

		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			printError("Invalid response from server")
			return err
		}

		if response.Type == "error" {
			printError("Subscription failed")
			return fmt.Errorf("server error")
		}

		printSuccess("Subscribed to notifications")
		fmt.Printf("Event types: %v\n", types)
		fmt.Println("\nListening for notifications... (Press Ctrl+C to stop)")

		for {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					heartbeatMsg := udp.CreateHeartbeatMessage("")
					conn.Write(heartbeatMsg)
					continue
				}
				printError("Connection lost")
				return err
			}

			var msg udp.Message
			if err := json.Unmarshal(buffer[:n], &msg); err != nil {
				continue
			}

			if msg.Type == "notification" {
				fmt.Printf("\n[%s] %s notification received\n", time.Now().Format("15:04:05"), msg.EventType)
				if len(msg.Data) > 0 {
					var data map[string]interface{}
					json.Unmarshal(msg.Data, &data)
					if mangaID, ok := data["manga_id"].(string); ok {
						fmt.Printf("  Manga ID: %s\n", mangaID)
					}
					if action, ok := data["action"].(string); ok {
						fmt.Printf("  Action: %s\n", action)
					}
				}
			}
		}
	},
}

var notifyUnsubscribeCmd = &cobra.Command{
	Use:   "unsubscribe",
	Short: "Unsubscribe from notifications",
	Long:  `Stop receiving UDP notifications and disconnect from the notification server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			return err
		}

		if cfg.User.Token == "" {
			printError("Not logged in")
			return fmt.Errorf("authentication required")
		}

		serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.UDPPort))
		conn, err := net.Dial("udp", serverAddr)
		if err != nil {
			printError(fmt.Sprintf("Failed to connect: %s", err.Error()))
			return err
		}
		defer conn.Close()

		registerMsg := udp.CreateRegisterMessage(cfg.User.Token)
		if _, err := conn.Write(registerMsg); err != nil {
			printError("Failed to register")
			return err
		}

		buffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			printError("Failed to receive registration response")
			return err
		}

		var response udp.Message
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			printError("Invalid response from server")
			return err
		}

		if response.Type == "error" {
			printError("Registration failed")
			return fmt.Errorf("server error")
		}

		unregisterMsg := udp.CreateUnregisterMessage()
		if _, err := conn.Write(unregisterMsg); err != nil {
			printError("Failed to unsubscribe")
			return err
		}

		printSuccess("Unsubscribed from notifications")
		return nil
	},
}

var notifyPreferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "View notification preferences",
	Long:  `Display current notification settings and configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			return err
		}

		fmt.Println("Notification Preferences:")
		fmt.Printf("  Enabled: %v\n", cfg.Notifications.Enabled)
		fmt.Printf("  Sound: %v\n", cfg.Notifications.Sound)
		fmt.Printf("  UDP Port: %d\n", cfg.Server.UDPPort)
		fmt.Printf("  Server: %s\n", cfg.Server.Host)
		return nil
	},
}

var notifyTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test notification system",
	Long:  `Send a test notification to verify the UDP notification system is working.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			printError("Configuration not initialized")
			return err
		}

		if cfg.User.Token == "" {
			printError("Not logged in")
			return fmt.Errorf("authentication required")
		}

		serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.UDPPort))
		fmt.Printf("Testing UDP connection to %s...\n", serverAddr)

		conn, err := net.Dial("udp", serverAddr)
		if err != nil {
			printError(fmt.Sprintf("Connection failed: %s", err.Error()))
			return err
		}
		defer conn.Close()

		registerMsg := udp.CreateRegisterMessage(cfg.User.Token)
		if _, err := conn.Write(registerMsg); err != nil {
			printError("Failed to send test message")
			return err
		}

		buffer := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			printError("No response from server")
			return err
		}

		var response udp.Message
		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			printError("Invalid response from server")
			return err
		}

		if response.Type == "error" {
			printError("Test failed")
			return fmt.Errorf("server error")
		}

		printSuccess("UDP notification system is working")
		fmt.Printf("Server responded: %s\n", response.Type)
		return nil
	},
}

func init() {
	notifyCmd.AddCommand(notifySubscribeCmd)
	notifyCmd.AddCommand(notifyUnsubscribeCmd)
	notifyCmd.AddCommand(notifyPreferencesCmd)
	notifyCmd.AddCommand(notifyTestCmd)

	notifySubscribeCmd.Flags().StringSliceVar(&eventTypes, "events", []string{}, "event types to subscribe to (progress_update, library_update)")
}
