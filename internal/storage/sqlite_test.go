package storage

import (
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
)

func TestSQLiteStore_OperationCRUD(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})

	op := &operations.Operation{
		ID:        operations.NewOperationID([]byte("test1")),
		Type:      operations.OpInsert,
		Position:  pos,
		Content:   "hello world",
		Author:    "author1",
		Timestamp: time.Now(),
		Parents:   []operations.OperationID{},
		Metadata: operations.OperationMeta{
			SessionID: "session1",
			Intent:    "test",
			Context:   map[string]string{"type": "content"},
		},
	}

	err := store.StoreOperation(op)
	if err != nil {
		t.Fatalf("Failed to store operation: %v", err)
	}

	retrieved, err := store.GetOperation(op.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve operation: %v", err)
	}

	if retrieved.Content != op.Content {
		t.Errorf("Expected content %q, got %q", op.Content, retrieved.Content)
	}

	if retrieved.Author != op.Author {
		t.Errorf("Expected author %q, got %q", op.Author, retrieved.Author)
	}

	if retrieved.Metadata.Intent != op.Metadata.Intent {
		t.Errorf("Expected intent %q, got %q", op.Metadata.Intent, retrieved.Metadata.Intent)
	}
}

func TestSQLiteStore_GetOperationsByAuthor(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	author1 := operations.AuthorID("author1")
	author2 := operations.AuthorID("author2")

	ops := []*operations.Operation{
		{
			ID:        operations.NewOperationID([]byte("op1")),
			Type:      operations.OpInsert,
			Position:  operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(1), AuthorID: author1}}),
			Content:   "content1",
			Author:    author1,
			Timestamp: time.Now(),
		},
		{
			ID:        operations.NewOperationID([]byte("op2")),
			Type:      operations.OpInsert,
			Position:  operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(2), AuthorID: author2}}),
			Content:   "content2",
			Author:    author2,
			Timestamp: time.Now(),
		},
		{
			ID:        operations.NewOperationID([]byte("op3")),
			Type:      operations.OpInsert,
			Position:  operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(3), AuthorID: author1}}),
			Content:   "content3",
			Author:    author1,
			Timestamp: time.Now(),
		},
	}

	for _, op := range ops {
		store.StoreOperation(op)
	}

	author1Ops, err := store.GetOperationsByAuthor(author1)
	if err != nil {
		t.Fatalf("Failed to get operations by author1: %v", err)
	}

	if len(author1Ops) != 2 {
		t.Errorf("Expected 2 operations for author1, got %d", len(author1Ops))
	}

	author2Ops, err := store.GetOperationsByAuthor(author2)
	if err != nil {
		t.Fatalf("Failed to get operations by author2: %v", err)
	}

	if len(author2Ops) != 1 {
		t.Errorf("Expected 1 operation for author2, got %d", len(author2Ops))
	}
}

func TestSQLiteStore_DocumentCRUD(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	doc := positioning.NewDocument("test.go")

	pos1 := operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}})
	pos2 := operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(2), AuthorID: "author1"}})

	construct1 := &positioning.Construct{
		ID:         "construct1",
		Content:    "package main",
		Type:       positioning.ConstructContent,
		Position:   pos1,
		CreatedBy:  operations.NewOperationID([]byte("op1")),
		ModifiedBy: operations.NewOperationID([]byte("op1")),
		Metadata: positioning.ConstructMeta{
			Semantic:   "package_declaration",
			Confidence: 1.0,
		},
	}

	construct2 := &positioning.Construct{
		ID:         "construct2",
		Content:    "\n",
		Type:       positioning.ConstructNewline,
		Position:   pos2,
		CreatedBy:  operations.NewOperationID([]byte("op2")),
		ModifiedBy: operations.NewOperationID([]byte("op2")),
	}

	doc.InsertConstruct(construct1)
	doc.InsertConstruct(construct2)
	doc.Version = 1

	err := store.StoreDocument(doc)
	if err != nil {
		t.Fatalf("Failed to store document: %v", err)
	}

	retrieved, err := store.GetDocument("test.go")
	if err != nil {
		t.Fatalf("Failed to retrieve document: %v", err)
	}

	if retrieved.Version != doc.Version {
		t.Errorf("Expected version %d, got %d", doc.Version, retrieved.Version)
	}

	if len(retrieved.Constructs) != 2 {
		t.Errorf("Expected 2 constructs, got %d", len(retrieved.Constructs))
	}

	construct1Retrieved, exists := retrieved.Constructs[pos1.Key()]
	if !exists {
		t.Error("Expected construct1 to exist")
	} else if construct1Retrieved.Content != "package main" {
		t.Errorf("Expected content 'package main', got %q", construct1Retrieved.Content)
	}
}

func TestSQLiteStore_ListDocuments(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	docs := []string{"file1.go", "file2.go", "file3.py"}

	for _, filePath := range docs {
		doc := positioning.NewDocument(filePath)
		doc.Version = 1
		store.StoreDocument(doc)
	}

	retrieved, err := store.ListDocuments()
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 documents, got %d", len(retrieved))
	}

	for i, expected := range docs {
		if retrieved[i] != expected {
			t.Errorf("Expected document %s at index %d, got %s", expected, i, retrieved[i])
		}
	}
}

func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	tmpFile, err := os.CreateTemp("", "contextdb_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create test store: %v", err)
	}

	return store, func() {
		store.Close()
		os.Remove(tmpFile.Name())
	}
}
