package tcp

import (
	"encoding/json"
	"errors"
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
