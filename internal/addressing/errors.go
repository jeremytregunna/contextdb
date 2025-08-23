package addressing

import "errors"

var (
	ErrAddressNotFound   = errors.New("address not found")
	ErrOperationNotFound = errors.New("operation not found")
	ErrInvalidAddress    = errors.New("invalid address format")
	ErrInvalidRange      = errors.New("invalid position range")
	ErrAddressExists     = errors.New("address already exists")
	ErrResolutionFailed  = errors.New("address resolution failed")
)
