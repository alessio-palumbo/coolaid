package vector

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"time"
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
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(project_root)
		DO UPDATE SET
			content = excluded.content,
			updated_at = excluded.updated_at
			`, s.ProjectRoot, s.Summary)
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

	summary, err := s.loadSummary()
	if err != nil {
		return err
	}

	s.Items = items
	s.Summary = summary
	s.loaded = true
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
func (s *Store) ensureMetadata() error {
	_, err := s.db.Exec(`
	    CREATE TABLE IF NOT EXISTS meta (
		project_root TEXT PRIMARY KEY,
		created_at   TEXT
	    )
	`)
	if err != nil {
		return err
	}

	var root string
	err = s.db.QueryRow(`SELECT project_root FROM meta LIMIT 1`).Scan(&root)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(
			`INSERT INTO meta(project_root, created_at) VALUES (?, ?)`,
			s.ProjectRoot,
			time.Now().UTC().Format(time.RFC3339),
		)
		return err
	}

	return err
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
