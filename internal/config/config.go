package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml"
)

const (
	configDirName  = ".ai"
	indexesDirName = "indexes"
	configFileName = "config.toml"

	defaultLLMModel       = "llama3"
	defaultEmbeddingModel = "nomic-embed-text"
)

type Config struct {
	ConfigDir  string `toml:"-"`
	IndexesDir string `toml:"-"`

	LLM struct {
		Model          string  `toml:"model"`
		EmbeddingModel string  `toml:"embedding_model"`
		Temperature    float64 `toml:"temperature"`
	} `toml:"llm"`
}

func LoadOrCreate() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(home, configDirName)
	indexesDir := filepath.Join(configDir, indexesDirName)
	configPath := filepath.Join(configDir, configFileName)

	// Ensure directories exist
	if err := os.MkdirAll(indexesDir, 0755); err != nil {
		return nil, err
	}

	// Write default config if missing
	if !fileExists(configPath) {
		if err := writeDefaultConfig(configPath); err != nil {
			return nil, err
		}
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	cfg.ConfigDir = configDir
	cfg.IndexesDir = indexesDir
	return cfg, nil
}

func writeDefaultConfig(path string) error {
	cfg := Config{}
	cfg.LLM.Model = defaultLLMModel
	cfg.LLM.EmbeddingModel = defaultEmbeddingModel
	cfg.LLM.Temperature = 0.2

	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal defaults: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

func loadConfig(cfgPath string) (*Config, error) {
	tree, err := toml.LoadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := tree.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
