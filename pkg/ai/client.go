package ai

import (
	"ai-cli/internal/config"
	"ai-cli/internal/llm"
	"ai-cli/internal/vector"
	"fmt"
	"io"
	"path/filepath"
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
