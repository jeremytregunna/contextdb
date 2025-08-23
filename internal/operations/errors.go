package operations

import "errors"

var (
	ErrOperationNotFound    = errors.New("operation not found")
	ErrInvalidOperation     = errors.New("invalid operation")
	ErrInvalidAuthor        = errors.New("invalid author")
	ErrInvalidOperationType = errors.New("invalid operation type")
	ErrPositionConflict     = errors.New("position conflict")
	ErrCausalityViolation   = errors.New("causality violation")
)
