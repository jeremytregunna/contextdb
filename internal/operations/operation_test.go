package operations

import (
	"math/big"
	"testing"
	"time"
)

func TestOperationDAG_AddOperation(t *testing.T) {
	dag := NewOperationDAG()

	op1 := &Operation{
		ID:        NewOperationID([]byte("test1")),
		Type:      OpInsert,
		Position:  NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}}),
		Content:   "hello",
		Author:    "author1",
		Timestamp: time.Now(),
		Parents:   []OperationID{},
	}

	err := dag.AddOperation(op1)
	if err != nil {
		t.Fatalf("Failed to add operation: %v", err)
	}

	retrievedOp, err := dag.GetOperation(op1.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve operation: %v", err)
	}

	if retrievedOp.Content != op1.Content {
		t.Errorf("Expected content %q, got %q", op1.Content, retrievedOp.Content)
	}
}

func TestOperationDAG_CausalHistory(t *testing.T) {
	dag := NewOperationDAG()

	op1 := &Operation{
		ID:        NewOperationID([]byte("test1")),
		Type:      OpInsert,
		Position:  NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}}),
		Content:   "hello",
		Author:    "author1",
		Timestamp: time.Now(),
		Parents:   []OperationID{},
	}

	op2 := &Operation{
		ID:        NewOperationID([]byte("test2")),
		Type:      OpInsert,
		Position:  NewLogootPosition([]PositionSegment{{Value: big.NewInt(2), AuthorID: "author1"}}),
		Content:   " world",
		Author:    "author1",
		Timestamp: time.Now(),
		Parents:   []OperationID{op1.ID},
	}

	dag.AddOperation(op1)
	dag.AddOperation(op2)

	history, err := dag.GetCausalHistory(op2.ID)
	if err != nil {
		t.Fatalf("Failed to get causal history: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 operations in history, got %d", len(history))
	}

	if history[0].ID != op1.ID {
		t.Errorf("Expected first operation to be op1")
	}

	if history[1].ID != op2.ID {
		t.Errorf("Expected second operation to be op2")
	}
}

func TestLogootPosition_Compare(t *testing.T) {
	pos1 := NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}})
	pos2 := NewLogootPosition([]PositionSegment{{Value: big.NewInt(2), AuthorID: "author1"}})
	pos3 := NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author2"}})

	if pos1.Compare(pos2) >= 0 {
		t.Error("pos1 should be less than pos2")
	}

	if pos2.Compare(pos1) <= 0 {
		t.Error("pos2 should be greater than pos1")
	}

	if pos1.Compare(pos3) >= 0 {
		t.Error("pos1 should be less than pos3 (author tie-breaker)")
	}
}

func TestGeneratePosition(t *testing.T) {
	pos1 := NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}})
	pos2 := NewLogootPosition([]PositionSegment{{Value: big.NewInt(3), AuthorID: "author1"}})

	newPos := GeneratePosition(pos1, pos2, "author2")

	if !newPos.IsValid() {
		t.Error("Generated position should be valid")
	}

	if newPos.Compare(pos1) <= 0 {
		t.Error("Generated position should be greater than left boundary")
	}

	if newPos.Compare(pos2) >= 0 {
		t.Error("Generated position should be less than right boundary")
	}
}

func TestOperationValidation(t *testing.T) {
	dag := NewOperationDAG()

	invalidOp := &Operation{}
	err := dag.ValidateOperation(invalidOp)
	if err == nil {
		t.Error("Should reject invalid operation")
	}

	validOp := &Operation{
		ID:        NewOperationID([]byte("test")),
		Type:      OpInsert,
		Position:  NewLogootPosition([]PositionSegment{{Value: big.NewInt(1), AuthorID: "author1"}}),
		Content:   "test",
		Author:    "author1",
		Timestamp: time.Now(),
	}

	err = dag.ValidateOperation(validOp)
	if err != nil {
		t.Errorf("Should accept valid operation: %v", err)
	}
}
