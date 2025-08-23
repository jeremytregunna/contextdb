package storage

import "errors"

var (
	ErrOperationNotFound = errors.New("operation not found")
	ErrDocumentNotFound  = errors.New("document not found")
	ErrStoreClosed       = errors.New("store is closed")
	ErrInvalidData       = errors.New("invalid data format")
)
