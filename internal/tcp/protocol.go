package tcp

import (
	"encoding/json"
	"errors"
	"time"
)

type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type AuthPayload struct {
	Token string `json:"token"`
}

type SyncProgressPayload struct {
	UserID         string `json:"user_id"`
	MangaID        string `json:"manga_id"`
	CurrentChapter int    `json:"current_chapter"`
	Status         string `json:"status"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

type SuccessPayload struct {
	Message string `json:"message"`
}

type GetLibraryPayload struct {
}

type GetProgressPayload struct {
	MangaID string `json:"manga_id"`
}

type AddToLibraryPayload struct {
	MangaID string `json:"manga_id"`
	Status  string `json:"status"`
}

type RemoveFromLibraryPayload struct {
	MangaID string `json:"manga_id"`
}

// ConnectPayload is sent when establishing a sync connection
type ConnectPayload struct {
	DeviceType string `json:"device_type"` // "mobile", "desktop", "web"
	DeviceName string `json:"device_name"` // User-friendly device name
}

// DisconnectPayload is sent when gracefully closing a connection
type DisconnectPayload struct {
	Reason string `json:"reason,omitempty"` // Optional disconnect reason
}

// StatusRequestPayload requests connection status information
type StatusRequestPayload struct {
}

// StatusResponsePayload returns connection status information
type StatusResponsePayload struct {
	ConnectionStatus string        `json:"connection_status"` // "active", "disconnected"
	ServerAddress    string        `json:"server_address"`
	Uptime           int64         `json:"uptime_seconds"` // Connection uptime in seconds
	LastHeartbeat    string        `json:"last_heartbeat"` // ISO timestamp
	SessionID        string        `json:"session_id"`
	DevicesOnline    int           `json:"devices_online"`
	MessagesSent     int64         `json:"messages_sent"`
	MessagesReceived int64         `json:"messages_received"`
	LastSync         *LastSyncInfo `json:"last_sync,omitempty"`
	NetworkQuality   string        `json:"network_quality"` // "Excellent", "Good", etc.
	RTT              int64         `json:"rtt_ms"`          // Round-trip time in milliseconds
}

// LastSyncInfo contains information about the last sync operation
type LastSyncInfo struct {
	MangaID    string `json:"manga_id"`
	MangaTitle string `json:"manga_title"`
	Chapter    int    `json:"chapter"`
	Timestamp  string `json:"timestamp"` // ISO timestamp
}

// SubscribeUpdatesPayload subscribes to real-time updates
type SubscribeUpdatesPayload struct {
	EventTypes []string `json:"event_types,omitempty"` // Optional filter: ["progress", "library"]
}

// UnsubscribeUpdatesPayload unsubscribes from real-time updates
type UnsubscribeUpdatesPayload struct {
}

// UpdateEventPayload is sent for real-time sync updates
type UpdateEventPayload struct {
	Timestamp   string `json:"timestamp"`   // ISO timestamp
	Direction   string `json:"direction"`   // "incoming" or "outgoing"
	DeviceType  string `json:"device_type"` // Source device type
	DeviceName  string `json:"device_name"` // Source device name
	MangaTitle  string `json:"manga_title"`
	Chapter     int    `json:"chapter"`
	Action      string `json:"action"`                 // "updated", "added", "removed"
	ConflictMsg string `json:"conflict_msg,omitempty"` // If there was a conflict
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if msg.Type == "" {
		return nil, errors.New("message type is required")
	}
	return &msg, nil
}

func CreateErrorMessage(errMsg string) []byte {
	msg := Message{
		Type:    "error",
		Payload: json.RawMessage(`{"message":"` + errMsg + `"}`),
	}
	data, _ := json.Marshal(msg)
	return append(data, '\n')
}

func CreateSuccessMessage(successMsg string) []byte {
	msg := Message{
		Type:    "success",
		Payload: json.RawMessage(`{"message":"` + successMsg + `"}`),
	}
	data, _ := json.Marshal(msg)
	return append(data, '\n')
}

func CreatePongMessage() []byte {
	msg := Message{
		Type:    "pong",
		Payload: json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(msg)
	return append(data, '\n')
}

func CreateDataMessage(msgType string, data interface{}) []byte {
	payload, _ := json.Marshal(data)
	msg := Message{
		Type:    msgType,
		Payload: json.RawMessage(payload),
	}
	msgData, _ := json.Marshal(msg)
	return append(msgData, '\n')
}

// CreateHeartbeatMessage creates a heartbeat message
func CreateHeartbeatMessage() []byte {
	msg := Message{
		Type:    "heartbeat",
		Payload: json.RawMessage(`{}`),
	}
	data, _ := json.Marshal(msg)
	return append(data, '\n')
}

// CreateConnectResponseMessage creates a successful connection response
func CreateConnectResponseMessage(sessionID string, deviceType string) []byte {
	response := map[string]interface{}{
		"session_id":   sessionID,
		"device_type":  deviceType,
		"connected_at": jsonTimestamp(),
	}
	return CreateDataMessage("connected", response)
}

// CreateDisconnectResponseMessage creates a disconnect acknowledgment
func CreateDisconnectResponseMessage() []byte {
	return CreateSuccessMessage("Disconnected successfully")
}

// CreateStatusResponseMessage creates a status response message
func CreateStatusResponseMessage(status StatusResponsePayload) []byte {
	return CreateDataMessage("status", status)
}

// CreateUpdateEventMessage creates a real-time update event message
func CreateUpdateEventMessage(event UpdateEventPayload) []byte {
	return CreateDataMessage("update_event", event)
}

// jsonTimestamp returns current time in ISO format
func jsonTimestamp() string {
	return time.Now().Format(time.RFC3339)
}
