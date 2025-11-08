package tcp_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
)

func TestTCPErrorCreation(t *testing.T) {
	err := tcp.NewNetworkConnectionError(errors.New("connection refused"))

	if err.Category != tcp.NetworkError {
		t.Errorf("Expected category tcp.NetworkError, got %s", err.Category)
	}
	if err.Code != tcp.ErrNetworkConnection {
		t.Errorf("Expected code %s, got %s", tcp.ErrNetworkConnection, err.Code)
	}
	if err.Cause == nil {
		t.Error("Expected non-nil cause")
	}
}

func TestTCPErrorString(t *testing.T) {
	cause := errors.New("original error")
	err := tcp.NewDatabaseQueryError(cause)

	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}

	if len(errStr) < 10 {
		t.Errorf("Error string too short: %s", errStr)
	}
}

func TestTCPErrorUnwrap(t *testing.T) {
	cause := errors.New("original error")
	err := tcp.NewAuthTokenInvalidError()
	err.Cause = cause

	unwrapped := errors.Unwrap(err)
	if unwrapped != cause {
		t.Error("Unwrap should return the original cause")
	}
}

func TestTCPErrorToJSON(t *testing.T) {
	err := tcp.NewBizMangaNotFoundError("manga-123")

	jsonBytes := err.ToJSON()
	if len(jsonBytes) == 0 {
		t.Error("Expected non-empty JSON")
	}

	var msg tcp.Message
	if unmarshalErr := json.Unmarshal(jsonBytes[:len(jsonBytes)-1], &msg); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal error JSON: %v", unmarshalErr)
	}

	if msg.Type != "error" {
		t.Errorf("Expected tcp.Message type 'error', got %s", msg.Type)
	}

	var payload map[string]interface{}
	if unmarshalErr := json.Unmarshal(msg.Payload, &payload); unmarshalErr != nil {
		t.Fatalf("Failed to unmarshal payload: %v", unmarshalErr)
	}

	if payload["code"] != string(tcp.ErrBizMangaNotFound) {
		t.Errorf("Expected code %s, got %v", tcp.ErrBizMangaNotFound, payload["code"])
	}
	if payload["category"] != string(tcp.BusinessLogicError) {
		t.Errorf("Expected category %s, got %v", tcp.BusinessLogicError, payload["category"])
	}
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *tcp.TCPError
		category tcp.ErrorCategory
		code     tcp.ErrorCode
	}{
		{
			name:     "NetworkConnection",
			err:      tcp.NewNetworkConnectionError(nil),
			category: tcp.NetworkError,
			code:     tcp.ErrNetworkConnection,
		},
		{
			name:     "NetworkTimeout",
			err:      tcp.NewNetworkTimeoutError(nil),
			category: tcp.NetworkError,
			code:     tcp.ErrNetworkTimeout,
		},
		{
			name:     "ProtocolInvalidFormat",
			err:      tcp.NewProtocolInvalidFormatError(nil),
			category: tcp.ProtocolError,
			code:     tcp.ErrProtocolInvalidFormat,
		},
		{
			name:     "ProtocolUnknownType",
			err:      tcp.NewProtocolUnknownTypeError("unknown"),
			category: tcp.ProtocolError,
			code:     tcp.ErrProtocolUnknownType,
		},
		{
			name:     "AuthTokenMissing",
			err:      tcp.NewAuthTokenMissingError(),
			category: tcp.AuthenticationError,
			code:     tcp.ErrAuthTokenMissing,
		},
		{
			name:     "AuthTokenInvalid",
			err:      tcp.NewAuthTokenInvalidError(),
			category: tcp.AuthenticationError,
			code:     tcp.ErrAuthTokenInvalid,
		},
		{
			name:     "BizMangaNotFound",
			err:      tcp.NewBizMangaNotFoundError("test-id"),
			category: tcp.BusinessLogicError,
			code:     tcp.ErrBizMangaNotFound,
		},
		{
			name:     "BizInvalidStatus",
			err:      tcp.NewBizInvalidStatusError("invalid"),
			category: tcp.BusinessLogicError,
			code:     tcp.ErrBizInvalidStatus,
		},
		{
			name:     "DatabaseQuery",
			err:      tcp.NewDatabaseQueryError(errors.New("query failed")),
			category: tcp.DatabaseError,
			code:     tcp.ErrDatabaseQuery,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Category != tt.category {
				t.Errorf("Expected category %s, got %s", tt.category, tt.err.Category)
			}
			if tt.err.Code != tt.code {
				t.Errorf("Expected code %s, got %s", tt.code, tt.err.Code)
			}
			if tt.err.Message == "" {
				t.Error("Expected non-empty message")
			}
		})
	}
}

func TestErrorCategories(t *testing.T) {
	categories := []tcp.ErrorCategory{
		tcp.NetworkError,
		tcp.ProtocolError,
		tcp.AuthenticationError,
		tcp.BusinessLogicError,
		tcp.DatabaseError,
	}

	for _, category := range categories {
		if string(category) == "" {
			t.Errorf("Category %v has empty string value", category)
		}
	}
}

func TestErrorCodes(t *testing.T) {
	codes := []tcp.ErrorCode{
		tcp.ErrNetworkConnection,
		tcp.ErrProtocolInvalidFormat,
		tcp.ErrAuthTokenInvalid,
		tcp.ErrBizMangaNotFound,
		tcp.ErrDatabaseQuery,
	}

	for _, code := range codes {
		if string(code) == "" {
			t.Errorf("Code %v has empty string value", code)
		}
		codeStr := string(code)
		if len(codeStr) < 5 {
			t.Errorf("Code %s is too short", codeStr)
		}
	}
}
