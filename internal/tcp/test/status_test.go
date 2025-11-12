package tcp_test

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
)

// TestStatusRequestBasic tests basic status_request message handling
func TestStatusRequestBasic(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9101", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9101", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message to establish session
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn) // Read connect response

	// Wait for heartbeat to be established
	time.Sleep(100 * time.Millisecond)

	// Send status_request
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn, statusRequestMsg)

	// Read and validate status response
	response := readResponse(t, conn)

	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	if msg.Type != "status" {
		t.Errorf("Expected message type 'status', got '%s'", msg.Type)
	}

	var statusPayload struct {
		ConnectionStatus string `json:"connection_status"`
		ServerAddress    string `json:"server_address"`
		Uptime           int64  `json:"uptime_seconds"`
		LastHeartbeat    string `json:"last_heartbeat"`
		SessionID        string `json:"session_id"`
		DevicesOnline    int    `json:"devices_online"`
		MessagesSent     int64  `json:"messages_sent"`
		MessagesReceived int64  `json:"messages_received"`
		NetworkQuality   string `json:"network_quality"`
		RTT              int64  `json:"rtt_ms"`
	}
	if err := json.Unmarshal(msg.Payload, &statusPayload); err != nil {
		t.Fatalf("Failed to parse status payload: %v", err)
	}

	// Validate status fields
	if statusPayload.ConnectionStatus != "active" {
		t.Errorf("Expected connection_status 'active', got '%s'", statusPayload.ConnectionStatus)
	}

	if statusPayload.SessionID == "" {
		t.Error("Expected non-empty session_id")
	}

	if statusPayload.DevicesOnline != 1 {
		t.Errorf("Expected devices_online 1, got %d", statusPayload.DevicesOnline)
	}

	if statusPayload.NetworkQuality == "" {
		t.Error("Expected non-empty network_quality")
	}

	if statusPayload.Uptime < 0 {
		t.Errorf("Expected non-negative uptime, got %d", statusPayload.Uptime)
	}
}

// TestStatusRequestMultipleDevices tests device counting with multiple connections
func TestStatusRequestMultipleDevices(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9102", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	// Connect first device
	conn1 := connectAndAuthenticateClient(t, "9102", "test-user-1", "testuser")
	defer conn1.Close()
	connectMsg1 := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Device 1",
	})
	sendMessage(t, conn1, connectMsg1)
	readResponse(t, conn1) // Read connect response

	// Connect second device (same user)
	conn2 := connectAndAuthenticateClient(t, "9102", "test-user-1", "testuser")
	defer conn2.Close()
	connectMsg2 := createMessage("connect", map[string]interface{}{
		"device_type": "mobile",
		"device_name": "Device 2",
	})
	sendMessage(t, conn2, connectMsg2)
	readResponse(t, conn2) // Read connect response

	// Connect third device (same user)
	conn3 := connectAndAuthenticateClient(t, "9102", "test-user-1", "testuser")
	defer conn3.Close()
	connectMsg3 := createMessage("connect", map[string]interface{}{
		"device_type": "web",
		"device_name": "Device 3",
	})
	sendMessage(t, conn3, connectMsg3)
	readResponse(t, conn3) // Read connect response

	time.Sleep(100 * time.Millisecond)

	// Request status from first device
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn1, statusRequestMsg)

	response := readResponse(t, conn1)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	var statusPayload struct {
		DevicesOnline int `json:"devices_online"`
	}
	if err := json.Unmarshal(msg.Payload, &statusPayload); err != nil {
		t.Fatalf("Failed to parse status payload: %v", err)
	}

	// Should show 3 devices for this user
	if statusPayload.DevicesOnline != 3 {
		t.Errorf("Expected devices_online 3, got %d", statusPayload.DevicesOnline)
	}
}

// TestStatusRequestWithLastSync tests status response with last sync information
func TestStatusRequestWithLastSync(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9103", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9103", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	time.Sleep(100 * time.Millisecond)

	// Sync progress to establish last sync
	syncMsg := createMessage("sync_progress", map[string]interface{}{
		"user_id":         "test-user-1",
		"manga_id":        "manga-1",
		"current_chapter": 42,
		"status":          "reading",
	})
	sendMessage(t, conn, syncMsg)
	readResponse(t, conn) // Read sync response

	time.Sleep(100 * time.Millisecond)

	// Request status
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn, statusRequestMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	var statusPayload struct {
		LastSync *struct {
			MangaID    string `json:"manga_id"`
			MangaTitle string `json:"manga_title"`
			Chapter    int    `json:"chapter"`
			Timestamp  string `json:"timestamp"`
		} `json:"last_sync,omitempty"`
	}
	if err := json.Unmarshal(msg.Payload, &statusPayload); err != nil {
		t.Fatalf("Failed to parse status payload: %v", err)
	}

	// Validate last sync information
	if statusPayload.LastSync == nil {
		t.Fatal("Expected last_sync information, got nil")
	}

	if statusPayload.LastSync.MangaID != "manga-1" {
		t.Errorf("Expected manga_id 'manga-1', got '%s'", statusPayload.LastSync.MangaID)
	}

	if statusPayload.LastSync.MangaTitle != "Test Manga" {
		t.Errorf("Expected manga_title 'Test Manga', got '%s'", statusPayload.LastSync.MangaTitle)
	}

	if statusPayload.LastSync.Chapter != 42 {
		t.Errorf("Expected chapter 42, got %d", statusPayload.LastSync.Chapter)
	}

	if statusPayload.LastSync.Timestamp == "" {
		t.Error("Expected non-empty timestamp")
	}
}

// TestStatusRequestWithoutAuthentication tests status request without auth
func TestStatusRequestWithoutAuthentication(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9104", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9104")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send status_request without authentication
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn, statusRequestMsg)

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

// TestStatusRequestWithoutSession tests status request without established session
func TestStatusRequestWithoutSession(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9105", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9105", "test-user-1", "testuser")
	defer conn.Close()

	// Send status_request without connecting (no session)
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn, statusRequestMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should receive error message about no active session
	if msg.Type != "error" {
		t.Errorf("Expected message type 'error', got '%s'", msg.Type)
	}
}

// TestStatusRequestNetworkQuality tests network quality calculation
func TestStatusRequestNetworkQuality(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9106", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9106", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	// Send heartbeat to establish RTT
	heartbeatMsg := createMessage("heartbeat", map[string]interface{}{})
	sendMessage(t, conn, heartbeatMsg)
	readResponse(t, conn)

	time.Sleep(100 * time.Millisecond)

	// Request status multiple times to check consistency
	for i := 0; i < 3; i++ {
		statusRequestMsg := createMessage("status_request", map[string]interface{}{})
		sendMessage(t, conn, statusRequestMsg)

		response := readResponse(t, conn)
		var msg tcp.Message
		if err := json.Unmarshal(response, &msg); err != nil {
			t.Fatalf("Failed to parse status response (iteration %d): %v", i, err)
		}

		var statusPayload struct {
			NetworkQuality string `json:"network_quality"`
			RTT            int64  `json:"rtt_ms"`
		}
		if err := json.Unmarshal(msg.Payload, &statusPayload); err != nil {
			t.Fatalf("Failed to parse status payload (iteration %d): %v", i, err)
		}

		// Network quality should be one of the valid values
		validQualities := map[string]bool{
			"Excellent": true,
			"Good":      true,
			"Fair":      true,
			"Poor":      true,
			"Unknown":   true,
		}
		if !validQualities[statusPayload.NetworkQuality] {
			t.Errorf("Invalid network quality: %s", statusPayload.NetworkQuality)
		}

		// RTT should be reasonable for local connection (< 100ms typically)
		if statusPayload.RTT < 0 {
			t.Errorf("Expected non-negative RTT, got %d", statusPayload.RTT)
		}
		if statusPayload.RTT > 1000 {
			t.Logf("Warning: High RTT detected: %dms (expected < 100ms for local connection)", statusPayload.RTT)
		}
	}
}

// TestStatusRequestMessageCounts tests that message counts are tracked correctly
func TestStatusRequestMessageCounts(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9107", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn := connectAndAuthenticateClient(t, "9107", "test-user-1", "testuser")
	defer conn.Close()

	// Send connect message
	connectMsg := createMessage("connect", map[string]interface{}{
		"device_type": "desktop",
		"device_name": "Test Device",
	})
	sendMessage(t, conn, connectMsg)
	readResponse(t, conn)

	// Send multiple sync messages
	for i := 0; i < 5; i++ {
		syncMsg := createMessage("sync_progress", map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "manga-1",
			"current_chapter": 10 + i,
			"status":          "reading",
		})
		sendMessage(t, conn, syncMsg)
		readResponse(t, conn)
	}

	time.Sleep(100 * time.Millisecond)

	// Request status
	statusRequestMsg := createMessage("status_request", map[string]interface{}{})
	sendMessage(t, conn, statusRequestMsg)

	response := readResponse(t, conn)
	var msg tcp.Message
	if err := json.Unmarshal(response, &msg); err != nil {
		t.Fatalf("Failed to parse status response: %v", err)
	}

	var statusPayload struct {
		MessagesSent     int64 `json:"messages_sent"`
		MessagesReceived int64 `json:"messages_received"`
	}
	if err := json.Unmarshal(msg.Payload, &statusPayload); err != nil {
		t.Fatalf("Failed to parse status payload: %v", err)
	}

	// Should have sent at least 5 sync messages
	if statusPayload.MessagesSent < 5 {
		t.Errorf("Expected messages_sent >= 5, got %d", statusPayload.MessagesSent)
	}

	// Should have received responses
	if statusPayload.MessagesReceived < 5 {
		t.Errorf("Expected messages_received >= 5, got %d", statusPayload.MessagesReceived)
	}
}
