package websocket

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	pingPeriod     = 30 * time.Second
	pongWait       = 60 * time.Second
	writeWait      = 10 * time.Second
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Server struct {
	manager     *Manager
	handler     *Handler
	db          *sql.DB
	jwtSecret   string
	broadcaster *WSBroadcaster
	bridge      *bridge.UnifiedBridge
}

func NewServer(db *sql.DB, jwtSecret string) *Server {
	manager := NewManager()
	handler := NewHandler(db, manager)
	broadcaster := NewWSBroadcaster(manager, logger.GetLogger())
	go manager.Run()

	return &Server{
		manager:     manager,
		handler:     handler,
		db:          db,
		jwtSecret:   jwtSecret,
		broadcaster: broadcaster,
		bridge:      nil,
	}
}

func (s *Server) SetBridge(b *bridge.UnifiedBridge) {
	s.bridge = b
	s.broadcaster.SetBridge(b)
	logger.Info("ws_server_bridge_set")
}

func (s *Server) HandleWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}

	claims, err := utils.ValidateJWT(token, s.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WebSocket upgrade failed", map[string]interface{}{"error": err.Error()})
		return
	}

	now := time.Now()
	client := &Client{
		ID:          claims.UserID,
		Username:    claims.Username,
		Conn:        conn,
		Send:        make(chan []byte, 256),
		Manager:     s.manager,
		Handler:     s.handler,
		LastActive:  now,
		ConnectedAt: now,
	}

	s.manager.register <- client

	connID := ""
	if s.bridge != nil {
		connID = s.bridge.RegisterProtocolClient(conn, claims.UserID, bridge.ProtocolWebSocket)
		s.broadcaster.RegisterConnection(connID, client)
		logger.Info("ws_client_registered_with_bridge", "user_id", claims.UserID, "conn_id", connID)
	}

	id, _ := utils.GenerateID(16)
	presenceMsg := ServerMessage{ID: id, Type: MessageTypePresence, From: "system", Room: "global", Content: claims.Username + " joined the chat", Timestamp: time.Now()}
	if data, err := json.Marshal(presenceMsg); err == nil {
		s.manager.broadcastRoom("global", data)
	}

	// Send welcome message after broadcast
	go s.sendWelcomeMessage(client)

	go client.WritePump()
	go client.ReadPump(connID, s.bridge, s.broadcaster)
}

func (s *Server) sendWelcomeMessage(client *Client) {
	defer func() {
		if r := recover(); r != nil {
			// Silent fail - client likely disconnected (e.g., quick send command)
		}
	}()

	time.Sleep(100 * time.Millisecond) // Allow client to be fully registered

	// Don't bother sending welcome if client disconnected quickly
	if time.Since(client.ConnectedAt) > 2*time.Second {
		userCount := s.manager.GetRoomClientCount("global")
		recentMessages, _ := s.handler.GetMessageHistory(client.ID, 5)

		id, _ := utils.GenerateID(16)
		welcomeMsg := ServerMessage{
			ID:        id,
			Type:      MessageTypeWelcome,
			From:      "system",
			Room:      "global",
			Content:   "Connected to General Chat",
			Timestamp: time.Now(),
			Metadata: map[string]interface{}{
				"user_count":      userCount,
				"room":            "general",
				"recent_messages": recentMessages,
			},
		}

		data, err := json.Marshal(welcomeMsg)
		if err == nil {
			select {
			case client.Send <- data:
			default:
				// Channel is full or closed, skip welcome message
			}
		}
	}
}

func (s *Server) GetActiveUsers() []string {
	return s.manager.GetActiveUsers()
}
