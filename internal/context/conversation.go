package context

import (
	"fmt"
	"time"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/operations"
)

type ConversationThread struct {
	ID            ThreadID                 `json:"id"`
	Title         string                   `json:"title"`
	AnchorAddress addressing.StableAddress `json:"anchor_address"`
	Participants  []operations.AuthorID    `json:"participants"`
	Messages      []Message                `json:"messages"`
	Status        ThreadStatus             `json:"status"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	Tags          []string                 `json:"tags,omitempty"`
	Metadata      ConversationMeta         `json:"metadata"`
}

type ThreadID string

type Message struct {
	ID          MessageID                  `json:"id"`
	AuthorID    operations.AuthorID        `json:"author_id"`
	Content     string                     `json:"content"`
	MessageType MessageType                `json:"message_type"`
	References  []addressing.StableAddress `json:"references,omitempty"`
	Reactions   []Reaction                 `json:"reactions,omitempty"`
	Timestamp   time.Time                  `json:"timestamp"`
	EditHistory []EditRecord               `json:"edit_history,omitempty"`
}

type MessageID string

type MessageType string

const (
	MsgComment    MessageType = "comment"
	MsgQuestion   MessageType = "question"
	MsgAnswer     MessageType = "answer"
	MsgDecision   MessageType = "decision"
	MsgSuggestion MessageType = "suggestion"
	MsgReview     MessageType = "review"
)

type Reaction struct {
	AuthorID  operations.AuthorID `json:"author_id"`
	Emoji     string              `json:"emoji"`
	Timestamp time.Time           `json:"timestamp"`
}

type EditRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	PrevContent string    `json:"prev_content"`
	Reason      string    `json:"reason,omitempty"`
}

type ThreadStatus string

const (
	StatusOpen     ThreadStatus = "open"
	StatusResolved ThreadStatus = "resolved"
	StatusArchived ThreadStatus = "archived"
	StatusPinned   ThreadStatus = "pinned"
)

type ConversationMeta struct {
	Priority    Priority            `json:"priority,omitempty"`
	Labels      []string            `json:"labels,omitempty"`
	Assignee    operations.AuthorID `json:"assignee,omitempty"`
	DueDate     *time.Time          `json:"due_date,omitempty"`
	LinkedIssue string              `json:"linked_issue,omitempty"`
}

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

func NewConversationThread(anchorAddr addressing.StableAddress, authorID operations.AuthorID, title, content string) *ConversationThread {
	now := time.Now()
	threadID := ThreadID(generateThreadID())
	messageID := MessageID(generateMessageID())

	message := Message{
		ID:          messageID,
		AuthorID:    authorID,
		Content:     content,
		MessageType: MsgComment,
		Timestamp:   now,
	}

	return &ConversationThread{
		ID:            threadID,
		Title:         title,
		AnchorAddress: anchorAddr,
		Participants:  []operations.AuthorID{authorID},
		Messages:      []Message{message},
		Status:        StatusOpen,
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      ConversationMeta{},
	}
}

func (ct *ConversationThread) AddMessage(authorID operations.AuthorID, content string, msgType MessageType) *Message {
	messageID := MessageID(generateMessageID())
	message := Message{
		ID:          messageID,
		AuthorID:    authorID,
		Content:     content,
		MessageType: msgType,
		Timestamp:   time.Now(),
	}

	ct.Messages = append(ct.Messages, message)
	ct.UpdatedAt = time.Now()

	// Add author to participants if not already present
	ct.addParticipant(authorID)

	return &message
}

func (ct *ConversationThread) EditMessage(messageID MessageID, authorID operations.AuthorID, newContent string, reason string) error {
	for i, msg := range ct.Messages {
		if msg.ID == messageID {
			if msg.AuthorID != authorID {
				return ErrUnauthorized
			}

			// Record edit history
			editRecord := EditRecord{
				Timestamp:   time.Now(),
				PrevContent: msg.Content,
				Reason:      reason,
			}

			ct.Messages[i].EditHistory = append(ct.Messages[i].EditHistory, editRecord)
			ct.Messages[i].Content = newContent
			ct.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrMessageNotFound
}

func (ct *ConversationThread) AddReaction(messageID MessageID, authorID operations.AuthorID, emoji string) error {
	for i, msg := range ct.Messages {
		if msg.ID == messageID {
			// Remove existing reaction from this author if any
			ct.Messages[i].Reactions = removeReactionByAuthor(msg.Reactions, authorID)

			// Add new reaction
			reaction := Reaction{
				AuthorID:  authorID,
				Emoji:     emoji,
				Timestamp: time.Now(),
			}
			ct.Messages[i].Reactions = append(ct.Messages[i].Reactions, reaction)
			ct.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrMessageNotFound
}

func (ct *ConversationThread) SetStatus(status ThreadStatus) {
	ct.Status = status
	ct.UpdatedAt = time.Now()
}

func (ct *ConversationThread) AddReference(messageID MessageID, address addressing.StableAddress) error {
	for i, msg := range ct.Messages {
		if msg.ID == messageID {
			ct.Messages[i].References = append(ct.Messages[i].References, address)
			ct.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrMessageNotFound
}

func (ct *ConversationThread) GetMessage(messageID MessageID) (*Message, error) {
	for _, msg := range ct.Messages {
		if msg.ID == messageID {
			return &msg, nil
		}
	}
	return nil, ErrMessageNotFound
}

func (ct *ConversationThread) GetMessagesByAuthor(authorID operations.AuthorID) []Message {
	var messages []Message
	for _, msg := range ct.Messages {
		if msg.AuthorID == authorID {
			messages = append(messages, msg)
		}
	}
	return messages
}

func (ct *ConversationThread) GetMessagesByType(msgType MessageType) []Message {
	var messages []Message
	for _, msg := range ct.Messages {
		if msg.MessageType == msgType {
			messages = append(messages, msg)
		}
	}
	return messages
}

func (ct *ConversationThread) addParticipant(authorID operations.AuthorID) {
	for _, participant := range ct.Participants {
		if participant == authorID {
			return // Already a participant
		}
	}
	ct.Participants = append(ct.Participants, authorID)
}

func removeReactionByAuthor(reactions []Reaction, authorID operations.AuthorID) []Reaction {
	filtered := make([]Reaction, 0, len(reactions))
	for _, reaction := range reactions {
		if reaction.AuthorID != authorID {
			filtered = append(filtered, reaction)
		}
	}
	return filtered
}

func generateThreadID() string {
	return "thread_" + generateID()
}

func generateMessageID() string {
	return "msg_" + generateID()
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
