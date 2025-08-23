package logging

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	level      LogLevel
	component  string
	jsonFormat bool
}

type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Component string                 `json:"component"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func NewLogger(component string) *Logger {
	return &Logger{
		level:      INFO,
		component:  component,
		jsonFormat: os.Getenv("LOG_FORMAT") == "json",
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) log(level LogLevel, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Component: l.component,
		Message:   message,
		Fields:    fields,
	}

	if l.jsonFormat {
		if data, err := json.Marshal(entry); err == nil {
			log.Println(string(data))
		}
	} else {
		fieldsStr := ""
		if len(fields) > 0 {
			if data, err := json.Marshal(fields); err == nil {
				fieldsStr = " " + string(data)
			}
		}
		log.Printf("[%s] %s: %s%s", entry.Level, entry.Component, entry.Message, fieldsStr)
	}
}

func (l *Logger) Debug(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, message, f)
}

func (l *Logger) Info(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, message, f)
}

func (l *Logger) Warn(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, message, f)
}

func (l *Logger) Error(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, message, f)
}

func (l *Logger) Fatal(message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, message, f)
	os.Exit(1)
}

func (l *Logger) WithFields(fields map[string]interface{}) *LogContext {
	return &LogContext{
		logger: l,
		fields: fields,
	}
}

type LogContext struct {
	logger *Logger
	fields map[string]interface{}
}

func (lc *LogContext) Debug(message string) {
	lc.logger.log(DEBUG, message, lc.fields)
}

func (lc *LogContext) Info(message string) {
	lc.logger.log(INFO, message, lc.fields)
}

func (lc *LogContext) Warn(message string) {
	lc.logger.log(WARN, message, lc.fields)
}

func (lc *LogContext) Error(message string) {
	lc.logger.log(ERROR, message, lc.fields)
}

func (lc *LogContext) Fatal(message string) {
	lc.logger.log(FATAL, message, lc.fields)
	os.Exit(1)
}

// Convenience functions for common patterns
func (l *Logger) LogClientConnect(clientID, authorID string) {
	l.Info("Client connected", map[string]interface{}{
		"client_id": clientID,
		"author_id": authorID,
	})
}

func (l *Logger) LogClientDisconnect(clientID string) {
	l.Info("Client disconnected", map[string]interface{}{
		"client_id": clientID,
	})
}

func (l *Logger) LogWebSocketError(clientID string, err error) {
	l.Error("WebSocket error", map[string]interface{}{
		"client_id": clientID,
		"error":     err.Error(),
	})
}

func (l *Logger) LogOperationBroadcastError(clientID string, err error) {
	l.Warn("Failed to broadcast operation to client", map[string]interface{}{
		"client_id": clientID,
		"error":     err.Error(),
	})
}

func (l *Logger) LogPresenceBroadcastError(clientID string, err error) {
	l.Warn("Failed to broadcast presence to client", map[string]interface{}{
		"client_id": clientID,
		"error":     err.Error(),
	})
}
