package ai

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

type IndexStatus int

const (
	IndexOK IndexStatus = iota
	IndexMissing
	IndexStale
)

const defaulDBName = "index"

type Config struct {
	// ProjectRoot is the directory to scan and index.
	// Typically the root of your repository.
	ProjectRoot string
	StoreDir    string
	DBName      string

	Model          string
	EmbeddingModel string
	Temperature    float64

	IncludeExtensions []string
	IgnorePatterns    []string
}

type Client struct {
	cfg   *config.Config
	llm   *llm.Client
	store *vector.Store

	writer io.Writer
}

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

	return &Client{
		cfg:    cfg,
		llm:    llmClient,
		store:  store,
		writer: writer,
	}, nil
}

func (c *Client) Close() error {
	return c.store.Close()
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
func (c *Client) EnsureIndex(ctx context.Context) error {
	status, err := c.IndexStatus(ctx)
	if err != nil {
		return err
	}

	switch status {
	case IndexMissing, IndexStale:
		return c.Index(ctx)
	default:
		return nil
	}
}

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
		c.DBName = defaulDBName
	}

	if err := c.Apply(); err != nil {
		return nil, err
	}
	return &c, nil
}
