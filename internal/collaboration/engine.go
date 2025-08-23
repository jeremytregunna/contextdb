package collaboration

import (
	"fmt"
	"sync"
	"time"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/context"
	"github.com/jeremytregunna/contextdb/internal/logging"
	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
	"github.com/jeremytregunna/contextdb/internal/storage"
)

type CollaborationEngine struct {
	documents           map[string]*positioning.Document
	operationDAG        *operations.OperationDAG
	clients             map[ClientID]*ClientConnection
	store               storage.Store
	broadcaster         *MessageBroadcaster
	presenceTracker     *PresenceTracker
	addressResolver     *addressing.AddressResolver
	conversationManager *context.ConversationManager
	contextAnalyzer     *context.ContextAnalyzer
	logger              *logging.Logger
	mutex               sync.RWMutex
}

func NewCollaborationEngine(store storage.Store) *CollaborationEngine {
	addressResolver := addressing.NewAddressResolver()
	conversationManager := context.NewConversationManager()
	operationDAG := operations.NewOperationDAG()

	contextAnalyzer := context.NewContextAnalyzer(
		operationDAG,
		addressResolver,
		conversationManager,
	)

	return &CollaborationEngine{
		documents:           make(map[string]*positioning.Document),
		operationDAG:        operationDAG,
		clients:             make(map[ClientID]*ClientConnection),
		store:               store,
		broadcaster:         NewMessageBroadcaster(),
		presenceTracker:     NewPresenceTracker(),
		addressResolver:     addressResolver,
		conversationManager: conversationManager,
		contextAnalyzer:     contextAnalyzer,
		logger:              logging.NewLogger("collaboration"),
	}
}

func (ce *CollaborationEngine) AddClient(client *ClientConnection) error {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()

	ce.clients[client.ID] = client
	ce.presenceTracker.AddClient(client.ID, client.AuthorID)

	ce.logger.LogClientConnect(string(client.ID), string(client.AuthorID))
	return nil
}

func (ce *CollaborationEngine) RemoveClient(clientID ClientID) error {
	ce.mutex.Lock()
	client, exists := ce.clients[clientID]
	if !exists {
		ce.mutex.Unlock()
		return ErrClientNotFound
	}

	delete(ce.clients, clientID)
	ce.mutex.Unlock()

	ce.presenceTracker.RemoveClient(clientID)
	client.Close()

	ce.logger.LogClientDisconnect(string(clientID))
	return nil
}

func (ce *CollaborationEngine) ProcessOperation(op *operations.Operation, fromClient ClientID) error {
	// Validate the operation
	if err := ce.operationDAG.ValidateOperation(op); err != nil {
		return fmt.Errorf("invalid operation: %w", err)
	}

	// Add to operation DAG
	if err := ce.operationDAG.AddOperation(op); err != nil {
		return fmt.Errorf("failed to add operation to DAG: %w", err)
	}

	// Store the operation
	if err := ce.store.StoreOperation(op); err != nil {
		return fmt.Errorf("failed to store operation: %w", err)
	}

	// Update address resolver with new operation
	ce.addressResolver.ProcessOperation(op)

	// Determine which document this operation affects
	documentID := op.Metadata.Context["document_id"]
	if documentID == "" {
		// Try to infer document from operation position or context
		if sessionID := op.Metadata.SessionID; sessionID != "" {
			// Could use session context to determine document
			// For now, use a default document if none specified
			documentID = "default"
		} else {
			return fmt.Errorf("operation missing document_id in metadata and cannot infer from context")
		}
	}

	doc, err := ce.getOrLoadDocument(documentID)
	if err != nil {
		return fmt.Errorf("failed to load document: %w", err)
	}

	if err := doc.ApplyOperation(op); err != nil {
		return fmt.Errorf("failed to apply operation to document: %w", err)
	}

	// Store updated document
	if err := ce.store.StoreDocument(doc); err != nil {
		return fmt.Errorf("failed to store updated document: %w", err)
	}

	// Index document with address resolver
	ce.addressResolver.IndexDocument(doc)

	// Broadcast to all clients except sender
	return ce.BroadcastOperation(op, documentID, fromClient)
}

func (ce *CollaborationEngine) BroadcastOperation(op *operations.Operation, documentID string, excludeClient ClientID) error {
	payload := &OperationPayload{
		Operation:  op,
		DocumentID: documentID,
		Metadata:   map[string]interface{}{"source": "collaboration"},
	}

	msg := &Message{
		Type:      MsgOperation,
		Payload:   payload,
		MessageID: generateMessageID(),
		Timestamp: time.Now(),
		AuthorID:  op.Author,
	}

	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	for clientID, client := range ce.clients {
		if clientID == excludeClient {
			continue
		}

		if client.IsSubscribedTo(documentID) {
			if err := client.SendMessage(msg); err != nil {
				ce.logger.LogOperationBroadcastError(string(clientID), err)
			}
		}
	}

	return nil
}

func (ce *CollaborationEngine) SyncClient(clientID ClientID, documentID string, sinceVersion uint64) error {
	ce.mutex.RLock()
	client, exists := ce.clients[clientID]
	if !exists {
		ce.mutex.RUnlock()
		return ErrClientNotFound
	}
	ce.mutex.RUnlock()

	// Load document
	doc, err := ce.getOrLoadDocument(documentID)
	if err != nil {
		return fmt.Errorf("failed to load document: %w", err)
	}

	// Get operations since version
	var operations []*operations.Operation
	if sinceVersion > 0 {
		// In a proper implementation, we'd track document versions
		// and get operations since that specific version
		// For now, get recent operations that affected this document
		since := time.Now().Add(-1 * time.Hour)
		allOps, err := ce.store.GetOperationsSince(since)
		if err != nil {
			return fmt.Errorf("failed to get operations: %w", err)
		}

		// Filter to operations that affected this document
		for _, op := range allOps {
			if opDocID := op.Metadata.Context["document_id"]; opDocID == documentID {
				operations = append(operations, op)
			}
		}
	}

	payload := &SyncPayload{
		DocumentID:   documentID,
		Operations:   operations,
		CurrentState: doc,
		SinceVersion: sinceVersion,
	}

	msg := &Message{
		Type:      MsgSync,
		Payload:   payload,
		MessageID: generateMessageID(),
		Timestamp: time.Now(),
		AuthorID:  client.AuthorID,
	}

	client.SubscribeToDocument(documentID)
	return client.SendMessage(msg)
}

func (ce *CollaborationEngine) UpdatePresence(clientID ClientID, presence PresencePayload) error {
	ce.mutex.RLock()
	client, exists := ce.clients[clientID]
	if !exists {
		ce.mutex.RUnlock()
		return ErrClientNotFound
	}
	ce.mutex.RUnlock()

	client.UpdatePresence(presence)
	ce.presenceTracker.UpdatePresence(clientID, presence)

	// Broadcast presence update to other clients in the same document
	if presence.DocumentID != "" {
		return ce.broadcastPresence(presence, clientID)
	}

	return nil
}

func (ce *CollaborationEngine) broadcastPresence(presence PresencePayload, excludeClient ClientID) error {
	msg := &Message{
		Type:      MsgPresence,
		Payload:   presence,
		MessageID: generateMessageID(),
		Timestamp: time.Now(),
		AuthorID:  presence.AuthorID,
	}

	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	for clientID, client := range ce.clients {
		if clientID == excludeClient {
			continue
		}

		if client.IsSubscribedTo(presence.DocumentID) {
			if err := client.SendMessage(msg); err != nil {
				ce.logger.LogPresenceBroadcastError(string(clientID), err)
			}
		}
	}

	return nil
}

func (ce *CollaborationEngine) getOrLoadDocument(documentID string) (*positioning.Document, error) {
	ce.mutex.RLock()
	doc, exists := ce.documents[documentID]
	ce.mutex.RUnlock()

	if exists {
		return doc, nil
	}

	// Load from storage
	storedDoc, err := ce.store.GetDocument(documentID)
	if err != nil {
		if err == storage.ErrDocumentNotFound {
			// Create new document
			doc = positioning.NewDocument(documentID)
			ce.mutex.Lock()
			ce.documents[documentID] = doc
			ce.mutex.Unlock()
			return doc, nil
		}
		return nil, err
	}

	ce.mutex.Lock()
	ce.documents[documentID] = storedDoc
	ce.mutex.Unlock()

	return storedDoc, nil
}

func (ce *CollaborationEngine) GetDocumentState(documentID string) (*positioning.Document, error) {
	return ce.getOrLoadDocument(documentID)
}

func (ce *CollaborationEngine) GetConnectedClients() []ClientInfo {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	clients := make([]ClientInfo, 0, len(ce.clients))
	for _, client := range ce.clients {
		clients = append(clients, client.GetInfo())
	}

	return clients
}

func (ce *CollaborationEngine) GetDocumentClients(documentID string) []ClientInfo {
	ce.mutex.RLock()
	defer ce.mutex.RUnlock()

	var clients []ClientInfo
	for _, client := range ce.clients {
		if client.IsSubscribedTo(documentID) {
			clients = append(clients, client.GetInfo())
		}
	}

	return clients
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// Address and Context Methods

func (ce *CollaborationEngine) CreateStableAddress(repo addressing.RepositoryID, creationOpID operations.OperationID, posRange addressing.PositionRange) (addressing.StableAddress, error) {
	return ce.addressResolver.CreateAddress(repo, creationOpID, posRange)
}

func (ce *CollaborationEngine) ResolveAddress(addr addressing.StableAddress) (*addressing.ResolvedAddress, error) {
	return ce.addressResolver.ResolveAddress(addr)
}

func (ce *CollaborationEngine) GetAddressHistory(addr addressing.StableAddress) ([]addressing.MovementRecord, error) {
	return ce.addressResolver.GetAddressHistory(addr)
}

func (ce *CollaborationEngine) CreateConversation(anchorAddr addressing.StableAddress, authorID operations.AuthorID, title, content string) (*context.ConversationThread, error) {
	return ce.conversationManager.CreateConversation(anchorAddr, authorID, title, content)
}

func (ce *CollaborationEngine) GetConversation(threadID context.ThreadID) (*context.ConversationThread, error) {
	return ce.conversationManager.GetConversation(threadID)
}

func (ce *CollaborationEngine) GetConversationsByAddress(addr addressing.StableAddress) ([]*context.ConversationThread, error) {
	return ce.conversationManager.GetConversationsByAddress(addr)
}

func (ce *CollaborationEngine) AddMessageToConversation(threadID context.ThreadID, authorID operations.AuthorID, content string, msgType context.MessageType) (*context.Message, error) {
	return ce.conversationManager.AddMessage(threadID, authorID, content, msgType)
}

func (ce *CollaborationEngine) GetOperationContext(opID operations.OperationID) (*context.OperationContext, error) {
	return ce.contextAnalyzer.GetOperationContext(opID)
}

func (ce *CollaborationEngine) GetAuthorActivity(authorID operations.AuthorID, since time.Time) (*context.AuthorActivity, error) {
	return ce.contextAnalyzer.GetAuthorActivity(authorID, since)
}

func (ce *CollaborationEngine) AnalyzeChangeIntent(ops []*operations.Operation) (*context.IntentAnalysis, error) {
	return ce.contextAnalyzer.AnalyzeChangeIntent(ops)
}
