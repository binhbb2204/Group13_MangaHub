package udp

import (
	"encoding/json"
	"time"
)

type Message struct {
	Type      string          `json:"type"`
	EventType string          `json:"event_type,omitempty"`
	UserID    string          `json:"user_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp string          `json:"timestamp"`
}

type RegisterPayload struct {
	Token string `json:"token"`
}

type SubscribePayload struct {
	EventTypes []string `json:"event_types"`
}

type HeartbeatPayload struct {
	ClientID string `json:"client_id"`
}

type NotificationData struct {
	MangaID   string `json:"manga_id,omitempty"`
	ChapterID int    `json:"chapter_id,omitempty"`
	Status    string `json:"status,omitempty"`
	Action    string `json:"action,omitempty"`
}

type SuccessPayload struct {
	Message string `json:"message"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func CreateRegisterMessage(token string) []byte {
	payload := RegisterPayload{Token: token}
	msg := Message{
		Type:      "register",
		Data:      mustMarshal(payload),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateUnregisterMessage() []byte {
	msg := Message{
		Type:      "unregister",
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateSubscribeMessage(eventTypes []string) []byte {
	payload := SubscribePayload{EventTypes: eventTypes}
	msg := Message{
		Type:      "subscribe",
		Data:      mustMarshal(payload),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateHeartbeatMessage(clientID string) []byte {
	payload := HeartbeatPayload{ClientID: clientID}
	msg := Message{
		Type:      "heartbeat",
		Data:      mustMarshal(payload),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateNotificationMessage(userID, eventType string, data interface{}) []byte {
	msg := Message{
		Type:      "notification",
		EventType: eventType,
		UserID:    userID,
		Data:      mustMarshal(data),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateSuccessMessage(message string) []byte {
	payload := SuccessPayload{Message: message}
	msg := Message{
		Type:      "success",
		Data:      mustMarshal(payload),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func CreateErrorMessage(code, message string) []byte {
	payload := ErrorPayload{Code: code, Message: message}
	msg := Message{
		Type:      "error",
		Data:      mustMarshal(payload),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	return mustMarshal(msg)
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
