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

		cfg, err := config.Load()
		if err != nil {
			displayCachedStatus(connInfo, cfg)
			return nil
		}

		liveStatus, err := queryServerStatus(cfg, connInfo)
		if err != nil {
			fmt.Printf("  ⚠ Unable to fetch live status: %s\n", err.Error())
			fmt.Println("  Showing cached information:")
			displayCachedStatus(connInfo, cfg)
			return nil
		}

		displayLiveStatus(liveStatus, cfg)
		return nil
	},
}

var syncMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor real-time sync updates",
	Long:  `Display real-time synchronization updates as they happen across devices.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		active, _, err := config.IsConnectionActive()
		if err != nil {
			printError("Failed to check connection status")
			return err
		}

		if !active {
			printError("Not connected to sync server")
			fmt.Println("Run: mangahub sync connect")
			return fmt.Errorf("no active connection")
		}

		cfg, err := config.Load()
		if err != nil {
			printError("Failed to load configuration")
			return err
		}

		return startMonitoring(cfg)
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

func queryServerStatus(cfg *config.Config, connInfo *config.ConnectionInfo) (*statusResponse, error) {
	serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))

	conn, err := net.DialTimeout("tcp", serverAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": cfg.User.Token,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	authJSON = append(authJSON, '\n')

	if _, err := conn.Write(authJSON); err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read auth response: %w", err)
	}

	var authResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &authResponse); err != nil {
		return nil, fmt.Errorf("invalid auth response: %w", err)
	}

	if authResponse["type"] != "success" {
		return nil, fmt.Errorf("authentication failed")
	}

	statusMsg := map[string]interface{}{
		"type":    "status_request",
		"payload": map[string]interface{}{},
	}
	statusJSON, _ := json.Marshal(statusMsg)
	statusJSON = append(statusJSON, '\n')

	if _, err := conn.Write(statusJSON); err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	var statusResp statusResponse
	if err := json.Unmarshal([]byte(response), &statusResp); err != nil {
		return nil, fmt.Errorf("failed to parse status response: %w", err)
	}

	return &statusResp, nil
}

// statusResponse matches the server's status message format
type statusResponse struct {
	Type    string `json:"type"`
	Payload struct {
		ConnectionStatus string `json:"connection_status"`
		ServerAddress    string `json:"server_address"`
		Uptime           int64  `json:"uptime_seconds"`
		LastHeartbeat    string `json:"last_heartbeat"`
		SessionID        string `json:"session_id"`
		DevicesOnline    int    `json:"devices_online"`
		MessagesSent     int64  `json:"messages_sent"`
		MessagesReceived int64  `json:"messages_received"`
		LastSync         *struct {
			MangaID    string `json:"manga_id"`
			MangaTitle string `json:"manga_title"`
			Chapter    int    `json:"chapter"`
			Timestamp  string `json:"timestamp"`
		} `json:"last_sync,omitempty"`
		NetworkQuality string `json:"network_quality"`
		RTT            int64  `json:"rtt_ms"`
	} `json:"payload"`
}

func displayLiveStatus(status *statusResponse, cfg *config.Config) {
	fmt.Println("  Connection: ✓ Active")
	fmt.Printf("  Server: %s\n", status.Payload.ServerAddress)

	uptime := time.Duration(status.Payload.Uptime) * time.Second
	fmt.Printf("  Uptime: %s\n", formatDuration(uptime))

	if status.Payload.LastHeartbeat != "" {
		lastHeartbeat, err := time.Parse(time.RFC3339, status.Payload.LastHeartbeat)
		if err == nil {
			timeSince := time.Since(lastHeartbeat)
			fmt.Printf("  Last heartbeat: %s ago\n", formatDuration(timeSince))
		}
	}

	fmt.Println()
	fmt.Println("Session Info:")
	if cfg != nil {
		fmt.Printf("  User: %s\n", cfg.User.Username)
	}
	fmt.Printf("  Session ID: %s\n", status.Payload.SessionID)
	fmt.Printf("  Devices online: %d\n", status.Payload.DevicesOnline)

	fmt.Println()
	fmt.Println("Sync Statistics:")
	fmt.Printf("  Messages sent: %d\n", status.Payload.MessagesSent)
	fmt.Printf("  Messages received: %d\n", status.Payload.MessagesReceived)

	if status.Payload.LastSync != nil {
		lastSyncTime, err := time.Parse(time.RFC3339, status.Payload.LastSync.Timestamp)
		if err == nil {
			timeSince := time.Since(lastSyncTime)
			fmt.Printf("  Last sync: %s ago (%s ch. %d)\n",
				formatDuration(timeSince),
				status.Payload.LastSync.MangaTitle,
				status.Payload.LastSync.Chapter)
		}
	} else {
		fmt.Println("  Last sync: N/A")
	}
	fmt.Println("  Sync conflicts: 0")

	fmt.Println()
	fmt.Printf("Network Quality: %s", status.Payload.NetworkQuality)
	if status.Payload.RTT > 0 {
		fmt.Printf(" (RTT: %dms)", status.Payload.RTT)
	}
	fmt.Println()
}

func displayCachedStatus(connInfo *config.ConnectionInfo, cfg *config.Config) {
	fmt.Println("  Connection: ✓ Active (cached)")
	fmt.Printf("  Server: %s\n", connInfo.Server)

	uptime := time.Since(connInfo.ConnectedAt)
	fmt.Printf("  Uptime: %s\n", formatDuration(uptime))

	timeSinceHeartbeat := time.Since(connInfo.LastHeartbeat)
	fmt.Printf("  Last heartbeat: %s ago\n", formatDuration(timeSinceHeartbeat))

	fmt.Println()
	fmt.Println("Session Info:")
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

	fmt.Println()
	if timeSinceHeartbeat < 30*time.Second {
		fmt.Println("Network Quality: Good")
	} else if timeSinceHeartbeat < 60*time.Second {
		fmt.Println("Network Quality: Fair")
	} else {
		fmt.Println("Network Quality: Poor (connection may be stale)")
	}
}

func startMonitoring(cfg *config.Config) error {
	serverAddr := net.JoinHostPort(cfg.Server.Host, fmt.Sprintf("%d", cfg.Server.TCPPort))

	fmt.Printf("Connecting to sync server at %s...\n", serverAddr)
	conn, err := net.DialTimeout("tcp", serverAddr, 10*time.Second)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %s", err.Error()))
		return err
	}
	defer conn.Close()

	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": cfg.User.Token,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	authJSON = append(authJSON, '\n')

	if _, err := conn.Write(authJSON); err != nil {
		printError("Failed to send authentication")
		return err
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		printError("Failed to receive authentication response")
		return err
	}

	var authResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &authResponse); err != nil {
		printError("Invalid authentication response")
		return err
	}

	if authResponse["type"] != "success" {
		printError("Authentication failed")
		return fmt.Errorf("authentication rejected by server")
	}

	subscribeMsg := map[string]interface{}{
		"type": "subscribe_updates",
		"payload": map[string]interface{}{
			"event_types": []string{},
		},
	}
	subscribeJSON, _ := json.Marshal(subscribeMsg)
	subscribeJSON = append(subscribeJSON, '\n')

	if _, err := conn.Write(subscribeJSON); err != nil {
		printError("Failed to send subscribe request")
		return err
	}

	response, err = reader.ReadString('\n')
	if err != nil {
		printError("Failed to receive subscribe response")
		return err
	}

	var subscribeResponse map[string]interface{}
	if err := json.Unmarshal([]byte(response), &subscribeResponse); err != nil {
		printError("Invalid subscribe response")
		return err
	}

	if subscribeResponse["type"] == "error" {
		printError("Subscription failed")
		return fmt.Errorf("subscription rejected by server")
	}

	printSuccess("Subscribed to real-time updates")
	fmt.Println("Monitoring sync events... (Press Ctrl+C to exit)")
	fmt.Println()

	return monitorEventLoop(conn, reader)
}

func monitorEventLoop(conn net.Conn, reader *bufio.Reader) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	eventChan := make(chan string, 10)
	errChan := make(chan error, 1)

	go func() {
		for {
			response, err := reader.ReadString('\n')
			if err != nil {
				errChan <- err
				return
			}
			eventChan <- response
		}
	}()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n\nStopping monitor...")
			unsubscribeMsg := map[string]interface{}{
				"type":    "unsubscribe_updates",
				"payload": map[string]interface{}{},
			}
			unsubscribeJSON, _ := json.Marshal(unsubscribeMsg)
			unsubscribeJSON = append(unsubscribeJSON, '\n')
			conn.Write(unsubscribeJSON)
			time.Sleep(100 * time.Millisecond)
			fmt.Println("✓ Monitoring stopped")
			return nil

		case err := <-errChan:
			printError(fmt.Sprintf("Connection lost: %s", err.Error()))
			return err

		case response := <-eventChan:
			displayEvent(response)
		}
	}
}

func displayEvent(jsonMsg string) {
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(jsonMsg), &msg); err != nil {
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok || msgType != "update_event" {
		return
	}

	payload, ok := msg["payload"].(map[string]interface{})
	if !ok {
		return
	}

	timestamp, _ := payload["timestamp"].(string)
	direction, _ := payload["direction"].(string)
	action, _ := payload["action"].(string)
	mangaTitle, _ := payload["manga_title"].(string)
	chapter, _ := payload["chapter"].(float64)

	timeStr := formatTimestamp(timestamp)
	directionStr := formatDirection(direction)

	fmt.Printf("[%s] %s %s: %s (Chapter %d)\n",
		timeStr,
		directionStr,
		action,
		mangaTitle,
		int(chapter))

	conflict, hasConflict := payload["conflict"].(string)
	if hasConflict && conflict != "" {
		fmt.Printf("         ⚠ %s\n", conflict)
	}
}

func formatTimestamp(timestamp string) string {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	return t.Format("15:04:05")
}

func formatDirection(direction string) string {
	if direction == "incoming" {
		return "←"
	} else if direction == "outgoing" {
		return "→"
	}
	return "•"
}

func init() {
	syncConnectCmd.Flags().StringVar(&syncDeviceType, "device-type", "", "Device type (mobile, desktop, web)")
	syncConnectCmd.Flags().StringVar(&syncDeviceName, "device-name", "", "Device name")
	syncCmd.AddCommand(syncConnectCmd)
	syncCmd.AddCommand(syncDisconnectCmd)
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncMonitorCmd)
}
