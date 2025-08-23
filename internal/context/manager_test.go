package context

import (
	"math/big"
	"testing"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/operations"
)

func TestConversationManager_CreateAndGet(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	authorID := operations.AuthorID("author1")
	title := "Test Discussion"
	content := "Initial message"

	// Create conversation
	thread, err := manager.CreateConversation(anchorAddr, authorID, title, content)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Get conversation
	retrieved, err := manager.GetConversation(thread.ID)
	if err != nil {
		t.Fatalf("Failed to get conversation: %v", err)
	}

	if retrieved.Title != title {
		t.Errorf("Expected title %s, got %s", title, retrieved.Title)
	}

	if retrieved.AnchorAddress.Repository != anchorAddr.Repository {
		t.Errorf("Expected repository %s, got %s", anchorAddr.Repository, retrieved.AnchorAddress.Repository)
	}
}

func TestConversationManager_GetByAddress(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address with operation ID and position range
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	// Create multiple conversations with same anchor
	thread1, _ := manager.CreateConversation(anchorAddr, "author1", "Discussion 1", "Message 1")
	thread2, _ := manager.CreateConversation(anchorAddr, "author2", "Discussion 2", "Message 2")

	// Create conversation with different anchor
	differentOpID := operations.NewOperationID([]byte("other-op"))
	differentPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(2), AuthorID: "author2"},
	})
	differentRange := addressing.PositionRange{Start: differentPos, End: differentPos}
	differentAddr := addressing.NewStableAddress(addressing.RepositoryID("other-repo"), differentOpID, differentRange)
	manager.CreateConversation(differentAddr, "author3", "Other Discussion", "Other Message")

	// Get conversations by address
	conversations, err := manager.GetConversationsByAddress(anchorAddr)
	if err != nil {
		t.Fatalf("Failed to get conversations by address: %v", err)
	}

	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}

	// Check that we got the right conversations
	foundThread1 := false
	foundThread2 := false

	for _, conv := range conversations {
		if conv.ID == thread1.ID {
			foundThread1 = true
		}
		if conv.ID == thread2.ID {
			foundThread2 = true
		}
	}

	if !foundThread1 {
		t.Error("Expected to find thread1")
	}

	if !foundThread2 {
		t.Error("Expected to find thread2")
	}
}

func TestConversationManager_GetByAuthor(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	authorID := operations.AuthorID("author1")

	// Create conversations by the author
	thread1, _ := manager.CreateConversation(anchorAddr, authorID, "Discussion 1", "Message 1")
	thread2, _ := manager.CreateConversation(anchorAddr, authorID, "Discussion 2", "Message 2")

	// Create conversation by different author
	manager.CreateConversation(anchorAddr, "author2", "Other Discussion", "Other Message")

	// Get conversations by author
	conversations, err := manager.GetConversationsByAuthor(authorID)
	if err != nil {
		t.Fatalf("Failed to get conversations by author: %v", err)
	}

	if len(conversations) != 2 {
		t.Errorf("Expected 2 conversations, got %d", len(conversations))
	}

	// Check that we got the right conversations
	foundThread1 := false
	foundThread2 := false

	for _, conv := range conversations {
		if conv.ID == thread1.ID {
			foundThread1 = true
		}
		if conv.ID == thread2.ID {
			foundThread2 = true
		}
	}

	if !foundThread1 {
		t.Error("Expected to find thread1")
	}

	if !foundThread2 {
		t.Error("Expected to find thread2")
	}
}

func TestConversationManager_AddMessage(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	thread, _ := manager.CreateConversation(anchorAddr, "author1", "Discussion", "Initial")

	// Add message
	authorID := operations.AuthorID("author2")
	content := "Reply message"

	msg, err := manager.AddMessage(thread.ID, authorID, content, MsgComment)
	if err != nil {
		t.Fatalf("Failed to add message: %v", err)
	}

	if msg.Content != content {
		t.Errorf("Expected content %s, got %s", content, msg.Content)
	}

	if msg.AuthorID != authorID {
		t.Errorf("Expected author %s, got %s", authorID, msg.AuthorID)
	}

	// Verify the conversation was updated
	updated, _ := manager.GetConversation(thread.ID)
	if len(updated.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(updated.Messages))
	}

	// Verify author was added to participants
	if len(updated.Participants) != 2 {
		t.Errorf("Expected 2 participants, got %d", len(updated.Participants))
	}
}

func TestConversationManager_SearchConversations(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	// Create conversations with different content
	manager.CreateConversation(anchorAddr, "author1", "Bug Discussion", "Found a bug in the parser")
	manager.CreateConversation(anchorAddr, "author2", "Feature Request", "Need a new feature for logging")
	manager.CreateConversation(anchorAddr, "author3", "Code Review", "This looks good to me")

	// Search for "bug"
	results, err := manager.SearchConversations("bug")
	if err != nil {
		t.Fatalf("Failed to search conversations: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'bug', got %d", len(results))
	}

	if results[0].Title != "Bug Discussion" {
		t.Errorf("Expected title 'Bug Discussion', got %s", results[0].Title)
	}

	// Search for "feature"
	results, err = manager.SearchConversations("feature")
	if err != nil {
		t.Fatalf("Failed to search conversations: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'feature', got %d", len(results))
	}

	// Search for something that appears in multiple conversations
	manager.CreateConversation(anchorAddr, "author4", "Another Discussion", "This code needs review")

	results, err = manager.SearchConversations("code")
	if err != nil {
		t.Fatalf("Failed to search conversations: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'code', got %d", len(results))
	}
}

func TestConversationManager_GetByStatus(t *testing.T) {
	manager := NewConversationManager()

	// Create proper stable address
	opID := operations.NewOperationID([]byte("test-op"))
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	posRange := addressing.PositionRange{Start: pos, End: pos}
	anchorAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, posRange)

	// Create conversations with different statuses
	_, _ = manager.CreateConversation(anchorAddr, "author1", "Open Discussion", "Message 1")
	_, _ = manager.CreateConversation(anchorAddr, "author2", "Another Open", "Message 2")
	thread3, _ := manager.CreateConversation(anchorAddr, "author3", "Resolved Discussion", "Message 3")

	// Resolve one conversation
	manager.ResolveConversation(thread3.ID, "author3")

	// Get open conversations
	openConversations, err := manager.GetConversationsByStatus(StatusOpen)
	if err != nil {
		t.Fatalf("Failed to get open conversations: %v", err)
	}

	if len(openConversations) != 2 {
		t.Errorf("Expected 2 open conversations, got %d", len(openConversations))
	}

	// Get resolved conversations
	resolvedConversations, err := manager.GetConversationsByStatus(StatusResolved)
	if err != nil {
		t.Fatalf("Failed to get resolved conversations: %v", err)
	}

	if len(resolvedConversations) != 1 {
		t.Errorf("Expected 1 resolved conversation, got %d", len(resolvedConversations))
	}

	if resolvedConversations[0].ID != thread3.ID {
		t.Errorf("Expected resolved conversation to be thread3")
	}
}

func TestConversationManager_UpdateAddressLocation(t *testing.T) {
	manager := NewConversationManager()

	// Create proper addresses with different ranges
	opID := operations.NewOperationID([]byte("test-op"))
	oldPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	oldRange := addressing.PositionRange{Start: oldPos, End: oldPos}
	oldAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo"), opID, oldRange)

	newPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(2), AuthorID: "author1"},
	})
	newRange := addressing.PositionRange{Start: newPos, End: newPos}
	newAddr := addressing.NewStableAddress(addressing.RepositoryID("test-repo-moved"), opID, newRange)

	// Create conversation with old address
	thread, _ := manager.CreateConversation(oldAddr, "author1", "Discussion", "Message")

	// Update address location
	err := manager.UpdateAddressLocation(oldAddr, newAddr)
	if err != nil {
		t.Fatalf("Failed to update address location: %v", err)
	}

	// Verify conversation now has new address
	updated, _ := manager.GetConversation(thread.ID)
	if updated.AnchorAddress.Repository != newAddr.Repository {
		t.Errorf("Expected repository %s, got %s", newAddr.Repository, updated.AnchorAddress.Repository)
	}

	// Verify old address query returns empty
	oldAddrConversations, _ := manager.GetConversationsByAddress(oldAddr)
	if len(oldAddrConversations) != 0 {
		t.Errorf("Expected 0 conversations for old address, got %d", len(oldAddrConversations))
	}

	// Verify new address query returns the conversation
	newAddrConversations, _ := manager.GetConversationsByAddress(newAddr)
	if len(newAddrConversations) != 1 {
		t.Errorf("Expected 1 conversation for new address, got %d", len(newAddrConversations))
	}
}
