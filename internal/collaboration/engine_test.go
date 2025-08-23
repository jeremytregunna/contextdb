package collaboration

import (
	"math/big"
	"testing"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
	"github.com/jeremytregunna/contextdb/internal/storage"
)

func TestCollaborationEngine_AddRemoveClient(t *testing.T) {
	store := setupTestStorage(t)
	engine := NewCollaborationEngine(store)

	// Mock client (we can't create a real WebSocket connection in tests)
	clientID := ClientID("test_client")
	authorID := operations.AuthorID("test_author")

	mockClient := &ClientConnection{
		ID:        clientID,
		AuthorID:  authorID,
		Documents: make(map[string]bool),
		LastSeen:  time.Now(),
		sendChan:  make(chan *Message, 10),
		closeChan: make(chan struct{}),
	}

	err := engine.AddClient(mockClient)
	if err != nil {
		t.Fatalf("Failed to add client: %v", err)
	}

	clients := engine.GetConnectedClients()
	if len(clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(clients))
	}

	if clients[0].ID != clientID {
		t.Errorf("Expected client ID %s, got %s", clientID, clients[0].ID)
	}

	err = engine.RemoveClient(clientID)
	if err != nil {
		t.Fatalf("Failed to remove client: %v", err)
	}

	clients = engine.GetConnectedClients()
	if len(clients) != 0 {
		t.Errorf("Expected 0 clients, got %d", len(clients))
	}
}

func TestCollaborationEngine_ProcessOperation(t *testing.T) {
	store := setupTestStorage(t)
	engine := NewCollaborationEngine(store)

	// Add a test client
	clientID := ClientID("test_client")
	authorID := operations.AuthorID("test_author")

	mockClient := &ClientConnection{
		ID:        clientID,
		AuthorID:  authorID,
		Documents: make(map[string]bool),
		LastSeen:  time.Now(),
		sendChan:  make(chan *Message, 10),
		closeChan: make(chan struct{}),
	}

	engine.AddClient(mockClient)

	// Create a test operation
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: authorID},
	})

	op := &operations.Operation{
		ID:        operations.NewOperationID([]byte("test_op")),
		Type:      operations.OpInsert,
		Position:  pos,
		Content:   "hello world",
		Author:    authorID,
		Timestamp: time.Now(),
		Parents:   []operations.OperationID{},
		Metadata: operations.OperationMeta{
			SessionID: "session1",
			Intent:    "test",
			Context:   map[string]string{"document_id": "test.go"},
		},
	}

	err := engine.ProcessOperation(op, clientID)
	if err != nil {
		t.Fatalf("Failed to process operation: %v", err)
	}

	// Verify operation was stored
	stored, err := store.GetOperation(op.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve stored operation: %v", err)
	}

	if stored.Content != op.Content {
		t.Errorf("Expected content %q, got %q", op.Content, stored.Content)
	}

	// Verify document was updated
	doc, err := engine.GetDocumentState("test.go")
	if err != nil {
		t.Fatalf("Failed to get document state: %v", err)
	}

	if doc.Version != 1 {
		t.Errorf("Expected document version 1, got %d", doc.Version)
	}

	if len(doc.Constructs) != 1 {
		t.Errorf("Expected 1 construct, got %d", len(doc.Constructs))
	}
}

func TestCollaborationEngine_SyncClient(t *testing.T) {
	store := setupTestStorage(t)
	engine := NewCollaborationEngine(store)

	// Add a test client
	clientID := ClientID("test_client")
	authorID := operations.AuthorID("test_author")

	mockClient := &ClientConnection{
		ID:        clientID,
		AuthorID:  authorID,
		Documents: make(map[string]bool),
		LastSeen:  time.Now(),
		sendChan:  make(chan *Message, 10),
		closeChan: make(chan struct{}),
	}

	engine.AddClient(mockClient)

	// Create a test document with some content
	doc := positioning.NewDocument("test.go")
	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: authorID},
	})

	construct := &positioning.Construct{
		ID:         "construct1",
		Content:    "package main",
		Type:       positioning.ConstructContent,
		Position:   pos,
		CreatedBy:  operations.NewOperationID([]byte("op1")),
		ModifiedBy: operations.NewOperationID([]byte("op1")),
	}

	doc.InsertConstruct(construct)
	doc.Version = 1

	store.StoreDocument(doc)

	// Test sync
	err := engine.SyncClient(clientID, "test.go", 0)
	if err != nil {
		t.Fatalf("Failed to sync client: %v", err)
	}

	// Check if client is subscribed
	if !mockClient.IsSubscribedTo("test.go") {
		t.Error("Expected client to be subscribed to test.go")
	}

	// Check if sync message was sent
	select {
	case msg := <-mockClient.sendChan:
		if msg.Type != MsgSync {
			t.Errorf("Expected sync message, got %s", msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected sync message to be sent")
	}
}

func TestPresenceTracker(t *testing.T) {
	tracker := NewPresenceTracker()

	clientID := ClientID("test_client")
	authorID := operations.AuthorID("test_author")

	// Add client
	tracker.AddClient(clientID, authorID)

	// Get presence
	info, err := tracker.GetPresence(clientID)
	if err != nil {
		t.Fatalf("Failed to get presence: %v", err)
	}

	if info.ClientID != clientID {
		t.Errorf("Expected client ID %s, got %s", clientID, info.ClientID)
	}

	if info.AuthorID != authorID {
		t.Errorf("Expected author ID %s, got %s", authorID, info.AuthorID)
	}

	// Update presence
	presence := PresencePayload{
		AuthorID:   authorID,
		DocumentID: "test.go",
		Status:     StatusActive,
		LastActive: time.Now(),
	}

	err = tracker.UpdatePresence(clientID, presence)
	if err != nil {
		t.Fatalf("Failed to update presence: %v", err)
	}

	// Get document presence
	docPresence := tracker.GetDocumentPresence("test.go")
	if len(docPresence) != 1 {
		t.Errorf("Expected 1 presence entry, got %d", len(docPresence))
	}

	if docPresence[0].Presence.DocumentID != "test.go" {
		t.Errorf("Expected document ID test.go, got %s", docPresence[0].Presence.DocumentID)
	}

	// Remove client
	tracker.RemoveClient(clientID)

	_, err = tracker.GetPresence(clientID)
	if err != ErrClientNotFound {
		t.Errorf("Expected client not found error, got %v", err)
	}
}

func setupTestStorage(t *testing.T) storage.Store {
	store, err := storage.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test storage: %v", err)
	}
	return store
}
