package addressing

import (
	"fmt"
	"math/big"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

type StableAddress struct {
	Scheme        string                 `json:"scheme"` // "contextdb"
	Repository    RepositoryID           `json:"repository"`
	OperationID   operations.OperationID `json:"operation_id"`       // Operation that created the content
	PositionRange PositionRange          `json:"position_range"`     // Current location
	Fragment      string                 `json:"fragment,omitempty"` // Semantic hint: "function:calculateTotal"
}

type PositionRange struct {
	Start operations.LogootPosition `json:"start"`
	End   operations.LogootPosition `json:"end"`
}

type RepositoryID string

func NewStableAddress(repo RepositoryID, creationOp operations.OperationID, posRange PositionRange) StableAddress {
	return StableAddress{
		Scheme:        "contextdb",
		Repository:    repo,
		OperationID:   creationOp,
		PositionRange: posRange,
	}
}

func (addr StableAddress) String() string {
	return fmt.Sprintf("%s://%s/%x/%s-%s",
		addr.Scheme,
		addr.Repository,
		addr.OperationID[:8], // Show first 8 bytes for readability
		addr.PositionRange.Start.String(),
		addr.PositionRange.End.String(),
	)
}

func (addr StableAddress) Key() AddressKey {
	// Use operation ID + position range hash as unique key
	startKey := addr.PositionRange.Start.Key()
	endKey := addr.PositionRange.End.Key()
	key := fmt.Sprintf("%x:%x:%x",
		addr.OperationID[:16],
		startKey[:8],
		endKey[:8],
	)
	return AddressKey(key)
}

func (addr StableAddress) IsValid() bool {
	return addr.Scheme == "contextdb" &&
		addr.Repository != "" &&
		addr.PositionRange.Start.IsValid() &&
		addr.PositionRange.End.IsValid() &&
		addr.PositionRange.Start.Compare(addr.PositionRange.End) <= 0
}

type AddressKey string

func (pr PositionRange) Contains(pos operations.LogootPosition) bool {
	return pos.Compare(pr.Start) >= 0 && pos.Compare(pr.End) <= 0
}

func (pr PositionRange) Overlaps(other PositionRange) bool {
	return !(pr.End.Compare(other.Start) < 0 || other.End.Compare(pr.Start) < 0)
}

func (pr PositionRange) IsEmpty() bool {
	return pr.Start.Compare(pr.End) > 0
}

func (pr PositionRange) Size() int {
	if pr.IsEmpty() {
		return 0
	}

	// Calculate approximate size based on position difference
	// For big.Int values, we'll use a simplified approach
	if len(pr.Start.Segments) == 0 || len(pr.End.Segments) == 0 {
		return 1
	}

	// Use the first segment for comparison (simplified)
	startVal := pr.Start.Segments[0].Value
	endVal := pr.End.Segments[0].Value

	if startVal == nil || endVal == nil {
		return 1
	}

	diff := new(big.Int).Sub(endVal, startVal)

	// Convert to int64 for practical size representation
	if !diff.IsInt64() {
		return 1000000 // Very large range
	}

	diffInt := diff.Int64()
	if diffInt <= 0 {
		return 1 // Minimum size for valid range
	}

	// Cap at reasonable maximum for int conversion
	if diffInt > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1) // Max int value
	}

	return int(diffInt)
}
