package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ContextDBClient provides a Go client for the ContextDB REST API
type ContextDBClient struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

// Operation represents a ContextDB operation
type Operation struct {
	ID         string                 `json:"id,omitempty"`
	Type       string                 `json:"type"`
	Position   Position               `json:"position"`
	Content    string                 `json:"content"`
	Author     string                 `json:"author"`
	DocumentID string                 `json:"document_id"`
	Timestamp  time.Time              `json:"timestamp,omitempty"`
	Parents    []string               `json:"parents,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Position represents a Logoot position in the CRDT
type Position struct {
	Segments []Segment `json:"segments"`
	Hash     string    `json:"hash"`
}

// Segment represents a segment in a Logoot position
type Segment struct {
	Value  int    `json:"value"`
	Author string `json:"author"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    string      `json:"code,omitempty"`
}

// SearchResult represents search response data
type SearchResult struct {
	Results []SearchItem `json:"results"`
	Total   int          `json:"total"`
}

// SearchItem represents a single search result
type SearchItem struct {
	Type           string  `json:"type"`
	ID             string  `json:"id"`
	Content        string  `json:"content"`
	Author         string  `json:"author"`
	DocumentID     string  `json:"document_id"`
	Timestamp      string  `json:"timestamp"`
	RelevanceScore float64 `json:"relevance_score"`
}

// IntentAnalysis represents intent analysis results
type IntentAnalysis struct {
	BasicIntent      string                 `json:"basic_intent"`
	CollectiveIntent string                 `json:"collective_intent,omitempty"`
	Evidence         map[string]interface{} `json:"evidence"`
}

// NewContextDBClient creates a new client instance
func NewContextDBClient(baseURL string, apiKey string) *ContextDBClient {
	return &ContextDBClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// makeRequest performs an HTTP request with proper error handling
func (c *ContextDBClient) makeRequest(method, path string, body interface{}) (*APIResponse, error) {
	var reqBody io.Reader

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API error (%d): %s", resp.StatusCode, apiResp.Error)
	}

	return &apiResp, nil
}

// CreateOperation creates a new operation in ContextDB
func (c *ContextDBClient) CreateOperation(operation Operation) (*Operation, error) {
	resp, err := c.makeRequest("POST", "/api/v1/operations", operation)
	if err != nil {
		return nil, err
	}

	// Parse the operation data
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal operation data: %w", err)
	}

	var op Operation
	if err := json.Unmarshal(dataBytes, &op); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation: %w", err)
	}

	return &op, nil
}

// GetOperation retrieves an operation by ID
func (c *ContextDBClient) GetOperation(operationID string) (*Operation, error) {
	path := "/api/v1/operations/" + operationID
	resp, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal operation data: %w", err)
	}

	var op Operation
	if err := json.Unmarshal(dataBytes, &op); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operation: %w", err)
	}

	return &op, nil
}

// ListOperations retrieves operations with optional filtering
func (c *ContextDBClient) ListOperations(documentID, author string, limit, offset int) ([]Operation, error) {
	params := url.Values{}
	if documentID != "" {
		params.Set("document_id", documentID)
	}
	if author != "" {
		params.Set("author", author)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/api/v1/operations"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal operations data: %w", err)
	}

	var operations []Operation
	if err := json.Unmarshal(dataBytes, &operations); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operations: %w", err)
	}

	return operations, nil
}

// Search performs a full-text search across operations and documents
func (c *ContextDBClient) Search(query, contentType string, limit, offset int) (*SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	if contentType != "" {
		params.Set("type", contentType)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	path := "/api/v1/search?" + params.Encode()
	resp, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search data: %w", err)
	}

	var result SearchResult
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search result: %w", err)
	}

	return &result, nil
}

// GetOperationIntent analyzes the intent of a single operation
func (c *ContextDBClient) GetOperationIntent(operationID string) (*IntentAnalysis, error) {
	path := "/api/v1/operations/" + operationID + "/intent"
	resp, err := c.makeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal intent data: %w", err)
	}

	var intent IntentAnalysis
	if err := json.Unmarshal(dataBytes, &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent analysis: %w", err)
	}

	return &intent, nil
}

// AnalyzeIntent performs batch intent analysis on multiple operations
func (c *ContextDBClient) AnalyzeIntent(operationIDs []string) (*IntentAnalysis, error) {
	body := map[string]interface{}{
		"operations": operationIDs,
	}

	resp, err := c.makeRequest("POST", "/api/v1/analyze/intent", body)
	if err != nil {
		return nil, err
	}

	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal intent data: %w", err)
	}

	var intent IntentAnalysis
	if err := json.Unmarshal(dataBytes, &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent analysis: %w", err)
	}

	return &intent, nil
}

// HealthCheck checks the server health status
func (c *ContextDBClient) HealthCheck() (bool, error) {
	resp, err := c.makeRequest("GET", "/api/v1/health", nil)
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

// Helper function to create insert operations
func CreateInsertOperation(content, author, documentID string, positionValue int) Operation {
	return Operation{
		Type: "insert",
		Position: Position{
			Segments: []Segment{
				{Value: positionValue, Author: author},
			},
			Hash: fmt.Sprintf("%s-%d", author, positionValue),
		},
		Content:    content,
		Author:     author,
		DocumentID: documentID,
	}
}

// Example usage
func main() {
	fmt.Println("🚀 ContextDB Go Client Example")

	// Create client (add API key if authentication is enabled)
	client := NewContextDBClient("http://localhost:8080", "")

	// Health check
	healthy, err := client.HealthCheck()
	if err != nil {
		fmt.Printf("❌ Health check failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Server is healthy: %v\n", healthy)

	// Create a sample operation
	fmt.Println("\n📝 Creating sample operation...")
	operation := CreateInsertOperation(
		"func main() { fmt.Println(\"Hello from Go client!\") }",
		"go-example",
		"main.go",
		int(time.Now().Unix()),
	)

	createdOp, err := client.CreateOperation(operation)
	if err != nil {
		fmt.Printf("❌ Failed to create operation: %v\n", err)
		return
	}
	fmt.Printf("✅ Created operation: %s\n", createdOp.ID[:16]+"...")

	// Retrieve the operation
	fmt.Println("\n🔍 Retrieving operation...")
	retrievedOp, err := client.GetOperation(createdOp.ID)
	if err != nil {
		fmt.Printf("❌ Failed to retrieve operation: %v\n", err)
		return
	}
	fmt.Printf("✅ Retrieved: %s\n", retrievedOp.Content[:50]+"...")

	// Search for operations
	fmt.Println("\n🔎 Searching for operations...")
	searchResult, err := client.Search("func", "", 10, 0)
	if err != nil {
		fmt.Printf("❌ Search failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Found %d results for 'func'\n", len(searchResult.Results))

	// Analyze intent
	fmt.Println("\n🧠 Analyzing operation intent...")
	intent, err := client.GetOperationIntent(createdOp.ID)
	if err != nil {
		fmt.Printf("❌ Intent analysis failed: %v\n", err)
		return
	}
	fmt.Printf("✅ Intent: %s\n", intent.BasicIntent)

	// List operations for the document
	fmt.Println("\n📋 Listing operations for main.go...")
	operations, err := client.ListOperations("main.go", "", 10, 0)
	if err != nil {
		fmt.Printf("❌ Failed to list operations: %v\n", err)
		return
	}
	fmt.Printf("✅ Found %d operations for main.go\n", len(operations))

	fmt.Println("\n🎉 Go client example completed successfully!")
}
