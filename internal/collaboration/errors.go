package collaboration

import "errors"

var (
	ErrConnectionClosed     = errors.New("connection closed")
	ErrSendBufferFull       = errors.New("send buffer full")
	ErrClientNotFound       = errors.New("client not found")
	ErrDocumentNotFound     = errors.New("document not found")
	ErrInvalidMessage       = errors.New("invalid message format")
	ErrOperationRejected    = errors.New("operation rejected")
	ErrSyncFailed           = errors.New("synchronization failed")
	ErrPresenceUpdateFailed = errors.New("presence update failed")
)
