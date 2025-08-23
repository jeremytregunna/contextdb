package context

import "errors"

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrUnauthorized         = errors.New("unauthorized action")
	ErrInvalidMessageType   = errors.New("invalid message type")
	ErrInvalidStatus        = errors.New("invalid thread status")
	ErrDuplicateReaction    = errors.New("duplicate reaction")
)
