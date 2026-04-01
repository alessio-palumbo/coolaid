package ai

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
)

const (
	defaultDBName   = "index"
	defaultStoreDir = ".ai"
)

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

// Config defines the configuration used to initialize a Client.
//
// It controls how the project is indexed and how the LLM is used
// for querying and code generation.
//
// Defaults are applied for certain fields when left empty:
//   - StoreDir defaults to "<ProjectRoot>/.ai"
//   - DBName defaults to "index"
//   - Logger defaults to a no-op logger
//
// IncludeExtensions augments the default set of supported file types.
// Extensions are normalized (lowercased, with a leading dot).
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

	extensions map[string]struct{}
}

// applyDefaultsAndValidate applies default values, resolves derived fields,
// and validates the configuration.
//
// It must be called before using the Config to ensure all required fields
// are set and internal state is properly initialized.
func (c *Config) applyDefaultsAndValidate() error {
	if c == nil {
		return fmt.Errorf("config is required: provide at least Model, EmbeddingModel, and ProjectRoot")
	}

	if c.StoreDir == "" {
		c.StoreDir = filepath.Join(c.ProjectRoot, defaultStoreDir)
	}
	if c.DBName == "" {
		c.DBName = defaultDBName
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	c.resolveExtensions()
	return c.validate()
}

// resolveExtensions builds the effective set of file extensions to index.
//
// It combines a default set of supported extensions with any user-provided
// IncludeExtensions. All extensions are normalized.
func (c *Config) resolveExtensions() {
	c.extensions = make(map[string]struct{}, len(defaultExtensions))
	for _, e := range defaultExtensions {
		c.extensions[e] = struct{}{}
	}
	for _, e := range c.IncludeExtensions {
		if ext := normalizeExt(e); ext != "" {
			c.extensions[ext] = struct{}{}
		}
	}
}

// validate checks that all required configuration fields are set
// and contain valid values.
func (c *Config) validate() error {
	if c.Model == "" {
		return fmt.Errorf("llm model is required")
	}
	if c.EmbeddingModel == "" {
		return fmt.Errorf("embedding model is required")
	}
	if c.Temperature < 0 || c.Temperature > 1 {
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

// computeConfigHash returns a hash representing the indexing-relevant
// configuration.
//
// It is used to derive a stable identifier for the index, based on
// IncludeExtensions and IgnorePatterns. Changes to these fields will
// result in a different hash.
func (c *Config) computeConfigHash() string {
	h := sha1.New()
	for _, ext := range c.IncludeExtensions {
		h.Write([]byte(ext))
	}
	for _, p := range c.IgnorePatterns {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// normalizeExt normalizes a file extension by trimming whitespace,
// ensuring a leading dot, and converting it to lowercase.
// Returns an empty string if the input is empty.
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
