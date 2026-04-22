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

// LLM defines the language model operations used by task execution,
// supporting both buffered and streaming generation, as well as chat as embedding.
type LLM interface {
	Generate(ctx context.Context, prompt string) (string, error)
	GenerateStream(ctx context.Context, prompt string, writer io.Writer) error
	ChatStream(ctx context.Context, messages []llm.Message, writer io.Writer) (string, error)
	Embed(ctx context.Context, text string) ([]float64, error)
}

// Memory defines the interface for asynchronous project memory management.
//
// It accepts interaction inputs for background processing and supports
// graceful shutdown of any internal workers.
type Memory interface {
	Capture(w io.Writer, userPrompt string, fn func(w io.Writer) error) error
	FlushMemory(ctx context.Context) (int, error)
}

type Store interface {
	Index(ctx context.Context, onProgress func(IndexProgress), onComplete func(IndexResult)) error
	ValidateIndex() error
	ResetIndex() (err error)
	Save() (err error)
}

// Client is the main entry point for interacting with the indexing
// and querying system.
type Client struct {
	cfg    *Config
	engine *engine.Engine

	llm   *llm.Client
	store *store.Store

	memory Memory
	writer io.Writer
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
		cfg:    cfg,
		engine: engine,
		llm:    llmClient,
		store:  store,
		memory: memory,
		writer: writer,
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
	return c.store.DBPath
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
	if err := indexer.Build(ctx, c.llm, c.store, c.cfg.Logger, opts, func(p indexer.Progress) {
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
	if onComplete != nil {
		onComplete(IndexResult{
			Chunks:        len(c.store.Items),
			StoreLocation: c.store.DBPath,
			Elapsed:       elapsed,
		})
	}

	c.cfg.Logger.Info("Indexing completed",
		slog.Int("chunks", len(c.store.Items)),
		slog.String("store_location", c.store.DBPath),
		slog.Duration("elapsed_time", elapsed),
	)
	return nil
}
