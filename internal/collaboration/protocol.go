package collaboration

import (
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
)

type MessageType string

const (
	MsgOperation      MessageType = "operation"
	MsgPresence       MessageType = "presence"
	MsgSync           MessageType = "sync"
	MsgAcknowledgment MessageType = "ack"
	MsgError          MessageType = "error"
	MsgComment        MessageType = "comment"
)

type Message struct {
	Type      MessageType         `json:"type"`
	Payload   interface{}         `json:"payload"`
	MessageID string              `json:"message_id"`
	Timestamp time.Time           `json:"timestamp"`
	AuthorID  operations.AuthorID `json:"author_id"`
}

type OperationPayload struct {
	Operation  *operations.Operation  `json:"operation"`
	DocumentID string                 `json:"document_id"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type PresencePayload struct {
	AuthorID       operations.AuthorID       `json:"author_id"`
	DocumentID     string                    `json:"document_id"`
	CursorPosition operations.LogootPosition `json:"cursor_position"`
	Selection      *PositionRange            `json:"selection,omitempty"`
	LastActive     time.Time                 `json:"last_active"`
	Status         PresenceStatus            `json:"status"`
}

type PositionRange struct {
	Start operations.LogootPosition `json:"start"`
	End   operations.LogootPosition `json:"end"`
}

type PresenceStatus string

const (
	StatusActive  PresenceStatus = "active"
	StatusIdle    PresenceStatus = "idle"
	StatusOffline PresenceStatus = "offline"
)

type SyncPayload struct {
	DocumentID   string                  `json:"document_id"`
	Operations   []*operations.Operation `json:"operations"`
	CurrentState *positioning.Document   `json:"current_state,omitempty"`
	SinceVersion uint64                  `json:"since_version,omitempty"`
}

type AckPayload struct {
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

type ErrorPayload struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}
