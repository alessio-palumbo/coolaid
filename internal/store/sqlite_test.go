package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetIndex(t *testing.T) {
	s, _ := newTestStore(t)

	s.Items = []Item{{FilePath: "a.go", StartLine: 1, EndLine: 2, Content: "x", Embedding: []float64{1, 2}}}
	s.summary = "hello"

	require.NoError(t, s.Save())
	require.NoError(t, s.ResetIndex())

	var count int
	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&count))
	require.Equal(t, 0, count)

	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM summary`).Scan(&count))
	require.Equal(t, 0, count)

	// Memory check
	require.Nil(t, s.Items)
	require.Equal(t, "", s.summary)
}

func TestSave(t *testing.T) {
	s, _ := newTestStore(t)
	s.ProjectRoot = "/test"
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	s.configHash = "abc123"
	require.NoError(t, s.Save())

	// Writes Meta
	var (
		root string
		hash string
	)
	err := s.db.QueryRow(`
		SELECT project_root, config_hash FROM meta LIMIT 1
	`).Scan(&root, &hash)
	require.NoError(t, err)
	require.Equal(t, s.ProjectRoot, root)
	require.Equal(t, s.configHash, hash)

	// Writes embeddings
	s.Items = []Item{{FilePath: "a.go", StartLine: 1, EndLine: 2, Content: "x", Embedding: []float64{1, 2}}}
	require.NoError(t, s.Save())

	var count int
	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM embeddings`).Scan(&count))
	require.Equal(t, 1, count)

	// Writes Summary
	s.summary = "summary text"
	require.NoError(t, s.Save())

	var content string
	require.NoError(t, s.db.QueryRow(`SELECT content FROM summary`).Scan(&content))
	require.Equal(t, "summary text", content)
}

func TestLoad(t *testing.T) {
	s, _ := newTestStore(t)

	// No data
	err := s.Load()
	require.ErrorIs(t, err, ErrNotIndexed)

	// Add data
	item0 := Item{FilePath: "a.go", StartLine: 1, EndLine: 2, Content: "x", Embedding: []float64{1, 2}}
	item1 := Item{FilePath: "b.go", Symbol: "B", Kind: "function", StartLine: 1, EndLine: 2, Content: "func B()", Embedding: []float64{1, 1}}
	s.Items = []Item{item0, item1}
	s.summary = "hello"
	require.NoError(t, s.Save())

	// Reset memory
	s.Items = nil
	s.summary = ""
	require.NoError(t, s.Load())
	require.True(t, s.loaded)

	require.Len(t, s.Items, 2)
	require.Equal(t, item0, s.Items[0])
	require.Equal(t, item1, s.Items[1])
	require.Equal(t, "hello", s.summary)
}

func TestValidateIndex(t *testing.T) {
	now := time.Now()

	t.Run("no meta row → ErrNotIndexed", func(t *testing.T) {
		s, _ := newTestStore(t)
		require.ErrorIs(t, s.ValidateIndex(), ErrNotIndexed)
	})

	t.Run("valid index", func(t *testing.T) {
		s, _ := newTestStore(t)
		s.configHash = "abc"

		_, err := s.db.Exec(`
			INSERT INTO meta(id, project_root, config_hash, version, created_at)
			VALUES (1, ?, ?, ?, ?)
		`, "/project", "abc", metaVersion, now.Format(time.RFC3339))

		assert.NoError(t, err)
		require.NoError(t, s.ValidateIndex())
	})

	t.Run("version mismatch → ErrReindexRequired", func(t *testing.T) {
		s, _ := newTestStore(t)
		s.configHash = "abc"

		_, err := s.db.Exec(`
			INSERT INTO meta(id, project_root, config_hash, version, created_at)
			VALUES (1, ?, ?, ?, ?)
		`, "/project", "abc", "old-version", now.Format(time.RFC3339))
		assert.NoError(t, err)
		require.ErrorIs(t, s.ValidateIndex(), ErrReindexRequired)
	})

	t.Run("config hash mismatch → ErrReindexRequired", func(t *testing.T) {
		s, _ := newTestStore(t)
		s.configHash = "abc"

		_, err := s.db.Exec(`
			INSERT INTO meta(id, project_root, config_hash, version, created_at)
			VALUES (1, ?, ?, ?, ?)
		`, "/project", "different", metaVersion, now.Format(time.RFC3339))
		assert.NoError(t, err)
		require.ErrorIs(t, s.ValidateIndex(), ErrReindexRequired)
	})

	t.Run("null config hash → ErrReindexRequired", func(t *testing.T) {
		s, _ := newTestStore(t)
		s.configHash = "abc"

		_, err := s.db.Exec(`
			INSERT INTO meta(id, project_root, config_hash, version, created_at)
			VALUES (1, ?, NULL, ?, ?)
		`, "/project", metaVersion, now.Format(time.RFC3339))
		assert.NoError(t, err)
		require.ErrorIs(t, s.ValidateIndex(), ErrReindexRequired)
	})
}

func TestCommitMemoryUpdate(t *testing.T) {
	store, _ := newTestStore(t)

	// seed initial memory
	initial := Memory{
		ProjectSummary: "old",
		Topics:         []string{"old"},
		Preferences:    []string{"verbose"},
	}
	assert.NoError(t, store.CommitMemoryUpdate(context.Background(), initial, nil))

	// seed queue
	assert.NoError(t, store.SaveMemoryQueue(context.Background(), MemoryQueueItem{ID: "a", Payload: []byte("payload a")}))
	assert.NoError(t, store.SaveMemoryQueue(context.Background(), MemoryQueueItem{ID: "b", Payload: []byte("payload b")}))
	assert.NoError(t, store.SaveMemoryQueue(context.Background(), MemoryQueueItem{ID: "c", Payload: []byte("payload c")}))

	// updated memory
	updated := Memory{
		ProjectSummary: "new summary",
		Topics:         []string{"go"},
		Preferences:    []string{"concise"},
	}

	err := store.CommitMemoryUpdate(context.Background(), updated, []string{"a", "b"})
	assert.NoError(t, err)

	// verify memory updated
	gotMem, err := store.getMemory(context.Background())
	assert.NoError(t, err)

	assert.Equal(t, updated.ProjectSummary, gotMem.ProjectSummary)
	assert.Equal(t, updated.Topics, gotMem.Topics)
	assert.Equal(t, updated.Preferences, gotMem.Preferences)

	// verify queue state
	items, err := store.GetMemoryQueue(context.Background())
	assert.NoError(t, err)

	assert.Len(t, items, 1)
	assert.Equal(t, "c", items[0].ID)
}

func TestSaveLoad(t *testing.T) {
	store, tmpDir := newTestStore(t)
	store.AddItem(Item{FilePath: "file.go", Content: "func test()", StartLine: 1, EndLine: 1, Embedding: []float64{1, 2, 3}})
	store.AddSummary("summary")
	assert.NoError(t, store.Save())

	store2, err := NewStore("", tmpDir, "", "")
	assert.NoError(t, err)
	defer store2.Close()

	store2.EnsureLoaded()
	assert.Equal(t, 1, len(store2.Items))
	assert.Equal(t, "summary", store2.summary)
}

func Test_getMemory(t *testing.T) {
	store, _ := newTestStore(t)

	got, err := store.getMemory(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "", got.ProjectSummary)
	assert.Equal(t, []string{}, got.Topics)
	assert.Equal(t, []string{}, got.Preferences)
}

func Test_init(t *testing.T) {
	store, _ := newTestStore(t)
	require.NoError(t, store.init())

	t.Run("memory table exists", func(t *testing.T) {
		var name string
		err := store.db.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name='memory'
		`).Scan(&name)

		require.NoError(t, err)
		require.Equal(t, "memory", name)
	})

	t.Run("memory queue table exists", func(t *testing.T) {
		var name string
		err := store.db.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name='memory_queue'
		`).Scan(&name)

		require.NoError(t, err)
		require.Equal(t, "memory_queue", name)
	})

	t.Run("memory is seeded", func(t *testing.T) {
		var count int
		err := store.db.QueryRow(`SELECT COUNT(*) FROM memory WHERE id = 1`).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("memory cache is loaded", func(t *testing.T) {
		mem := store.GetMemory()
		require.NotNil(t, mem) // or compare to expected empty struct
	})
}

func Test_ensureMetadata(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	store := &Store{db: db}
	err = store.ensureMetadata()
	require.NoError(t, err)

	t.Run("verify table exists", func(t *testing.T) {
		var name string
		err = store.db.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name='meta'
		`).Scan(&name)

		require.NoError(t, err)
		require.Equal(t, "meta", name)
	})

	t.Run("does not insert row", func(t *testing.T) {
		var count int
		err = store.db.QueryRow(`SELECT COUNT(*) FROM meta`).Scan(&count)
		require.NoError(t, err)

		require.Equal(t, 0, count)
	})

	t.Run("verify version exists", func(t *testing.T) {
		has, err := hasColumn(store.db, "meta", "version")
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("old schema is updated", func(t *testing.T) {
		_, err := db.Exec(`DROP TABLE IF EXISTS meta`)
		require.NoError(t, err)

		// Old schema (no version)
		_, err = store.db.Exec(`
			CREATE TABLE meta (
				id INTEGER PRIMARY KEY CHECK (id = 1),
				project_root TEXT NOT NULL,
				config_hash TEXT,
				created_at TEXT NOT NULL
			)
		`)
		require.NoError(t, err)

		err = store.ensureMetadata()
		require.NoError(t, err)

		// Table should be recreated with version column
		has, err := hasColumn(db, "meta", "version")
		require.NoError(t, err)
		require.True(t, has)
	})

	t.Run("multiple calls are safe", func(t *testing.T) {
		require.NoError(t, store.ensureMetadata())
		require.NoError(t, store.ensureMetadata())
		require.NoError(t, store.ensureMetadata())
	})
}

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()

	tmpDir := t.TempDir()
	store, err := NewStore("", tmpDir, "", "")
	assert.NoError(t, err)
	assert.NoError(t, store.ResetIndex())

	t.Cleanup(func() { store.Close() })
	return store, tmpDir
}
