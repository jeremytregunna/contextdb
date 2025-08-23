package context

import (
	"strings"
	"sync"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/operations"
)

type ConversationManager struct {
	conversations map[ThreadID]*ConversationThread
	addressIndex  map[addressing.AddressKey][]ThreadID // Address -> Thread IDs
	authorIndex   map[operations.AuthorID][]ThreadID   // Author -> Thread IDs
	mutex         sync.RWMutex
}

func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		conversations: make(map[ThreadID]*ConversationThread),
		addressIndex:  make(map[addressing.AddressKey][]ThreadID),
		authorIndex:   make(map[operations.AuthorID][]ThreadID),
	}
}

func (cm *ConversationManager) CreateConversation(anchorAddr addressing.StableAddress, authorID operations.AuthorID, title, content string) (*ConversationThread, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread := NewConversationThread(anchorAddr, authorID, title, content)

	cm.conversations[thread.ID] = thread
	cm.indexConversation(thread)

	return thread, nil
}

func (cm *ConversationManager) GetConversation(threadID ThreadID) (*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	// Return a copy to avoid race conditions
	return cm.copyThread(thread), nil
}

func (cm *ConversationManager) GetConversationsByAddress(addr addressing.StableAddress) ([]*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	addressKey := addr.Key()
	threadIDs, exists := cm.addressIndex[addressKey]
	if !exists {
		return []*ConversationThread{}, nil
	}

	threads := make([]*ConversationThread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		if thread, exists := cm.conversations[threadID]; exists {
			threads = append(threads, cm.copyThread(thread))
		}
	}

	return threads, nil
}

func (cm *ConversationManager) GetConversationsByAuthor(authorID operations.AuthorID) ([]*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	threadIDs, exists := cm.authorIndex[authorID]
	if !exists {
		return []*ConversationThread{}, nil
	}

	threads := make([]*ConversationThread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		if thread, exists := cm.conversations[threadID]; exists {
			threads = append(threads, cm.copyThread(thread))
		}
	}

	return threads, nil
}

func (cm *ConversationManager) AddMessage(threadID ThreadID, authorID operations.AuthorID, content string, msgType MessageType) (*Message, error) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	message := thread.AddMessage(authorID, content, msgType)
	cm.updateAuthorIndex(thread)

	return message, nil
}

func (cm *ConversationManager) EditMessage(threadID ThreadID, messageID MessageID, authorID operations.AuthorID, newContent string, reason string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return ErrConversationNotFound
	}

	return thread.EditMessage(messageID, authorID, newContent, reason)
}

func (cm *ConversationManager) AddReaction(threadID ThreadID, messageID MessageID, authorID operations.AuthorID, emoji string) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return ErrConversationNotFound
	}

	return thread.AddReaction(messageID, authorID, emoji)
}

func (cm *ConversationManager) ResolveConversation(threadID ThreadID, authorID operations.AuthorID) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return ErrConversationNotFound
	}

	thread.SetStatus(StatusResolved)

	// Add resolution message
	thread.AddMessage(authorID, "Conversation resolved", MsgDecision)

	return nil
}

func (cm *ConversationManager) ArchiveConversation(threadID ThreadID) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	thread, exists := cm.conversations[threadID]
	if !exists {
		return ErrConversationNotFound
	}

	thread.SetStatus(StatusArchived)
	return nil
}

func (cm *ConversationManager) GetActiveConversations() ([]*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var active []*ConversationThread
	for _, thread := range cm.conversations {
		if thread.Status == StatusOpen || thread.Status == StatusPinned {
			active = append(active, cm.copyThread(thread))
		}
	}

	return active, nil
}

func (cm *ConversationManager) GetConversationsByStatus(status ThreadStatus) ([]*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var filtered []*ConversationThread
	for _, thread := range cm.conversations {
		if thread.Status == status {
			filtered = append(filtered, cm.copyThread(thread))
		}
	}

	return filtered, nil
}

func (cm *ConversationManager) SearchConversations(query string) ([]*ConversationThread, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var results []*ConversationThread
	queryLower := strings.ToLower(query)

	for _, thread := range cm.conversations {
		if cm.threadMatchesQuery(thread, queryLower) {
			results = append(results, cm.copyThread(thread))
		}
	}

	return results, nil
}

func (cm *ConversationManager) UpdateAddressLocation(oldAddr, newAddr addressing.StableAddress) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	oldKey := oldAddr.Key()
	threadIDs, exists := cm.addressIndex[oldKey]
	if !exists {
		return nil // No conversations to update
	}

	// Update all affected conversations
	newKey := newAddr.Key()
	for _, threadID := range threadIDs {
		if thread, exists := cm.conversations[threadID]; exists {
			thread.AnchorAddress = newAddr
		}
	}

	// Update index
	cm.addressIndex[newKey] = threadIDs
	delete(cm.addressIndex, oldKey)

	return nil
}

func (cm *ConversationManager) indexConversation(thread *ConversationThread) {
	// Index by address
	addressKey := thread.AnchorAddress.Key()
	cm.addressIndex[addressKey] = append(cm.addressIndex[addressKey], thread.ID)

	// Index by participants
	for _, participant := range thread.Participants {
		cm.authorIndex[participant] = append(cm.authorIndex[participant], thread.ID)
	}
}

func (cm *ConversationManager) updateAuthorIndex(thread *ConversationThread) {
	// Rebuild author index for this thread
	for _, participant := range thread.Participants {
		threadIDs := cm.authorIndex[participant]

		// Check if thread is already indexed
		found := false
		for _, id := range threadIDs {
			if id == thread.ID {
				found = true
				break
			}
		}

		if !found {
			cm.authorIndex[participant] = append(cm.authorIndex[participant], thread.ID)
		}
	}
}

func (cm *ConversationManager) copyThread(thread *ConversationThread) *ConversationThread {
	// Create a deep copy to prevent race conditions
	copyThread := &ConversationThread{
		ID:            thread.ID,
		Title:         thread.Title,
		AnchorAddress: thread.AnchorAddress,
		Participants:  make([]operations.AuthorID, len(thread.Participants)),
		Messages:      make([]Message, len(thread.Messages)),
		Status:        thread.Status,
		CreatedAt:     thread.CreatedAt,
		UpdatedAt:     thread.UpdatedAt,
		Tags:          make([]string, len(thread.Tags)),
		Metadata:      thread.Metadata,
	}

	copy(copyThread.Participants, thread.Participants)
	copy(copyThread.Messages, thread.Messages)
	copy(copyThread.Tags, thread.Tags)

	return copyThread
}

func (cm *ConversationManager) threadMatchesQuery(thread *ConversationThread, queryLower string) bool {
	// Search in title
	if strings.Contains(strings.ToLower(thread.Title), queryLower) {
		return true
	}

	// Search in messages
	for _, msg := range thread.Messages {
		if strings.Contains(strings.ToLower(msg.Content), queryLower) {
			return true
		}
	}

	// Search in tags
	for _, tag := range thread.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}

	return false
}
