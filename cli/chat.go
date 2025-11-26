package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/cli/config"
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

var chatRoom string

func init() {
	chatCmd.AddCommand(chatJoinCmd)
	chatCmd.AddCommand(chatSendCmd)
	chatJoinCmd.Flags().StringVarP(&chatRoom, "room", "r", "global", "Room to join")
	chatSendCmd.Flags().StringVarP(&chatRoom, "room", "r", "global", "Room to send message to")
}

func runChatJoin(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		printError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	if cfg.User.Token == "" {
		printError("Not authenticated. Run 'mangahub-cli auth login' first")
		return
	}

	wsURL := fmt.Sprintf("ws://localhost:%s/ws/chat?token=%s",
		getEnvOrDefault("WEBSOCKET_PORT", "9093"),
		url.QueryEscape(cfg.User.Token))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer conn.Close()

	printSuccess(fmt.Sprintf("Connected to chat server (room: %s)", chatRoom))

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(message, &msg); err == nil {
				room := "global"
				if r, ok := msg["room"].(string); ok && r != "" {
					room = r
				}
				fmt.Printf("[%s/%s] %s: %s\n",
					room,
					msg["type"],
					msg["from"],
					msg["content"])
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			printSuccess("Disconnecting...")
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
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
		printError("Not authenticated. Run 'mangahub-cli auth login' first")
		return
	}

	wsURL := fmt.Sprintf("ws://localhost:%s/ws/chat?token=%s",
		getEnvOrDefault("WEBSOCKET_PORT", "9093"),
		url.QueryEscape(cfg.User.Token))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		printError(fmt.Sprintf("Failed to connect: %v", err))
		return
	}
	defer conn.Close()

	message := map[string]interface{}{
		"type":    "text",
		"content": args[0],
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

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
