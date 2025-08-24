package operations

import (
	"encoding/hex"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"
)

type OperationID string

func NewOperationID(content []byte) OperationID {
	hash := sha3.Sum256(content)
	return OperationID(hex.EncodeToString(hash[:]))
}

type Operation struct {
	ID          OperationID    `json:"id"`
	Type        OperationType  `json:"type"`
	Position    LogootPosition `json:"position"`
	Content     string         `json:"content"`
	ContentType string         `json:"content_type,omitempty"`
	Length      int            `json:"length,omitempty"`
	Author      AuthorID       `json:"author"`
	Timestamp   time.Time      `json:"timestamp"`
	Parents     []OperationID  `json:"parents"`
	Metadata    OperationMeta  `json:"metadata"`
}

type OperationType string

const (
	OpInsert OperationType = "insert"
	OpDelete OperationType = "delete"
	OpMove   OperationType = "move"
)

// Content type constants
const (
	ContentTypeText   = "text"
	ContentTypeJSON   = "json"
	ContentTypeBinary = "binary"
)

type OperationMeta struct {
	SessionID string            `json:"session_id"`
	Intent    string            `json:"intent,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
}

type AuthorID string

func NewAuthorID(name string) AuthorID {
	hash := sha3.Sum256([]byte(name))
	return AuthorID(hex.EncodeToString(hash[:]))
}

type OperationDAG struct {
	operations map[OperationID]*Operation
	children   map[OperationID][]OperationID
	roots      []OperationID
	heads      []OperationID
	mutex      sync.RWMutex
}

func NewOperationDAG() *OperationDAG {
	return &OperationDAG{
		operations: make(map[OperationID]*Operation),
		children:   make(map[OperationID][]OperationID),
		roots:      make([]OperationID, 0),
		heads:      make([]OperationID, 0),
	}
}

func (dag *OperationDAG) AddOperation(op *Operation) error {
	dag.mutex.Lock()
	defer dag.mutex.Unlock()

	if _, exists := dag.operations[op.ID]; exists {
		return nil
	}

	dag.operations[op.ID] = op

	if len(op.Parents) == 0 {
		dag.roots = append(dag.roots, op.ID)
	} else {
		for _, parentID := range op.Parents {
			dag.children[parentID] = append(dag.children[parentID], op.ID)
			dag.removeFromHeads(parentID)
		}
	}

	dag.heads = append(dag.heads, op.ID)
	return nil
}

func (dag *OperationDAG) GetOperation(id OperationID) (*Operation, error) {
	dag.mutex.RLock()
	defer dag.mutex.RUnlock()

	op, exists := dag.operations[id]
	if !exists {
		return nil, ErrOperationNotFound
	}
	return op, nil
}

func (dag *OperationDAG) GetOperationsSince(timestamp time.Time) ([]*Operation, error) {
	dag.mutex.RLock()
	defer dag.mutex.RUnlock()

	var operations []*Operation
	for _, op := range dag.operations {
		if op.Timestamp.After(timestamp) {
			operations = append(operations, op)
		}
	}
	return operations, nil
}

func (dag *OperationDAG) GetOperationsByAuthor(author AuthorID) ([]*Operation, error) {
	dag.mutex.RLock()
	defer dag.mutex.RUnlock()

	var operations []*Operation
	for _, op := range dag.operations {
		if op.Author == author {
			operations = append(operations, op)
		}
	}
	return operations, nil
}

func (dag *OperationDAG) GetCausalHistory(id OperationID) ([]*Operation, error) {
	dag.mutex.RLock()
	defer dag.mutex.RUnlock()

	visited := make(map[OperationID]bool)
	var history []*Operation

	var traverse func(OperationID)
	traverse = func(opID OperationID) {
		if visited[opID] {
			return
		}
		visited[opID] = true

		op, exists := dag.operations[opID]
		if !exists {
			return
		}

		for _, parentID := range op.Parents {
			traverse(parentID)
		}

		history = append(history, op)
	}

	traverse(id)
	return history, nil
}

func (dag *OperationDAG) ValidateOperation(op *Operation) error {
	if op == nil {
		return ErrInvalidOperation
	}

	if op.Author == "" {
		return ErrInvalidAuthor
	}

	if op.Type != OpInsert && op.Type != OpDelete && op.Type != OpMove {
		return ErrInvalidOperationType
	}

	return nil
}

func (dag *OperationDAG) removeFromHeads(id OperationID) {
	for i, headID := range dag.heads {
		if headID == id {
			dag.heads = append(dag.heads[:i], dag.heads[i+1:]...)
			return
		}
	}
}
