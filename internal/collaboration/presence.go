package collaboration

import (
	"sync"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
)

type PresenceTracker struct {
	clients map[ClientID]*PresenceInfo
	mutex   sync.RWMutex
}

type PresenceInfo struct {
	ClientID   ClientID            `json:"client_id"`
	AuthorID   operations.AuthorID `json:"author_id"`
	Presence   PresencePayload     `json:"presence"`
	LastUpdate time.Time           `json:"last_update"`
}

func NewPresenceTracker() *PresenceTracker {
	return &PresenceTracker{
		clients: make(map[ClientID]*PresenceInfo),
	}
}

func (pt *PresenceTracker) AddClient(clientID ClientID, authorID operations.AuthorID) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.clients[clientID] = &PresenceInfo{
		ClientID: clientID,
		AuthorID: authorID,
		Presence: PresencePayload{
			AuthorID:   authorID,
			LastActive: time.Now(),
			Status:     StatusActive,
		},
		LastUpdate: time.Now(),
	}
}

func (pt *PresenceTracker) RemoveClient(clientID ClientID) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	delete(pt.clients, clientID)
}

func (pt *PresenceTracker) UpdatePresence(clientID ClientID, presence PresencePayload) error {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	info, exists := pt.clients[clientID]
	if !exists {
		return ErrClientNotFound
	}

	info.Presence = presence
	info.LastUpdate = time.Now()
	return nil
}

func (pt *PresenceTracker) GetPresence(clientID ClientID) (*PresenceInfo, error) {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	info, exists := pt.clients[clientID]
	if !exists {
		return nil, ErrClientNotFound
	}

	// Create a copy to avoid race conditions
	return &PresenceInfo{
		ClientID:   info.ClientID,
		AuthorID:   info.AuthorID,
		Presence:   info.Presence,
		LastUpdate: info.LastUpdate,
	}, nil
}

func (pt *PresenceTracker) GetDocumentPresence(documentID string) []*PresenceInfo {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	var presence []*PresenceInfo
	for _, info := range pt.clients {
		if info.Presence.DocumentID == documentID {
			// Create a copy
			presence = append(presence, &PresenceInfo{
				ClientID:   info.ClientID,
				AuthorID:   info.AuthorID,
				Presence:   info.Presence,
				LastUpdate: info.LastUpdate,
			})
		}
	}

	return presence
}

func (pt *PresenceTracker) GetAllPresence() []*PresenceInfo {
	pt.mutex.RLock()
	defer pt.mutex.RUnlock()

	presence := make([]*PresenceInfo, 0, len(pt.clients))
	for _, info := range pt.clients {
		// Create a copy
		presence = append(presence, &PresenceInfo{
			ClientID:   info.ClientID,
			AuthorID:   info.AuthorID,
			Presence:   info.Presence,
			LastUpdate: info.LastUpdate,
		})
	}

	return presence
}

func (pt *PresenceTracker) CleanupStale(timeout time.Duration) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	cutoff := time.Now().Add(-timeout)

	for _, info := range pt.clients {
		if info.LastUpdate.Before(cutoff) {
			info.Presence.Status = StatusOffline
		} else if info.Presence.LastActive.Before(time.Now().Add(-5 * time.Minute)) {
			info.Presence.Status = StatusIdle
		}
	}
}
