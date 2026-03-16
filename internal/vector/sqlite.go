package vector

import (
	"bytes"
	"database/sql"
	"encoding/binary"
	"time"
)

func (s *Store) Clear() error {
	_, err := s.db.Exec(`DELETE FROM embeddings`)
	if err != nil {
		return err
	}

	s.Items = nil
	return nil
}

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

	return tx.Commit()
}

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

	s.Items = items
	return nil
}

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
	`

	_, err := s.db.Exec(query)
	return err
}

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

func encodeEmbedding(vec []float64) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, vec)
	return buf.Bytes(), err
}

func decodeEmbedding(data []byte) ([]float64, error) {
	count := len(data) / 8
	vec := make([]float64, count)

	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.LittleEndian, &vec)
	return vec, err
}
