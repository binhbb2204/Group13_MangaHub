package tcp

import (
	"encoding/json"
	"fmt"
)

type ErrorCategory string

const (
	NetworkError        ErrorCategory = "NETWORK"
	ProtocolError       ErrorCategory = "PROTOCOL"
	AuthenticationError ErrorCategory = "AUTH"
	BusinessLogicError  ErrorCategory = "BUSINESS"
	DatabaseError       ErrorCategory = "DATABASE"
)

type ErrorCode string

const (
	ErrNetworkConnection   ErrorCode = "NET-001"
	ErrNetworkTimeout      ErrorCode = "NET-002"
	ErrNetworkDisconnected ErrorCode = "NET-003"
	ErrNetworkRead         ErrorCode = "NET-004"
	ErrNetworkWrite        ErrorCode = "NET-005"

	ErrProtocolInvalidFormat   ErrorCode = "PROTO-001"
	ErrProtocolUnknownType     ErrorCode = "PROTO-002"
	ErrProtocolInvalidPayload  ErrorCode = "PROTO-003"
	ErrProtocolMessageTooLarge ErrorCode = "PROTO-004"

	ErrAuthTokenMissing     ErrorCode = "AUTH-001"
	ErrAuthTokenInvalid     ErrorCode = "AUTH-002"
	ErrAuthTokenExpired     ErrorCode = "AUTH-003"
	ErrAuthNotAuthenticated ErrorCode = "AUTH-004"
	ErrAuthPermissionDenied ErrorCode = "AUTH-005"

	ErrBizMangaNotFound    ErrorCode = "BIZ-001"
	ErrBizInvalidChapter   ErrorCode = "BIZ-002"
	ErrBizInvalidStatus    ErrorCode = "BIZ-003"
	ErrBizAlreadyInLibrary ErrorCode = "BIZ-004"
	ErrBizNotInLibrary     ErrorCode = "BIZ-005"
	ErrBizInvalidMangaID   ErrorCode = "BIZ-006"

	ErrDatabaseQuery      ErrorCode = "DB-001"
	ErrDatabaseConnection ErrorCode = "DB-002"
	ErrDatabaseConstraint ErrorCode = "DB-003"
	ErrDatabaseNotFound   ErrorCode = "DB-004"
)

type TCPError struct {
	Category ErrorCategory `json:"category"`
	Code     ErrorCode     `json:"code"`
	Message  string        `json:"message"`
	Cause    error         `json:"-"`
}

func (e *TCPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s-%s] %s: %v", e.Category, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s-%s] %s", e.Category, e.Code, e.Message)
}

func (e *TCPError) Unwrap() error {
	return e.Cause
}

func (e *TCPError) ToJSON() []byte {
	payload := map[string]interface{}{
		"code":     string(e.Code),
		"message":  e.Message,
		"category": string(e.Category),
	}

	msg := Message{
		Type:    "error",
		Payload: json.RawMessage(mustMarshal(payload)),
	}

	data, _ := json.Marshal(msg)
	return append(data, '\n')
}

func NewTCPError(category ErrorCategory, code ErrorCode, message string, cause error) *TCPError {
	return &TCPError{
		Category: category,
		Code:     code,
		Message:  message,
		Cause:    cause,
	}
}

func NewNetworkConnectionError(cause error) *TCPError {
	return NewTCPError(NetworkError, ErrNetworkConnection, "Connection failed", cause)
}

func NewNetworkTimeoutError(cause error) *TCPError {
	return NewTCPError(NetworkError, ErrNetworkTimeout, "Network timeout", cause)
}

func NewNetworkDisconnectedError(cause error) *TCPError {
	return NewTCPError(NetworkError, ErrNetworkDisconnected, "Client disconnected", cause)
}

func NewNetworkReadError(cause error) *TCPError {
	return NewTCPError(NetworkError, ErrNetworkRead, "Failed to read from connection", cause)
}

func NewNetworkWriteError(cause error) *TCPError {
	return NewTCPError(NetworkError, ErrNetworkWrite, "Failed to write to connection", cause)
}

func NewProtocolInvalidFormatError(cause error) *TCPError {
	return NewTCPError(ProtocolError, ErrProtocolInvalidFormat, "Invalid message format", cause)
}

func NewProtocolUnknownTypeError(messageType string) *TCPError {
	return NewTCPError(ProtocolError, ErrProtocolUnknownType,
		fmt.Sprintf("Unknown message type: %s", messageType), nil)
}

func NewProtocolInvalidPayloadError(details string) *TCPError {
	return NewTCPError(ProtocolError, ErrProtocolInvalidPayload,
		fmt.Sprintf("Invalid payload: %s", details), nil)
}

func NewAuthTokenMissingError() *TCPError {
	return NewTCPError(AuthenticationError, ErrAuthTokenMissing, "Token is required", nil)
}

func NewAuthTokenInvalidError() *TCPError {
	return NewTCPError(AuthenticationError, ErrAuthTokenInvalid, "Invalid or expired token", nil)
}

func NewAuthTokenExpiredError() *TCPError {
	return NewTCPError(AuthenticationError, ErrAuthTokenExpired, "Token has expired", nil)
}

func NewAuthNotAuthenticatedError() *TCPError {
	return NewTCPError(AuthenticationError, ErrAuthNotAuthenticated, "Authentication required", nil)
}

func NewBizMangaNotFoundError(mangaID string) *TCPError {
	return NewTCPError(BusinessLogicError, ErrBizMangaNotFound,
		fmt.Sprintf("Manga not found: %s", mangaID), nil)
}

func NewBizInvalidChapterError(chapter int) *TCPError {
	return NewTCPError(BusinessLogicError, ErrBizInvalidChapter,
		fmt.Sprintf("Invalid chapter number: %d", chapter), nil)
}

func NewBizInvalidStatusError(status string) *TCPError {
	return NewTCPError(BusinessLogicError, ErrBizInvalidStatus,
		fmt.Sprintf("Invalid status. Must be: reading, completed, or plan_to_read. Got: %s", status), nil)
}

func NewBizNotInLibraryError(mangaID string) *TCPError {
	return NewTCPError(BusinessLogicError, ErrBizNotInLibrary,
		fmt.Sprintf("Manga not in library: %s", mangaID), nil)
}

func NewBizInvalidMangaIDError() *TCPError {
	return NewTCPError(BusinessLogicError, ErrBizInvalidMangaID, "Manga ID is required", nil)
}

func NewDatabaseQueryError(cause error) *TCPError {
	return NewTCPError(DatabaseError, ErrDatabaseQuery, "Database query failed", cause)
}

func NewDatabaseConnectionError(cause error) *TCPError {
	return NewTCPError(DatabaseError, ErrDatabaseConnection, "Database connection failed", cause)
}

func NewDatabaseNotFoundError() *TCPError {
	return NewTCPError(DatabaseError, ErrDatabaseNotFound, "Record not found", nil)
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"message": "Error marshaling: %v"}`, err))
	}
	return data
}

func SendError(client *Client, err error) {
	if tcpErr, ok := err.(*TCPError); ok {
		client.Conn.Write(tcpErr.ToJSON())
	} else {
		genericErr := NewTCPError(BusinessLogicError, "BIZ-999", err.Error(), err)
		client.Conn.Write(genericErr.ToJSON())
	}
}
