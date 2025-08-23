package positioning

import "errors"

var (
	ErrInvalidPosition      = errors.New("invalid position")
	ErrPositionOccupied     = errors.New("position already occupied")
	ErrConstructNotFound    = errors.New("construct not found")
	ErrUnsupportedOperation = errors.New("unsupported operation type")
	ErrInvalidRange         = errors.New("invalid position range")
)
