package context

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jeremytregunna/contextdb/internal/addressing"
	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
)

type ContextAnalyzer struct {
	operationDAG        *operations.OperationDAG
	documents           map[string]*positioning.Document
	addressResolver     *addressing.AddressResolver
	conversationManager *ConversationManager
	mutex               sync.RWMutex
}

type OperationContext struct {
	Operation    *operations.Operation   `json:"operation"`
	CausalChain  []*operations.Operation `json:"causal_chain"`
	Consequences []*operations.Operation `json:"consequences"`
	Discussions  []*ConversationThread   `json:"discussions"`
	CodeContext  *CodeContext            `json:"code_context"`
	Intent       string                  `json:"intent,omitempty"`
	Summary      string                  `json:"summary"`
}

type CodeContext struct {
	BeforeContent   string                   `json:"before_content"`
	AfterContent    string                   `json:"after_content"`
	SurroundingCode string                   `json:"surrounding_code"`
	AffectedRange   addressing.PositionRange `json:"affected_range"`
	SemanticInfo    map[string]interface{}   `json:"semantic_info,omitempty"`
}

type IntentAnalysis struct {
	PrimaryIntent string         `json:"primary_intent"`
	Confidence    float64        `json:"confidence"`
	Evidence      []string       `json:"evidence"`
	Keywords      []string       `json:"keywords"`
	Category      IntentCategory `json:"category"`
}

type IntentCategory string

const (
	IntentFeature  IntentCategory = "feature"
	IntentBugfix   IntentCategory = "bugfix"
	IntentRefactor IntentCategory = "refactor"
	IntentCleanup  IntentCategory = "cleanup"
	IntentDoc      IntentCategory = "documentation"
	IntentTest     IntentCategory = "test"
	IntentUnknown  IntentCategory = "unknown"
)

type AuthorActivity struct {
	AuthorID   operations.AuthorID     `json:"author_id"`
	Period     TimePeriod              `json:"period"`
	Operations []*operations.Operation `json:"operations"`
	Summary    ActivitySummary         `json:"summary"`
	Patterns   []ActivityPattern       `json:"patterns"`
}

type TimePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type ActivitySummary struct {
	TotalOperations   int                    `json:"total_operations"`
	OperationTypes    map[string]int         `json:"operation_types"`
	IntentTypes       map[IntentCategory]int `json:"intent_types"`
	DocumentsModified []string               `json:"documents_modified"`
	Conversations     int                    `json:"conversations"`
	LinesAdded        int                    `json:"lines_added"`
	LinesDeleted      int                    `json:"lines_deleted"`
}

type ActivityPattern struct {
	Type        PatternType `json:"type"`
	Description string      `json:"description"`
	Frequency   float64     `json:"frequency"`
	Confidence  float64     `json:"confidence"`
}

type PatternType string

const (
	PatternBursty      PatternType = "bursty"      // Intense periods of activity
	PatternSteady      PatternType = "steady"      // Consistent activity
	PatternRefactoring PatternType = "refactoring" // Heavy refactoring periods
	PatternBugfixing   PatternType = "bugfixing"   // Bug fix focused
)

func NewContextAnalyzer(
	operationDAG *operations.OperationDAG,
	addressResolver *addressing.AddressResolver,
	conversationManager *ConversationManager,
) *ContextAnalyzer {
	return &ContextAnalyzer{
		operationDAG:        operationDAG,
		documents:           make(map[string]*positioning.Document),
		addressResolver:     addressResolver,
		conversationManager: conversationManager,
	}
}

func (ca *ContextAnalyzer) GetOperationContext(opID operations.OperationID) (*OperationContext, error) {
	ca.mutex.RLock()
	defer ca.mutex.RUnlock()

	op, err := ca.operationDAG.GetOperation(opID)
	if err != nil {
		return nil, err
	}

	// Get causal chain
	causalChain, err := ca.operationDAG.GetCausalHistory(opID)
	if err != nil {
		return nil, err
	}

	// Get consequences (operations that depend on this one)
	consequences := ca.getConsequences(opID)

	// Get related discussions
	discussions := ca.getRelatedDiscussions(op)

	// Build code context
	codeContext := ca.buildCodeContext(op)

	// Analyze intent
	intent := ca.analyzeOperationIntent(op)

	// Generate summary
	summary := ca.generateOperationSummary(op, intent)

	return &OperationContext{
		Operation:    op,
		CausalChain:  causalChain,
		Consequences: consequences,
		Discussions:  discussions,
		CodeContext:  codeContext,
		Intent:       intent.PrimaryIntent,
		Summary:      summary,
	}, nil
}

func (ca *ContextAnalyzer) AnalyzeChangeIntent(ops []*operations.Operation) (*IntentAnalysis, error) {
	if len(ops) == 0 {
		return &IntentAnalysis{
			PrimaryIntent: "unknown",
			Confidence:    0.0,
			Category:      IntentUnknown,
		}, nil
	}

	// Aggregate evidence from all operations
	var evidence []string
	var keywords []string

	for _, op := range ops {
		if op.Metadata.Intent != "" {
			evidence = append(evidence, "explicit_intent:"+op.Metadata.Intent)
		}

		// Extract keywords from content
		contentKeywords := ca.extractKeywords(op.Content)
		keywords = append(keywords, contentKeywords...)
	}

	// Determine primary intent
	intent, confidence := ca.classifyIntent(evidence, keywords)
	category := ca.categorizeIntent(intent)

	return &IntentAnalysis{
		PrimaryIntent: intent,
		Confidence:    confidence,
		Evidence:      evidence,
		Keywords:      removeDuplicates(keywords),
		Category:      category,
	}, nil
}

func (ca *ContextAnalyzer) GetAuthorActivity(authorID operations.AuthorID, since time.Time) (*AuthorActivity, error) {
	ca.mutex.RLock()
	defer ca.mutex.RUnlock()

	ops, err := ca.operationDAG.GetOperationsByAuthor(authorID)
	if err != nil {
		return nil, err
	}

	// Filter by time period
	var filteredOps []*operations.Operation
	for _, op := range ops {
		if op.Timestamp.After(since) {
			filteredOps = append(filteredOps, op)
		}
	}

	if len(filteredOps) == 0 {
		return &AuthorActivity{
			AuthorID: authorID,
			Period: TimePeriod{
				Start: since,
				End:   time.Now(),
			},
			Operations: []*operations.Operation{},
			Summary: ActivitySummary{
				TotalOperations: 0,
				OperationTypes:  make(map[string]int),
				IntentTypes:     make(map[IntentCategory]int),
			},
		}, nil
	}

	// Build activity summary
	summary := ca.buildActivitySummary(filteredOps)

	// Detect patterns
	patterns := ca.detectActivityPatterns(filteredOps)

	return &AuthorActivity{
		AuthorID:   authorID,
		Period:     TimePeriod{Start: since, End: time.Now()},
		Operations: filteredOps,
		Summary:    summary,
		Patterns:   patterns,
	}, nil
}

func (ca *ContextAnalyzer) GetCodeHistory(addr addressing.StableAddress) ([]*operations.Operation, error) {
	resolved, err := ca.addressResolver.ResolveAddress(addr)
	if err != nil {
		return nil, err
	}

	var history []*operations.Operation

	// Add creation operation
	if resolved.CreationOp != nil {
		history = append(history, resolved.CreationOp)
	}

	// Add operations from movement history
	for _, movement := range resolved.MovementHistory {
		if movement.CausedBy != "" {
			if op, err := ca.operationDAG.GetOperation(movement.CausedBy); err == nil {
				history = append(history, op)
			}
		}
	}

	// Sort by timestamp
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	return history, nil
}

func (ca *ContextAnalyzer) getConsequences(opID operations.OperationID) []*operations.Operation {
	ca.mutex.RLock()
	defer ca.mutex.RUnlock()

	// Find operations that depend on this operation
	// by checking if this operation is in their causal history
	var consequences []*operations.Operation

	// This is an expensive operation - in production we'd want to index this
	for _, doc := range ca.documents {
		for _, construct := range doc.Constructs {
			if construct.CreatedBy == opID || construct.ModifiedBy == opID {
				// Find operations that modified this construct after the given operation
				for _, otherConstruct := range doc.Constructs {
					if otherConstruct.Position.Compare(construct.Position) == 0 &&
						otherConstruct.ModifiedBy != opID &&
						otherConstruct.ModifiedBy != "" {
						// This is a consequence - another operation modified the same position
						if op, err := ca.operationDAG.GetOperation(otherConstruct.ModifiedBy); err == nil {
							consequences = append(consequences, op)
						}
					}
				}
			}
		}
	}

	return consequences
}

func (ca *ContextAnalyzer) getRelatedDiscussions(op *operations.Operation) []*ConversationThread {
	// Find conversations that reference this operation's position or content
	var discussions []*ConversationThread

	// Create a simple address to search for related conversations
	// In a real implementation, we'd have proper address resolution
	addr := addressing.StableAddress{
		Scheme:      "contextdb",
		Repository:  addressing.RepositoryID("local"),
		OperationID: op.ID,
		PositionRange: addressing.PositionRange{
			Start: op.Position,
			End:   op.Position,
		},
	}

	// Search for conversations anchored at this position
	if threads, err := ca.conversationManager.GetConversationsByAddress(addr); err == nil {
		discussions = append(discussions, threads...)
	}

	// Also search for conversations that mention the operation content or ID
	if op.Content != "" && len(op.Content) > 3 {
		if threads, err := ca.conversationManager.SearchConversations(op.Content); err == nil {
			discussions = append(discussions, threads...)
		}
	}

	// Search by operation ID (shortened)
	opIDShort := string(op.ID)[:min(len(string(op.ID)), 8)]
	if threads, err := ca.conversationManager.SearchConversations(opIDShort); err == nil {
		discussions = append(discussions, threads...)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []*ConversationThread
	for _, thread := range discussions {
		if !seen[string(thread.ID)] {
			seen[string(thread.ID)] = true
			unique = append(unique, thread)
		}
	}

	return unique
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (ca *ContextAnalyzer) buildCodeContext(op *operations.Operation) *CodeContext {
	// Build context around the operation
	return &CodeContext{
		BeforeContent:   "",
		AfterContent:    op.Content,
		SurroundingCode: "",
		AffectedRange: addressing.PositionRange{
			Start: op.Position,
			End:   op.Position,
		},
		SemanticInfo: make(map[string]interface{}),
	}
}

func (ca *ContextAnalyzer) analyzeOperationIntent(op *operations.Operation) *IntentAnalysis {
	evidence := []string{}
	keywords := []string{}

	// Use explicit intent if available
	if op.Metadata.Intent != "" {
		evidence = append(evidence, "explicit_intent:"+op.Metadata.Intent)
	}

	// Extract keywords from content
	keywords = ca.extractKeywords(op.Content)

	intent, confidence := ca.classifyIntent(evidence, keywords)
	category := ca.categorizeIntent(intent)

	return &IntentAnalysis{
		PrimaryIntent: intent,
		Confidence:    confidence,
		Evidence:      evidence,
		Keywords:      keywords,
		Category:      category,
	}
}

func (ca *ContextAnalyzer) generateOperationSummary(op *operations.Operation, intent *IntentAnalysis) string {
	summary := fmt.Sprintf("Operation %s by %s",
		op.Type, op.Author)

	if intent.PrimaryIntent != "" && intent.PrimaryIntent != "unknown" {
		summary += fmt.Sprintf(" (%s)", intent.PrimaryIntent)
	}

	if op.Content != "" {
		contentPreview := op.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50] + "..."
		}
		summary += fmt.Sprintf(": %q", contentPreview)
	}

	return summary
}

func (ca *ContextAnalyzer) extractKeywords(content string) []string {
	// Simple keyword extraction
	words := strings.Fields(strings.ToLower(content))
	var keywords []string

	// Common programming keywords that indicate intent
	intentKeywords := map[string]bool{
		"fix": true, "bug": true, "error": true, "issue": true,
		"add": true, "new": true, "feature": true, "implement": true,
		"refactor": true, "clean": true, "optimize": true, "improve": true,
		"test": true, "spec": true, "unit": true, "integration": true,
		"doc": true, "comment": true, "readme": true, "documentation": true,
		"todo": true, "fixme": true, "hack": true, "temporary": true,
	}

	for _, word := range words {
		if intentKeywords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

func (ca *ContextAnalyzer) classifyIntent(evidence []string, keywords []string) (string, float64) {
	// Simple intent classification
	intentScores := make(map[string]float64)

	// Score based on evidence
	for _, e := range evidence {
		if strings.HasPrefix(e, "explicit_intent:") {
			intent := strings.TrimPrefix(e, "explicit_intent:")
			intentScores[intent] += 1.0
		}
	}

	// Score based on keywords
	for _, keyword := range keywords {
		switch keyword {
		case "fix", "bug", "error", "issue":
			intentScores["bugfix"] += 0.5
		case "add", "new", "feature", "implement":
			intentScores["feature"] += 0.5
		case "refactor", "clean", "optimize", "improve":
			intentScores["refactor"] += 0.5
		case "test", "spec", "unit", "integration":
			intentScores["test"] += 0.5
		case "doc", "comment", "readme", "documentation":
			intentScores["documentation"] += 0.5
		}
	}

	// Find highest scoring intent
	var bestIntent string
	var bestScore float64

	for intent, score := range intentScores {
		if score > bestScore {
			bestIntent = intent
			bestScore = score
		}
	}

	if bestIntent == "" {
		return "unknown", 0.0
	}

	// Normalize confidence to 0-1 range
	confidence := bestScore / (bestScore + 1.0)

	return bestIntent, confidence
}

func (ca *ContextAnalyzer) categorizeIntent(intent string) IntentCategory {
	switch intent {
	case "feature", "add", "new", "implement":
		return IntentFeature
	case "bugfix", "fix", "bug", "error":
		return IntentBugfix
	case "refactor", "clean", "optimize", "improve":
		return IntentRefactor
	case "test", "spec", "unit", "integration":
		return IntentTest
	case "doc", "documentation", "comment":
		return IntentDoc
	case "cleanup":
		return IntentCleanup
	default:
		return IntentUnknown
	}
}

func (ca *ContextAnalyzer) buildActivitySummary(ops []*operations.Operation) ActivitySummary {
	summary := ActivitySummary{
		TotalOperations:   len(ops),
		OperationTypes:    make(map[string]int),
		IntentTypes:       make(map[IntentCategory]int),
		DocumentsModified: []string{},
	}

	documents := make(map[string]bool)

	for _, op := range ops {
		summary.OperationTypes[string(op.Type)]++

		if docID, exists := op.Metadata.Context["document_id"]; exists {
			documents[docID] = true
		}

		// Analyze intent
		intent := ca.analyzeOperationIntent(op)
		summary.IntentTypes[intent.Category]++

		// Count lines (simplified)
		if op.Type == operations.OpInsert {
			summary.LinesAdded += strings.Count(op.Content, "\n") + 1
		} else if op.Type == operations.OpDelete {
			summary.LinesDeleted += op.Length
		}
	}

	// Convert document set to slice
	for doc := range documents {
		summary.DocumentsModified = append(summary.DocumentsModified, doc)
	}

	return summary
}

func (ca *ContextAnalyzer) detectActivityPatterns(ops []*operations.Operation) []ActivityPattern {
	var patterns []ActivityPattern

	if len(ops) < 2 {
		return patterns
	}

	// Detect bursty pattern (many operations in short time)
	timeSpan := ops[len(ops)-1].Timestamp.Sub(ops[0].Timestamp)
	avgRate := float64(len(ops)) / timeSpan.Hours()

	if avgRate > 5.0 { // More than 5 ops per hour
		patterns = append(patterns, ActivityPattern{
			Type:        PatternBursty,
			Description: "High frequency of operations in short time period",
			Frequency:   avgRate,
			Confidence:  0.8,
		})
	}

	// Detect refactoring pattern
	refactorCount := 0
	for _, op := range ops {
		intent := ca.analyzeOperationIntent(op)
		if intent.Category == IntentRefactor {
			refactorCount++
		}
	}

	refactorRatio := float64(refactorCount) / float64(len(ops))
	if refactorRatio > 0.3 {
		patterns = append(patterns, ActivityPattern{
			Type:        PatternRefactoring,
			Description: "High proportion of refactoring operations",
			Frequency:   refactorRatio,
			Confidence:  0.7,
		})
	}

	return patterns
}

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}
