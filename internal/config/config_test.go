package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrCreate(t *testing.T) {
	testCases := map[string]struct {
		setup    func(home string) error
		wantToml string
		wantCfg  func(home string) *Config
	}{
		"default config (create)": {
			setup: func(home string) error {
				return nil
			},
			wantToml: strings.TrimSpace(`
[llm]
model = 'llama3'
embedding_model = 'nomic-embed-text'
temperature = 0.2

[index]
include_extensions = []
ignore_patterns = []
`),
			wantCfg: func(home string) *Config {
				cfg := &Config{}
				cfg.LLM.Model = defaultLLMModel
				cfg.LLM.EmbeddingModel = defaultEmbeddingModel
				cfg.LLM.Temperature = defaultTemperature

				cfg.Index.IncludeExtensions = []string{}
				cfg.Index.IgnorePatterns = []string{}

				cfg.Extensions = make(map[string]struct{})
				for _, e := range defaultExtensions {
					cfg.Extensions[e] = struct{}{}
				}

				cfg.ProjectRoot = configDirName
				cfg.StoreDir = filepath.Join(home, configDirName, storeDirName)
				return cfg
			},
		},
		"custom config (load user input)": {
			setup: func(home string) error {
				configDir := filepath.Join(home, configDirName)
				if err := os.MkdirAll(configDir, 0755); err != nil {
					return err
				}

				configPath := filepath.Join(configDir, configFileName)
				custom := strings.TrimSpace(`
[llm]
model = 'custom-model'
embedding_model = 'custom-embed'
temperature = 0.5

[index]
include_extensions = ['.rs']
ignore_patterns = ['node_modules/']
`)
				return os.WriteFile(configPath, []byte(custom), 0644)
			},
			wantToml: strings.TrimSpace(`
[llm]
model = 'custom-model'
embedding_model = 'custom-embed'
temperature = 0.5

[index]
include_extensions = ['.rs']
ignore_patterns = ['node_modules/']
`),
			wantCfg: func(home string) *Config {
				cfg := &Config{}
				cfg.LLM.Model = "custom-model"
				cfg.LLM.EmbeddingModel = "custom-embed"
				cfg.LLM.Temperature = 0.5

				cfg.Index.IncludeExtensions = []string{".rs"}
				cfg.Index.IgnorePatterns = []string{"node_modules/"}

				cfg.Extensions = make(map[string]struct{})
				for _, e := range defaultExtensions {
					cfg.Extensions[e] = struct{}{}
				}
				cfg.Extensions[".rs"] = struct{}{}

				cfg.ProjectRoot = configDirName
				cfg.StoreDir = filepath.Join(home, configDirName, storeDirName)
				return cfg
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("HOME", tmpDir)

			if tc.setup != nil {
				require.NoError(t, tc.setup(tmpDir))
			}

			cfg, err := LoadOrCreate()
			require.NoError(t, err)

			// ---- Check TOML output ----
			configPath := filepath.Join(tmpDir, configDirName, configFileName)
			b, err := os.ReadFile(configPath)
			require.NoError(t, err)

			gotToml := strings.TrimSpace(string(b))
			assert.Equal(t, tc.wantToml, gotToml)

			// ---- Check Config struct ----
			want := tc.wantCfg(tmpDir)

			assert.Equal(t, want.LLM, cfg.LLM)
			assert.Equal(t, want.Index, cfg.Index)
			assert.Equal(t, want.StoreDir, cfg.StoreDir)

			// compare maps safely
			assert.Equal(t, want.Extensions, cfg.Extensions)
		})
	}
}
