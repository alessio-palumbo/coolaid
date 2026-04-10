// This package handles configuration loading and runtime enrichment.
// Static configuration is loaded from TOML, while dynamic values such as
// project root and database name are derived at runtime.
package config

import (
	"coolaid/pkg/ai"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	configDirName  = ".ai"
	storeDirName   = "indexes"
	configFileName = "config.toml"

	defaultLLMModel       = "llama3.1"
	defaultEmbeddingModel = "nomic-embed-text"
	defaultTemperature    = 0.2
)

var dbNameValidChars = regexp.MustCompile(`[^a-z0-9._-]+`)

type config struct {
	// Runtime-only fields (not loaded from TOML)
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

// LoadOrCreate initializes the application configuration.
// It ensures required directories exist, creates a default config file if missing,
// loads user configuration, and enriches it with runtime-derived values
// such as project root and database name.
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
		DBName:            dbName(projectRoot),
		Model:             c.LLM.Model,
		EmbeddingModel:    c.LLM.EmbeddingModel,
		Temperature:       c.LLM.Temperature,
		IncludeExtensions: c.Index.IncludeExtensions,
		IgnorePatterns:    c.Index.IgnorePatterns,
		Logger:            logger,
	}, nil
}

// writeDefaultConfig writes a minimal default configuration file
// with predefined LLM and embedding settings.
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

// projectRoot walks up from the current working directory to find the nearest
// directory containing a .git folder. If none is found, it returns the current
// working directory.
//
// This ensures the CLI operates at the repository level rather than per subdirectory.
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

// projectRootHash returns a short, deterministic hash of the project root path.
// This is used to avoid collisions between projects with the same name.
func projectRootHash(projectRoot string) string {
	hash := sha1.Sum([]byte(projectRoot))
	return hex.EncodeToString(hash[:])[:8]
}

// dbName generates a human-readable yet collision-resistant database name
// based on the project root directory.
//
// Format: <sanitized-project-name>_<short-hash>
func dbName(root string) string {
	return fmt.Sprintf("%s_%s", sanitize(filepath.Base(root)), projectRootHash(root))
}

// sanitize normalizes a string to be filesystem-safe by:
// - converting to lowercase
// - replacing invalid characters with underscores
// - trimming leading/trailing underscores
func sanitize(s string) string {
	s = strings.ToLower(s)
	s = dbNameValidChars.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// fileExists returns true if the given path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
