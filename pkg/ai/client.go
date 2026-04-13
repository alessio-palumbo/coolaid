package ai

import (
	"context"
	"coolaid/internal/llm"
	"coolaid/internal/llm/memory"
	"coolaid/internal/store"
	"errors"
	"io"
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

// Memory defines the interface for asynchronous project memory management.
//
// It accepts interaction inputs for background processing and supports
// graceful shutdown of any internal workers.
type Memory interface {
	Capture(w io.Writer, in memory.Input, fn func(w io.Writer) error) error
	Close(ctx context.Context)
}

// Client is the main entry point for interacting with the indexing
// and querying system.
type Client struct {
	cfg   *Config
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

	var memSvc Memory
	if cfg.DisableMemory {
		memSvc = memory.NewNoop()
	} else {
		memSvc = memory.NewService(store, llmClient)
	}

	return &Client{
		cfg:    cfg,
		llm:    llmClient,
		store:  store,
		memory: memSvc,
		writer: writer,
	}, nil
}

// Close releases any resources held by the Client.
//
// It should be called when the Client is no longer needed.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.memory.Close(ctx)

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
