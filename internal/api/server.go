package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/auth"
	"github.com/jeremytregunna/contextdb/internal/collaboration"
	"github.com/jeremytregunna/contextdb/internal/context"
	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/storage"
)

type APIServer struct {
	mux             *http.ServeMux
	engine          *collaboration.CollaborationEngine
	store           storage.OperationStore
	documentStore   storage.DocumentStore
	resolver        *addressing.AddressResolver
	contextManager  *context.ConversationManager
	contextAnalyzer *context.ContextAnalyzer
	authManager     *auth.AuthManager
}

func NewAPIServer(
	engine *collaboration.CollaborationEngine,
	store storage.OperationStore,
	documentStore storage.DocumentStore,
	resolver *addressing.AddressResolver,
	contextManager *context.ConversationManager,
	contextAnalyzer *context.ContextAnalyzer,
	authManager *auth.AuthManager,
) *APIServer {
	s := &APIServer{
		mux:             http.NewServeMux(),
		engine:          engine,
		store:           store,
		documentStore:   documentStore,
		resolver:        resolver,
		contextManager:  contextManager,
		contextAnalyzer: contextAnalyzer,
		authManager:     authManager,
	}
	s.setupRoutes()
	return s
}

func (s *APIServer) setupRoutes() {
	// Operation endpoints
	s.mux.HandleFunc("GET /api/v1/operations", s.listOperations)
	s.mux.HandleFunc("POST /api/v1/operations", s.createOperation)
	s.mux.HandleFunc("GET /api/v1/operations/{id}", s.getOperation)

	// Document endpoints
	s.mux.HandleFunc("GET /api/v1/documents/{path}", s.getDocument)
	s.mux.HandleFunc("GET /api/v1/documents/{path}/history", s.getDocumentHistory)

	// Address endpoints
	s.mux.HandleFunc("POST /api/v1/addresses/resolve", s.resolveAddress)
	s.mux.HandleFunc("GET /api/v1/addresses/{address}/history", s.getAddressHistory)

	// Operation analysis endpoints
	s.mux.HandleFunc("GET /api/v1/operations/{id}/context", s.getOperationContext)
	s.mux.HandleFunc("GET /api/v1/operations/{id}/intent", s.getOperationIntent)
	s.mux.HandleFunc("POST /api/v1/analyze/intent", s.analyzeBatchIntent)

	// Authentication endpoints
	s.mux.HandleFunc("POST /api/v1/auth/keys", s.createAPIKey)
	s.mux.HandleFunc("GET /api/v1/auth/keys", s.listAPIKeys)
	s.mux.HandleFunc("DELETE /api/v1/auth/keys/{id}", s.revokeAPIKey)
	s.mux.HandleFunc("GET /api/v1/auth/status", s.getAuthStatus)
	s.mux.HandleFunc("POST /api/v1/auth/enable", s.enableAuth)
	s.mux.HandleFunc("POST /api/v1/auth/disable", s.disableAuth)

	// Conversation endpoints
	s.mux.HandleFunc("POST /api/v1/conversations", s.createConversation)
	s.mux.HandleFunc("GET /api/v1/conversations/{id}", s.getConversation)
	s.mux.HandleFunc("POST /api/v1/conversations/{id}/messages", s.addMessage)

	// Analysis endpoints
	s.mux.HandleFunc("GET /api/v1/analysis/context/{operation_id}", s.getOperationContext)
	s.mux.HandleFunc("POST /api/v1/analysis/intent", s.analyzeIntent)

	// Search endpoints
	s.mux.HandleFunc("GET /api/v1/search", s.search)

	// Health check
	s.mux.HandleFunc("GET /api/v1/health", s.healthCheck)
}

func (s *APIServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Apply auth middleware
	authMiddleware := auth.AuthMiddleware(s.authManager)
	authMiddleware(s.mux).ServeHTTP(w, r)
}

// Helper methods for JSON responses
func (s *APIServer) jsonResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *APIServer) jsonError(w http.ResponseWriter, message string, statusCode int) {
	s.jsonResponse(w, map[string]string{"error": message}, statusCode)
}

func (s *APIServer) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
}

// API endpoint handlers

// Operation endpoints
func (s *APIServer) createOperation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       operations.OperationType  `json:"type"`
		Position   operations.LogootPosition `json:"position"`
		Content    string                    `json:"content"`
		Length     int                       `json:"length,omitempty"`
		Author     operations.AuthorID       `json:"author"`
		Parents    []operations.OperationID  `json:"parents,omitempty"`
		Metadata   operations.OperationMeta  `json:"metadata,omitempty"`
		DocumentID string                    `json:"document_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Ensure metadata has the required context
	if req.Metadata.Context == nil {
		req.Metadata.Context = make(map[string]string)
	}
	if req.DocumentID != "" {
		req.Metadata.Context["document_id"] = req.DocumentID
	}

	op := &operations.Operation{
		Type:      req.Type,
		Position:  req.Position,
		Content:   req.Content,
		Length:    req.Length,
		Author:    req.Author,
		Timestamp: time.Now(),
		Parents:   req.Parents,
		Metadata:  req.Metadata,
	}

	op.ID = operations.NewOperationID([]byte(fmt.Sprintf("%s-%s-%d",
		req.Author, req.Content, op.Timestamp.UnixNano())))

	if err := s.engine.ProcessOperation(op, collaboration.ClientID(req.Author)); err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to process operation: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, SuccessResponse{
		Data:    op,
		Message: "Operation created successfully",
	}, http.StatusCreated)
}

func (s *APIServer) getOperation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		s.jsonError(w, "Operation ID is required", http.StatusBadRequest)
		return
	}

	opID := operations.OperationID(idStr)

	op, err := s.store.GetOperation(opID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Operation not found: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, SuccessResponse{Data: op}, http.StatusOK)
}

func (s *APIServer) listOperations(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	var ops []*operations.Operation
	var err error

	if sinceStr := query.Get("since"); sinceStr != "" {
		since, parseErr := time.Parse(time.RFC3339, sinceStr)
		if parseErr != nil {
			s.jsonError(w, "Invalid 'since' timestamp format", http.StatusBadRequest)
			return
		}
		ops, err = s.store.GetOperationsSince(since)
	} else if author := query.Get("author"); author != "" {
		ops, err = s.store.GetOperationsByAuthor(operations.AuthorID(author))
	} else {
		// Get recent operations (last 24 hours by default)
		since := time.Now().Add(-24 * time.Hour)
		ops, err = s.store.GetOperationsSince(since)
	}

	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to retrieve operations: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply limit if specified
	if limitStr := query.Get("limit"); limitStr != "" {
		if limit, parseErr := strconv.Atoi(limitStr); parseErr == nil && limit > 0 && limit < len(ops) {
			ops = ops[:limit]
		}
	}

	s.jsonResponse(w, SuccessResponse{Data: ops}, http.StatusOK)
}

// Document endpoints
func (s *APIServer) getDocument(w http.ResponseWriter, r *http.Request) {
	filePath := r.PathValue("path")
	if filePath == "" {
		s.jsonError(w, "Document path is required", http.StatusBadRequest)
		return
	}

	doc, err := s.documentStore.GetDocument(filePath)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Document not found: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, SuccessResponse{Data: doc}, http.StatusOK)
}

func (s *APIServer) getDocumentHistory(w http.ResponseWriter, r *http.Request) {
	filePath := r.PathValue("path")
	if filePath == "" {
		s.jsonError(w, "Document path is required", http.StatusBadRequest)
		return
	}

	// Get all addresses for this document
	addresses, err := s.resolver.GetAddressesByDocument(filePath)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to get document addresses: %v", err), http.StatusInternalServerError)
		return
	}

	type DocumentHistory struct {
		FilePath   string                     `json:"file_path"`
		Addresses  []addressing.StableAddress `json:"addresses"`
		Operations []*operations.Operation    `json:"operations,omitempty"`
	}

	history := DocumentHistory{
		FilePath:  filePath,
		Addresses: addresses,
	}

	s.jsonResponse(w, SuccessResponse{Data: history}, http.StatusOK)
}

// Address endpoints
func (s *APIServer) resolveAddress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address addressing.StableAddress `json:"address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	resolved, err := s.resolver.ResolveAddress(req.Address)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to resolve address: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, SuccessResponse{Data: resolved}, http.StatusOK)
}

func (s *APIServer) getAddressHistory(w http.ResponseWriter, r *http.Request) {
	addressStr := r.PathValue("address")
	if addressStr == "" {
		s.jsonError(w, "Address is required", http.StatusBadRequest)
		return
	}

	// Parse address string - simplified for MVP
	var addr addressing.StableAddress
	if err := json.Unmarshal([]byte(addressStr), &addr); err != nil {
		s.jsonError(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	history, err := s.resolver.GetAddressHistory(addr)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to get address history: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, SuccessResponse{Data: history}, http.StatusOK)
}

// Conversation endpoints
func (s *APIServer) createConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AnchorAddress addressing.StableAddress `json:"anchor_address"`
		AuthorID      operations.AuthorID      `json:"author_id"`
		Title         string                   `json:"title"`
		Content       string                   `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	thread, err := s.contextManager.CreateConversation(req.AnchorAddress, req.AuthorID, req.Title, req.Content)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to create conversation: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, SuccessResponse{
		Data:    thread,
		Message: "Conversation created successfully",
	}, http.StatusCreated)
}

func (s *APIServer) getConversation(w http.ResponseWriter, r *http.Request) {
	threadIDStr := r.PathValue("id")
	if threadIDStr == "" {
		s.jsonError(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	threadID := context.ThreadID(threadIDStr)
	thread, err := s.contextManager.GetConversation(threadID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Conversation not found: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, SuccessResponse{Data: thread}, http.StatusOK)
}

func (s *APIServer) addMessage(w http.ResponseWriter, r *http.Request) {
	threadIDStr := r.PathValue("id")
	if threadIDStr == "" {
		s.jsonError(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	threadID := context.ThreadID(threadIDStr)

	var req struct {
		AuthorID    operations.AuthorID `json:"author_id"`
		Content     string              `json:"content"`
		MessageType context.MessageType `json:"message_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	message, err := s.contextManager.AddMessage(threadID, req.AuthorID, req.Content, req.MessageType)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to add message: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, SuccessResponse{
		Data:    message,
		Message: "Message added successfully",
	}, http.StatusCreated)
}

// Analysis endpoints (basic implementation for MVP)
func (s *APIServer) getOperationContext(w http.ResponseWriter, r *http.Request) {
	opIDStr := r.PathValue("id")
	if opIDStr == "" {
		s.jsonError(w, "Operation ID is required", http.StatusBadRequest)
		return
	}

	opID := operations.OperationID(opIDStr)

	op, err := s.store.GetOperation(opID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Operation not found: %v", err), http.StatusNotFound)
		return
	}

	// Basic context analysis
	contextInfo := struct {
		Operation  *operations.Operation `json:"operation"`
		Intent     string                `json:"intent"`
		Confidence float64               `json:"confidence"`
	}{
		Operation:  op,
		Intent:     s.analyzeBasicIntent(op),
		Confidence: 0.7, // Basic confidence for MVP
	}

	s.jsonResponse(w, SuccessResponse{Data: contextInfo}, http.StatusOK)
}

func (s *APIServer) analyzeIntent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Operations []*operations.Operation `json:"operations"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	analysis := struct {
		PrimaryIntent string   `json:"primary_intent"`
		Confidence    float64  `json:"confidence"`
		Evidence      []string `json:"evidence"`
		Category      string   `json:"category"`
	}{
		PrimaryIntent: "code_change",
		Confidence:    0.8,
		Evidence:      []string{"operation_type_analysis"},
		Category:      "development",
	}

	s.jsonResponse(w, SuccessResponse{Data: analysis}, http.StatusOK)
}

// Search endpoint with enhanced functionality
func (s *APIServer) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	searchQuery := query.Get("q")
	searchType := query.Get("type")
	authorFilter := query.Get("author")
	limitStr := query.Get("limit")

	if searchQuery == "" {
		s.jsonError(w, "Search query 'q' parameter is required", http.StatusBadRequest)
		return
	}

	// Parse limit
	limit := 50 // Default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	var results []SearchResult

	// Enhanced search implementation
	switch searchType {
	case "conversation":
		results = s.searchConversations(searchQuery, authorFilter, limit)
	case "operation":
		results = s.searchOperations(searchQuery, authorFilter, limit)
	case "code":
		results = s.searchCode(searchQuery, limit)
	default:
		// Search all types
		conversationResults := s.searchConversations(searchQuery, authorFilter, limit/3)
		operationResults := s.searchOperations(searchQuery, authorFilter, limit/3)
		codeResults := s.searchCode(searchQuery, limit/3)

		results = append(results, conversationResults...)
		results = append(results, operationResults...)
		results = append(results, codeResults...)

		// Sort by relevance score (descending)
		s.sortResultsByScore(results)

		// Apply final limit
		if len(results) > limit {
			results = results[:limit]
		}
	}

	searchResults := struct {
		Query   string         `json:"query"`
		Type    string         `json:"type"`
		Author  string         `json:"author,omitempty"`
		Results []SearchResult `json:"results"`
		Total   int            `json:"total"`
		Limit   int            `json:"limit"`
	}{
		Query:   searchQuery,
		Type:    searchType,
		Author:  authorFilter,
		Results: results,
		Total:   len(results),
		Limit:   limit,
	}

	s.jsonResponse(w, SuccessResponse{Data: searchResults}, http.StatusOK)
}

type SearchResult struct {
	Type      string      `json:"type"` // "conversation", "operation", "code"
	ID        string      `json:"id"`
	Title     string      `json:"title,omitempty"`
	Content   string      `json:"content"`
	Author    string      `json:"author,omitempty"`
	Score     float64     `json:"score"`
	Snippet   string      `json:"snippet"`
	Timestamp *time.Time  `json:"timestamp,omitempty"`
	Address   interface{} `json:"address,omitempty"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

func (s *APIServer) searchConversations(query, authorFilter string, limit int) []SearchResult {
	var results []SearchResult

	conversations, err := s.contextManager.SearchConversations(query)
	if err != nil {
		return results
	}

	for i, conv := range conversations {
		if i >= limit {
			break
		}

		// Apply author filter if specified
		if authorFilter != "" {
			found := false
			for _, participant := range conv.Participants {
				if string(participant) == authorFilter {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Calculate basic relevance score
		score := s.calculateConversationScore(conv, query)

		// Create snippet from first message
		snippet := ""
		if len(conv.Messages) > 0 {
			content := conv.Messages[0].Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}
			snippet = content
		}

		results = append(results, SearchResult{
			Type:      "conversation",
			ID:        string(conv.ID),
			Title:     conv.Title,
			Content:   snippet,
			Score:     score,
			Snippet:   snippet,
			Timestamp: &conv.CreatedAt,
			Address:   conv.AnchorAddress,
			Metadata:  map[string]interface{}{"participants": len(conv.Participants), "messages": len(conv.Messages)},
		})
	}

	return results
}

func (s *APIServer) searchOperations(query, authorFilter string, limit int) []SearchResult {
	var results []SearchResult

	// Get recent operations (last week)
	since := time.Now().Add(-7 * 24 * time.Hour)
	operations, err := s.store.GetOperationsSince(since)
	if err != nil {
		return results
	}

	count := 0
	for _, op := range operations {
		if count >= limit {
			break
		}

		// Apply author filter if specified
		if authorFilter != "" && string(op.Author) != authorFilter {
			continue
		}

		// Check if operation content matches query
		if !s.matchesQuery(op.Content, query) && !s.matchesQuery(string(op.Author), query) {
			continue
		}

		// Calculate relevance score
		score := s.calculateOperationScore(op, query)

		// Create snippet
		snippet := op.Content
		if len(snippet) > 150 {
			snippet = snippet[:150] + "..."
		}

		results = append(results, SearchResult{
			Type:      "operation",
			ID:        fmt.Sprintf("%x", op.ID),
			Content:   op.Content,
			Author:    string(op.Author),
			Score:     score,
			Snippet:   snippet,
			Timestamp: &op.Timestamp,
			Metadata:  map[string]interface{}{"type": op.Type, "position": op.Position},
		})
		count++
	}

	return results
}

func (s *APIServer) searchCode(query string, limit int) []SearchResult {
	var results []SearchResult

	// Basic code search - search through stored documents
	// This is a simplified implementation for MVP
	documents, err := s.documentStore.ListDocuments()
	if err != nil {
		return results
	}

	count := 0
	for _, docPath := range documents {
		if count >= limit {
			break
		}

		doc, err := s.documentStore.GetDocument(docPath)
		if err != nil {
			continue
		}

		// Render document content
		content, err := doc.Render()
		if err != nil {
			continue
		}

		// Check if content matches query
		if !s.matchesQuery(content, query) && !s.matchesQuery(docPath, query) {
			continue
		}

		// Calculate relevance score
		score := s.calculateCodeScore(content, docPath, query)

		// Create snippet with context around match
		snippet := s.createCodeSnippet(content, query)

		results = append(results, SearchResult{
			Type:     "code",
			ID:       docPath,
			Title:    docPath,
			Content:  snippet,
			Score:    score,
			Snippet:  snippet,
			Metadata: map[string]interface{}{"constructs": len(doc.Constructs), "version": doc.Version},
		})
		count++
	}

	return results
}

// Helper functions for search scoring and matching
func (s *APIServer) matchesQuery(text, query string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

func (s *APIServer) calculateConversationScore(conv *context.ConversationThread, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Title match gets higher score
	if strings.Contains(strings.ToLower(conv.Title), queryLower) {
		score += 2.0
	}

	// Message content matches
	for _, msg := range conv.Messages {
		if strings.Contains(strings.ToLower(msg.Content), queryLower) {
			score += 1.0
		}
	}

	// Recent conversations get slight boost
	age := time.Since(conv.UpdatedAt).Hours()
	if age < 24 {
		score += 0.5
	} else if age < 168 { // 1 week
		score += 0.2
	}

	return score
}

func (s *APIServer) calculateOperationScore(op *operations.Operation, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Content match
	if strings.Contains(strings.ToLower(op.Content), queryLower) {
		score += 1.0
	}

	// Author match
	if strings.Contains(strings.ToLower(string(op.Author)), queryLower) {
		score += 0.5
	}

	// Recent operations get boost
	age := time.Since(op.Timestamp).Hours()
	if age < 1 {
		score += 1.0
	} else if age < 24 {
		score += 0.5
	} else if age < 168 { // 1 week
		score += 0.2
	}

	return score
}

func (s *APIServer) calculateCodeScore(content, path, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)

	// Path/filename match gets higher score
	if strings.Contains(strings.ToLower(path), queryLower) {
		score += 1.5
	}

	// Content matches
	contentLower := strings.ToLower(content)
	matches := strings.Count(contentLower, queryLower)
	score += float64(matches) * 0.5

	return score
}

func (s *APIServer) createCodeSnippet(content, query string) string {
	lines := strings.Split(content, "\n")
	queryLower := strings.ToLower(query)

	// Find first line containing the query
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), queryLower) {
			// Return context around the match (3 lines before and after)
			start := i - 3
			if start < 0 {
				start = 0
			}
			end := i + 4
			if end > len(lines) {
				end = len(lines)
			}

			snippet := strings.Join(lines[start:end], "\n")
			if len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			return snippet
		}
	}

	// If no match found, return first 200 chars
	if len(content) > 200 {
		return content[:200] + "..."
	}
	return content
}

func (s *APIServer) sortResultsByScore(results []SearchResult) {
	// Simple bubble sort by score (descending)
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}

// Health check endpoint
func (s *APIServer) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := struct {
		Status    string    `json:"status"`
		Timestamp time.Time `json:"timestamp"`
		Version   string    `json:"version"`
	}{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0-mvp",
	}

	s.jsonResponse(w, health, http.StatusOK)
}

// Operation intent analysis endpoint
func (s *APIServer) getOperationIntent(w http.ResponseWriter, r *http.Request) {
	opIDStr := r.PathValue("id")
	if opIDStr == "" {
		s.jsonError(w, "Operation ID is required", http.StatusBadRequest)
		return
	}

	opID := operations.OperationID(opIDStr)

	op, err := s.store.GetOperation(opID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Operation not found: %v", err), http.StatusNotFound)
		return
	}

	// Get full context analysis
	context, err := s.contextAnalyzer.GetOperationContext(opID)
	if err != nil {
		// Fallback to basic analysis
		response := map[string]interface{}{
			"operation_id": opID,
			"basic_intent": s.analyzeBasicIntent(op),
			"confidence":   0.5,
		}
		s.jsonResponse(w, response, http.StatusOK)
		return
	}

	response := map[string]interface{}{
		"operation_id": opID,
		"intent":       context.Intent,
		"basic_intent": s.analyzeBasicIntent(op),
		"confidence":   0.8,
		"summary":      context.Summary,
	}

	s.jsonResponse(w, response, http.StatusOK)
}

func (s *APIServer) analyzeBatchIntent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Operations []operations.OperationID `json:"operations"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(request.Operations) == 0 {
		s.jsonError(w, "At least one operation ID is required", http.StatusBadRequest)
		return
	}

	// Get operations from store
	ops, err := s.store.GetOperations(request.Operations)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to retrieve operations: %v", err), http.StatusInternalServerError)
		return
	}

	// Analyze collective intent
	analysis, err := s.contextAnalyzer.AnalyzeChangeIntent(ops)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to analyze intent: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"operations_count":   len(ops),
		"collective_intent":  analysis,
		"individual_intents": make([]map[string]interface{}, 0, len(ops)),
	}

	// Add individual analysis for each operation
	for _, op := range ops {
		individual := map[string]interface{}{
			"operation_id": op.ID,
			"basic_intent": s.analyzeBasicIntent(op),
		}
		response["individual_intents"] = append(response["individual_intents"].([]map[string]interface{}), individual)
	}

	s.jsonResponse(w, response, http.StatusOK)
}

// Authentication endpoints
func (s *APIServer) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string              `json:"name"`
		AuthorID    operations.AuthorID `json:"author_id"`
		Permissions []auth.Permission   `json:"permissions"`
		ExpiresIn   *int                `json:"expires_in_hours,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.jsonError(w, "Name is required", http.StatusBadRequest)
		return
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != nil {
		duration := time.Duration(*req.ExpiresIn) * time.Hour
		expiresIn = &duration
	}

	keyString, err := s.authManager.CreateAPIKey(req.Name, req.AuthorID, req.Permissions, expiresIn)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to create API key: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"api_key": keyString,
		"message": "API key created successfully. Store this key securely - it won't be shown again.",
	}

	s.jsonResponse(w, response, http.StatusCreated)
}

func (s *APIServer) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.authManager.ListAPIKeys()
	s.jsonResponse(w, map[string]interface{}{"keys": keys}, http.StatusOK)
}

func (s *APIServer) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("id")
	if keyID == "" {
		s.jsonError(w, "Key ID is required", http.StatusBadRequest)
		return
	}

	if err := s.authManager.RevokeAPIKey(keyID); err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to revoke key: %v", err), http.StatusNotFound)
		return
	}

	s.jsonResponse(w, map[string]string{"message": "API key revoked successfully"}, http.StatusOK)
}

func (s *APIServer) getAuthStatus(w http.ResponseWriter, r *http.Request) {
	authContext := auth.GetAuthContext(r.Context())

	status := map[string]interface{}{
		"auth_required": s.authManager.IsAuthRequired(),
		"authenticated": authContext != nil && authContext.Authenticated,
		"author_id":     "",
		"permissions":   []string{},
	}

	if authContext != nil {
		status["author_id"] = authContext.AuthorID
		status["permissions"] = authContext.Permissions
	}

	s.jsonResponse(w, status, http.StatusOK)
}

func (s *APIServer) enableAuth(w http.ResponseWriter, r *http.Request) {
	if err := s.authManager.EnableAuth(); err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to enable auth: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"message": "Authentication enabled"}, http.StatusOK)
}

func (s *APIServer) disableAuth(w http.ResponseWriter, r *http.Request) {
	if err := s.authManager.DisableAuth(); err != nil {
		s.jsonError(w, fmt.Sprintf("Failed to disable auth: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"message": "Authentication disabled"}, http.StatusOK)
}

// Helper method for basic intent analysis
func (s *APIServer) analyzeBasicIntent(op *operations.Operation) string {
	switch op.Type {
	case operations.OpInsert:
		if len(op.Content) > 100 {
			return "major_addition"
		}
		return "content_addition"
	case operations.OpDelete:
		return "content_removal"
	case operations.OpMove:
		return "refactoring"
	default:
		return "unknown"
	}
}
