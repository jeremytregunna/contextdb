package addressing

import (
	"sync"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
)

type AddressResolver struct {
	operationIndex  map[operations.OperationID]*operations.Operation
	constructIndex  map[operations.PositionKey]*positioning.Construct
	addressIndex    map[AddressKey]*ResolvedAddress
	forwardingTable map[AddressKey]AddressKey // Handle content movement
	documents       map[string]*positioning.Document
	mutex           sync.RWMutex
}

type ResolvedAddress struct {
	Address         StableAddress            `json:"address"`
	CurrentRange    PositionRange            `json:"current_range"`
	Constructs      []*positioning.Construct `json:"constructs"`
	CreationOp      *operations.Operation    `json:"creation_op"`
	LastModified    time.Time                `json:"last_modified"`
	IsValid         bool                     `json:"is_valid"`
	MovementHistory []MovementRecord         `json:"movement_history,omitempty"`
}

type MovementRecord struct {
	Timestamp time.Time              `json:"timestamp"`
	FromRange PositionRange          `json:"from_range"`
	ToRange   PositionRange          `json:"to_range"`
	CausedBy  operations.OperationID `json:"caused_by"`
	Reason    MovementReason         `json:"reason"`
}

type MovementReason string

const (
	MovementRefactor MovementReason = "refactor"
	MovementMove     MovementReason = "move"
	MovementEdit     MovementReason = "edit"
	MovementDelete   MovementReason = "delete"
)

func NewAddressResolver() *AddressResolver {
	return &AddressResolver{
		operationIndex:  make(map[operations.OperationID]*operations.Operation),
		constructIndex:  make(map[operations.PositionKey]*positioning.Construct),
		addressIndex:    make(map[AddressKey]*ResolvedAddress),
		forwardingTable: make(map[AddressKey]AddressKey),
		documents:       make(map[string]*positioning.Document),
	}
}

func (r *AddressResolver) CreateAddress(repo RepositoryID, creationOpID operations.OperationID, posRange PositionRange) (StableAddress, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Validate that the operation exists
	creationOp, exists := r.operationIndex[creationOpID]
	if !exists {
		return StableAddress{}, ErrOperationNotFound
	}

	// Create the stable address
	address := NewStableAddress(repo, creationOpID, posRange)

	// Find constructs in the range
	constructs := r.getConstructsInRange(posRange)

	// Create resolved address
	resolved := &ResolvedAddress{
		Address:         address,
		CurrentRange:    posRange,
		Constructs:      constructs,
		CreationOp:      creationOp,
		LastModified:    time.Now(),
		IsValid:         true,
		MovementHistory: make([]MovementRecord, 0),
	}

	r.addressIndex[address.Key()] = resolved
	return address, nil
}

func (r *AddressResolver) ResolveAddress(addr StableAddress) (*ResolvedAddress, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	addressKey := addr.Key()

	// Check for forwarding first
	if forwardedKey, exists := r.forwardingTable[addressKey]; exists {
		addressKey = forwardedKey
	}

	resolved, exists := r.addressIndex[addressKey]
	if !exists {
		return nil, ErrAddressNotFound
	}

	// Create a copy to avoid race conditions
	return &ResolvedAddress{
		Address:         resolved.Address,
		CurrentRange:    resolved.CurrentRange,
		Constructs:      resolved.Constructs, // Slice is already copied
		CreationOp:      resolved.CreationOp,
		LastModified:    resolved.LastModified,
		IsValid:         resolved.IsValid,
		MovementHistory: resolved.MovementHistory,
	}, nil
}

func (r *AddressResolver) UpdateAddressLocation(addr StableAddress, newRange PositionRange, causedBy operations.OperationID, reason MovementReason) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	addressKey := addr.Key()
	resolved, exists := r.addressIndex[addressKey]
	if !exists {
		return ErrAddressNotFound
	}

	// Record the movement
	movement := MovementRecord{
		Timestamp: time.Now(),
		FromRange: resolved.CurrentRange,
		ToRange:   newRange,
		CausedBy:  causedBy,
		Reason:    reason,
	}

	resolved.MovementHistory = append(resolved.MovementHistory, movement)
	resolved.CurrentRange = newRange
	resolved.LastModified = time.Now()

	// Update constructs in new range
	resolved.Constructs = r.getConstructsInRange(newRange)

	// Validate the new location
	resolved.IsValid = !newRange.IsEmpty() && len(resolved.Constructs) > 0

	return nil
}

func (r *AddressResolver) TrackMovement(addr StableAddress, fromRange, toRange PositionRange, causedBy operations.OperationID, reason MovementReason) error {
	return r.UpdateAddressLocation(addr, toRange, causedBy, reason)
}

func (r *AddressResolver) GetAddressHistory(addr StableAddress) ([]MovementRecord, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	resolved, exists := r.addressIndex[addr.Key()]
	if !exists {
		return nil, ErrAddressNotFound
	}

	// Return a copy of the history
	history := make([]MovementRecord, len(resolved.MovementHistory))
	copy(history, resolved.MovementHistory)
	return history, nil
}

func (r *AddressResolver) InvalidateAddress(addr StableAddress, reason MovementReason) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	resolved, exists := r.addressIndex[addr.Key()]
	if !exists {
		return ErrAddressNotFound
	}

	resolved.IsValid = false
	resolved.LastModified = time.Now()

	// Record why it became invalid
	movement := MovementRecord{
		Timestamp: time.Now(),
		FromRange: resolved.CurrentRange,
		ToRange:   PositionRange{}, // Empty range indicates deletion
		Reason:    reason,
	}
	resolved.MovementHistory = append(resolved.MovementHistory, movement)

	return nil
}

func (r *AddressResolver) IndexOperation(op *operations.Operation) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.operationIndex[op.ID] = op
	return nil
}

func (r *AddressResolver) IndexDocument(doc *positioning.Document) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.documents[doc.FilePath] = doc

	// Index all constructs
	for posKey, construct := range doc.Constructs {
		r.constructIndex[posKey] = construct
	}

	return nil
}

func (r *AddressResolver) getConstructsInRange(posRange PositionRange) []*positioning.Construct {
	var constructs []*positioning.Construct

	for _, construct := range r.constructIndex {
		if posRange.Contains(construct.Position) {
			constructs = append(constructs, construct)
		}
	}

	return constructs
}

func (r *AddressResolver) GetAddressesByDocument(documentPath string) ([]StableAddress, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var addresses []StableAddress

	for _, resolved := range r.addressIndex {
		// Check if any constructs belong to this document
		for _, construct := range resolved.Constructs {
			if r.constructBelongsToDocument(construct, documentPath) {
				addresses = append(addresses, resolved.Address)
				break
			}
		}
	}

	return addresses, nil
}

func (r *AddressResolver) constructBelongsToDocument(construct *positioning.Construct, documentPath string) bool {
	doc, exists := r.documents[documentPath]
	if !exists {
		return false
	}

	// Check if the construct exists in the document
	posKey := construct.Position.Key()
	docConstruct, exists := doc.Constructs[posKey]
	if !exists {
		return false
	}

	// Verify it's the same construct by comparing ID and position
	return docConstruct.ID == construct.ID
}

func (r *AddressResolver) ProcessOperation(op *operations.Operation) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Index the operation
	r.operationIndex[op.ID] = op

	// Check if this operation affects any existing addresses
	for _, resolved := range r.addressIndex {
		if r.operationAffectsAddress(op, resolved) {
			r.updateAddressForOperation(op, resolved)
		}
	}

	return nil
}

func (r *AddressResolver) operationAffectsAddress(op *operations.Operation, resolved *ResolvedAddress) bool {
	// Check if the operation position overlaps with the address range
	return resolved.CurrentRange.Contains(op.Position)
}

func (r *AddressResolver) updateAddressForOperation(op *operations.Operation, resolved *ResolvedAddress) {
	reason := MovementEdit
	newRange := resolved.CurrentRange

	switch op.Type {
	case operations.OpDelete:
		reason = MovementDelete
		// If the deletion affects our range, adjust or invalidate
		if resolved.CurrentRange.Contains(op.Position) {
			resolved.IsValid = false
			newRange = PositionRange{} // Empty range indicates deletion
		}
	case operations.OpInsert:
		// If insertion is within our range, we might need to expand
		if resolved.CurrentRange.Contains(op.Position) {
			// For inserts, we generally maintain the same range
			// unless it's a significant structural change
			reason = MovementEdit
		}
	case operations.OpMove:
		reason = MovementMove
		// For move operations, we'd need to track the new position
		// This would require more sophisticated position tracking
	}

	movement := MovementRecord{
		Timestamp: time.Now(),
		FromRange: resolved.CurrentRange,
		ToRange:   newRange,
		CausedBy:  op.ID,
		Reason:    reason,
	}

	resolved.MovementHistory = append(resolved.MovementHistory, movement)
	resolved.CurrentRange = newRange
	resolved.LastModified = time.Now()

	// Update constructs to reflect current state
	resolved.Constructs = r.getConstructsInRange(newRange)
}
