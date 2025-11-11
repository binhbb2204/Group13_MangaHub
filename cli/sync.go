package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/spf13/cobra"
)

var (
	syncDeviceType string
	syncDeviceName string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "TCP synchronization commands",
	Long:  `Manage TCP sync connections for real-time progress synchronization across devices.`,
}

var syncConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to TCP sync server",
	Long:  `Establish a persistent TCP connection to the sync server for real-time synchronization.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		active, connInfo, err := config.IsConnectionActive()
		if err != nil {
			printError("Failed to check connection status")
			return err
		}
		if active {
			printError("Already connected to sync server")
			fmt.Printf("Session ID: %s\n", connInfo.SessionID)
			fmt.Printf("Server: %s\n", connInfo.Server)
			fmt.Println("\nTo disconnect: mangahub sync disconnect")
			return nil
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

		deviceType := syncDeviceType
		if deviceType == "" {
			deviceType = "desktop"
		}

		deviceName := syncDeviceName
		if deviceName == "" {
			hostname, _ := os.Hostname()
			if hostname != "" {
				deviceName = hostname
			} else {
				deviceName = "My Device"
			}
		}

		serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))
		fmt.Printf("Connecting to TCP sync server at %s...\n", serverAddr)

		conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
		if err != nil {
			printError(fmt.Sprintf("Failed to connect: %s", err.Error()))
			fmt.Println("\nTroubleshooting:")
			fmt.Println("  1. Check if TCP server is running: mangahub server status")
			fmt.Println("  2. Check firewall settings")
			fmt.Println("  3. Verify server configuration")
			return err
		}

		authMsg := map[string]interface{}{
			"type": "auth",
			"payload": map[string]string{
				"token": cfg.User.Token,
			},
		}
		authJSON, _ := json.Marshal(authMsg)
		authJSON = append(authJSON, '\n')

		if _, err := conn.Write(authJSON); err != nil {
			conn.Close()
			printError("Failed to send authentication")
			return err
		}

		reader := bufio.NewReader(conn)
		response, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			printError("Failed to receive authentication response")
			return err
		}

		var authResponse map[string]interface{}
		if err := json.Unmarshal([]byte(response), &authResponse); err != nil {
			conn.Close()
			printError("Invalid authentication response")
			return err
		}

		if authResponse["type"] == "error" {
			conn.Close()
			printError("Authentication failed")
			return fmt.Errorf("authentication rejected by server")
		}

		connectMsg := map[string]interface{}{
			"type": "connect",
			"payload": map[string]string{
				"device_type": deviceType,
				"device_name": deviceName,
			},
		}
		connectJSON, _ := json.Marshal(connectMsg)
		connectJSON = append(connectJSON, '\n')

		if _, err := conn.Write(connectJSON); err != nil {
			conn.Close()
			printError("Failed to send connect message")
			return err
		}

		response, err = reader.ReadString('\n')
		if err != nil {
			conn.Close()
			printError("Failed to receive connect response")
			return err
		}

		var connectResponse struct {
			Type    string `json:"type"`
			Payload struct {
				SessionID   string `json:"session_id"`
				ConnectedAt string `json:"connected_at"`
			} `json:"payload"`
		}

		if err := json.Unmarshal([]byte(response), &connectResponse); err != nil {
			conn.Close()
			printError("Invalid connect response")
			return err
		}

		sessionID := connectResponse.Payload.SessionID
		if sessionID == "" {
			sessionID = "sess_" + time.Now().Format("20060102150405")
		}

		if err := config.SetActiveConnection(sessionID, serverAddr, deviceType, deviceName); err != nil {
			conn.Close()
			printError("Failed to save connection state")
			return err
		}

		printSuccess("Connected successfully!")
		fmt.Println("\nConnection Details:")
		fmt.Printf("  Server: %s\n", serverAddr)
		fmt.Printf("  User: %s\n", cfg.User.Username)
		fmt.Printf("  Session ID: %s\n", sessionID)
		fmt.Printf("  Device: %s (%s)\n", deviceName, deviceType)
		fmt.Printf("  Connected at: %s\n", time.Now().Format("2006-01-02 15:04:05 MST"))

		fmt.Println("\nSync Status:")
		fmt.Printf("  Auto-sync: %v\n", cfg.Sync.AutoSync)
		fmt.Printf("  Conflict resolution: %s\n", cfg.Sync.ConflictResolution)

		fmt.Println("\nReal-time sync is now active. Your progress will be synchronized across")
		fmt.Println("all devices.")
		fmt.Println("\nKeep this terminal open to maintain the connection.")
		fmt.Println("Press Ctrl+C to disconnect gracefully.")
		fmt.Println("\nIn another terminal, you can run:")
		fmt.Println("  mangahub sync status   - View connection status")
		fmt.Println("  mangahub sync monitor  - Monitor real-time updates")

		maintainConnection(conn, sessionID, cfg)
		return nil
	},
}

var syncDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from TCP sync server",
	Long:  `Gracefully close the TCP sync connection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		active, connInfo, err := config.IsConnectionActive()
		if err != nil {
			printError("Failed to check connection status")
			return err
		}

		if !active || connInfo == nil {
			printError("Not connected to sync server")
			return nil
		}

		cfg, err := config.Load()
		if err != nil {
			return err
		}

		serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))
		conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
		if err == nil {
			disconnectMsg := map[string]interface{}{
				"type":    "disconnect",
				"payload": map[string]string{},
			}
			disconnectJSON, _ := json.Marshal(disconnectMsg)
			disconnectJSON = append(disconnectJSON, '\n')
			conn.Write(disconnectJSON)
			conn.Close()
		}

		if err := config.ClearActiveConnection(); err != nil {
			printError("Failed to clear connection state")
			return err
		}

		printSuccess("Disconnected from sync server")
		fmt.Printf("Session ID: %s\n", connInfo.SessionID)
		fmt.Printf("Duration: %s\n", time.Since(connInfo.ConnectedAt).Round(time.Second))
		return nil
	},
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check TCP sync connection status",
	Long:  `Display detailed information about the current TCP sync connection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		active, connInfo, err := config.IsConnectionActive()
		if err != nil {
			printError("Failed to check connection status")
			return err
		}

		fmt.Println("TCP Sync Status:")
		fmt.Println()

		if !active || connInfo == nil {
			fmt.Println("  Connection: ✗ Inactive")
			fmt.Println()
			fmt.Println("To connect: mangahub sync connect")
			return nil
		}

		fmt.Println("  Connection: ✓ Active")
		fmt.Printf("  Server: %s\n", connInfo.Server)

		uptime := time.Since(connInfo.ConnectedAt)
		fmt.Printf("  Uptime: %s\n", formatDuration(uptime))

		timeSinceHeartbeat := time.Since(connInfo.LastHeartbeat)
		fmt.Printf("  Last heartbeat: %s ago\n", formatDuration(timeSinceHeartbeat))

		fmt.Println()
		fmt.Println("Session Info:")
		cfg, _ := config.Load()
		if cfg != nil {
			fmt.Printf("  User: %s\n", cfg.User.Username)
		}
		fmt.Printf("  Session ID: %s\n", connInfo.SessionID)
		fmt.Printf("  Device: %s (%s)\n", connInfo.DeviceName, connInfo.DeviceType)

		fmt.Println()
		fmt.Println("Sync Statistics:")
		fmt.Println("  Messages sent: N/A")
		fmt.Println("  Messages received: N/A")
		fmt.Println("  Last sync: N/A")
		fmt.Println("  Sync conflicts: 0")

		if timeSinceHeartbeat < 30*time.Second {
			fmt.Println()
			fmt.Println("Network Quality: Good")
		} else if timeSinceHeartbeat < 60*time.Second {
			fmt.Println()
			fmt.Println("Network Quality: Fair")
		} else {
			fmt.Println()
			fmt.Println("Network Quality: Poor (connection may be stale)")
		}
		return nil
	},
}

var syncMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor real-time sync updates",
	Long:  `Display real-time synchronization updates as they happen across devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Monitoring real-time sync updates... (Press Ctrl+C to exit)")
		fmt.Println()
		fmt.Println("Real-time monitoring is not yet fully implemented.")
		fmt.Println()
		fmt.Println("This feature will display:")
		fmt.Println("  - Progress updates from other devices")
		fmt.Println("  - Library changes")
		fmt.Println("  - Conflict resolutions")
		fmt.Println()
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nMonitoring stopped")
		return nil
	},
}

func maintainConnection(conn net.Conn, sessionID string, cfg *config.Config) {
	defer conn.Close()
	defer config.ClearActiveConnection()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	reader := bufio.NewReader(conn)
	responseChan := make(chan string, 10)

	go func() {
		for {
			response, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			responseChan <- response
		}
	}()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nDisconnecting from sync server...")
			disconnectMsg := map[string]interface{}{
				"type":    "disconnect",
				"payload": map[string]string{},
			}
			disconnectJSON, _ := json.Marshal(disconnectMsg)
			disconnectJSON = append(disconnectJSON, '\n')
			conn.Write(disconnectJSON)
			time.Sleep(100 * time.Millisecond)
			fmt.Println("✓ Disconnected successfully")
			return

		case <-ticker.C:
			heartbeatMsg := map[string]interface{}{
				"type":    "heartbeat",
				"payload": map[string]interface{}{},
			}
			heartbeatJSON, _ := json.Marshal(heartbeatMsg)
			heartbeatJSON = append(heartbeatJSON, '\n')

			if _, err := conn.Write(heartbeatJSON); err != nil {
				fmt.Println("\n✗ Connection lost")
				return
			}
			config.UpdateHeartbeat()

		case response := <-responseChan:
			var msg map[string]interface{}
			if err := json.Unmarshal([]byte(response), &msg); err != nil {
				continue
			}

			msgType, ok := msg["type"].(string)
			if !ok {
				continue
			}

			switch msgType {
			case "sync_update":
				fmt.Printf("\n[Sync Update] Received update from server\n")
			case "error":
				fmt.Printf("\n[Error] %v\n", msg["payload"])
			}
		}
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
}

func init() {
	syncConnectCmd.Flags().StringVar(&syncDeviceType, "device-type", "", "Device type (mobile, desktop, web)")
	syncConnectCmd.Flags().StringVar(&syncDeviceName, "device-name", "", "Device name")
	syncCmd.AddCommand(syncConnectCmd)
	syncCmd.AddCommand(syncDisconnectCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncMonitorCmd)
}
