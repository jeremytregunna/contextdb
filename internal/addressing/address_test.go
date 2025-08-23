package addressing

import (
	"math/big"
	"testing"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

func TestStableAddress_Creation(t *testing.T) {
	repo := RepositoryID("test-repo")
	opID := operations.NewOperationID([]byte("test-operation"))

	startPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	endPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(2), AuthorID: "author1"},
	})

	posRange := PositionRange{Start: startPos, End: endPos}

	addr := NewStableAddress(repo, opID, posRange)

	if addr.Scheme != "contextdb" {
		t.Errorf("Expected scheme 'contextdb', got %s", addr.Scheme)
	}

	if addr.Repository != repo {
		t.Errorf("Expected repository %s, got %s", repo, addr.Repository)
	}

	if !addr.IsValid() {
		t.Error("Address should be valid")
	}
}

func TestStableAddress_String(t *testing.T) {
	repo := RepositoryID("test-repo")
	opID := operations.NewOperationID([]byte("test-operation"))

	startPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	endPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(2), AuthorID: "author1"},
	})

	posRange := PositionRange{Start: startPos, End: endPos}
	addr := NewStableAddress(repo, opID, posRange)

	str := addr.String()
	if str == "" {
		t.Error("Address string should not be empty")
	}

	// Should contain the scheme and repository
	if !contains(str, "contextdb://") {
		t.Error("Address string should contain scheme")
	}

	if !contains(str, string(repo)) {
		t.Error("Address string should contain repository")
	}
}

func TestPositionRange_Contains(t *testing.T) {
	startPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(1), AuthorID: "author1"},
	})
	endPos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(5), AuthorID: "author1"},
	})

	posRange := PositionRange{Start: startPos, End: endPos}

	// Position within range
	middlePos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(3), AuthorID: "author1"},
	})

	if !posRange.Contains(middlePos) {
		t.Error("Range should contain middle position")
	}

	// Position outside range
	outsidePos := operations.NewLogootPosition([]operations.PositionSegment{
		{Value: big.NewInt(10), AuthorID: "author1"},
	})

	if posRange.Contains(outsidePos) {
		t.Error("Range should not contain outside position")
	}
}

func TestPositionRange_Overlaps(t *testing.T) {
	range1 := PositionRange{
		Start: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(1), AuthorID: "author1"},
		}),
		End: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(5), AuthorID: "author1"},
		}),
	}

	// Overlapping range
	range2 := PositionRange{
		Start: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(3), AuthorID: "author1"},
		}),
		End: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(7), AuthorID: "author1"},
		}),
	}

	if !range1.Overlaps(range2) {
		t.Error("Ranges should overlap")
	}

	// Non-overlapping range
	range3 := PositionRange{
		Start: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(10), AuthorID: "author1"},
		}),
		End: operations.NewLogootPosition([]operations.PositionSegment{
			{Value: big.NewInt(15), AuthorID: "author1"},
		}),
	}

	if range1.Overlaps(range3) {
		t.Error("Ranges should not overlap")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
