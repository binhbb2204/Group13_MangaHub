package tcp_test

import (
	"encoding/json"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
)

func TestParseMessage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid ping message",
			input:   `{"type":"ping","payload":{}}`,
			wantErr: false,
		},
		{
			name:    "valid auth message",
			input:   `{"type":"auth","payload":{"token":"test-token"}}`,
			wantErr: false,
		},
		{
			name:    "valid sync_progress message",
			input:   `{"type":"sync_progress","payload":{"user_id":"123","manga_id":"456","current_chapter":7,"status":"reading"}}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "missing type field",
			input:   `{"payload":{}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tcp.ParseMessage([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && msg == nil {
				t.Error("ParseMessage() returned nil message but no error")
			}
		})
	}
}

func TestCreateErrorMessage(t *testing.T) {
	errMsg := "test error"
	result := tcp.CreateErrorMessage(errMsg)

	if len(result) == 0 {
		t.Fatal("CreateErrorMessage() returned empty result")
	}

	if result[len(result)-1] != '\n' {
		t.Error("CreateErrorMessage() should end with newline")
	}

	var msg tcp.Message
	if err := json.Unmarshal(result[:len(result)-1], &msg); err != nil {
		t.Fatalf("CreateErrorMessage() produced invalid JSON: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("CreateErrorMessage() type = %v, want error", msg.Type)
	}
}

func TestCreateSuccessMessage(t *testing.T) {
	successMsg := "test success"
	result := tcp.CreateSuccessMessage(successMsg)

	if len(result) == 0 {
		t.Fatal("CreateSuccessMessage() returned empty result")
	}

	if result[len(result)-1] != '\n' {
		t.Error("CreateSuccessMessage() should end with newline")
	}

	var msg tcp.Message
	if err := json.Unmarshal(result[:len(result)-1], &msg); err != nil {
		t.Fatalf("CreateSuccessMessage() produced invalid JSON: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("CreateSuccessMessage() type = %v, want success", msg.Type)
	}
}

func TestCreatePongMessage(t *testing.T) {
	result := tcp.CreatePongMessage()

	if len(result) == 0 {
		t.Fatal("CreatePongMessage() returned empty result")
	}

	if result[len(result)-1] != '\n' {
		t.Error("CreatePongMessage() should end with newline")
	}

	var msg tcp.Message
	if err := json.Unmarshal(result[:len(result)-1], &msg); err != nil {
		t.Fatalf("CreatePongMessage() produced invalid JSON: %v", err)
	}

	if msg.Type != "pong" {
		t.Errorf("CreatePongMessage() type = %v, want pong", msg.Type)
	}
}
