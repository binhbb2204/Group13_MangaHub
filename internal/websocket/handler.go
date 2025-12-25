package websocket

import (
	"database/sql"
	"encoding/json"
	"fmt"
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

	logger.Debug("Message received", map[string]interface{}{
		"from":    client.Username,
		"room":    room,
		"content": msg.Content,
	})

	// Get or create conversation
	convID, err := h.getOrCreateConversation(room, client.ID)
	if err != nil {
		logger.Warn("Failed to get/create conversation", map[string]interface{}{"error": err.Error()})
		return err
	}

	// Join user to conversation
	h.joinConversation(client.ID, convID)

	id, _ := utils.GenerateID(16)
	serverMsg := ServerMessage{ID: id, Type: MessageTypeText, From: client.Username, Room: room, Content: msg.Content, Timestamp: time.Now()}
	if msg.To != "" {
		serverMsg.Metadata = map[string]interface{}{"direct": true}
	}
	// Ensure client is a member of the target room before broadcasting
	client.Manager.joinRoom(client, room)
	if err := h.saveMessageToConversation(convID, client.ID, msg.Content); err != nil {
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

func (h *Handler) saveMessageToConversation(conversationID, senderID, content string) error {
	id, _ := utils.GenerateID(16)
	now := time.Now().Format("2006-01-02 15:04:05")
	query := `INSERT INTO messages (id, conversation_id, sender_id, content, created_at) 
			  VALUES (?, ?, ?, ?, ?)`
	_, err := h.db.Exec(query, id, conversationID, senderID, content, now)
	if err != nil {
		return err
	}
	// Update last_message_at for conversation
	h.db.Exec(`UPDATE conversations SET last_message_at = ? WHERE id = ?`, now, conversationID)
	return nil
}

func (h *Handler) getOrCreateConversation(roomName, userID string) (string, error) {
	if roomName == "" {
		roomName = "global"
	}

	// Determine conversation type and name
	var convType, convName, mangaID string
	if roomName == "global" {
		convType = "global"
		convName = "global"
	} else if strings.HasPrefix(roomName, "manga-") {
		convType = "manga"
		convName = roomName
		mangaID = strings.TrimPrefix(roomName, "manga-")
		// Validate manga exists
		var exists bool
		err := h.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM manga WHERE id = ?)`, mangaID).Scan(&exists)
		if err != nil || !exists {
			return "", &ValidationError{Field: "manga_id", Message: "manga not found"}
		}
	} else {
		convType = "custom"
		convName = roomName
	}

	// Check if conversation exists
	var convID string
	err := h.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, convName).Scan(&convID)
	if err == sql.ErrNoRows {
		// Create new conversation
		convID, _ = utils.GenerateID(16)
		var mangaIDVal interface{}
		if mangaID != "" {
			mangaIDVal = mangaID
		} else {
			mangaIDVal = nil
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		_, err = h.db.Exec(`INSERT INTO conversations (id, name, type, manga_id, created_by, created_at, last_message_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?)`, convID, convName, convType, mangaIDVal, userID, now, now)
		if err != nil {
			return "", err
		}
		logger.Info("Created new conversation", map[string]interface{}{"id": convID, "name": convName, "type": convType})

		// Set creator as owner for custom rooms
		if convType == "custom" {
			h.db.Exec(`INSERT OR IGNORE INTO user_conversation_history (user_id, conversation_id, role, joined_at, unread_count) VALUES (?, ?, 'owner', ?, 0)`, userID, convID, now)
		}
	}
	return convID, nil
}

func (h *Handler) joinConversation(userID, conversationID string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	h.db.Exec(`INSERT OR IGNORE INTO user_conversation_history (user_id, conversation_id, joined_at, unread_count)
		VALUES (?, ?, ?, 0)`, userID, conversationID, now)
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
	case "rooms":
		// List all available conversations/rooms
		rooms, _ := h.GetAllRooms()
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeSystem,
			From:      "system",
			Room:      msg.Room,
			Content:   "Available rooms",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"rooms": rooms,
				"count": len(rooms),
			},
		}
	case "create":
		// Create a custom room: /create <room-name>
		if len(args) == 0 {
			responseMsg = ServerMessage{
				ID:        id,
				Type:      MessageTypeError,
				From:      "system",
				Room:      msg.Room,
				Content:   "Usage: /create <room-name>",
				Timestamp: time.Now(),
			}
		} else {
			roomName := strings.Join(args, "-")
			convID, err := h.createCustomRoom(roomName, client.ID)
			if err != nil {
				responseMsg = ServerMessage{
					ID:        id,
					Type:      MessageTypeError,
					From:      "system",
					Room:      msg.Room,
					Content:   fmt.Sprintf("Failed to create room: %v", err),
					Timestamp: time.Now(),
				}
			} else {
				responseMsg = ServerMessage{
					ID:        id,
					Type:      MessageTypeSystem,
					From:      "system",
					Room:      roomName,
					Content:   fmt.Sprintf("Room '%s' created successfully! You are the owner.", roomName),
					Timestamp: time.Now(),
					Metadata: map[string]interface{}{
						"room_id":   convID,
						"room_name": roomName,
						"role":      "owner",
					},
				}
			}
		}
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
	case "join":
		// Join a room: /join <room-name>
		if len(args) == 0 {
			responseMsg = ServerMessage{
				ID:        id,
				Type:      MessageTypeError,
				From:      "system",
				Room:      msg.Room,
				Content:   "Usage: /join <room-name>",
				Timestamp: time.Now(),
			}
		} else {
			roomName := strings.Join(args, " ")
			// Get or create conversation for the room
			convID, err := h.getOrCreateConversation(roomName, client.ID)
			if err != nil {
				responseMsg = ServerMessage{
					ID:        id,
					Type:      MessageTypeError,
					From:      "system",
					Room:      msg.Room,
					Content:   fmt.Sprintf("Failed to join room: %v", err),
					Timestamp: time.Now(),
				}
			} else {
				// Join user to conversation in database
				h.joinConversation(client.ID, convID)
				// Add client to room in WebSocket manager
				client.Manager.joinRoom(client, roomName)

				// Send confirmation to client
				responseMsg = ServerMessage{
					ID:        id,
					Type:      MessageTypePresence,
					From:      "system",
					Room:      roomName,
					Content:   fmt.Sprintf("%s joined the chat", client.Username),
					Timestamp: time.Now(),
				}

				// Broadcast to room that user joined
				data, _ := json.Marshal(responseMsg)
				client.Manager.broadcastRoom(roomName, data)

				// Also send history to the joining client
				messages, _ := h.GetConversationHistory(convID, 50)
				historyMsg := ServerMessage{
					ID:        id,
					Type:      MessageTypeHistory,
					From:      "system",
					Room:      roomName,
					Timestamp: time.Now(),
					Metadata: map[string]interface{}{
						"messages": messages,
					},
				}
				historyData, _ := json.Marshal(historyMsg)
				client.Manager.SendToUser(client.ID, historyData)

				// Send userlist to the joining client
				users := h.manager.GetRoomUsers(roomName)
				userListMsg := ServerMessage{
					ID:        id,
					Type:      MessageTypeUserList,
					From:      "system",
					Room:      roomName,
					Timestamp: time.Now(),
					Metadata: map[string]interface{}{
						"users": users,
						"count": len(users),
					},
				}
				userListData, _ := json.Marshal(userListMsg)
				client.Manager.SendToUser(client.ID, userListData)
				return nil
			}
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
		room := msg.Room
		if room == "" {
			room = "global"
		}
		convID, _ := h.getOrCreateConversation(room, client.ID)
		messages, _ := h.GetConversationHistory(convID, limit)
		responseMsg = ServerMessage{
			ID:        id,
			Type:      MessageTypeHistory,
			From:      "system",
			Room:      room,
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

func (h *Handler) GetConversationHistory(conversationID string, limit int) ([]Message, error) {
	query := `SELECT m.id, u.username, m.content, m.created_at 
              FROM messages m
              JOIN users u ON m.sender_id = u.id
              WHERE m.conversation_id = ?
              ORDER BY m.created_at DESC LIMIT ?`
	rows, err := h.db.Query(query, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var messages []Message
	for rows.Next() {
		var msg Message
		if err := rows.Scan(&msg.ID, &msg.From, &msg.Content, &msg.Timestamp); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// GetAllRooms returns all available conversations/rooms
func (h *Handler) GetAllRooms() ([]map[string]interface{}, error) {
	query := `
		SELECT 
			c.id, 
			c.name, 
			c.type,
			c.created_at,
			COUNT(DISTINCT uch.user_id) as member_count,
			MAX(m.created_at) as last_message_at
		FROM conversations c
		LEFT JOIN user_conversation_history uch ON c.id = uch.conversation_id
		LEFT JOIN messages m ON c.id = m.conversation_id
		GROUP BY c.id, c.name, c.type, c.created_at
		ORDER BY last_message_at DESC
	`

	rows, err := h.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rooms := []map[string]interface{}{}
	for rows.Next() {
		var id, name, roomType, createdAt string
		var memberCount int
		var lastMessageAt sql.NullString

		if err := rows.Scan(&id, &name, &roomType, &createdAt, &memberCount, &lastMessageAt); err != nil {
			continue
		}

		room := map[string]interface{}{
			"id":           id,
			"name":         name,
			"type":         roomType,
			"created_at":   createdAt,
			"member_count": memberCount,
		}

		if lastMessageAt.Valid {
			room["last_message_at"] = lastMessageAt.String
		}

		rooms = append(rooms, room)
	}

	return rooms, nil
}

// createCustomRoom creates a new custom room with the creator as owner
func (h *Handler) createCustomRoom(roomName, creatorID string) (string, error) {
	// Check if room already exists
	var existingID string
	err := h.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, roomName).Scan(&existingID)
	if err == nil {
		return "", fmt.Errorf("room '%s' already exists", roomName)
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Create new room
	convID, _ := utils.GenerateID(16)
	now := time.Now().Format("2006-01-02 15:04:05")

	_, err = h.db.Exec(`
		INSERT INTO conversations (id, name, type, created_by, created_at, last_message_at) 
		VALUES (?, ?, 'custom', ?, ?, ?)
	`, convID, roomName, creatorID, now, now)

	if err != nil {
		return "", err
	}

	// Set creator as owner
	_, err = h.db.Exec(`
		INSERT INTO user_conversation_history (user_id, conversation_id, role, joined_at, unread_count) 
		VALUES (?, ?, 'owner', ?, 0)
	`, creatorID, convID, now)

	if err != nil {
		return "", err
	}

	logger.Info("Custom room created", map[string]interface{}{
		"id":      convID,
		"name":    roomName,
		"creator": creatorID,
	})

	return convID, nil
}
