package collaboration

import (
	"sync"
)

type MessageBroadcaster struct {
	channels map[string]chan *Message
	mutex    sync.RWMutex
}

func NewMessageBroadcaster() *MessageBroadcaster {
	return &MessageBroadcaster{
		channels: make(map[string]chan *Message),
	}
}

func (mb *MessageBroadcaster) Subscribe(channelID string, bufferSize int) <-chan *Message {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	ch := make(chan *Message, bufferSize)
	mb.channels[channelID] = ch
	return ch
}

func (mb *MessageBroadcaster) Unsubscribe(channelID string) {
	mb.mutex.Lock()
	defer mb.mutex.Unlock()

	if ch, exists := mb.channels[channelID]; exists {
		close(ch)
		delete(mb.channels, channelID)
	}
}

func (mb *MessageBroadcaster) Broadcast(msg *Message, excludeChannels ...string) {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	excluded := make(map[string]bool)
	for _, ch := range excludeChannels {
		excluded[ch] = true
	}

	for channelID, ch := range mb.channels {
		if excluded[channelID] {
			continue
		}

		select {
		case ch <- msg:
		default:
			// Channel buffer is full, skip
		}
	}
}

func (mb *MessageBroadcaster) Send(channelID string, msg *Message) bool {
	mb.mutex.RLock()
	defer mb.mutex.RUnlock()

	ch, exists := mb.channels[channelID]
	if !exists {
		return false
	}

	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}
