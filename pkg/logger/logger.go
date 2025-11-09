package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

type Logger struct {
	level      LogLevel
	output     io.Writer
	jsonFormat bool
	mu         sync.Mutex
	context    map[string]interface{}
}

type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	File      string                 `json:"file,omitempty"`
	Line      int                    `json:"line,omitempty"`
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Init(level LogLevel, jsonFormat bool, output io.Writer) {
	once.Do(func() {
		if output == nil {
			output = os.Stdout
		}
		defaultLogger = &Logger{
			level:      level,
			output:     output,
			jsonFormat: jsonFormat,
			context:    make(map[string]interface{}),
		}
	})
}

func GetLogger() *Logger {
	if defaultLogger == nil {
		Init(INFO, false, os.Stdout)
	}
	return defaultLogger
}

func New(level LogLevel, jsonFormat bool, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{
		level:      level,
		output:     output,
		jsonFormat: jsonFormat,
		context:    make(map[string]interface{}),
	}
}

func (l *Logger) WithContext(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newContext := make(map[string]interface{})
	for k, v := range l.context {
		newContext[k] = v
	}
	newContext[key] = value

	return &Logger{
		level:      l.level,
		output:     l.output,
		jsonFormat: l.jsonFormat,
		context:    newContext,
	}
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newContext := make(map[string]interface{})
	for k, v := range l.context {
		newContext[k] = v
	}
	for k, v := range fields {
		newContext[k] = v
	}

	return &Logger{
		level:      l.level,
		output:     l.output,
		jsonFormat: l.jsonFormat,
		context:    newContext,
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) shouldLog(level LogLevel) bool {
	levels := map[LogLevel]int{
		DEBUG: 0,
		INFO:  1,
		WARN:  2,
		ERROR: 3,
	}
	return levels[level] >= levels[l.level]
}

func (l *Logger) log(level LogLevel, message string, keyvals ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     string(level),
		Message:   message,
		Context:   make(map[string]interface{}),
	}

	for k, v := range l.context {
		entry.Context[k] = v
	}

	for i := 0; i < len(keyvals)-1; i += 2 {
		if key, ok := keyvals[i].(string); ok {
			entry.Context[key] = keyvals[i+1]
		}
	}

	if level == ERROR || level == WARN {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.File = file
			entry.Line = line
		}
	}

	var output string
	if l.jsonFormat {
		data, err := json.Marshal(entry)
		if err != nil {
			output = fmt.Sprintf("[%s] %s: %s\n", entry.Timestamp, entry.Level, entry.Message)
		} else {
			output = string(data) + "\n"
		}
	} else {
		contextStr := ""
		if len(entry.Context) > 0 {
			contextStr = fmt.Sprintf(" %v", entry.Context)
		}
		output = fmt.Sprintf("[%s] %s: %s%s\n", entry.Timestamp, entry.Level, entry.Message, contextStr)
	}

	l.output.Write([]byte(output))
}

func (l *Logger) Debug(message string, keyvals ...interface{}) {
	l.log(DEBUG, message, keyvals...)
}

func (l *Logger) Info(message string, keyvals ...interface{}) {
	l.log(INFO, message, keyvals...)
}

func (l *Logger) Warn(message string, keyvals ...interface{}) {
	l.log(WARN, message, keyvals...)
}

func (l *Logger) Error(message string, keyvals ...interface{}) {
	l.log(ERROR, message, keyvals...)
}

func Debug(message string, keyvals ...interface{}) {
	GetLogger().Debug(message, keyvals...)
}

func Info(message string, keyvals ...interface{}) {
	GetLogger().Info(message, keyvals...)
}

func Warn(message string, keyvals ...interface{}) {
	GetLogger().Warn(message, keyvals...)
}

func Error(message string, keyvals ...interface{}) {
	GetLogger().Error(message, keyvals...)
}

func WithContext(key string, value interface{}) *Logger {
	return GetLogger().WithContext(key, value)
}

func WithFields(fields map[string]interface{}) *Logger {
	return GetLogger().WithFields(fields)
}
