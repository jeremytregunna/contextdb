package storage

import (
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
)

type OperationStore interface {
	StoreOperation(op *operations.Operation) error
	GetOperation(id operations.OperationID) (*operations.Operation, error)
	GetOperations(ids []operations.OperationID) ([]*operations.Operation, error)
	GetOperationsSince(timestamp time.Time) ([]*operations.Operation, error)
	GetOperationsByAuthor(authorID operations.AuthorID) ([]*operations.Operation, error)
	DeleteOperation(id operations.OperationID) error
}

type DocumentStore interface {
	StoreDocument(doc *positioning.Document) error
	GetDocument(filePath string) (*positioning.Document, error)
	ListDocuments() ([]string, error)
	DeleteDocument(filePath string) error
}

type Store interface {
	OperationStore
	DocumentStore
	Close() error
}
