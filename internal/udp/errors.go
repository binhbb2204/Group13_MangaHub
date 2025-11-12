package udp

import (
	"fmt"
	"time"
)

type ErrorCode string

const (
	ErrUDPBindFailed         ErrorCode = "UDP-001"
	ErrUDPInvalidPacket      ErrorCode = "UDP-002"
	ErrUDPRegistrationFailed ErrorCode = "UDP-003"
	ErrUDPAuthFailed         ErrorCode = "UDP-004"
	ErrUDPBroadcastFailed    ErrorCode = "UDP-005"
	ErrUDPSubscriptionFailed ErrorCode = "UDP-006"
	ErrUDPHeartbeatFailed    ErrorCode = "UDP-007"
	ErrUDPInvalidEventType   ErrorCode = "UDP-008"
	ErrUDPWriteFailed        ErrorCode = "UDP-009"
	ErrUDPReadFailed         ErrorCode = "UDP-010"
)

type UDPError struct {
	Code      ErrorCode
	Message   string
	Cause     error
	Timestamp string
}

func (e *UDPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *UDPError) Unwrap() error {
	return e.Cause
}

func NewUDPError(code ErrorCode, message string, cause error) *UDPError {
	return &UDPError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

func NewBindError(cause error) *UDPError {
	return NewUDPError(ErrUDPBindFailed, "Failed to bind UDP port", cause)
}

func NewAuthError() *UDPError {
	return NewUDPError(ErrUDPAuthFailed, "Authentication failed", nil)
}

func NewInvalidPacketError(cause error) *UDPError {
	return NewUDPError(ErrUDPInvalidPacket, "Invalid packet format", cause)
}

func NewRegistrationError(cause error) *UDPError {
	return NewUDPError(ErrUDPRegistrationFailed, "Registration failed", cause)
}

func NewSubscriptionError(message string) *UDPError {
	return NewUDPError(ErrUDPSubscriptionFailed, message, nil)
}

func NewInvalidEventTypeError(eventType string) *UDPError {
	return NewUDPError(ErrUDPInvalidEventType, fmt.Sprintf("Invalid event type: %s", eventType), nil)
}

func NewWriteError(cause error) *UDPError {
	return NewUDPError(ErrUDPWriteFailed, "Failed to write UDP packet", cause)
}

func NewReadError(cause error) *UDPError {
	return NewUDPError(ErrUDPReadFailed, "Failed to read UDP packet", cause)
}
