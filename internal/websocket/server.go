package websocket

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

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
	manager   *Manager
	handler   *Handler
	db        *sql.DB
	jwtSecret string
}

func NewServer(db *sql.DB, jwtSecret string) *Server {
	manager := NewManager()
	handler := NewHandler(db, manager)
	go manager.Run()

	return &Server{
		manager:   manager,
		handler:   handler,
		db:        db,
		jwtSecret: jwtSecret,
	}
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

	client := &Client{
		ID:         claims.UserID,
		Username:   claims.Username,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Manager:    s.manager,
		Handler:    s.handler,
		LastActive: time.Now(),
	}

	s.manager.register <- client

	id, _ := utils.GenerateID(16)
	presenceMsg := ServerMessage{ID: id, Type: MessageTypePresence, From: "system", Room: "global", Content: claims.Username + " joined the chat", Timestamp: time.Now()}
	if data, err := json.Marshal(presenceMsg); err == nil {
		s.manager.broadcastRoom("global", data)
	}

	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) GetActiveUsers() []string {
	return s.manager.GetActiveUsers()
}
