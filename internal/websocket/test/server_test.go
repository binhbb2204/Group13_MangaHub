package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
)

func init() {
	logger.Init(logger.ERROR, false, nil)
	gin.SetMode(gin.TestMode)
}

func setupTestDB(t *testing.T) {
	if err := database.InitDatabase(":memory:"); err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}

	// Create test users to satisfy foreign key constraints
	testUsers := []struct {
		id       string
		username string
		email    string
		password string
	}{
		{"test-user-1", "testuser1", "test1@example.com", "hashedpass1"},
		{"test-user-2", "testuser2", "test2@example.com", "hashedpass2"},
	}

	for _, u := range testUsers {
		_, err := database.DB.Exec(
			`INSERT OR IGNORE INTO users (id, username, email, password_hash, created_at) 
			 VALUES (?, ?, ?, ?, ?)`,
			u.id, u.username, u.email, u.password, time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to create test user %s: %v", u.username, err)
		}
	}
}

func createTestToken(t *testing.T) string {
	token, err := utils.GenerateJWT("test-user-1", "testuser", "user", "test-secret-key-32-characters!!")
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}
	return token
}

func TestWebSocketConnection(t *testing.T) {
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
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	users := server.GetActiveUsers()
	if len(users) != 1 {
		t.Errorf("Expected 1 active user, got %d", len(users))
	}
}

func TestWebSocketPingPong(t *testing.T) {
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

	conn.SetPongHandler(func(appData string) error {
		return nil
	})

	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
}

func TestWebSocketBroadcast(t *testing.T) {
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

	time.Sleep(100 * time.Millisecond)

	msg := map[string]interface{}{
		"type":    "text",
		"content": "Hello, World!",
	}
	msgData, _ := json.Marshal(msg)

	if err := conn1.WriteMessage(ws.TextMessage, msgData); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, receivedMsg, err := conn2.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	if len(receivedMsg) == 0 {
		t.Error("Received empty message")
	}
}

func TestWebSocketDisconnection(t *testing.T) {
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

	time.Sleep(100 * time.Millisecond)

	users := server.GetActiveUsers()
	if len(users) != 1 {
		t.Errorf("Expected 1 active user, got %d", len(users))
	}

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	users = server.GetActiveUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 active users after disconnect, got %d", len(users))
	}
}

func TestWebSocketAuthenticationFailure(t *testing.T) {
	setupTestDB(t)
	server := websocket.NewServer(database.DB, "test-secret-key-32-characters!!")

	router := gin.New()
	router.GET("/ws/chat", server.HandleWebSocket)

	ts := httptest.NewServer(router)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/chat?token=invalid-token"

	conn, resp, err := ws.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		conn.Close()
		t.Fatal("Expected connection to fail with invalid token")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	}
}
