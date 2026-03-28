package vector

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"fmt"
	"time"
)

const metaVersion = "v1"

var (
	ErrReindexRequired = fmt.Errorf("index is outdated: rebuild required")
	ErrNotIndexed      = fmt.Errorf("not indexed: index must be built")
)

// Clear deletes all embeddings and summary data from the DB.
func (s *Store) Clear() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	_, err = tx.Exec(`DELETE FROM embeddings`)
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

// Saves writes the Store embeddings and summary to the DB.
func (s *Store) Save() error {
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
	    INSERT INTO embeddings(filepath, startline, endline, content, embedding)
	    VALUES(?, ?, ?, ?, ?)
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
			item.FilePath, item.StartLine,
			item.EndLine, item.Content, blob)
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
	if err := s.CheckIndex(); err != nil {
		return err
	}

	rows, err := s.db.Query(`
	    SELECT filepath, startline, endline, content, embedding
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
			&item.FilePath, &item.StartLine,
			&item.EndLine, &item.Content, &blob)
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

// CheckIndex checks that an index has been build and it's not
// out of date. If a version change or the config hash does not
// match it return an error prompting for an index rebuild.
func (s *Store) CheckIndex() error {
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
	if err := s.ensureMetadata(); err != nil {
		return err
	}

	query := `
	    CREATE TABLE IF NOT EXISTS embeddings (
	        id INTEGER PRIMARY KEY,
	        filepath TEXT,
	        startline INTEGER,
	        endline INTEGER,
	        content TEXT,
	        embedding BLOB
	    );

	    CREATE TABLE IF NOT EXISTS summary (
	        project_root TEXT PRIMARY KEY,
	        content TEXT,
	        updated_at TEXT
	    );
	`

	_, err := s.db.Exec(query)
	return err
}

// ensureMetadata creates the meta table if it does not exist
// and, if needed, writes the project_root details.
// If the version is out of date or the config hash has changed
// it returns an error prompting an index rebuild.
func (s *Store) ensureMetadata() error {
	if err := s.createMeta(); err != nil {
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

func (s *Store) createMeta() error {
	_, err := s.db.Exec(`
	    CREATE TABLE IF NOT EXISTS meta (
		id           INTEGER PRIMARY KEY CHECK (id = 1),
		project_root TEXT NOT NULL,
		config_hash  TEXT,
		version      TEXT NOT NULL DEFAULT 'v1',
		created_at   TEXT NOT NULL
	    )
	`)
	return err
}

func (s *Store) resetMetaTable() error {
	_, err := s.db.Exec(`DROP TABLE IF EXISTS meta`)
	if err != nil {
		return err
	}
	return s.createMeta()
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
