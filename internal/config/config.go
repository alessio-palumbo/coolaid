package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	configDirName  = ".ai"
	storeDirName   = "indexes"
	configFileName = "config.toml"

	defaultLLMModel       = "llama3"
	defaultEmbeddingModel = "nomic-embed-text"
	defaultTemperature    = 0.2
)

const maxFileSize = 200 * 1024 // 200 KB

var defaultExtensions = []string{
	// core code
	".go", ".py", ".js", ".ts", ".rb", ".rs",

	// configs (HIGH VALUE in RAG)
	".json", ".yaml", ".yml", ".toml",

	// scripting
	".sh", ".bash", ".zsh",

	// docs
	".md", ".txt",

	// infra
	".tf", ".tfvars",
}

type Config struct {
	StoreDir    string              `toml:"-"`
	ProjectRoot string              `toml:"-"`
	DBName      string              `toml:"-"`
	Extensions  map[string]struct{} `toml:"-"`

	LLM struct {
		Model          string  `toml:"model"`
		EmbeddingModel string  `toml:"embedding_model"`
		Temperature    float64 `toml:"temperature"`
	} `toml:"llm"`

	Index struct {
		IncludeExtensions []string `toml:"include_extensions"`
		IgnorePatterns    []string `toml:"ignore_patterns"`
	} `toml:"index"`
}

func LoadOrCreate() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(home, configDirName)
	storeDir := filepath.Join(configDir, storeDirName)
	configPath := filepath.Join(configDir, configFileName)

	// Ensure directories exist
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, err
	}

	// Write default config if missing
	if !fileExists(configPath) {
		if err := writeDefaultConfig(configPath); err != nil {
			return nil, err
		}
	}

	c, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	projectRoot, err := projectRoot()
	if err != nil {
		return nil, err
	}
	c.ProjectRoot = projectRoot
	c.StoreDir = storeDir

	if err := c.Apply(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Apply() error {
	c.resolveExtensions()
	return c.validate()
}

func (c *Config) resolveExtensions() {
	c.Extensions = make(map[string]struct{}, len(defaultExtensions))
	for _, e := range defaultExtensions {
		c.Extensions[e] = struct{}{}
	}
	for _, e := range c.Index.IncludeExtensions {
		if ext := normalizeExt(e); ext != "" {
			c.Extensions[ext] = struct{}{}
		}
	}
}

func (c *Config) validate() error {
	if c.LLM.Model == "" {
		return fmt.Errorf("llm model is required")
	}
	if c.LLM.EmbeddingModel == "" {
		return fmt.Errorf("embedding model is required")
	}
	if c.LLM.Temperature < 0 || c.LLM.Temperature > 1 {
		return fmt.Errorf("temperature must be between 0 and 1")
	}
	if c.ProjectRoot == "" {
		return fmt.Errorf("project root is required")
	}
	if c.StoreDir == "" {
		return fmt.Errorf("store directory is required to store vector DBs")
	}
	return nil
}

func writeDefaultConfig(path string) error {
	c := Config{}
	c.LLM.Model = defaultLLMModel
	c.LLM.EmbeddingModel = defaultEmbeddingModel
	c.LLM.Temperature = defaultTemperature

	b, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal defaults: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

func loadConfig(cfgPath string) (*Config, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		git := filepath.Join(dir, ".git")
		if _, err := os.Stat(git); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return dir, nil // reached filesystem root
		}

		dir = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func normalizeExt(e string) string {
	e = strings.TrimSpace(e)
	if e == "" {
		return ""
	}

	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	return strings.ToLower(e)
}
