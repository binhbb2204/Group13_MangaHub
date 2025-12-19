package udp

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEBroker manages SSE connections for frontend notifications
type SSEBroker struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

// SSEEvent represents an SSE event
type SSEEvent struct {
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// NewSSEBroker creates a new SSE broker
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan string]bool),
	}
}

// ServeSSE handles Server-Sent Events connections
func (b *SSEBroker) ServeSSE(c *gin.Context) {
	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Create a channel for this client
	messageChan := make(chan string, 10)

	// Register the client
	b.mu.Lock()
	b.clients[messageChan] = true
	b.mu.Unlock()

	// Send initial connection message
	initialEvent := SSEEvent{
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
			heartbeat := SSEEvent{
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

// Broadcast sends a notification to all connected SSE clients
func (b *SSEBroker) Broadcast(eventType, message string, data interface{}) {
	event := SSEEvent{
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
func (b *SSEBroker) GetClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
