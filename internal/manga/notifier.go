package manga

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// NotificationBroker manages SSE connections and broadcasts events
type NotificationBroker struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

// NotificationEvent represents a notification event
type NotificationEvent struct {
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// NewBroker creates a new notification broker
func NewBroker() *NotificationBroker {
	return &NotificationBroker{
		clients: make(map[chan string]bool),
	}
}

// ServeSSE handles Server-Sent Events connections
func (b *NotificationBroker) ServeSSE(c *gin.Context) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	// Create a channel for this client
	messageChan := make(chan string, 10)

	// Register the client
	b.mu.Lock()
	b.clients[messageChan] = true
	b.mu.Unlock()

	// Send initial connection message
	initialEvent := NotificationEvent{
		Type:      "connected",
		Message:   "Connected to MangaHub notifications",
		Timestamp: time.Now().Unix(),
	}
	initialJSON, _ := json.Marshal(initialEvent)
	fmt.Fprintf(c.Writer, "data: %s\n\n", initialJSON)
	c.Writer.Flush()

	// Set up cleanup on disconnect
	defer func() {
		b.mu.Lock()
		delete(b.clients, messageChan)
		close(messageChan)
		b.mu.Unlock()
	}()

	// Keep connection alive with heartbeat
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Listen for messages and client disconnect
	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			// Send heartbeat
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

// Broadcast sends a notification to all connected clients
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

	for clientChan := range b.clients {
		select {
		case clientChan <- string(eventJSON):
		default:
			// Client channel full, skip
		}
	}
}

// GetClientCount returns the number of connected clients
func (b *NotificationBroker) GetClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
