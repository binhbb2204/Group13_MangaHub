package udp_test

import (
	"encoding/json"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid register message",
			input:   `{"type":"register","data":{"token":"test123"},"timestamp":"2025-11-12T10:00:00Z"}`,
			wantErr: false,
		},
		{
			name:    "valid heartbeat message",
			input:   `{"type":"heartbeat","data":{"client_id":"client1"},"timestamp":"2025-11-12T10:00:00Z"}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "empty message",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := udp.ParseMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && msg == nil {
				t.Error("ParseMessage() returned nil message for valid input")
			}
		})
	}
}

func TestCreateRegisterMessage(t *testing.T) {
	token := "test-token-123"
	msgBytes := udp.CreateRegisterMessage(token)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal register message: %v", err)
	}

	if msg.Type != "register" {
		t.Errorf("Expected type 'register', got '%s'", msg.Type)
	}

	var payload udp.RegisterPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if payload.Token != token {
		t.Errorf("Expected token '%s', got '%s'", token, payload.Token)
	}
}

func TestCreateUnregisterMessage(t *testing.T) {
	msgBytes := udp.CreateUnregisterMessage()

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal unregister message: %v", err)
	}

	if msg.Type != "unregister" {
		t.Errorf("Expected type 'unregister', got '%s'", msg.Type)
	}
}

func TestCreateSubscribeMessage(t *testing.T) {
	eventTypes := []string{"progress_update", "library_update"}
	msgBytes := udp.CreateSubscribeMessage(eventTypes)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal subscribe message: %v", err)
	}

	if msg.Type != "subscribe" {
		t.Errorf("Expected type 'subscribe', got '%s'", msg.Type)
	}

	var payload udp.SubscribePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if len(payload.EventTypes) != len(eventTypes) {
		t.Errorf("Expected %d event types, got %d", len(eventTypes), len(payload.EventTypes))
	}
}

func TestCreateHeartbeatMessage(t *testing.T) {
	clientID := "client-123"
	msgBytes := udp.CreateHeartbeatMessage(clientID)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal heartbeat message: %v", err)
	}

	if msg.Type != "heartbeat" {
		t.Errorf("Expected type 'heartbeat', got '%s'", msg.Type)
	}

	var payload udp.HeartbeatPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if payload.ClientID != clientID {
		t.Errorf("Expected client_id '%s', got '%s'", clientID, payload.ClientID)
	}
}

func TestCreateNotificationMessage(t *testing.T) {
	userID := "user-123"
	eventType := "progress_update"
	data := map[string]interface{}{
		"manga_id":   "manga-1",
		"chapter_id": 42,
		"status":     "reading",
	}

	msgBytes := udp.CreateNotificationMessage(userID, eventType, data)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal notification message: %v", err)
	}

	if msg.Type != "notification" {
		t.Errorf("Expected type 'notification', got '%s'", msg.Type)
	}

	if msg.EventType != eventType {
		t.Errorf("Expected event_type '%s', got '%s'", eventType, msg.EventType)
	}

	if msg.UserID != userID {
		t.Errorf("Expected user_id '%s', got '%s'", userID, msg.UserID)
	}
}

func TestCreateSuccessMessage(t *testing.T) {
	message := "Operation successful"
	msgBytes := udp.CreateSuccessMessage(message)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal success message: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected type 'success', got '%s'", msg.Type)
	}

	var payload udp.SuccessPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if payload.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, payload.Message)
	}
}

func TestCreateErrorMessage(t *testing.T) {
	code := "UDP-004"
	message := "Authentication failed"
	msgBytes := udp.CreateErrorMessage(code, message)

	var msg udp.Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		t.Fatalf("Failed to unmarshal error message: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("Expected type 'error', got '%s'", msg.Type)
	}

	var payload udp.ErrorPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if payload.Code != code {
		t.Errorf("Expected code '%s', got '%s'", code, payload.Code)
	}

	if payload.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, payload.Message)
	}
}
