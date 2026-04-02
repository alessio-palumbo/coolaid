package config

import (
	"ai-cli/pkg/ai"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

type config struct {
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

func LoadOrCreate() (*ai.Config, error) {
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	return &ai.Config{
		ProjectRoot:       projectRoot,
		StoreDir:          storeDir,
		DBName:            projectRootHash(projectRoot),
		Model:             c.LLM.Model,
		EmbeddingModel:    c.LLM.EmbeddingModel,
		Temperature:       c.LLM.Temperature,
		IncludeExtensions: c.Index.IncludeExtensions,
		IgnorePatterns:    c.Index.IgnorePatterns,
		Logger:            logger,
	}, nil
}

func writeDefaultConfig(path string) error {
	c := config{}
	c.LLM.Model = defaultLLMModel
	c.LLM.EmbeddingModel = defaultEmbeddingModel
	c.LLM.Temperature = defaultTemperature

	b, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal defaults: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

func loadConfig(cfgPath string) (*config, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	var cfg config
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
			// reached filesystem root
			return dir, nil
		}

		dir = parent
	}
}

func projectRootHash(projectRoot string) string {
	hash := sha1.Sum([]byte(projectRoot))
	return hex.EncodeToString(hash[:8])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
