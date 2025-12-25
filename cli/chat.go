package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "WebSocket chat operations",
	Long:  "Connect to WebSocket chat server and send/receive messages",
}

var chatJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join the chat room",
	Run:   runChatJoin,
}

var chatSendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the chat",
	Args:  cobra.MinimumNArgs(1),
	Run:   runChatSend,
}

var chatHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "View chat history",
	Run:   runChatHistory,
}

var chatRoom string
var chatMangaID string
var chatRoomDisplayName string // For displaying manga titles instead of IDs
var historyLimit int

func init() {
	chatCmd.AddCommand(chatJoinCmd)
	chatCmd.AddCommand(chatSendCmd)
	chatCmd.AddCommand(chatHistoryCmd)

	chatJoinCmd.Flags().StringVarP(&chatRoom, "room", "r", "global", "Room to join")
	chatJoinCmd.Flags().StringVar(&chatMangaID, "manga-id", "", "Join manga-specific chat")

	chatSendCmd.Flags().StringVarP(&chatRoom, "room", "r", "global", "Room to send message to")
	chatSendCmd.Flags().StringVar(&chatMangaID, "manga-id", "", "Send to manga-specific chat")

	chatHistoryCmd.Flags().StringVar(&chatMangaID, "manga-id", "", "Get history for manga chat")
	chatHistoryCmd.Flags().IntVar(&historyLimit, "limit", 20, "Number of messages to retrieve")
}

func runChatJoin(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	if cfg.User.Token == "" {
		printError("Not authenticated. Run 'mangahub auth login' first")
		return
	}

	// Handle manga-specific chat
	if chatMangaID != "" {
		// Fetch manga name for display
		mangaName, err := fetchMangaName(chatMangaID)
		if err != nil {
			printError(fmt.Sprintf("Failed to fetch manga name: %v", err))
			return
		}
		// Use ID in room name so backend can validate the manga exists
		chatRoom = "manga-" + chatMangaID
		chatRoomDisplayName = mangaName // Store for display
		fmt.Printf("Joining manga room: %s\n", mangaName)
	} else {
		chatRoomDisplayName = chatRoom
	}

	// Get server endpoint from config or environment
	wsPort := getEnvOrDefault("WEBSOCKET_PORT", fmt.Sprintf("%d", cfg.Server.WebSocketPort))
	wsHost := getEnvOrDefault("WEBSOCKET_HOST", cfg.Server.Host)

	// Fallback to localhost if config doesn't specify
	if wsHost == "" {
		wsHost = "localhost"

		// Try to auto-detect from server health endpoint
		if detectedIP := detectServerIP(wsPort); detectedIP != "" {
			wsHost = detectedIP
		}
	}

	fmt.Printf("Connecting to WebSocket chat server at ws://%s:%s...\n", wsHost, wsPort)

	wsURL := fmt.Sprintf("ws://%s:%s/ws/chat?token=%s",
		wsHost,
		wsPort,
		url.QueryEscape(cfg.User.Token))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer conn.Close()

	// Immediately join the target room by sending a typing indicator
	// This ensures we're subscribed to the room before any messages arrive
	joinRoomMsg := map[string]interface{}{
		"type": "typing",
		"room": chatRoom,
	}
	if data, err := json.Marshal(joinRoomMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})
	inputChan := make(chan string)
	username := cfg.User.Username

	// Message reader goroutine
	welcomeReceived := false
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err == nil {
				// Skip own join message before welcome
				msgType, _ := msg["type"].(string)
				content, _ := msg["content"].(string)
				if !welcomeReceived && msgType == "presence" && strings.Contains(content, username+" joined") {
					continue
				}
				if msgType == "welcome" {
					welcomeReceived = true
				}
				handleIncomingMessage(msg, username, chatRoom)
			}
		}
	}()

	// Input reader goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
	}()

	// Show prompt
	fmt.Printf("%s> ", username)

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			printSuccess("\nDisconnecting...")
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		case input := <-inputChan:
			if input == "" {
				fmt.Printf("%s> ", username)
				continue
			}

			if strings.HasPrefix(input, "/") {
				if handleChatCommand(conn, input, username, chatRoom, &chatMangaID) {
					return // Exit chat
				}
			} else {
				// Local echo inside sendChatMessage handles prompt
				sendChatMessage(conn, input, chatRoom, username)
			}
		}
	}
}

func runChatSend(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	if cfg.User.Token == "" {
		printError("Not authenticated. Run 'mangahub auth login' first")
		return
	}

	// Handle manga-specific chat
	if chatMangaID != "" {
		// Use ID in room name so backend can validate the manga exists
		chatRoom = "manga-" + chatMangaID
	}

	wsPort := getEnvOrDefault("WEBSOCKET_PORT", fmt.Sprintf("%d", cfg.Server.WebSocketPort))
	wsHost := getEnvOrDefault("WEBSOCKET_HOST", cfg.Server.Host)
	if wsHost == "" {
		wsHost = "localhost"
		if detectedIP := detectServerIP(wsPort); detectedIP != "" {
			wsHost = detectedIP
		}
	}
	wsURL := fmt.Sprintf("ws://%s:%s/ws/chat?token=%s",
		wsHost,
		wsPort,
		url.QueryEscape(cfg.User.Token))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer conn.Close()

	message := map[string]interface{}{
		"type":    "text",
		"content": strings.Join(args, " "),
		"room":    chatRoom,
	}

	data, err := json.Marshal(message)
	if err != nil {
		printError(fmt.Sprintf("Failed to marshal message: %v", err))
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		printError(fmt.Sprintf("Failed to send message: %v", err))
		return
	}

	printSuccess("Message sent")
}

func runChatHistory(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	if cfg.User.Token == "" {
		printError("Not authenticated. Run 'mangahub auth login' first")
		return
	}

	// Handle manga-specific chat
	room := chatRoom
	if chatMangaID != "" {
		// Use ID in room name so backend can validate the manga exists
		room = "manga-" + chatMangaID
	}

	wsPort := getEnvOrDefault("WEBSOCKET_PORT", fmt.Sprintf("%d", cfg.Server.WebSocketPort))
	wsHost := getEnvOrDefault("WEBSOCKET_HOST", cfg.Server.Host)
	if wsHost == "" {
		wsHost = "localhost"
		if detectedIP := detectServerIP(wsPort); detectedIP != "" {
			wsHost = detectedIP
		}
	}
	wsURL := fmt.Sprintf("ws://%s:%s/ws/chat?token=%s",
		wsHost,
		wsPort,
		url.QueryEscape(cfg.User.Token))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer conn.Close()

	// Send history command with requested limit
	cmdStr := fmt.Sprintf("/history %d", historyLimit)
	message := map[string]interface{}{
		"type":    "command",
		"command": cmdStr,
		"room":    room,
	}

	data, err := json.Marshal(message)
	if err != nil {
		printError(fmt.Sprintf("Failed to marshal message: %v", err))
		return
	}

	// Set up message receiver
	done := make(chan bool)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				done <- true
				return
			}

			var response map[string]interface{}
			if err := json.Unmarshal(msg, &response); err == nil {
				msgType, _ := response["type"].(string)
				if msgType == "history" {
					metadata, _ := response["metadata"].(map[string]interface{})
					messages, _ := metadata["messages"].([]interface{})

					fmt.Printf("\nChat History (last %d messages):\n", historyLimit)
					fmt.Println("─────────────────────────────────────────────────────────────")

					if len(messages) == 0 {
						fmt.Println("No messages found.")
					} else {
						for _, m := range messages {
							if msgMap, ok := m.(map[string]interface{}); ok {
								timestamp, _ := msgMap["timestamp"].(string)
								fromUser, _ := msgMap["from"].(string)
								msgContent, _ := msgMap["content"].(string)

								t, _ := time.Parse(time.RFC3339, timestamp)
								fmt.Printf("[%s] %s: %s\n", t.Format("15:04"), fromUser, msgContent)
							}
						}
					}
					fmt.Println("─────────────────────────────────────────────────────────────")
					done <- true
					return
				}
			}
		}
	}()

	conn.WriteMessage(websocket.TextMessage, data)

	// Wait for response or timeout
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		printError("Timeout waiting for history")
	}
}

func handleIncomingMessage(msg map[string]interface{}, username, currentRoom string) {
	msgType, _ := msg["type"].(string)
	from, _ := msg["from"].(string)
	content, _ := msg["content"].(string)
	room, _ := msg["room"].(string)

	if room == "" {
		room = "global"
	}

	switch msgType {
	case "welcome":
		metadata, _ := msg["metadata"].(map[string]interface{})
		userCount, _ := metadata["user_count"].(float64)
		roomName, _ := metadata["room"].(string)

		printSuccess(fmt.Sprintf("✓ %s", content))
		fmt.Printf("Chat Room: #%s\n", roomName)
		fmt.Printf("Connected users: %d\n", int(userCount))
		fmt.Println("Your status: Online")

		// Show recent messages if available
		if recentMsgs, ok := metadata["recent_messages"].([]interface{}); ok && len(recentMsgs) > 0 {
			fmt.Println("\nRecent messages:")
			for _, rmsg := range recentMsgs {
				if msgMap, ok := rmsg.(map[string]interface{}); ok {
					timestamp, _ := msgMap["timestamp"].(string)
					fromUser, _ := msgMap["from"].(string)
					msgContent, _ := msgMap["content"].(string)

					t, _ := time.Parse(time.RFC3339, timestamp)
					fmt.Printf("[%s] %s: %s\n", t.Format("15:04"), fromUser, msgContent)
				}
			}
		}

		fmt.Println("─────────────────────────────────────────────────────────────")
		fmt.Println("You are now in chat. Type your message and press Enter.")
		fmt.Println("Type /help for commands or /quit to leave.")
		fmt.Println("─────────────────────────────────────────────────────────────")
		// After welcome, show prompt for input
		fmt.Printf("%s> ", username)
	case "presence":
		displayRoom := chatRoomDisplayName
		if displayRoom == "" {
			displayRoom = room
		}
		fmt.Printf("\n[%s/%s] %s: %s\n", displayRoom, msgType, from, content)
		// Reprint prompt after incoming message
		fmt.Printf("%s> ", username)

	case "text":
		timestamp, _ := msg["timestamp"].(string)
		t, _ := time.Parse(time.RFC3339, timestamp)
		// Skip server echo of our own message since we already locally echoed it
		if from != username {
			fmt.Printf("\n[%s] %s: %s\n", t.Format("15:04"), from, content)
			fmt.Printf("%s> ", username)
		}

	case "userlist":
		metadata, _ := msg["metadata"].(map[string]interface{})
		users, _ := metadata["users"].([]interface{})
		userCount, _ := metadata["count"].(float64)

		fmt.Printf("\n\nOnline Users (%d):\n", int(userCount))
		for _, u := range users {
			if userMap, ok := u.(map[string]interface{}); ok {
				uname, _ := userMap["username"].(string)
				uroom, _ := userMap["room"].(string)
				fmt.Printf("● %s (%s)\n", uname, strings.Title(uroom))
			}
		}
		fmt.Println()
		// Reprint prompt after the list
		fmt.Printf("%s> ", username)

	case "history":
		metadata, _ := msg["metadata"].(map[string]interface{})
		messages, _ := metadata["messages"].([]interface{})

		fmt.Println("\n\nChat History:")
		for _, m := range messages {
			if msgMap, ok := m.(map[string]interface{}); ok {
				timestamp, _ := msgMap["timestamp"].(string)
				fromUser, _ := msgMap["from"].(string)
				msgContent, _ := msgMap["content"].(string)

				t, _ := time.Parse(time.RFC3339, timestamp)
				fmt.Printf("[%s] %s: %s\n", t.Format("15:04"), fromUser, msgContent)
			}
		}
		fmt.Println()
		// Reprint prompt after history display
		fmt.Printf("%s> ", username)

	case "system":
		fmt.Printf("\n%s\n", content)

		// Check if this is a rooms list response
		if metadata, ok := msg["metadata"].(map[string]interface{}); ok {
			if rooms, ok := metadata["rooms"].([]interface{}); ok {
				roomCount, _ := metadata["count"].(float64)
				fmt.Printf("\nTotal Rooms: %d\n", int(roomCount))
				fmt.Println("─────────────────────────────────────────────────────")
				for _, r := range rooms {
					if roomMap, ok := r.(map[string]interface{}); ok {
						name, _ := roomMap["name"].(string)
						roomType, _ := roomMap["type"].(string)
						memberCount, _ := roomMap["member_count"].(float64)
						createdAt, _ := roomMap["created_at"].(string)

						fmt.Printf("📁 %s\n", name)
						fmt.Printf("   Type: %s | Members: %d | Created: %s\n",
							roomType, int(memberCount), createdAt)

						if lastMsg, ok := roomMap["last_message_at"].(string); ok {
							fmt.Printf("   Last activity: %s\n", lastMsg)
						}
						fmt.Println()
					}
				}
				fmt.Println("─────────────────────────────────────────────────────")
				fmt.Println("Use: mangahub chat join -r \"<room-name>\" to join a room")
			} else if roomID, ok := metadata["room_id"].(string); ok {
				// Room creation confirmation
				roomName, _ := metadata["room_name"].(string)
				role, _ := metadata["role"].(string)
				fmt.Printf("✓ Room ID: %s\n", roomID)
				fmt.Printf("✓ Your role: %s\n", role)
				fmt.Printf("✓ Join with: mangahub chat join -r \"%s\"\n", roomName)
			}
		}

		// Reprint prompt after system message
		fmt.Printf("\n%s> ", username)

	default:
		displayRoom := chatRoomDisplayName
		if displayRoom == "" {
			displayRoom = room
		}
		fmt.Printf("\n[%s/%s] %s: %s\n", displayRoom, msgType, from, content)
		// Reprint prompt for any other message types
		fmt.Printf("%s> ", username)
	}
}

func handleChatCommand(conn *websocket.Conn, input, username, currentRoom string, mangaID *string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	command := parts[0]

	switch command {
	case "/help":
		fmt.Println("\nChat Commands:")
		fmt.Println("  /help             - Show this help")
		fmt.Println("  /rooms            - List all available rooms")
		fmt.Println("  /create <name>    - Create a new custom room")
		fmt.Println("  /users            - List online users")
		fmt.Println("  /quit             - Leave chat")
		fmt.Println("  /pm <user> <msg>  - Private message")
		fmt.Println("  /manga <id>       - Switch to manga chat")
		fmt.Println("  /history          - Show recent history")
		fmt.Println("  /status           - Connection status")
		fmt.Println()
		return false

	case "/quit", "/exit":
		return true

	case "/rooms":
		sendCommand(conn, "rooms", currentRoom)
		return false

	case "/create":
		if len(parts) < 2 {
			fmt.Println("Usage: /create <room-name>")
			return false
		}
		roomName := strings.Join(parts[1:], " ")
		sendCommand(conn, "create "+roomName, currentRoom)
		return false

	case "/users":
		sendCommand(conn, "users", currentRoom)
		return false

	case "/history":
		sendCommand(conn, "history", currentRoom)
		return false

	case "/status":
		sendCommand(conn, "status", currentRoom)
		return false

	case "/pm":
		if len(parts) < 3 {
			fmt.Println("Usage: /pm <username> <message>")
			return false
		}
		toUser := parts[1]
		message := strings.Join(parts[2:], " ")
		sendPrivateMessage(conn, toUser, message, currentRoom)
		return false

	case "/manga":
		if len(parts) < 2 {
			fmt.Println("Usage: /manga <manga-id>")
			return false
		}
		*mangaID = parts[1]
		fmt.Printf("\nSwitching to %s Discussion...\n", *mangaID)
		fmt.Println("Note: Please reconnect with --manga-id flag to switch rooms")
		sendChatMessage(conn, fmt.Sprintf("Switching to %s chat", *mangaID), currentRoom, username)
		return false

	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", command)
		return false
	}
}

func sendChatMessage(conn *websocket.Conn, content, room, username string) {
	message := map[string]interface{}{
		"type":    "text",
		"content": content,
		"room":    room,
	}

	data, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, data)
	// Local echo to avoid waiting for server and to keep ordering tidy
	now := time.Now().Format("15:04")
	fmt.Printf("\n[%s] %s: %s\n", now, username, content)
	fmt.Printf("%s> ", username)
}

func sendCommand(conn *websocket.Conn, command, room string) {
	// Accept commands with arguments and preserve leading '/'
	cmd := command
	if !strings.HasPrefix(cmd, "/") {
		cmd = "/" + cmd
	}
	message := map[string]interface{}{
		"type":    "command",
		"command": cmd,
		"room":    room,
	}

	data, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, data)
}

func sendPrivateMessage(conn *websocket.Conn, toUser, content, room string) {
	message := map[string]interface{}{
		"type":    "text",
		"to":      toUser,
		"content": content,
		"room":    room,
	}

	data, _ := json.Marshal(message)
	conn.WriteMessage(websocket.TextMessage, data)
	fmt.Printf("\n[PM to %s] %s\n", toUser, content)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
func detectServerIP(port string) string {
	// Import utils package if not already imported
	// Try common local IPs first (localhost, 127.0.0.1)
	candidates := []string{"localhost", "127.0.0.1"}

	// Add detected local IP using existing utility
	if detectedIP := utils.GetLocalIP(); detectedIP != "" && detectedIP != "127.0.0.1" {
		candidates = append(candidates, detectedIP)
	}

	// Try to reach health endpoint and get local_ip from response
	for _, host := range candidates {
		resp, err := http.Get(fmt.Sprintf("http://%s:%s/health", host, port))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var healthData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&healthData); err != nil {
			continue
		}

		if localIP, ok := healthData["local_ip"].(string); ok && localIP != "" {
			return localIP
		}
	}

	return ""
}

// fetchMangaName fetches the manga title by ID to use as room name
func fetchMangaName(mangaID string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	// Get API server endpoint from config or environment
	apiHost := getEnvOrDefault("API_HOST", cfg.Server.Host)
	apiPort := getEnvOrDefault("API_PORT", fmt.Sprintf("%d", cfg.Server.HTTPPort))

	// Fallback to localhost if config doesn't specify
	if apiHost == "" {
		apiHost = "localhost"

		if detectedIP := detectServerIP(apiPort); detectedIP != "" {
			apiHost = detectedIP
		}
	}

	apiURL := fmt.Sprintf("http://%s:%s/manga/info/%s", apiHost, apiPort, mangaID)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var mangaData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&mangaData); err != nil {
		return "", err
	}

	// Try to get title
	if title, ok := mangaData["title"].(string); ok && title != "" {
		return title, nil
	}

	// Fallback to manga-id if title not found
	return "manga-" + mangaID, nil
}
