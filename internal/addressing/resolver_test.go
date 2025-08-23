package addressing

import (
	"math/big"
	"testing"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

func TestAddressResolver_CreateAndResolve(t *testing.T) {
	resolver := NewAddressResolver()

	// Create a test operation
	opID := operations.NewOperationID([]byte("test-operation"))
	op := &operations.Operation{
		ID:   opID,
		Type: operations.OpInsert,
		Position: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(1), AuthorID: "author1"},
		}),
		Content:   "hello world",
		Author:    "author1",
		Timestamp: time.Now(),
	}

	// Index the operation
	err := resolver.IndexOperation(op)
	if err != nil {
		t.Fatalf("Failed to index operation: %v", err)
	}

	// Create address
	repo := RepositoryID("test-repo")
	posRange := PositionRange{
		Start: op.Position,
		End:   op.Position,
	}

	addr, err := resolver.CreateAddress(repo, opID, posRange)
	if err != nil {
		t.Fatalf("Failed to create address: %v", err)
	}

	// Resolve address
	resolved, err := resolver.ResolveAddress(addr)
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	if resolved.Address.Repository != repo {
		t.Errorf("Expected repository %s, got %s", repo, resolved.Address.Repository)
	}

	if resolved.CreationOp.ID != opID {
		t.Errorf("Expected operation ID %x, got %x", opID, resolved.CreationOp.ID)
	}

	if !resolved.IsValid {
		t.Error("Resolved address should be valid")
	}
}

func TestAddressResolver_TrackMovement(t *testing.T) {
	resolver := NewAddressResolver()

	// Create initial setup
	opID := operations.NewOperationID([]byte("test-operation"))
	op := &operations.Operation{
		ID:   opID,
		Type: operations.OpInsert,
		Position: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(1), AuthorID: "author1"},
		}),
		Content:   "hello world",
		Author:    "author1",
		Timestamp: time.Now(),
	}

	resolver.IndexOperation(op)

	repo := RepositoryID("test-repo")
	initialRange := PositionRange{Start: op.Position, End: op.Position}

	addr, err := resolver.CreateAddress(repo, opID, initialRange)
	if err != nil {
		t.Fatalf("Failed to create address: %v", err)
	}

	// Create new position for movement
	newPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(5), AuthorID: "author1"},
	})
	newRange := PositionRange{Start: newPos, End: newPos}

	// Track movement
	moveOpID := operations.NewOperationID([]byte("move-operation"))
	err = resolver.UpdateAddressLocation(addr, newRange, moveOpID, MovementRefactor)
	if err != nil {
		t.Fatalf("Failed to update address location: %v", err)
	}

	// Get movement history
	history, err := resolver.GetAddressHistory(addr)
	if err != nil {
		t.Fatalf("Failed to get address history: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected movement history to contain entries")
	}

	lastMovement := history[len(history)-1]
	if lastMovement.Reason != MovementRefactor {
		t.Errorf("Expected movement reason %s, got %s", MovementRefactor, lastMovement.Reason)
	}

	if lastMovement.CausedBy != moveOpID {
		t.Errorf("Expected movement caused by %x, got %x", moveOpID, lastMovement.CausedBy)
	}
}

func TestAddressResolver_InvalidateAddress(t *testing.T) {
	resolver := NewAddressResolver()

	// Create initial setup
	opID := operations.NewOperationID([]byte("test-operation"))
	op := &operations.Operation{
		ID:   opID,
		Type: operations.OpInsert,
		Position: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(1), AuthorID: "author1"},
		}),
		Content:   "hello world",
		Author:    "author1",
		Timestamp: time.Now(),
	}

	resolver.IndexOperation(op)

	repo := RepositoryID("test-repo")
	posRange := PositionRange{Start: op.Position, End: op.Position}

	addr, err := resolver.CreateAddress(repo, opID, posRange)
	if err != nil {
		t.Fatalf("Failed to create address: %v", err)
	}

	// Invalidate address
	err = resolver.InvalidateAddress(addr, MovementDelete)
	if err != nil {
		t.Fatalf("Failed to invalidate address: %v", err)
	}

	// Resolve address should show it's invalid
	resolved, err := resolver.ResolveAddress(addr)
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	if resolved.IsValid {
		t.Error("Address should be invalid after invalidation")
	}

	// Check history contains invalidation
	history, err := resolver.GetAddressHistory(addr)
	if err != nil {
		t.Fatalf("Failed to get address history: %v", err)
	}

	if len(history) == 0 {
		t.Error("Expected movement history to contain invalidation")
	}

	lastMovement := history[len(history)-1]
	if lastMovement.Reason != MovementDelete {
		t.Errorf("Expected movement reason %s, got %s", MovementDelete, lastMovement.Reason)
	}
}

func TestAddressResolver_ProcessOperation(t *testing.T) {
	resolver := NewAddressResolver()

	// Create initial operation and address
	opID1 := operations.NewOperationID([]byte("operation-1"))
	pos1 := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})

	op1 := &operations.Operation{
		ID:        opID1,
		Type:      operations.OpInsert,
		Position:  pos1,
		Content:   "hello",
		Author:    "author1",
		Timestamp: time.Now(),
	}

	resolver.IndexOperation(op1)

	repo := RepositoryID("test-repo")
	posRange := PositionRange{Start: pos1, End: pos1}
	addr, _ := resolver.CreateAddress(repo, opID1, posRange)

	// Process new operation that affects the address
	opID2 := operations.NewOperationID([]byte("operation-2"))
	op2 := &operations.Operation{
		ID:        opID2,
		Type:      operations.OpDelete,
		Position:  pos1, // Same position as the address
		Author:    "author2",
		Timestamp: time.Now(),
	}

	err := resolver.ProcessOperation(op2)
	if err != nil {
		t.Fatalf("Failed to process operation: %v", err)
	}

	// Check that the address was updated
	resolved, err := resolver.ResolveAddress(addr)
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}

	// Should be invalid due to deletion
	if resolved.IsValid {
		t.Error("Address should be invalid after delete operation")
	}

	// Should have movement history
	if len(resolved.MovementHistory) == 0 {
		t.Error("Expected movement history from operation processing")
	}
}
