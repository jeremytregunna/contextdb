package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"golang.org/x/crypto/sha3"
)

type AuthManager struct {
	configPath string
	config     *AuthConfig
}

type AuthConfig struct {
	APIKeys       []APIKey            `json:"api_keys"`
	DefaultAuthor operations.AuthorID `json:"default_author"`
	RequireAuth   bool                `json:"require_auth"`
	CreatedAt     time.Time           `json:"created_at"`
	LastModified  time.Time           `json:"last_modified"`
}

type APIKey struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	KeyHash     string              `json:"key_hash"`
	AuthorID    operations.AuthorID `json:"author_id"`
	Permissions []Permission        `json:"permissions"`
	CreatedAt   time.Time           `json:"created_at"`
	LastUsed    *time.Time          `json:"last_used,omitempty"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
}

type Permission string

const (
	PermissionReadOperations  Permission = "read:operations"
	PermissionWriteOperations Permission = "write:operations"
	PermissionReadDocuments   Permission = "read:documents"
	PermissionWriteDocuments  Permission = "write:documents"
	PermissionAnalyze         Permission = "analyze"
	PermissionSearch          Permission = "search"
	PermissionAdmin           Permission = "admin"
	PermissionAll             Permission = "*"
)

type AuthContext struct {
	AuthorID      operations.AuthorID
	APIKeyID      string
	Permissions   []Permission
	Authenticated bool
}

func NewAuthManager(basePath string) (*AuthManager, error) {
	configPath := filepath.Join(basePath, ".context", "auth.json")

	// Check if auth config exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		return createDefaultAuthConfig(configPath)
	}

	// Load existing config
	return loadAuthConfig(configPath)
}

func createDefaultAuthConfig(configPath string) (*AuthManager, error) {
	// Generate default author ID
	defaultAuthor := operations.NewAuthorID("local-dev")

	config := &AuthConfig{
		APIKeys:       []APIKey{},
		DefaultAuthor: defaultAuthor,
		RequireAuth:   false, // Start with auth disabled for ease of use
		CreatedAt:     time.Now(),
		LastModified:  time.Now(),
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create auth directory: %w", err)
	}

	// Save config
	if err := writeJSON(configPath, config); err != nil {
		return nil, fmt.Errorf("failed to save auth config: %w", err)
	}

	return &AuthManager{
		configPath: configPath,
		config:     config,
	}, nil
}

func loadAuthConfig(configPath string) (*AuthManager, error) {
	var config AuthConfig
	if err := readJSON(configPath, &config); err != nil {
		return nil, fmt.Errorf("failed to load auth config: %w", err)
	}

	return &AuthManager{
		configPath: configPath,
		config:     &config,
	}, nil
}

func (am *AuthManager) CreateAPIKey(name string, authorID operations.AuthorID, permissions []Permission, expiresIn *time.Duration) (string, error) {
	// Generate random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}

	keyString := hex.EncodeToString(keyBytes)
	keyHash := hashKey(keyString)

	// Create expiration if specified
	var expiresAt *time.Time
	if expiresIn != nil {
		exp := time.Now().Add(*expiresIn)
		expiresAt = &exp
	}

	apiKey := APIKey{
		ID:          generateKeyID(),
		Name:        name,
		KeyHash:     keyHash,
		AuthorID:    authorID,
		Permissions: permissions,
		CreatedAt:   time.Now(),
		ExpiresAt:   expiresAt,
	}

	am.config.APIKeys = append(am.config.APIKeys, apiKey)
	am.config.LastModified = time.Now()

	if err := am.saveConfig(); err != nil {
		return "", err
	}

	return keyString, nil
}

func (am *AuthManager) ValidateAPIKey(keyString string) (*AuthContext, error) {
	keyHash := hashKey(keyString)

	for i := range am.config.APIKeys {
		key := &am.config.APIKeys[i]

		// Constant-time comparison
		if subtle.ConstantTimeCompare([]byte(key.KeyHash), []byte(keyHash)) == 1 {
			// Check if expired
			if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
				return nil, fmt.Errorf("API key expired")
			}

			// Update last used
			now := time.Now()
			key.LastUsed = &now
			am.config.LastModified = time.Now()
			am.saveConfig() // Best effort, don't fail validation if this fails

			return &AuthContext{
				AuthorID:      key.AuthorID,
				APIKeyID:      key.ID,
				Permissions:   key.Permissions,
				Authenticated: true,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

func (am *AuthManager) GetAnonymousContext() *AuthContext {
	return &AuthContext{
		AuthorID:      am.config.DefaultAuthor,
		APIKeyID:      "",
		Permissions:   []Permission{PermissionAll}, // Anonymous gets all permissions when auth disabled
		Authenticated: false,
	}
}

func (am *AuthManager) IsAuthRequired() bool {
	return am.config.RequireAuth
}

func (am *AuthManager) EnableAuth() error {
	am.config.RequireAuth = true
	am.config.LastModified = time.Now()
	return am.saveConfig()
}

func (am *AuthManager) DisableAuth() error {
	am.config.RequireAuth = false
	am.config.LastModified = time.Now()
	return am.saveConfig()
}

func (am *AuthManager) ListAPIKeys() []APIKeySummary {
	var summaries []APIKeySummary
	for _, key := range am.config.APIKeys {
		summaries = append(summaries, APIKeySummary{
			ID:          key.ID,
			Name:        key.Name,
			AuthorID:    key.AuthorID,
			Permissions: key.Permissions,
			CreatedAt:   key.CreatedAt,
			LastUsed:    key.LastUsed,
			ExpiresAt:   key.ExpiresAt,
		})
	}
	return summaries
}

type APIKeySummary struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	AuthorID    operations.AuthorID `json:"author_id"`
	Permissions []Permission        `json:"permissions"`
	CreatedAt   time.Time           `json:"created_at"`
	LastUsed    *time.Time          `json:"last_used,omitempty"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
}

func (am *AuthManager) RevokeAPIKey(keyID string) error {
	for i, key := range am.config.APIKeys {
		if key.ID == keyID {
			// Remove key by slicing
			am.config.APIKeys = append(am.config.APIKeys[:i], am.config.APIKeys[i+1:]...)
			am.config.LastModified = time.Now()
			return am.saveConfig()
		}
	}
	return fmt.Errorf("API key not found")
}

func (ac *AuthContext) HasPermission(perm Permission) bool {
	for _, p := range ac.Permissions {
		if p == PermissionAll || p == perm {
			return true
		}
	}
	return false
}

func (am *AuthManager) saveConfig() error {
	return writeJSON(am.configPath, am.config)
}

// Helper functions
func hashKey(key string) string {
	hash := sha3.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func generateKeyID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func writeJSON(filePath string, data interface{}) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func readJSON(filePath string, data interface{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(data)
}
