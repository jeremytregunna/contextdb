package positioning

import (
	"math/big"
	"testing"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

func TestDocument_InsertConstruct(t *testing.T) {
	doc := NewDocument("test.go")

	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})

	construct := &Construct{
		ID:         "construct1",
		Content:    "package main",
		Type:       ConstructContent,
		Position:   pos,
		CreatedBy:  operations.NewOperationID([]byte("op1")),
		ModifiedBy: operations.NewOperationID([]byte("op1")),
	}

	err := doc.InsertConstruct(construct)
	if err != nil {
		t.Fatalf("Failed to insert construct: %v", err)
	}

	retrieved, err := doc.GetConstruct(pos)
	if err != nil {
		t.Fatalf("Failed to retrieve construct: %v", err)
	}

	if retrieved.Content != construct.Content {
		t.Errorf("Expected content %q, got %q", construct.Content, retrieved.Content)
	}
}

func TestDocument_GetConstructsByType(t *testing.T) {
	doc := NewDocument("test.go")

	constructs := []*Construct{
		{
			ID:       "construct1",
			Content:  "package main",
			Type:     ConstructContent,
			Position: operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}}),
		},
		{
			ID:       "construct2",
			Content:  "// This is a comment",
			Type:     ConstructDocumentation,
			Position: operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(2), AuthorID: "author1"}}),
		},
		{
			ID:       "construct3",
			Content:  "func main() {",
			Type:     ConstructContent,
			Position: operations.NewLogootPosition([]operations.PositionSegment{{Value: big.NewInt(3), AuthorID: "author1"}}),
		},
	}

	for _, construct := range constructs {
		doc.InsertConstruct(construct)
	}

	contentConstructs, err := doc.GetConstructsByType(ConstructContent)
	if err != nil {
		t.Fatalf("Failed to get constructs by type: %v", err)
	}

	if len(contentConstructs) != 2 {
		t.Errorf("Expected 2 content constructs, got %d", len(contentConstructs))
	}

	docConstructs, err := doc.GetConstructsByType(ConstructDocumentation)
	if err != nil {
		t.Fatalf("Failed to get documentation constructs: %v", err)
	}

	if len(docConstructs) != 1 {
		t.Errorf("Expected 1 documentation construct, got %d", len(docConstructs))
	}
}

func TestDocument_ApplyOperation(t *testing.T) {
	doc := NewDocument("test.go")

	pos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})

	insertOp := &operations.Operation{
		ID:        operations.NewOperationID([]byte("insert1")),
		Type:      operations.OpInsert,
		Position:  pos,
		Content:   "hello",
		Author:    "author1",
		Timestamp: time.Now(),
		Metadata: operations.OperationMeta{
			Intent: "test",
		},
	}

	err := doc.ApplyOperation(insertOp)
	if err != nil {
		t.Fatalf("Failed to apply insert operation: %v", err)
	}

	construct, err := doc.GetConstruct(pos)
	if err != nil {
		t.Fatalf("Failed to retrieve construct after insert: %v", err)
	}

	if construct.Content != "hello" {
		t.Errorf("Expected content 'hello', got %q", construct.Content)
	}

	deleteOp := &operations.Operation{
		ID:        operations.NewOperationID([]byte("delete1")),
		Type:      operations.OpDelete,
		Position:  pos,
		Author:    "author1",
		Timestamp: time.Now(),
	}

	err = doc.ApplyOperation(deleteOp)
	if err != nil {
		t.Fatalf("Failed to apply delete operation: %v", err)
	}

	_, err = doc.GetConstruct(pos)
	if err != ErrConstructNotFound {
		t.Error("Expected construct to be deleted")
	}
}

func TestDocument_Render(t *testing.T) {
	doc := NewDocument("test.go")

	constructs := []struct {
		pos     *big.Int
		content string
	}{
		{big.NewInt(1), "package"},
		{big.NewInt(2), " "},
		{big.NewInt(3), "main"},
		{big.NewInt(4), "\n"},
	}

	for _, c := range constructs {
		pos := operations.NewLogootPosition([]operations.PositionSegment{{Value: c.pos, AuthorID: "author1"}})
		construct := &Construct{
			ID:       ConstructID("construct" + c.pos.String()),
			Content:  c.content,
			Type:     ConstructContent,
			Position: pos,
		}
		doc.InsertConstruct(construct)
	}

	rendered, err := doc.Render()
	if err != nil {
		t.Fatalf("Failed to render document: %v", err)
	}

	expected := "package main\n"
	if rendered != expected {
		t.Errorf("Expected rendered content %q, got %q", expected, rendered)
	}
}

func TestInferConstructType(t *testing.T) {
	doc := NewDocument("test.go")

	testCases := []struct {
		content  string
		metadata operations.OperationMeta
		expected ConstructType
	}{
		{
			content:  "package main",
			metadata: operations.OperationMeta{},
			expected: ConstructContent,
		},
		{
			content:  "// This is a comment",
			metadata: operations.OperationMeta{Intent: "documentation"},
			expected: ConstructDocumentation,
		},
		{
			content:  "func TestSomething(t *testing.T) {",
			metadata: operations.OperationMeta{Intent: "test"},
			expected: ConstructTest,
		},
		{
			content:  "\n",
			metadata: operations.OperationMeta{},
			expected: ConstructNewline,
		},
		{
			content:  "    ",
			metadata: operations.OperationMeta{},
			expected: ConstructWhitespace,
		},
	}

	for _, tc := range testCases {
		result := doc.inferConstructType(tc.content, tc.metadata)
		if result != tc.expected {
			t.Errorf("Expected construct type %s for content %q, got %s", tc.expected, tc.content, result)
		}
	}
}
