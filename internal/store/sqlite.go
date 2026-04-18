package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const metaVersion = "v2"

var (
	ErrReindexRequired = fmt.Errorf("index is outdated: rebuild required")
	ErrNotIndexed      = fmt.Errorf("not indexed: index must be built")
)

// MemoryQueueItem represents a persisted memory processing job stored in the database.
// Payload contains the serialized Input used to reconstruct the memory update task.
type MemoryQueueItem struct {
	ID      string
	Payload []byte
}

// ResetIndex resets the embeddings table and clears summary data from the DB.
func (s *Store) ResetIndex() (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(dropEmbeddingsSchema)
	if err != nil {
		return err
	}

	_, err = tx.Exec(createEmbeddingsSchema)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM summary`)
	if err != nil {
		return err
	}

	s.Items = nil
	s.Summary = ""
	return tx.Commit()
}

// Save writes the Store embeddings and summary to the DB.
func (s *Store) Save() (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(`
		INSERT INTO meta (id, project_root, config_hash, version, created_at)
		VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_root = excluded.project_root,
			config_hash  = excluded.config_hash,
			version      = excluded.version,
			created_at   = excluded.created_at
		`, s.ProjectRoot, s.configHash, metaVersion, s.now().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
	    INSERT INTO embeddings(filepath, symbol, kind, startline, endline, content, embedding)
	    VALUES(?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range s.Items {
		blob, err := encodeEmbedding(item.Embedding)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			item.FilePath, item.Symbol, item.Kind,
			item.StartLine, item.EndLine, item.Content, blob)
		if err != nil {
			return err
		}
	}

	if s.Summary != "" {
		_, err := tx.Exec(`
		INSERT INTO summary (project_root, content, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(project_root)
		DO UPDATE SET
			content = excluded.content,
			updated_at = excluded.updated_at
			`, s.ProjectRoot, s.Summary, s.now().Format(time.RFC3339))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Load loads embeddings and summary in memory and sets the DB as loaded.
// Prefer using EnsureLoaded() to lazy load and avoid re-loading the DB.
// Use Load() to force re-load the DB, if needed.
func (s *Store) Load() error {
	if err := s.ValidateIndex(); err != nil {
		return err
	}

	rows, err := s.db.Query(`
	    SELECT filepath, symbol, kind, startline, endline, content, embedding
	    FROM embeddings
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var (
			item Item
			blob []byte
		)

		err := rows.Scan(
			&item.FilePath, &item.Symbol, &item.Kind,
			&item.StartLine, &item.EndLine, &item.Content, &blob)
		if err != nil {
			return err
		}

		vec, err := decodeEmbedding(blob)
		if err != nil {
			return err
		}

		item.Embedding = vec
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if len(items) == 0 {
		return ErrNotIndexed
	}

	summary, err := s.loadSummary()
	if err != nil {
		return err
	}

	s.Items = items
	s.Summary = summary
	s.loaded = true
	return nil
}

// ValidateIndex validates whether the index metadata stored in the database
// is present and compatible with the current configuration.
//
// It returns:
//   - nil if the index exists and is valid
//   - ErrNotIndexed if no index metadata is found (initial indexing required)
//   - ErrReindexRequired if the index is outdated or incompatible (e.g. version
//     mismatch or configuration hash change)
//   - any other error for unexpected failures (e.g. database issues)
//
// This method is intended for internal use. Callers are expected to interpret
// the returned error and decide how to handle indexing or reindexing.
func (s *Store) ValidateIndex() error {
	var (
		root       string
		version    string
		configHash *string
	)

	err := s.db.QueryRow(`
		SELECT project_root, version, config_hash
		FROM meta
		LIMIT 1
	`).Scan(&root, &version, &configHash)

	switch err {
	case nil:
	case sql.ErrNoRows:
		return ErrNotIndexed
	default:
		return err
	}

	if version != metaVersion {
		return ErrReindexRequired
	}
	if configHash == nil || *configHash != s.configHash {
		return ErrReindexRequired
	}

	return nil
}

// GetMemory returns the loaded memory snapshot.
func (s *Store) GetMemory() Memory {
	return s.memory
}

// CommitMemoryUpdate atomically persists the updated project memory
// and removes successfully processed items from the memory queue.
//
// It ensures consistency by wrapping both operations in a single
// database transaction:
//   - updates the memory table
//   - deletes processed queue items (if any)
//
// If any step fails, the transaction is rolled back.
func (s *Store) CommitMemoryUpdate(ctx context.Context, m Memory, ids []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err := s.saveMemory(ctx, tx, m); err != nil {
		return err
	}
	if err := s.deleteMemoryQueueItems(ctx, tx, ids); err != nil {
		return err
	}

	return tx.Commit()
}

// GetMemoryQueue returns all pending memory queue items ordered by creation time.
// These items represent unprocessed memory updates that must be handled on startup.
func (s *Store) GetMemoryQueue(ctx context.Context) ([]MemoryQueueItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, payload FROM memory_queue ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []MemoryQueueItem

	for rows.Next() {
		var it MemoryQueueItem
		if err := rows.Scan(&it.ID, &it.Payload); err != nil {
			return nil, err
		}
		items = append(items, it)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// SaveMemoryQueue inserts a new memory queue item into persistent storage.
// This is used to persist memory updates for later processing or recovery.
func (s *Store) SaveMemoryQueue(ctx context.Context, in MemoryQueueItem) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_queue (id, payload) VALUES (?, ?)`,
		in.ID, in.Payload,
	)
	return err
}

// getMemory retrive and returns the current project memory snapshot.
// Assumes a single row (id = 1) exists, initialized at DB setup.
func (s *Store) getMemory(ctx context.Context) (Memory, error) {
	var m Memory
	var topicsJSON, prefsJSON string

	row := s.db.QueryRowContext(ctx, `
		SELECT project_summary, topics, preferences, updated_at
		FROM memory
		WHERE id = 1
	`)

	err := row.Scan(&m.ProjectSummary, &topicsJSON, &prefsJSON, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return Memory{}, nil // empty memory
	}
	if err != nil {
		return Memory{}, err
	}

	_ = json.Unmarshal([]byte(topicsJSON), &m.Topics)
	_ = json.Unmarshal([]byte(prefsJSON), &m.Preferences)

	return m, nil
}

// saveMemory persists the updated project memory.
// Performs an UPDATE on the single memory row (id = 1).
func (s *Store) saveMemory(ctx context.Context, tx *sql.Tx, m Memory) error {
	topicsJSON, err := json.Marshal(m.Topics)
	if err != nil {
		return err
	}

	prefsJSON, err := json.Marshal(m.Preferences)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE memory
		SET project_summary = ?, topics = ?, preferences = ?, updated_at = ?
		WHERE id = 1
	`, m.ProjectSummary, string(topicsJSON), string(prefsJSON), s.now().Format(time.RFC3339),
	)
	return err
}

// deleteMemoryQueueItems deletes the given processed memory queue items by ID.
func (s *Store) deleteMemoryQueueItems(ctx context.Context, tx *sql.Tx, ids []string) error {
	if len(ids) < 1 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))

	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	_, err := tx.ExecContext(ctx,
		"DELETE FROM memory_queue WHERE id IN ("+strings.Join(placeholders, ",")+")",
		args...,
	)
	return err
}

// loadSummary retrieves the repository summary.
func (s *Store) loadSummary() (string, error) {
	var summary string
	err := s.db.QueryRow(`
		SELECT content FROM summary WHERE project_root = ?
	`, s.ProjectRoot).Scan(&summary)

	if err == sql.ErrNoRows {
		return "", nil
	}
	return summary, err
}

// init initialise the DB creating tables if they do not exist.
func (s *Store) init() error {
	if err := s.applyPragmas(); err != nil {
		return err
	}
	if err := s.ensureMetadata(); err != nil {
		return err
	}
	if err := s.ensureMemory(); err != nil {
		return err
	}
	_, err := s.db.Exec(createSummarySchema)
	return err
}

// applyPragmas configures SQLite connection settings required for
// concurrent access across multiple processes.
//
// - journal_mode=WAL enables concurrent readers/writers and persists at DB level.
// - busy_timeout prevents "database is locked" errors by waiting before failing.
//
// Note: busy_timeout is connection-specific and must be set on every DB open,
// including subprocesses.
func (s *Store) applyPragmas() error {
	if _, err := s.db.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return err
	}
	_, err := s.db.Exec(`PRAGMA busy_timeout = 5000;`)
	return err
}

// ensureMetadata creates the meta table if it does not exist
// and, if needed, writes the project_root details.
// If the version is out of date or the config hash has changed
// it returns an error prompting an index rebuild.
func (s *Store) ensureMetadata() error {
	if _, err := s.db.Exec(createMetaSchema); err != nil {
		return err
	}

	// Rebuild table after latest schema change.
	hasVersion, err := hasColumn(s.db, "meta", "version")
	if err != nil {
		return err
	}
	if !hasVersion {
		// schema too old → reset
		return s.resetMetaTable()
	}

	return nil
}

// resetMetaTable drops and recreate the meta table to allow for
// schema changes.
func (s *Store) resetMetaTable() error {
	_, err := s.db.Exec(dropMetaSchema)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(createMetaSchema)
	return err
}

// ensureMemory initializes memory persistence state.
//
// It:
//   - creates memory-related schema if missing
//   - seeds the default memory row
//   - creates the memory queue table
//   - loads current memory into the in-memory cache
func (s *Store) ensureMemory() error {
	if _, err := s.db.Exec(createMemorySchema); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		INSERT OR IGNORE INTO memory (id, project_summary, topics, preferences)
		VALUES (1, '', '[]', '[]');
	`); err != nil {
		return err
	}

	if _, err := s.db.Exec(createMemoryQueueSchema); err != nil {
		return err
	}

	mem, err := s.getMemory(context.Background())
	if err != nil {
		return err
	}
	s.memory = mem
	return nil
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var (
		cid       int
		name      string
		colType   string
		notnull   int
		dfltValue any
		pk        int
	)

	for rows.Next() {
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}

	return false, nil
}

// encodeEmbedding parses a vector slice into a blob for storage.
func encodeEmbedding(vec []float64) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, vec)
	return buf.Bytes(), err
}

// decodeEmbedding decodes a blob into a vector slice.
func decodeEmbedding(data []byte) ([]float64, error) {
	count := len(data) / 8
	vec := make([]float64, count)

	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.LittleEndian, &vec)
	return vec, err
}
