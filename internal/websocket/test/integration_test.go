package test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

func TestMessageValidation(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	token := createTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=" + token

	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	longContent := strings.Repeat("a", 5000)
	msg := map[string]interface{}{
		"type":    "text",
		"content": longContent,
	}
	msgData, _ := json.Marshal(msg)

	if err := conn.WriteMessage(ws.TextMessage, msgData); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestTextMessageFlow(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	token := createTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=" + token

	conn1, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect client 2: %v", err)
	}
	defer conn2.Close()

	time.Sleep(200 * time.Millisecond)

	done := make(chan bool)
	go func() {
		conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, msg, err := conn2.ReadMessage()
			if err != nil {
				return
			}
			var serverMsg map[string]interface{}
			if err := json.Unmarshal(msg, &serverMsg); err == nil {
				if serverMsg["type"] == "text" && serverMsg["content"] == "Test message" {
					done <- true
					return
				}
			}
		}
	}()

	msg := map[string]interface{}{
		"type":    "text",
		"content": "Test message",
	}
	msgData, _ := json.Marshal(msg)

	if err := conn1.WriteMessage(ws.TextMessage, msgData); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestEmptyMessageValidation(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	token := createTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=" + token

	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	msg := map[string]interface{}{
		"type":    "text",
		"content": "",
	}
	msgData, _ := json.Marshal(msg)

	if err := conn.WriteMessage(ws.TextMessage, msgData); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestGetMessageHistoryPagination(t *testing.T) {
	setupTestDB(t)

	// insert 5 messages for test-user-1 with increasing timestamps
	now := time.Now()
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("msg-%d", i)
		content := fmt.Sprintf("message-%d", i)
		created := now.Add(time.Duration(i) * time.Second)
		if _, err := database.DB.Exec(
			`INSERT INTO chat_messages (id, from_user_id, to_user_id, content, created_at) VALUES (?, ?, ?, ?, ?)`,
			id, "test-user-1", nil, content, created,
		); err != nil {
			t.Fatalf("failed to insert message %d: %v", i, err)
		}
	}

	h := websocket.NewHandler(database.DB, nil)

	msgs, err := h.GetMessageHistory("test-user-1", 3)
	if err != nil {
		t.Fatalf("GetMessageHistory error: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}
	// messages should be returned in descending order by created_at
	if msgs[0].Content != "message-5" {
		t.Fatalf("expected newest message content 'message-5', got '%s'", msgs[0].Content)
	}
}

func TestRoomIsolation(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	token := createTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=" + token

	connGlobal, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect global client: %v", err)
	}
	defer connGlobal.Close()

	connAnime, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect anime client: %v", err)
	}
	defer connAnime.Close()

	// Drain initial presence messages
	time.Sleep(150 * time.Millisecond)

	receivedInGlobal := make(chan bool, 1)
	go func() {
		connGlobal.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		for {
			_, data, err := connGlobal.ReadMessage()
			if err != nil {
				return
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				if msg["type"] == "text" && msg["room"] == "anime" {
					receivedInGlobal <- true
					return
				}
			}
		}
	}()

	// Send a message to room 'anime' from second client
	payload := map[string]interface{}{"type": "text", "content": "Anime room message", "room": "anime"}
	b, _ := json.Marshal(payload)
	if err := connAnime.WriteMessage(ws.TextMessage, b); err != nil {
		t.Fatalf("Failed to send anime room message: %v", err)
	}

	// Confirm the sender receives its own message (room echo)
	connAnime.SetReadDeadline(time.Now().Add(1 * time.Second))
	sawOwn := false
	for !sawOwn {
		_, data, err := connAnime.ReadMessage()
		if err != nil {
			t.Fatalf("anime client did not receive its own message: %v", err)
		}
		var msg map[string]interface{}
		if json.Unmarshal(data, &msg) == nil {
			if msg["type"] == "text" && msg["content"] == "Anime room message" && msg["room"] == "anime" {
				sawOwn = true
				break
			}
		}
	}

	select {
	case <-receivedInGlobal:
		t.Fatalf("Global client should not receive anime room message")
	case <-time.After(700 * time.Millisecond):
		// success: no leakage
	}
}

func TestRateLimiting(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	token := createTestToken(t)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=" + token

	conn, _, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(150 * time.Millisecond)

	// send 25 messages quickly to exceed 20/10s rate
	for i := 0; i < 25; i++ {
		payload := map[string]interface{}{"type": "text", "content": "spam", "room": "global"}
		b, _ := json.Marshal(payload)
		if err := conn.WriteMessage(ws.TextMessage, b); err != nil {
			t.Fatalf("send failed: %v", err)
		}
	}

	deadline := time.Now().Add(1 * time.Second)
	sawRateLimit := false
	for time.Now().Before(deadline) {
		conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg map[string]interface{}
		if json.Unmarshal(data, &msg) == nil {
			if msg["type"] == "system" && msg["content"] == "rate limit exceeded" {
				sawRateLimit = true
				break
			}
		}
	}
	if !sawRateLimit {
		t.Fatalf("expected rate limit system message but did not see it")
	}
}
