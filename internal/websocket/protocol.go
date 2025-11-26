package websocket

import "time"

type MessageType string

const (
	MessageTypeText     MessageType = "text"
	MessageTypeSystem   MessageType = "system"
	MessageTypeTyping   MessageType = "typing"
	MessageTypePresence MessageType = "presence"
)

type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	From      string                 `json:"from"`
	To        string                 `json:"to,omitempty"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type ClientMessage struct {
	Type    MessageType `json:"type"`
	To      string      `json:"to,omitempty"`
	Room    string      `json:"room,omitempty"`
	Content string      `json:"content"`
}

type ServerMessage struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	From      string                 `json:"from"`
	Room      string                 `json:"room,omitempty"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type PresenceEvent struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type ErrorMessage struct {
	Error     string    `json:"error"`
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}
