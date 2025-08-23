package collaboration

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jeremytregunna/contextdb/internal/logging"
	"github.com/jeremytregunna/contextdb/internal/operations"
)

type ClientID string

type ClientConnection struct {
	ID        ClientID            `json:"id"`
	AuthorID  operations.AuthorID `json:"author_id"`
	WebSocket *websocket.Conn     `json:"-"`
	Documents map[string]bool     `json:"documents"`
	LastSeen  time.Time           `json:"last_seen"`
	Presence  PresencePayload     `json:"presence"`
	sendChan  chan *Message       `json:"-"`
	closeChan chan struct{}       `json:"-"`
	logger    *logging.Logger     `json:"-"`
	mutex     sync.RWMutex        `json:"-"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Same origin requests don't have Origin header
		}

		// Allow localhost and same-origin requests for development
		allowed := []string{
			"http://localhost",
			"https://localhost",
			"http://127.0.0.1",
			"https://127.0.0.1",
		}

		for _, allowedOrigin := range allowed {
			if strings.HasPrefix(origin, allowedOrigin) {
				return true
			}
		}

		// In production, this should be configured via environment variables
		return false
	},
}

func NewClientConnection(clientID ClientID, authorID operations.AuthorID, w http.ResponseWriter, r *http.Request) (*ClientConnection, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	client := &ClientConnection{
		ID:        clientID,
		AuthorID:  authorID,
		WebSocket: conn,
		Documents: make(map[string]bool),
		LastSeen:  time.Now(),
		sendChan:  make(chan *Message, 256),
		closeChan: make(chan struct{}),
		logger:    logging.NewLogger("websocket"),
	}

	client.Presence = PresencePayload{
		AuthorID:   authorID,
		LastActive: time.Now(),
		Status:     StatusActive,
	}

	return client, nil
}

func (c *ClientConnection) Start() {
	go c.writePump()
	go c.readPump()
}

func (c *ClientConnection) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	select {
	case <-c.closeChan:
		return nil // Already closed
	default:
		close(c.closeChan)
		close(c.sendChan)
		if c.WebSocket != nil {
			return c.WebSocket.Close()
		}
		return nil
	}
}

func (c *ClientConnection) SendMessage(msg *Message) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	select {
	case c.sendChan <- msg:
		return nil
	case <-c.closeChan:
		return ErrConnectionClosed
	default:
		return ErrSendBufferFull
	}
}

func (c *ClientConnection) SubscribeToDocument(documentID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Documents[documentID] = true
	c.Presence.DocumentID = documentID
}

func (c *ClientConnection) UnsubscribeFromDocument(documentID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.Documents, documentID)
	if c.Presence.DocumentID == documentID {
		c.Presence.DocumentID = ""
	}
}

func (c *ClientConnection) IsSubscribedTo(documentID string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Documents[documentID]
}

func (c *ClientConnection) UpdatePresence(presence PresencePayload) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Presence = presence
	c.LastSeen = time.Now()
}

func (c *ClientConnection) readPump() {
	defer func() {
		c.Close()
	}()

	c.WebSocket.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.WebSocket.SetPongHandler(func(string) error {
		c.WebSocket.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		select {
		case <-c.closeChan:
			return
		default:
		}

		var msg Message
		err := c.WebSocket.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.LogWebSocketError(string(c.ID), err)
			}
			return
		}

		c.mutex.Lock()
		c.LastSeen = time.Now()
		c.mutex.Unlock()

		// Process message through collaboration engine
		// This will be handled by the engine when we implement it
	}
}

func (c *ClientConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.sendChan:
			c.WebSocket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.WebSocket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.WebSocket.WriteJSON(msg); err != nil {
				c.logger.WithFields(map[string]interface{}{
					"client_id": string(c.ID),
					"error":     err.Error(),
				}).Error("WebSocket write error")
				return
			}

		case <-ticker.C:
			c.WebSocket.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.WebSocket.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.closeChan:
			return
		}
	}
}

func (c *ClientConnection) GetInfo() ClientInfo {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return ClientInfo{
		ID:        c.ID,
		AuthorID:  c.AuthorID,
		Documents: c.getDocumentList(),
		LastSeen:  c.LastSeen,
		Presence:  c.Presence,
	}
}

func (c *ClientConnection) getDocumentList() []string {
	docs := make([]string, 0, len(c.Documents))
	for doc := range c.Documents {
		docs = append(docs, doc)
	}
	return docs
}

type ClientInfo struct {
	ID        ClientID            `json:"id"`
	AuthorID  operations.AuthorID `json:"author_id"`
	Documents []string            `json:"documents"`
	LastSeen  time.Time           `json:"last_seen"`
	Presence  PresencePayload     `json:"presence"`
}
