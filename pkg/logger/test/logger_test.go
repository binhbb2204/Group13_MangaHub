package logger_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

func TestLoggerBasicLogging(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.INFO, false, &buf)

	log.Info("test_message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log level INFO in output, got: %s", output)
	}
	if !strings.Contains(output, "test_message") {
		t.Errorf("Expected message 'test_message' in output, got: %s", output)
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.INFO, true, &buf)

	log.Info("test_message", "key", "value")

	var entry logger.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("Expected level INFO, got %s", entry.Level)
	}
	if entry.Message != "test_message" {
		t.Errorf("Expected message 'test_message', got %s", entry.Message)
	}
	if entry.Context["key"] != "value" {
		t.Errorf("Expected context key=value, got %v", entry.Context)
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		logLevel   logger.LogLevel
		shouldLog  bool
		testLevel  logger.LogLevel
		testMethod func(*logger.Logger)
	}{
		{logger.DEBUG, true, logger.DEBUG, func(l *logger.Logger) { l.Debug("debug") }},
		{logger.DEBUG, true, logger.INFO, func(l *logger.Logger) { l.Info("info") }},
		{logger.INFO, false, logger.DEBUG, func(l *logger.Logger) { l.Debug("debug") }},
		{logger.INFO, true, logger.INFO, func(l *logger.Logger) { l.Info("info") }},
		{logger.WARN, false, logger.INFO, func(l *logger.Logger) { l.Info("info") }},
		{logger.WARN, true, logger.WARN, func(l *logger.Logger) { l.Warn("warn") }},
		{logger.ERROR, false, logger.WARN, func(l *logger.Logger) { l.Warn("warn") }},
		{logger.ERROR, true, logger.ERROR, func(l *logger.Logger) { l.Error("error") }},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		log := logger.New(tt.logLevel, false, &buf)
		tt.testMethod(log)

		hasOutput := buf.Len() > 0
		if hasOutput != tt.shouldLog {
			t.Errorf("LogLevel=%s, TestLevel=%s: expected shouldLog=%v, got hasOutput=%v",
				tt.logLevel, tt.testLevel, tt.shouldLog, hasOutput)
		}
	}
}

func TestLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.INFO, true, &buf)

	contextLog := log.WithContext("user_id", "123")
	contextLog.Info("test_message")

	var entry logger.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Context["user_id"] != "123" {
		t.Errorf("Expected context user_id=123, got %v", entry.Context)
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.INFO, true, &buf)

	fields := map[string]interface{}{
		"user_id":  "123",
		"username": "testuser",
	}
	contextLog := log.WithFields(fields)
	contextLog.Info("test_message")

	var entry logger.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Context["user_id"] != "123" {
		t.Errorf("Expected context user_id=123, got %v", entry.Context)
	}
	if entry.Context["username"] != "testuser" {
		t.Errorf("Expected context username=testuser, got %v", entry.Context)
	}
}

func TestLoggerKeyValues(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.INFO, true, &buf)

	log.Info("test_message", "key1", "value1", "key2", 42)

	var entry logger.LogEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if entry.Context["key1"] != "value1" {
		t.Errorf("Expected context key1=value1, got %v", entry.Context)
	}
	if entry.Context["key2"] != float64(42) {
		t.Errorf("Expected context key2=42, got %v", entry.Context)
	}
}

func TestGlobalLogger(t *testing.T) {
	var buf bytes.Buffer
	logger.Init(logger.INFO, false, &buf)

	logger.Info("test_message")

	output := buf.String()
	if !strings.Contains(output, "test_message") {
		t.Errorf("Expected 'test_message' in output, got: %s", output)
	}
}
