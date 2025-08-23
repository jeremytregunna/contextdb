package context

import (
	"math/big"
	"testing"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/operations"
)

func TestConversationThread_Creation(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	authorID := operations.AuthorID("author1")
	title := "Test Discussion"
	content := "This is a test comment"

	thread := NewConversationThread(anchorAddr, authorID, title, content)

	if thread.Title != title {
		t.Errorf("Expected title %s, got %s", title, thread.Title)
	}

	if thread.AnchorAddress.Repository != anchorAddr.Repository {
		t.Errorf("Expected repository %s, got %s", anchorAddr.Repository, thread.AnchorAddress.Repository)
	}

	if len(thread.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(thread.Messages))
	}

	if thread.Messages[0].Content != content {
		t.Errorf("Expected content %s, got %s", content, thread.Messages[0].Content)
	}

	if len(thread.Participants) != 1 {
		t.Errorf("Expected 1 participant, got %d", len(thread.Participants))
	}

	if thread.Participants[0] != authorID {
		t.Errorf("Expected participant %s, got %s", authorID, thread.Participants[0])
	}

	if thread.Status != StatusOpen {
		t.Errorf("Expected status %s, got %s", StatusOpen, thread.Status)
	}
}

func TestConversationThread_AddMessage(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread := NewConversationThread(anchorAddr, "author1", "Test", "Initial message")

	// Add message from different author
	author2 := operations.AuthorID("author2")
	content2 := "Reply message"

	msg := thread.AddMessage(author2, content2, MsgComment)

	if msg.Content != content2 {
		t.Errorf("Expected content %s, got %s", content2, msg.Content)
	}

	if msg.AuthorID != author2 {
		t.Errorf("Expected author %s, got %s", author2, msg.AuthorID)
	}

	if len(thread.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(thread.Messages))
	}

	if len(thread.Participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(thread.Participants))
	}

	// Check that both authors are in participants
	foundAuthor1 := false
	foundAuthor2 := false
	for _, participant := range thread.Participants {
		if participant == "author1" {
			foundAuthor1 = true
		}
		if participant == author2 {
			foundAuthor2 = true
		}
	}

	if !foundAuthor1 {
		t.Error("Expected author1 to be in participants")
	}

	if !foundAuthor2 {
		t.Error("Expected author2 to be in participants")
	}
}

func TestConversationThread_EditMessage(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	authorID := operations.AuthorID("author1")
	thread := NewConversationThread(anchorAddr, authorID, "Test", "Initial message")

	messageID := thread.Messages[0].ID
	newContent := "Edited message"
	reason := "typo fix"

	err := thread.EditMessage(messageID, authorID, newContent, reason)
	if err != nil {
		t.Fatalf("Failed to edit message: %v", err)
	}

	if thread.Messages[0].Content != newContent {
		t.Errorf("Expected content %s, got %s", newContent, thread.Messages[0].Content)
	}

	if len(thread.Messages[0].EditHistory) != 1 {
		t.Errorf("Expected 1 edit record, got %d", len(thread.Messages[0].EditHistory))
	}

	editRecord := thread.Messages[0].EditHistory[0]
	if editRecord.PrevContent != "Initial message" {
		t.Errorf("Expected previous content 'Initial message', got %s", editRecord.PrevContent)
	}

	if editRecord.Reason != reason {
		t.Errorf("Expected reason %s, got %s", reason, editRecord.Reason)
	}
}

func TestConversationThread_EditMessage_Unauthorized(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread := NewConversationThread(anchorAddr, "author1", "Test", "Initial message")

	messageID := thread.Messages[0].ID
	unauthorizedAuthor := operations.AuthorID("author2")

	err := thread.EditMessage(messageID, unauthorizedAuthor, "Hacked content", "")
	if err != ErrUnauthorized {
		t.Errorf("Expected ErrUnauthorized, got %v", err)
	}
}

func TestConversationThread_AddReaction(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread := NewConversationThread(anchorAddr, "author1", "Test", "Initial message")
	messageID := thread.Messages[0].ID

	// Add reaction
	authorID := operations.AuthorID("author2")
	emoji := "👍"

	err := thread.AddReaction(messageID, authorID, emoji)
	if err != nil {
		t.Fatalf("Failed to add reaction: %v", err)
	}

	if len(thread.Messages[0].Reactions) != 1 {
		t.Errorf("Expected 1 reaction, got %d", len(thread.Messages[0].Reactions))
	}

	reaction := thread.Messages[0].Reactions[0]
	if reaction.AuthorID != authorID {
		t.Errorf("Expected author %s, got %s", authorID, reaction.AuthorID)
	}

	if reaction.Emoji != emoji {
		t.Errorf("Expected emoji %s, got %s", emoji, reaction.Emoji)
	}

	// Replace reaction from same author
	newEmoji := "❤️"
	err = thread.AddReaction(messageID, authorID, newEmoji)
	if err != nil {
		t.Fatalf("Failed to replace reaction: %v", err)
	}

	if len(thread.Messages[0].Reactions) != 1 {
		t.Errorf("Expected 1 reaction after replacement, got %d", len(thread.Messages[0].Reactions))
	}

	if thread.Messages[0].Reactions[0].Emoji != newEmoji {
		t.Errorf("Expected emoji %s, got %s", newEmoji, thread.Messages[0].Reactions[0].Emoji)
	}
}

func TestConversationThread_SetStatus(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread := NewConversationThread(anchorAddr, "author1", "Test", "Initial message")

	if thread.Status != StatusOpen {
		t.Errorf("Expected initial status %s, got %s", StatusOpen, thread.Status)
	}

	thread.SetStatus(StatusResolved)

	if thread.Status != StatusResolved {
		t.Errorf("Expected status %s, got %s", StatusResolved, thread.Status)
	}
}

func TestConversationThread_GetMessagesByType(t *testing.T) {
	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread := NewConversationThread(anchorAddr, "author1", "Test", "Initial comment")

	// Add different message types
	thread.AddMessage("author2", "This is a question?", MsgQuestion)
	thread.AddMessage("author1", "Here's the answer", MsgAnswer)
	thread.AddMessage("author3", "Another comment", MsgComment)

	questions := thread.GetMessagesByType(MsgQuestion)
	if len(questions) != 1 {
		t.Errorf("Expected 1 question, got %d", len(questions))
	}

	answers := thread.GetMessagesByType(MsgAnswer)
	if len(answers) != 1 {
		t.Errorf("Expected 1 answer, got %d", len(answers))
	}

	comments := thread.GetMessagesByType(MsgComment)
	if len(comments) != 2 { // Initial message + added comment
		t.Errorf("Expected 2 comments, got %d", len(comments))
	}
}
