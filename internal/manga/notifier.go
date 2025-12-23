package manga

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
)

type SSEClient struct {
	UserID  string
	Channel chan string
}

type NotificationBroker struct {
	clients map[string]*SSEClient
	mu      sync.RWMutex
}

type NotificationEvent struct {
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

func NewBroker() *NotificationBroker {
	return &NotificationBroker{
		clients: make(map[string]*SSEClient),
	}
}

func (b *NotificationBroker) ServeSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	userID := ""
	token := c.Query("token")
	if token != "" {
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "your-secret-key-change-this-in-production"
		}
		claims, err := utils.ValidateJWT(token, jwtSecret)
		if err == nil {
			userID = claims.UserID
		}
	}

	messageChan := make(chan string, 10)
	connectionID := fmt.Sprintf("%s_%d", c.ClientIP(), time.Now().UnixNano())

	client := &SSEClient{
		UserID:  userID,
		Channel: messageChan,
	}

	b.mu.Lock()
	b.clients[connectionID] = client
	b.mu.Unlock()

	initialEvent := NotificationEvent{
		Type:      "connected",
		Message:   "Connected to MangaHub notifications",
		Timestamp: time.Now().Unix(),
	}
	initialJSON, _ := json.Marshal(initialEvent)
	fmt.Fprintf(c.Writer, "data: %s\n\n", initialJSON)
	c.Writer.Flush()

	defer func() {
		b.mu.Lock()
		delete(b.clients, connectionID)
		close(messageChan)
		b.mu.Unlock()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			heartbeat := NotificationEvent{
				Type:      "heartbeat",
				Timestamp: time.Now().Unix(),
			}
			heartbeatJSON, _ := json.Marshal(heartbeat)
			fmt.Fprintf(c.Writer, "data: %s\n\n", heartbeatJSON)
			c.Writer.Flush()
		case msg := <-messageChan:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Writer.Flush()
		}
	}
}

func (b *NotificationBroker) Broadcast(eventType, message string, data interface{}) {
	event := NotificationEvent{
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, client := range b.clients {
		select {
		case client.Channel <- string(eventJSON):
		default:
		}
	}
}

func (b *NotificationBroker) BroadcastToUser(userID, eventType, message string, data interface{}) {
	if userID == "" {
		return
	}

	event := NotificationEvent{
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, client := range b.clients {
		if client.UserID == userID {
			select {
			case client.Channel <- string(eventJSON):
			default:
			}
		}
	}
}

func (b *NotificationBroker) GetClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
