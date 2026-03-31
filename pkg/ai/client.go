package ai

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
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

const defaultDBName = "index"

// Config defines the configuration used to initialize a Client.
//
// Defaults are applied for certain fields when left empty:
//   - StoreDir defaults to "<ProjectRoot>/.ai"
//   - DBName defaults to "index"
//
// It controls how the project is indexed and how the LLM is used
// for querying and code generation.
type Config struct {
	// ProjectRoot is the root directory of the project to scan and index.
	// This is typically the root of your repository.
	ProjectRoot string

	// StoreDir is the directory where the index database will be stored.
	// If empty, a default location may be used.
	StoreDir string

	// DBName is the name of the index database.
	// Defaults to "index" if not specified.
	DBName string

	// Model is the LLM used for generation tasks (e.g. answering queries,
	// generating code, or tests).
	Model string

	// EmbeddingModel is the model used to generate vector embeddings
	// for indexed content.
	EmbeddingModel string

	// Temperature controls the randomness of the LLM output.
	// Lower values produce more deterministic results.
	Temperature float64

	// IncludeExtensions restricts indexing to files with the given extensions
	// (e.g. [".go", ".py"]). If empty, all supported files may be included.
	IncludeExtensions []string

	// IgnorePatterns defines glob patterns for files or directories to exclude
	// from indexing (e.g. ["vendor/**", "node_modules/**"]).
	IgnorePatterns []string

	Logger *slog.Logger
}

// Client is the main entry point for interacting with the indexing
// and querying system.
type Client struct {
	cfg   *config.Config
	llm   *llm.Client
	store *vector.Store

	writer io.Writer
	logger *slog.Logger
}

// NewClient initializes a new Client using the provided configuration.
//
// It sets up the underlying LLM client and vector store. The returned
// Client is ready to be used for indexing and querying.
//
// writer is used for user-facing output such as progress logs or generated results.
func NewClient(userCfg *Config, writer io.Writer) (*Client, error) {
	cfg, err := parseConfig(userCfg)
	if err != nil {
		return nil, err
	}

	llmClient, err := llm.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	store, err := vector.NewStore(cfg)
	if err != nil {
		return nil, err
	}

	if userCfg.Logger == nil {
		userCfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Client{
		cfg:    cfg,
		llm:    llmClient,
		store:  store,
		writer: writer,
		logger: userCfg.Logger,
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

// StoreLocation returns the full path to the underlying vector store database.
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
	case errors.Is(err, vector.ErrNotIndexed):
		return IndexMissing, nil
	case errors.Is(err, vector.ErrReindexRequired):
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

// parseConfig validates the user-provided Config and applies defaults.
//
// The following defaults are applied when fields are empty:
//   - StoreDir: "<ProjectRoot>/.ai"
//   - DBName:   "index"
//
// It also derives any internal configuration required by the system.
func parseConfig(userCfg *Config) (*config.Config, error) {
	if userCfg == nil {
		return nil, fmt.Errorf("config is required: provide at least Model, EmbeddingModel, and ProjectRoot")
	}

	var c config.Config
	c.LLM.Model = userCfg.Model
	c.LLM.EmbeddingModel = userCfg.EmbeddingModel
	c.LLM.Temperature = userCfg.Temperature
	c.Index.IncludeExtensions = userCfg.IncludeExtensions
	c.Index.IgnorePatterns = userCfg.IgnorePatterns

	c.ProjectRoot = userCfg.ProjectRoot
	if userCfg.StoreDir == "" {
		userCfg.StoreDir = filepath.Join(userCfg.ProjectRoot, ".ai")
	}
	c.StoreDir = userCfg.StoreDir
	c.DBName = userCfg.DBName
	if c.DBName == "" {
		c.DBName = defaultDBName
	}

	if err := c.Apply(); err != nil {
		return nil, err
	}
	return &c, nil
}
