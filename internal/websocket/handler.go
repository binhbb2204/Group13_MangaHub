package websocket

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

type Handler struct {
	db      *sql.DB
	manager *Manager
}

func NewHandler(db *sql.DB, manager *Manager) *Handler {
	return &Handler{db: db, manager: manager}
}

func (h *Handler) HandleClientMessage(client *Client, data []byte) error {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	if err := h.validateMessage(&msg); err != nil {
		return err
	}
	switch msg.Type {
	case MessageTypeText:
		return h.handleTextMessage(client, msg)
	case MessageTypeTyping:
		return h.handleTypingIndicator(client, msg)
	case MessageTypeCommand:
		return h.handleCommand(client, msg)
	default:
		logger.Warn("Unknown message type", map[string]interface{}{"type": msg.Type})
	}
	return nil
}

func (h *Handler) validateMessage(msg *ClientMessage) error {
	if len(msg.Content) > 4096 {
		return &ValidationError{Field: "content", Message: "message too long"}
	}
	if msg.Type == MessageTypeText && len(msg.Content) == 0 {
		return &ValidationError{Field: "content", Message: "message content required"}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

func (h *Handler) handleTextMessage(client *Client, msg ClientMessage) error {
	room := msg.Room
	if room == "" {
		room = "global"
	}
	id, _ := utils.GenerateID(16)
	serverMsg := ServerMessage{ID: id, Type: MessageTypeText, From: client.Username, Room: room, Content: msg.Content, Timestamp: time.Now()}
	if msg.To != "" {
		serverMsg.Metadata = map[string]interface{}{"direct": true}
	}
	// Ensure client is a member of the target room before broadcasting
	client.Manager.joinRoom(client, room)
	if err := h.saveMessage(client.ID, msg.To, msg.Content); err != nil {
		logger.Warn("Failed to save message", map[string]interface{}{"error": err.Error()})
	}
	data, err := json.Marshal(serverMsg)
	if err != nil {
		return err
	}
	if msg.To != "" {
		client.Manager.SendToUser(msg.To, data)
		client.Manager.SendToUser(client.ID, data)
	} else {
		client.Manager.broadcastRoom(room, data)
	}
	return nil
}

func (h *Handler) handleTypingIndicator(client *Client, msg ClientMessage) error {
	room := msg.Room
	if room == "" {
		room = "global"
	}
	id, _ := utils.GenerateID(16)
	serverMsg := ServerMessage{ID: id, Type: MessageTypeTyping, From: client.Username, Room: room, Timestamp: time.Now()}
	data, err := json.Marshal(serverMsg)
	if err != nil {
		return err
	}
	if msg.To != "" {
		client.Manager.SendToUser(msg.To, data)
	} else {
		// Ensure client is a member of the target room before broadcasting typing state
		client.Manager.joinRoom(client, room)
		client.Manager.broadcastRoom(room, data)
	}
	return nil
}

func (h *Handler) saveMessage(fromUserID, toUserID, content string) error {
	id, _ := utils.GenerateID(16)
	var toUserIDValue interface{}
	if toUserID == "" {
		toUserIDValue = nil
	} else {
		toUserIDValue = toUserID
	}
	query := `INSERT INTO chat_messages (id, from_user_id, to_user_id, content, created_at) 
              VALUES (?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, id, fromUserID, toUserIDValue, content, time.Now())
	if err != nil {
		logger.Warn("Failed to save message to DB", map[string]interface{}{"error": err.Error()})
	}
	return err
}

func (h *Handler) handleCommand(client *Client, msg ClientMessage) error {
	// Determine the raw command string (prefer Command field, fallback to Content)
	raw := msg.Command
	if raw == "" {
		raw = msg.Content
	}

	// Parse command and args
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil
	}
	cmd := parts[0]
	if strings.HasPrefix(cmd, "/") {
		cmd = cmd[1:]
	}
	args := parts[1:]

	id, _ := utils.GenerateID(16)
	var responseMsg ServerMessage

	switch cmd {
	case "users":
		users := h.manager.GetRoomUsers(msg.Room)
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeUserList,
			From:      "system",
			Room:      msg.Room,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"users": users,
				"count": len(users),
			},
		}
	case "history":
		// Default history limit; override if a valid argument is provided
		limit := 20
		if len(args) > 0 {
			if n, err := strconv.Atoi(args[0]); err == nil {
				// Clamp to sane bounds
				if n < 1 {
					n = 1
				}
				if n > 500 {
					n = 500
				}
				limit = n
			}
		}
		messages, _ := h.GetMessageHistory(client.ID, limit)
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeHistory,
			From:      "system",
			Room:      msg.Room,
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"messages": messages,
			},
		}
	case "status":
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeSystem,
			From:      "system",
			Room:      msg.Room,
			Content:   "Connection status: Online",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"status": "online",
				"user":   client.Username,
			},
		}
	default:
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeSystem,
			From:      "system",
			Room:      msg.Room,
			Content:   "Unknown command. Type /help for available commands",
			Timestamp: time.Now(),
		}
	}

	data, err := json.Marshal(responseMsg)
	if err != nil {
		return err
	}
	client.Manager.SendToUser(client.ID, data)
	return nil
}

func (h *Handler) GetMessageHistory(userID string, limit int) ([]Message, error) {
	query := `SELECT cm.id, u.username, cm.to_user_id, cm.content, cm.created_at 
              FROM chat_messages cm
              JOIN users u ON cm.from_user_id = u.id
              WHERE cm.from_user_id = ? OR cm.to_user_id = ? OR cm.to_user_id IS NULL
              ORDER BY cm.created_at DESC LIMIT ?`
	rows, err := h.db.Query(query, userID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var msg Message
		var toUser sql.NullString
		if err := rows.Scan(&msg.ID, &msg.From, &toUser, &msg.Content, &msg.Timestamp); err != nil {
			continue
		}
		if toUser.Valid {
			msg.To = toUser.String
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
