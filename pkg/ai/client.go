package ai

import (
	"context"
	"coolaid/internal/core/engine"
	"coolaid/internal/indexer"
	"coolaid/internal/llm"
	"coolaid/internal/llm/memory"
	"coolaid/internal/store"
	"errors"
	"io"
	"log/slog"
	"time"
)

// IndexStatus represents the current state of the index.
type IndexStatus int

const (
	// IndexOK indicates that the index exists and is valid.
	IndexOK IndexStatus = iota

	// IndexMissing indicates that no index has been created yet.
	// An initial indexing is required.
	IndexMissing

	// IndexStale indicates that the index exists but is outdated or
	// incompatible with the current configuration or version.
	// A full reindex is required.
	IndexStale
)

// embedder generates vector embeddings for a given text input.
//
// It is used by the indexing pipeline to convert chunks of content
// into high-dimensional vectors for similarity search and retrieval.
type embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// memStore defines the interface for asynchronous project memory management.
//
// It is responsible for capturing interaction context for background processing
// and for flushing any buffered or pending memory state.
type memStore interface {
	// Capture records an interaction and optionally wraps execution to allow
	// streaming or buffered context collection during processing.
	Capture(w io.Writer, userPrompt string, fn func(w io.Writer) error) error

	// FlushMemory forces any pending memory state to be processed and persisted.
	// It returns the number of processed items and any error encountered.
	FlushMemory(ctx context.Context) (int, error)
}

// indexStore defines the persistence layer used by the AI client.
//
// It is responsible for storing indexed items (chunks + embeddings),
// maintaining metadata such as summaries, and managing the lifecycle
// of the underlying storage (e.g. file-backed DB, in-memory store).
type indexStore interface {
	// ValidateIndex ensures the underlying index is usable.
	//
	// It should verify that required structures exist and are compatible
	// with the current schema/version. It does not modify state.
	ValidateIndex() error

	// ResetIndex clears all indexed data.
	//
	// This removes all stored items and summaries, effectively returning
	// the store to an empty state.
	ResetIndex() error

	// DBPath returns the location of the underlying storage.
	//
	// This is typically a filesystem path and is useful for debugging,
	// logging, or user-facing output.
	DBPath() string

	// Save persists any in-memory state to durable storage.
	//
	// For fully persistent stores this may be a no-op.
	Save() error

	// Close releases any resources held by the store.
	//
	// After calling Close, the store should not be used.
	Close() error

	// ItemCount returns the number of indexed items currently stored.
	//
	// An item typically represents a chunk of content with its embedding.
	ItemCount() int

	// AddItem stores a single indexed item.
	//
	// It is expected to be called during indexing and may be invoked
	// concurrently depending on the implementation.
	AddItem(i store.Item)

	// AddSummary stores a high-level summary of the indexed project.
	//
	// This is typically called once after indexing completes.
	AddSummary(summary string)
}

// Client is the main entry point for interacting with the indexing
// and querying system.
type Client struct {
	cfg    *Config
	engine *engine.Engine

	embedder embedder
	store    indexStore

	memStore memStore
	writer   io.Writer
}

// NewClient initializes a new Client using the provided configuration.
//
// It sets up the underlying LLM client and store. The returned
// Client is ready to be used for indexing and querying.
//
// writer is used for user-facing output such as progress logs or generated results.
func NewClient(cfg *Config, writer io.Writer) (*Client, error) {
	if err := cfg.applyDefaultsAndValidate(); err != nil {
		return nil, err
	}

	llmClient, err := llm.NewClient(cfg.Model, cfg.EmbeddingModel)
	if err != nil {
		return nil, err
	}

	store, err := store.NewStore(cfg.ProjectRoot, cfg.StoreDir, cfg.DBName, cfg.computeConfigHash())
	if err != nil {
		return nil, err
	}

	memory := memory.NewService(store, llmClient)
	engine := engine.NewEngine(llmClient, store, memory, writer)

	return &Client{
		cfg:      cfg,
		engine:   engine,
		embedder: llmClient,
		store:    store,
		memStore: memory,
		writer:   writer,
	}, nil
}

// Close releases any resources held by the Client.
//
// It should be called when the Client is no longer needed.
func (c *Client) Close() error {
	return c.store.Close()
}

// ProjectRoot returns the root directory of the project being indexed.
//
// This is the resolved path used by the Client during indexing.
func (c *Client) ProjectRoot() string {
	return c.cfg.ProjectRoot
}

// StoreLocation returns the full path to the underlying database.
//
// This path is determined during client initialization and points to the
// persisted index file used for semantic search operations.
func (c *Client) StoreLocation() string {
	return c.store.DBPath()
}

// IndexStatus reports the current state of the index for the client.
//
// It maps internal storage validation errors into a stable, user-facing status.
//
// The returned status will be one of:
//   - IndexOK: index exists and is valid
//   - IndexMissing: no index has been created yet
//   - IndexStale: index exists but must be rebuilt due to configuration or version changes
//
// An error is returned only for unexpected failures (e.g. storage access issues).
func (c *Client) IndexStatus(ctx context.Context) (IndexStatus, error) {
	err := c.store.ValidateIndex()
	switch {
	case err == nil:
		return IndexOK, nil
	case errors.Is(err, store.ErrNotIndexed):
		return IndexMissing, nil
	case errors.Is(err, store.ErrReindexRequired):
		return IndexStale, nil
	default:
		return IndexOK, err
	}
}

// EnsureIndex ensures that a valid index is available.
//
// If the index is missing, it triggers an initial indexing.
// If the index is stale, it performs a full reindex.
// If the index is already valid, it does nothing.
//
// It returns an error only if the indexing or validation process fails.
func (c *Client) EnsureIndex(ctx context.Context, onProgress func(IndexProgress), onComplete func(IndexResult)) error {
	status, err := c.IndexStatus(ctx)
	if err != nil {
		return err
	}

	switch status {
	case IndexMissing, IndexStale:
		return c.Index(ctx, onProgress, onComplete)
	default:
		return nil
	}
}

// IndexProgress represents the current progress of an indexing operation.
//
// It is emitted during Client.Index via the optional progress callback.
// Done indicates the number of files processed so far, out of Total.
// File is the current file being indexed, and Size is its size in bytes.
type IndexProgress struct {
	Done  int64
	Total int64
	File  string
	Size  int64
}

// IndexResult summarizes the outcome of a completed indexing operation.
//
// It is passed to the onComplete callback of Client.Index and provides
// high-level information about the indexing process, such as the number
// of chunks stored, the database location, and the total elapsed time.
type IndexResult struct {
	Chunks        int
	StoreLocation string
	Elapsed       time.Duration
}

// Index builds a fresh index for the configured project.
//
// It clears any existing index, scans the project, generates embeddings,
// and persists the results to the store.
//
// Progress updates are reported via the onProgress callback (if provided).
// When indexing completes successfully, onComplete is invoked with a summary
// of the operation (if provided).
//
// This method does not control how progress or results are rendered;
// that responsibility is left to the caller.
func (c *Client) Index(ctx context.Context, onProgress func(IndexProgress), onComplete func(IndexResult)) error {
	if err := c.store.ResetIndex(); err != nil {
		return err
	}

	c.cfg.Logger.Info("Indexing project", slog.String("root", c.ProjectRoot()))
	start := time.Now()

	opts := indexer.IndexOptions{
		ProjectRoot:    c.ProjectRoot(),
		IgnorePatterns: c.cfg.IgnorePatterns,
		Extensions:     c.cfg.extensions,
		MaxWorkers:     c.cfg.IndexMaxWorkers,
	}
	if err := indexer.Build(ctx, c.embedder, c.store, c.cfg.Logger, opts, func(p indexer.Progress) {
		if onProgress != nil {
			onProgress(IndexProgress{
				Done:  p.Done,
				Total: p.Total,
				File:  p.File,
				Size:  p.Size,
			})
		}
	}); err != nil {
		return err
	}

	if err := c.store.Save(); err != nil {
		return err
	}

	elapsed := time.Since(start)
	dbPath := c.store.DBPath()
	nChunks := c.store.ItemCount()
	if onComplete != nil {
		onComplete(IndexResult{
			Chunks:        nChunks,
			StoreLocation: dbPath,
			Elapsed:       elapsed,
		})
	}

	c.cfg.Logger.Info("Indexing completed",
		slog.Int("chunks", nChunks),
		slog.String("store_location", dbPath),
		slog.Duration("elapsed_time", elapsed),
	)
	return nil
}
