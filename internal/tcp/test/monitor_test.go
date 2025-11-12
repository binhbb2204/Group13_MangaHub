package tcp_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

// TestSubscribeUpdatesBasic tests basic subscription functionality
func TestSubscribeUpdatesBasic(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9201", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9201", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	// Subscribe to updates
	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress", "library"},
	})
	sendMessage(t, conn, subscribeMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse subscribe response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected message type 'success', got '%s'", msg.Type)
	}
}

// TestSubscribeWithoutAuthentication tests subscription without auth
func TestSubscribeWithoutAuthentication(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9202", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9202")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Try to subscribe without authentication
	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, conn, subscribeMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should receive error message
	if msg.Type != "error" {
		t.Errorf("Expected message type 'error', got '%s'", msg.Type)
	}
}

// TestUnsubscribeUpdates tests unsubscribe functionality
func TestUnsubscribeUpdates(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9203", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9203", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	// Subscribe to updates
	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, conn, subscribeMsg)
	readResponse(t, conn)

	// Unsubscribe from updates
	unsubscribeMsg := createMessage("unsubscribe_updates", map[string]interface{}{})
	sendMessage(t, conn, unsubscribeMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse unsubscribe response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected message type 'success', got '%s'", msg.Type)
	}
}

// TestEventBroadcasting tests that events are broadcast to subscribed clients
func TestEventBroadcasting(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9204", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect and subscribe first device (monitoring device)
	monitorConn := connectAndAuthenticateClient(t, "9204", "test-user-1", "testuser")
	defer monitorConn.Close()

	connectMsg1 := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Monitor Device",
	})
	sendMessage(t, monitorConn, connectMsg1)
	readResponse(t, monitorConn)

	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, monitorConn, subscribeMsg)
	readResponse(t, monitorConn)

	// Connect second device (updating device)
	updateConn := connectAndAuthenticateClient(t, "9204", "test-user-1", "testuser")
	defer updateConn.Close()

	connectMsg2 := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "Update Device",
	})
	sendMessage(t, updateConn, connectMsg2)
	readResponse(t, updateConn)

	// Start monitoring in background
	eventReceived := make(chan bool, 1)
	var receivedEvent map[string]interface{}
	var mu sync.Mutex

	go func() {
		scanner := bufio.NewScanner(monitorConn)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var msg tcp.Message
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			if msg.Type == "update_event" {
				mu.Lock()
				json.Unmarshal(msg.Payload, &receivedEvent)
				mu.Unlock()
				eventReceived <- true
				return
			}
		}
	}()

	// Update device sends sync_progress
	time.Sleep(100 * time.Millisecond)
	syncMsg := createMessage("sync_progress", map[string]interface{}{
		"user_id":         "test-user-1",
		"manga_id":        "manga-1",
		"current_chapter": 50,
		"status":          "reading",
	})
	sendMessage(t, updateConn, syncMsg)
	readResponse(t, updateConn)

	// Wait for event to be received
	select {
	case <-eventReceived:
		mu.Lock()
		defer mu.Unlock()

		// Validate event structure
		if receivedEvent["action"] != "updated" {
			t.Errorf("Expected action 'updated', got '%v'", receivedEvent["action"])
		}

		if receivedEvent["manga_title"] != "Test Manga" {
			t.Errorf("Expected manga_title 'Test Manga', got '%v'", receivedEvent["manga_title"])
		}

		chapterFloat, ok := receivedEvent["chapter"].(float64)
		if !ok || int(chapterFloat) != 50 {
			t.Errorf("Expected chapter 50, got %v", receivedEvent["chapter"])
		}

	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for update event")
	}
}

// TestMultipleSubscribers tests broadcasting to multiple subscribed clients
func TestMultipleSubscribers(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9205", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect three monitoring devices
	monitors := make([]net.Conn, 3)
	eventChannels := make([]chan bool, 3)

	for i := 0; i < 3; i++ {
		monitors[i] = connectAndAuthenticateClient(t, "9205", "test-user-1", "testuser")
		defer monitors[i].Close()

		connectMsg := createMessage("connect", map[string]interface{}{
			"device_type": "desktop",
			"device_name": "Monitor " + string(rune('1'+i)),
		})
		sendMessage(t, monitors[i], connectMsg)
		readResponse(t, monitors[i])

		subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
			"event_types": []string{"progress"},
		})
		sendMessage(t, monitors[i], subscribeMsg)
		readResponse(t, monitors[i])

		// Start monitoring
		eventChannels[i] = make(chan bool, 1)
		go monitorEvents(monitors[i], eventChannels[i])
	}

	// Connect update device
	updateConn := connectAndAuthenticateClient(t, "9205", "test-user-1", "testuser")
	defer updateConn.Close()

	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "Update Device",
	})
	sendMessage(t, updateConn, connectMsg)
	readResponse(t, updateConn)

	// Send sync update
	time.Sleep(100 * time.Millisecond)
	syncMsg := createMessage("sync_progress", map[string]interface{}{
		"user_id":         "test-user-1",
		"manga_id":        "manga-1",
		"current_chapter": 75,
		"status":          "reading",
	})
	sendMessage(t, updateConn, syncMsg)
	readResponse(t, updateConn)

	// All monitors should receive the event
	timeout := time.After(3 * time.Second)
	receivedCount := 0

	for i := 0; i < 3; i++ {
		select {
		case <-eventChannels[i]:
			receivedCount++
		case <-timeout:
			t.Errorf("Timeout waiting for event on monitor %d", i+1)
		}
	}

	if receivedCount != 3 {
		t.Errorf("Expected 3 monitors to receive event, got %d", receivedCount)
	}
}

// TestEventFilteringByUser tests that events are only sent to the correct user
func TestEventFilteringByUser(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	// Insert second test user
	_, err := database.DB.Exec(`
		INSERT INTO users (id, username, email, password_hash) 
		VALUES ('test-user-2', 'testuser2', 'test2@example.com', 'hash456')
	`)
	if err != nil {
		t.Fatalf("Failed to insert second test user: %v", err)
	}

	server := tcp.NewServer("9206", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect user 1 monitor
	user1Monitor := connectAndAuthenticateClient(t, "9206", "test-user-1", "testuser")
	defer user1Monitor.Close()

	connectMsg1 := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "User1 Monitor",
	})
	sendMessage(t, user1Monitor, connectMsg1)
	readResponse(t, user1Monitor)

	subscribeMsg1 := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, user1Monitor, subscribeMsg1)
	readResponse(t, user1Monitor)

	// Connect user 2 monitor
	user2Monitor := connectAndAuthenticateClient(t, "9206", "test-user-2", "testuser2")
	defer user2Monitor.Close()

	connectMsg2 := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "User2 Monitor",
	})
	sendMessage(t, user2Monitor, connectMsg2)
	readResponse(t, user2Monitor)

	subscribeMsg2 := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, user2Monitor, subscribeMsg2)
	readResponse(t, user2Monitor)

	// Start monitoring both
	user1Events := make(chan bool, 1)
	user2Events := make(chan bool, 1)

	go monitorEvents(user1Monitor, user1Events)
	go monitorEvents(user2Monitor, user2Events)

	// User 1 sends update
	time.Sleep(100 * time.Millisecond)
	user1Update := connectAndAuthenticateClient(t, "9206", "test-user-1", "testuser")
	defer user1Update.Close()

	connectMsg3 := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "User1 Update",
	})
	sendMessage(t, user1Update, connectMsg3)
	readResponse(t, user1Update)

	syncMsg := createMessage("sync_progress", map[string]interface{}{
		"user_id":         "test-user-1",
		"manga_id":        "manga-1",
		"current_chapter": 99,
		"status":          "reading",
	})
	sendMessage(t, user1Update, syncMsg)
	readResponse(t, user1Update)

	// User 1 should receive event, user 2 should not
	select {
	case <-user1Events:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("User 1 did not receive event")
	}

	select {
	case <-user2Events:
		t.Error("User 2 should not have received event for user 1")
	case <-time.After(500 * time.Millisecond):
		// Expected - no event for user 2
	}
}

// TestConcurrentMonitoring tests multiple clients monitoring simultaneously
func TestConcurrentMonitoring(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9207", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numMonitors := 5
	monitors := make([]net.Conn, numMonitors)
	eventChannels := make([]chan bool, numMonitors)
	var wg sync.WaitGroup

	// Connect multiple monitors
	for i := 0; i < numMonitors; i++ {
		monitors[i] = connectAndAuthenticateClient(t, "9207", "test-user-1", "testuser")
		defer monitors[i].Close()

		connectMsg := createMessage("connect", map[string]interface{}{
			"device_type": "desktop",
			"device_name": "Monitor " + string(rune('A'+i)),
		})
		sendMessage(t, monitors[i], connectMsg)
		readResponse(t, monitors[i])

		subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
			"event_types": []string{"progress"},
		})
		sendMessage(t, monitors[i], subscribeMsg)
		readResponse(t, monitors[i])

		eventChannels[i] = make(chan bool, 1)
		wg.Add(1)
		go func(conn net.Conn, ch chan bool) {
			defer wg.Done()
			monitorEvents(conn, ch)
		}(monitors[i], eventChannels[i])
	}

	// Connect update device
	updateConn := connectAndAuthenticateClient(t, "9207", "test-user-1", "testuser")
	defer updateConn.Close()

	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "Update Device",
	})
	sendMessage(t, updateConn, connectMsg)
	readResponse(t, updateConn)

	// Send multiple updates rapidly
	time.Sleep(100 * time.Millisecond)
	for chapter := 1; chapter <= 10; chapter++ {
		syncMsg := createMessage("sync_progress", map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "manga-1",
			"current_chapter": chapter,
			"status":          "reading",
		})
		sendMessage(t, updateConn, syncMsg)
		readResponse(t, updateConn)
		time.Sleep(50 * time.Millisecond)
	}

	// All monitors should receive at least one event
	timeout := time.After(5 * time.Second)
	for i := 0; i < numMonitors; i++ {
		select {
		case <-eventChannels[i]:
			// Success
		case <-timeout:
			t.Errorf("Monitor %d did not receive any events", i)
		}
	}
}

// TestSubscribeDefaultEventTypes tests subscribing with no event types specified
func TestSubscribeDefaultEventTypes(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9208", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9208", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	// Subscribe without specifying event types (should default to all)
	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{})
	sendMessage(t, conn, subscribeMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse subscribe response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected message type 'success', got '%s'", msg.Type)
	}
}

// TestUpdateEventPayloadStructure tests the structure of update events
func TestUpdateEventPayloadStructure(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9209", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect monitor
	monitorConn := connectAndAuthenticateClient(t, "9209", "test-user-1", "testuser")
	defer monitorConn.Close()

	connectMsg1 := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Monitor Device",
	})
	sendMessage(t, monitorConn, connectMsg1)
	readResponse(t, monitorConn)

	subscribeMsg := createMessage("subscribe_updates", map[string]interface{}{
		"event_types": []string{"progress"},
	})
	sendMessage(t, monitorConn, subscribeMsg)
	readResponse(t, monitorConn)

	// Connect update device
	updateConn := connectAndAuthenticateClient(t, "9209", "test-user-1", "testuser")
	defer updateConn.Close()

	connectMsg2 := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "Update Device",
	})
	sendMessage(t, updateConn, connectMsg2)
	readResponse(t, updateConn)

	// Monitor for event
	eventReceived := make(chan map[string]interface{}, 1)
	go func() {
		scanner := bufio.NewScanner(monitorConn)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var msg tcp.Message
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			if msg.Type == "update_event" {
				var event map[string]interface{}
				json.Unmarshal(msg.Payload, &event)
				eventReceived <- event
				return
			}
		}
	}()

	// Send update
	time.Sleep(100 * time.Millisecond)
	syncMsg := createMessage("sync_progress", map[string]interface{}{
		"user_id":         "test-user-1",
		"manga_id":        "manga-1",
		"current_chapter": 33,
		"status":          "reading",
	})
	sendMessage(t, updateConn, syncMsg)
	readResponse(t, updateConn)

	// Validate event structure
	select {
	case event := <-eventReceived:
		// Check required fields
		requiredFields := []string{"timestamp", "direction", "device_type", "device_name", "manga_title", "chapter", "action"}
		for _, field := range requiredFields {
			if _, ok := event[field]; !ok {
				t.Errorf("Event missing required field: %s", field)
			}
		}

		// Validate timestamp format
		timestamp, ok := event["timestamp"].(string)
		if !ok || timestamp == "" {
			t.Error("Invalid or empty timestamp")
		}

		// Validate direction
		direction, ok := event["direction"].(string)
		if !ok || (direction != "incoming" && direction != "outgoing") {
			t.Errorf("Invalid direction: %v", event["direction"])
		}

		// Validate action
		action, ok := event["action"].(string)
		if !ok || action == "" {
			t.Error("Invalid or empty action")
		}

	case <-time.After(3 * time.Second):
		t.Error("Timeout waiting for update event")
	}
}

// Helper function to monitor events
func monitorEvents(conn net.Conn, eventCh chan bool) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg tcp.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Type == "update_event" {
			eventCh <- true
			return
		}
	}
}

// Helper functions for testing

func connectAndAuthenticateClient(t *testing.T, port, userID, username string) net.Conn {
	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT(userID, username, jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	authMsg := createMessage("auth", map[string]string{"token": token})
	sendMessage(t, conn, authMsg)

	// Read auth response
	readResponse(t, conn)

	return conn
}

func createMessage(msgType string, payload interface{}) []byte {
	payloadBytes, _ := json.Marshal(payload)
	msg := map[string]interface{}{
		"type":    msgType,
		"payload": json.RawMessage(payloadBytes),
	}
	msgBytes, _ := json.Marshal(msg)
	return append(msgBytes, '\n')
}

func sendMessage(t *testing.T, conn net.Conn, message []byte) {
	_, err := conn.Write(message)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
}

func readResponse(t *testing.T, conn net.Conn) []byte {
	buffer := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	return buffer[:n]
}
