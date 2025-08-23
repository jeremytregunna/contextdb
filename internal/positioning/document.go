package positioning

import (
	"crypto/sha256"
	"sync"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

type Construct struct {
	ID         ConstructID               `json:"id"`
	Content    string                    `json:"content"`
	Type       ConstructType             `json:"type"`
	Position   operations.LogootPosition `json:"position"`
	CreatedBy  operations.OperationID    `json:"created_by"`
	ModifiedBy operations.OperationID    `json:"modified_by"`
	Metadata   ConstructMeta             `json:"metadata"`
}

type ConstructID string

type ConstructType string

const (
	ConstructContent       ConstructType = "content"
	ConstructDocumentation ConstructType = "documentation"
	ConstructTest          ConstructType = "test"
	ConstructConfiguration ConstructType = "configuration"
	ConstructReviewComment ConstructType = "review_comment"
	ConstructDiscussion    ConstructType = "discussion"
	ConstructDecision      ConstructType = "decision"
	ConstructQuestion      ConstructType = "question"
	ConstructIntent        ConstructType = "intent"
	ConstructContext       ConstructType = "context"
	ConstructReference     ConstructType = "reference"
	ConstructWhitespace    ConstructType = "whitespace"
	ConstructNewline       ConstructType = "newline"
)

type ConstructMeta struct {
	Semantic   string            `json:"semantic,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	References []string          `json:"references,omitempty"`
	Confidence float64           `json:"confidence,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type Document struct {
	FilePath      string                                               `json:"file_path"`
	Constructs    map[operations.PositionKey]*Construct                `json:"constructs"`
	PositionIndex map[operations.PositionKey]operations.LogootPosition `json:"position_index"`
	PositionIdx   []operations.LogootPosition                          `json:"position_idx"`
	ContentHash   [32]byte                                             `json:"content_hash"`
	Version       uint64                                               `json:"version"`
	LastOperation operations.OperationID                               `json:"last_operation"`
	mutex         sync.RWMutex
}

func NewDocument(filePath string) *Document {
	return &Document{
		FilePath:      filePath,
		Constructs:    make(map[operations.PositionKey]*Construct),
		PositionIndex: make(map[operations.PositionKey]operations.LogootPosition),
		PositionIdx:   make([]operations.LogootPosition, 0),
		Version:       0,
	}
}

func (doc *Document) InsertConstruct(construct *Construct) error {
	doc.mutex.Lock()
	defer doc.mutex.Unlock()

	if !construct.Position.IsValid() {
		return ErrInvalidPosition
	}

	posKey := construct.Position.Key()
	if _, exists := doc.Constructs[posKey]; exists {
		return ErrPositionOccupied
	}

	doc.Constructs[posKey] = construct
	doc.PositionIndex[posKey] = construct.Position
	doc.insertPositionSorted(construct.Position)
	doc.Version++
	doc.updateContentHash()

	return nil
}

func (doc *Document) DeleteConstruct(pos operations.LogootPosition) (*Construct, error) {
	doc.mutex.Lock()
	defer doc.mutex.Unlock()

	posKey := pos.Key()
	construct, exists := doc.Constructs[posKey]
	if !exists {
		return nil, ErrConstructNotFound
	}

	delete(doc.Constructs, posKey)
	delete(doc.PositionIndex, posKey)
	doc.removePositionFromIndex(pos)
	doc.Version++
	doc.updateContentHash()

	return construct, nil
}

func (doc *Document) GetConstruct(pos operations.LogootPosition) (*Construct, error) {
	doc.mutex.RLock()
	defer doc.mutex.RUnlock()

	posKey := pos.Key()
	construct, exists := doc.Constructs[posKey]
	if !exists {
		return nil, ErrConstructNotFound
	}
	return construct, nil
}

func (doc *Document) GetConstructsInRange(start, end operations.LogootPosition) ([]*Construct, error) {
	doc.mutex.RLock()
	defer doc.mutex.RUnlock()

	var constructs []*Construct
	for _, pos := range doc.PositionIdx {
		if pos.Compare(start) >= 0 && pos.Compare(end) <= 0 {
			posKey := pos.Key()
			if construct, exists := doc.Constructs[posKey]; exists {
				constructs = append(constructs, construct)
			}
		}
	}
	return constructs, nil
}

func (doc *Document) GetConstructsByType(constructType ConstructType) ([]*Construct, error) {
	doc.mutex.RLock()
	defer doc.mutex.RUnlock()

	var constructs []*Construct
	for _, construct := range doc.Constructs {
		if construct.Type == constructType {
			constructs = append(constructs, construct)
		}
	}
	return constructs, nil
}

func (doc *Document) GetPositionBetween(left, right operations.LogootPosition) (operations.LogootPosition, error) {
	doc.mutex.RLock()
	authorID := operations.AuthorID("system")
	doc.mutex.RUnlock()

	return operations.GeneratePosition(left, right, authorID), nil
}

func (doc *Document) Render() (string, error) {
	doc.mutex.RLock()
	defer doc.mutex.RUnlock()

	var content string
	for _, pos := range doc.PositionIdx {
		posKey := pos.Key()
		if construct, exists := doc.Constructs[posKey]; exists {
			content += construct.Content
		}
	}
	return content, nil
}

func (doc *Document) ApplyOperation(op *operations.Operation) error {
	doc.mutex.Lock()
	defer doc.mutex.Unlock()

	switch op.Type {
	case operations.OpInsert:
		return doc.applyInsert(op)
	case operations.OpDelete:
		return doc.applyDelete(op)
	default:
		return ErrUnsupportedOperation
	}
}

func (doc *Document) applyInsert(op *operations.Operation) error {
	posKey := op.Position.Key()
	if _, exists := doc.Constructs[posKey]; exists {
		return ErrPositionOccupied
	}

	constructType := doc.inferConstructType(op.Content, op.Metadata)
	construct := &Construct{
		ID:         ConstructID(op.ID),
		Content:    op.Content,
		Type:       constructType,
		Position:   op.Position,
		CreatedBy:  op.ID,
		ModifiedBy: op.ID,
		Metadata:   doc.buildConstructMeta(op),
	}

	doc.Constructs[posKey] = construct
	doc.PositionIndex[posKey] = op.Position
	doc.insertPositionSorted(op.Position)
	doc.LastOperation = op.ID
	doc.Version++
	doc.updateContentHash()

	return nil
}

func (doc *Document) applyDelete(op *operations.Operation) error {
	posKey := op.Position.Key()
	construct, exists := doc.Constructs[posKey]
	if !exists {
		return nil
	}

	delete(doc.Constructs, posKey)
	delete(doc.PositionIndex, posKey)
	doc.removePositionFromIndex(op.Position)
	doc.LastOperation = op.ID
	doc.Version++
	doc.updateContentHash()

	if construct != nil {
		construct.ModifiedBy = op.ID
	}
	return nil
}

func (doc *Document) insertPositionSorted(pos operations.LogootPosition) {
	// Binary search to find insertion point
	low, high := 0, len(doc.PositionIdx)

	for low < high {
		mid := (low + high) / 2
		if doc.PositionIdx[mid].Compare(pos) < 0 {
			low = mid + 1
		} else {
			high = mid
		}
	}

	// Insert at the correct position
	doc.PositionIdx = append(doc.PositionIdx, operations.LogootPosition{})
	copy(doc.PositionIdx[low+1:], doc.PositionIdx[low:])
	doc.PositionIdx[low] = pos
}

func (doc *Document) removePositionFromIndex(pos operations.LogootPosition) {
	for i, p := range doc.PositionIdx {
		if p.Compare(pos) == 0 {
			doc.PositionIdx = append(doc.PositionIdx[:i], doc.PositionIdx[i+1:]...)
			break
		}
	}
}

func (doc *Document) updateContentHash() {
	// This method is called from within locked methods, so don't take locks here
	var content string
	for _, pos := range doc.PositionIdx {
		posKey := pos.Key()
		if construct, exists := doc.Constructs[posKey]; exists {
			content += construct.Content
		}
	}
	doc.ContentHash = sha256.Sum256([]byte(content))
}

func (doc *Document) inferConstructType(content string, metadata operations.OperationMeta) ConstructType {
	if intent := metadata.Intent; intent != "" {
		switch intent {
		case "documentation", "comment", "doc":
			return ConstructDocumentation
		case "test", "testing":
			return ConstructTest
		case "config", "configuration":
			return ConstructConfiguration
		case "review":
			return ConstructReviewComment
		case "discussion":
			return ConstructDiscussion
		case "decision":
			return ConstructDecision
		case "question":
			return ConstructQuestion
		case "intent":
			return ConstructIntent
		case "context":
			return ConstructContext
		case "reference":
			return ConstructReference
		}
	}

	if contextType, exists := metadata.Context["type"]; exists {
		switch contextType {
		case "documentation":
			return ConstructDocumentation
		case "test":
			return ConstructTest
		case "configuration":
			return ConstructConfiguration
		case "review_comment":
			return ConstructReviewComment
		case "discussion":
			return ConstructDiscussion
		case "decision":
			return ConstructDecision
		case "question":
			return ConstructQuestion
		}
	}

	if content == "\n" || content == "\r\n" {
		return ConstructNewline
	}

	if isOnlyWhitespace(content) {
		return ConstructWhitespace
	}

	return ConstructContent
}

func (doc *Document) buildConstructMeta(op *operations.Operation) ConstructMeta {
	meta := ConstructMeta{
		Attributes: make(map[string]string),
		Confidence: 1.0,
	}

	if intent := op.Metadata.Intent; intent != "" {
		meta.Semantic = intent
	}

	for key, value := range op.Metadata.Context {
		meta.Attributes[key] = value
	}

	return meta
}

func isOnlyWhitespace(content string) bool {
	for _, r := range content {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return len(content) > 0
}
