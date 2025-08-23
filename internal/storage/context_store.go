package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeremytregunna/contextdb/internal/operations"
	"github.com/jeremytregunna/contextdb/internal/positioning"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ContextDir         = ".context"
	ManifestFile       = "manifest.json"
	DatabaseFile       = "contextdb.sqlite"
	CurrentVersion     = "1.0.0"
	CompatibleVersions = "^1.0.0"
)

type ContextStore struct {
	basePath string
	db       *sql.DB
	manifest *Manifest
}

type Manifest struct {
	Version       string            `json:"version"`
	Created       time.Time         `json:"created"`
	LastModified  time.Time         `json:"last_modified"`
	SchemaVersion string            `json:"schema_version"`
	StorageType   string            `json:"storage_type"`
	DatabaseFile  string            `json:"database_file"`
	Metadata      map[string]string `json:"metadata"`
}

func NewContextStore(basePath string) (*ContextStore, error) {
	contextPath := filepath.Join(basePath, ContextDir)

	// Check if .context exists
	if _, err := os.Stat(contextPath); err == nil {
		// Directory exists, validate and open
		return openExistingContextStore(contextPath)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to access .context directory: %w", err)
	}

	// Create new store
	return createNewContextStore(contextPath)
}

func createNewContextStore(contextPath string) (*ContextStore, error) {
	// Create .context directory
	if err := os.MkdirAll(contextPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .context directory: %w", err)
	}

	// Create manifest
	manifest := &Manifest{
		Version:       CurrentVersion,
		Created:       time.Now(),
		LastModified:  time.Now(),
		SchemaVersion: "1.0",
		StorageType:   "sqlite",
		DatabaseFile:  DatabaseFile,
		Metadata: map[string]string{
			"created_by":  "contextdb",
			"description": "ContextDB SQLite storage with manifest",
		},
	}

	manifestPath := filepath.Join(contextPath, ManifestFile)
	if err := writeJSON(manifestPath, manifest); err != nil {
		return nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	// Initialize SQLite database
	dbPath := filepath.Join(contextPath, DatabaseFile)
	db, err := initSQLiteDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &ContextStore{
		basePath: contextPath,
		db:       db,
		manifest: manifest,
	}, nil
}

func openExistingContextStore(contextPath string) (*ContextStore, error) {
	manifestPath := filepath.Join(contextPath, ManifestFile)

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return nil, fmt.Errorf(".context directory exists but no manifest found - not a valid ContextDB storage")
	}

	// Read and validate manifest
	var manifest Manifest
	if err := readJSON(manifestPath, &manifest); err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Validate version compatibility
	if !isCompatibleVersion(manifest.Version) {
		return nil, fmt.Errorf("incompatible storage version: %s (need %s)", manifest.Version, CompatibleVersions)
	}

	// Validate it's our storage
	if manifest.Metadata["created_by"] != "contextdb" {
		return nil, fmt.Errorf(".context directory not created by ContextDB")
	}

	// Open SQLite database
	dbPath := filepath.Join(contextPath, manifest.DatabaseFile)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file %s not found", manifest.DatabaseFile)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	// Update last modified
	manifest.LastModified = time.Now()
	if err := writeJSON(manifestPath, &manifest); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to update manifest: %w", err)
	}

	return &ContextStore{
		basePath: contextPath,
		db:       db,
		manifest: &manifest,
	}, nil
}

func initSQLiteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS operations (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		position_segments TEXT NOT NULL,
		content TEXT NOT NULL,
		length INTEGER,
		author TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		parents TEXT,
		metadata TEXT
	);

	CREATE TABLE IF NOT EXISTS documents (
		file_path TEXT PRIMARY KEY,
		version INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		last_operation TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS constructs (
		id TEXT PRIMARY KEY,
		document_path TEXT NOT NULL,
		position_segments TEXT NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL,
		created_by TEXT NOT NULL,
		modified_by TEXT NOT NULL,
		metadata TEXT,
		FOREIGN KEY (document_path) REFERENCES documents(file_path),
		FOREIGN KEY (created_by) REFERENCES operations(id),
		FOREIGN KEY (modified_by) REFERENCES operations(id)
	);

	CREATE INDEX IF NOT EXISTS idx_operations_timestamp ON operations(timestamp);
	CREATE INDEX IF NOT EXISTS idx_operations_author ON operations(author);
	CREATE INDEX IF NOT EXISTS idx_operations_type ON operations(type);
	CREATE INDEX IF NOT EXISTS idx_constructs_document ON constructs(document_path);
	CREATE INDEX IF NOT EXISTS idx_constructs_type ON constructs(type);
	CREATE INDEX IF NOT EXISTS idx_constructs_position ON constructs(position_segments);
	`

	_, err = db.Exec(schema)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// Implement the Store interface by embedding SQLite operations

func (cs *ContextStore) StoreOperation(op *operations.Operation) error {
	positionJSON, err := json.Marshal(op.Position.Segments)
	if err != nil {
		return fmt.Errorf("failed to marshal position: %w", err)
	}

	parentsJSON, err := json.Marshal(op.Parents)
	if err != nil {
		return fmt.Errorf("failed to marshal parents: %w", err)
	}

	metadataJSON, err := json.Marshal(op.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO operations 
		(id, type, position_segments, content, length, author, timestamp, parents, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = cs.db.Exec(query,
		string(op.ID),
		string(op.Type),
		string(positionJSON),
		op.Content,
		op.Length,
		string(op.Author),
		op.Timestamp.Unix(),
		string(parentsJSON),
		string(metadataJSON),
	)

	return err
}

func (cs *ContextStore) GetOperation(id operations.OperationID) (*operations.Operation, error) {
	query := `
		SELECT id, type, position_segments, content, length, author, timestamp, parents, metadata
		FROM operations WHERE id = ?
	`

	row := cs.db.QueryRow(query, string(id))
	return cs.scanOperation(row)
}

func (cs *ContextStore) GetOperations(ids []operations.OperationID) ([]*operations.Operation, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	args := make([]interface{}, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = string(id)
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(`
		SELECT id, type, position_segments, content, length, author, timestamp, parents, metadata
		FROM operations WHERE id IN (%s)
		ORDER BY timestamp
	`, strings.Join(placeholders, ","))

	rows, err := cs.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*operations.Operation
	for rows.Next() {
		op, err := cs.scanOperation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, op)
	}

	return result, rows.Err()
}

func (cs *ContextStore) GetOperationsSince(timestamp time.Time) ([]*operations.Operation, error) {
	query := `
		SELECT id, type, position_segments, content, length, author, timestamp, parents, metadata
		FROM operations WHERE timestamp >= ?
		ORDER BY timestamp
	`

	rows, err := cs.db.Query(query, timestamp.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*operations.Operation
	for rows.Next() {
		op, err := cs.scanOperation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, op)
	}

	return result, rows.Err()
}

func (cs *ContextStore) GetOperationsByAuthor(authorID operations.AuthorID) ([]*operations.Operation, error) {
	query := `
		SELECT id, type, position_segments, content, length, author, timestamp, parents, metadata
		FROM operations WHERE author = ?
		ORDER BY timestamp
	`

	rows, err := cs.db.Query(query, string(authorID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*operations.Operation
	for rows.Next() {
		op, err := cs.scanOperation(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, op)
	}

	return result, rows.Err()
}

func (cs *ContextStore) DeleteOperation(id operations.OperationID) error {
	_, err := cs.db.Exec("DELETE FROM operations WHERE id = ?", string(id))
	return err
}

func (cs *ContextStore) StoreDocument(doc *positioning.Document) error {
	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	docQuery := `
		INSERT OR REPLACE INTO documents 
		(file_path, version, content_hash, last_operation, created_at, updated_at)
		VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM documents WHERE file_path = ?), ?), ?)
	`

	_, err = tx.Exec(docQuery,
		doc.FilePath,
		doc.Version,
		fmt.Sprintf("%x", doc.ContentHash),
		string(doc.LastOperation),
		doc.FilePath,
		now,
		now,
	)
	if err != nil {
		return err
	}

	// Clear existing constructs
	_, err = tx.Exec("DELETE FROM constructs WHERE document_path = ?", doc.FilePath)
	if err != nil {
		return err
	}

	// Insert new constructs
	constructQuery := `
		INSERT INTO constructs 
		(id, document_path, position_segments, content, type, created_by, modified_by, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, construct := range doc.Constructs {
		positionJSON, err := json.Marshal(construct.Position.Segments)
		if err != nil {
			return fmt.Errorf("failed to marshal position: %w", err)
		}

		metadataJSON, err := json.Marshal(construct.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		_, err = tx.Exec(constructQuery,
			string(construct.ID),
			doc.FilePath,
			string(positionJSON),
			construct.Content,
			string(construct.Type),
			string(construct.CreatedBy),
			string(construct.ModifiedBy),
			string(metadataJSON),
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (cs *ContextStore) GetDocument(filePath string) (*positioning.Document, error) {
	docQuery := `
		SELECT file_path, version, content_hash, last_operation
		FROM documents WHERE file_path = ?
	`

	var doc positioning.Document
	var contentHashStr string
	var lastOpStr string

	err := cs.db.QueryRow(docQuery, filePath).Scan(
		&doc.FilePath,
		&doc.Version,
		&contentHashStr,
		&lastOpStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}

	doc.Constructs = make(map[operations.PositionKey]*positioning.Construct)
	doc.PositionIndex = make(map[operations.PositionKey]operations.LogootPosition)
	doc.PositionIdx = make([]operations.LogootPosition, 0)

	doc.LastOperation = operations.OperationID(lastOpStr)

	// Load constructs
	constructQuery := `
		SELECT id, position_segments, content, type, created_by, modified_by, metadata
		FROM constructs WHERE document_path = ?
		ORDER BY position_segments
	`

	rows, err := cs.db.Query(constructQuery, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var construct positioning.Construct
		var positionJSON string
		var metadataJSON string
		var createdByStr string
		var modifiedByStr string

		err := rows.Scan(
			&construct.ID,
			&positionJSON,
			&construct.Content,
			&construct.Type,
			&createdByStr,
			&modifiedByStr,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		var segments []operations.PositionSegment
		if err := json.Unmarshal([]byte(positionJSON), &segments); err != nil {
			return nil, fmt.Errorf("failed to unmarshal position: %w", err)
		}
		construct.Position = operations.NewLogootPosition(segments)

		if err := json.Unmarshal([]byte(metadataJSON), &construct.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		construct.CreatedBy = operations.OperationID(createdByStr)
		construct.ModifiedBy = operations.OperationID(modifiedByStr)

		posKey := construct.Position.Key()
		doc.Constructs[posKey] = &construct
		doc.PositionIndex[posKey] = construct.Position
		doc.PositionIdx = append(doc.PositionIdx, construct.Position)
	}

	return &doc, rows.Err()
}

func (cs *ContextStore) ListDocuments() ([]string, error) {
	query := "SELECT file_path FROM documents ORDER BY file_path"
	rows, err := cs.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var documents []string
	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return nil, err
		}
		documents = append(documents, filePath)
	}

	return documents, rows.Err()
}

func (cs *ContextStore) DeleteDocument(filePath string) error {
	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM constructs WHERE document_path = ?", filePath)
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM documents WHERE file_path = ?", filePath)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (cs *ContextStore) Close() error {
	// Update manifest one last time
	cs.manifest.LastModified = time.Now()
	manifestPath := filepath.Join(cs.basePath, ManifestFile)
	if err := writeJSON(manifestPath, cs.manifest); err != nil {
		// Log but don't fail on manifest update
	}

	return cs.db.Close()
}

func (cs *ContextStore) scanOperation(scanner interface {
	Scan(dest ...interface{}) error
}) (*operations.Operation, error) {
	var op operations.Operation
	var idStr, positionJSON, parentsJSON, metadataJSON string
	var timestampUnix int64

	err := scanner.Scan(
		&idStr,
		&op.Type,
		&positionJSON,
		&op.Content,
		&op.Length,
		&op.Author,
		&timestampUnix,
		&parentsJSON,
		&metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	op.ID = operations.OperationID(idStr)
	op.Timestamp = time.Unix(timestampUnix, 0)

	var segments []operations.PositionSegment
	if err := json.Unmarshal([]byte(positionJSON), &segments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal position: %w", err)
	}
	op.Position = operations.NewLogootPosition(segments)

	if err := json.Unmarshal([]byte(parentsJSON), &op.Parents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal parents: %w", err)
	}

	if err := json.Unmarshal([]byte(metadataJSON), &op.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &op, nil
}

// Helper functions
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

func isCompatibleVersion(version string) bool {
	// Simple version check - in production, use proper semver
	return version == CurrentVersion || version == "1.0.0"
}
