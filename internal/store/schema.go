package store

const (
	createMetaSchema = `
	    CREATE TABLE IF NOT EXISTS meta (
		id           INTEGER PRIMARY KEY CHECK (id = 1),
		project_root TEXT NOT NULL,
		config_hash  TEXT,
		version      TEXT NOT NULL DEFAULT 'v1',
		created_at   TEXT NOT NULL
	    );
	`

	dropMetaSchema = `
	    DROP TABLE IF EXISTS meta;
	`

	createEmbeddingsSchema = `
	    CREATE TABLE IF NOT EXISTS embeddings (
		id INTEGER PRIMARY KEY,
		filepath TEXT,
		symbol TEXT,
		kind TEXT,
		startline INTEGER,
		endline INTEGER,
		content TEXT,
		embedding BLOB
	    );

	    CREATE INDEX IF NOT EXISTS idx_embeddings_symbol ON embeddings(symbol);
	`

	dropEmbeddingsSchema = `
	    DROP TABLE IF EXISTS embeddings;
	    DROP INDEX IF EXISTS idx_embeddings_symbol;
	`

	createSummarySchema = `
	    CREATE TABLE IF NOT EXISTS summary (
		project_root TEXT PRIMARY KEY,
		content TEXT,
		updated_at TEXT
	    );
	`

	createMemorySchema = `
	    CREATE TABLE IF NOT EXISTS memory (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		project_summary TEXT NOT NULL DEFAULT '',
		topics TEXT NOT NULL DEFAULT '[]',        -- JSON: stable themes
		preferences TEXT NOT NULL DEFAULT '[]',   -- JSON: user prefs / style
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`

	createMemoryQueueSchema = `
	    CREATE TABLE IF NOT EXISTS memory_queue (
		id TEXT PRIMARY KEY,
		payload TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	    );
	`
)
