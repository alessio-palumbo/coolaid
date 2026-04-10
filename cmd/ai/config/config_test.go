package config

import (
	"coolaid/pkg/ai"
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
		wantCfg  func(home string) *ai.Config
	}{
		"default config (create)": {
			setup: func(home string) error {
				return nil
			},
			wantToml: strings.TrimSpace(`
[llm]
model = 'llama3.1'
embedding_model = 'nomic-embed-text'
temperature = 0.2

[index]
include_extensions = []
ignore_patterns = []
`),
			wantCfg: func(home string) *ai.Config {
				cfg := &ai.Config{}
				cfg.ProjectRoot = home
				cfg.StoreDir = filepath.Join(home, configDirName, storeDirName)

				cfg.Model = defaultLLMModel
				cfg.EmbeddingModel = defaultEmbeddingModel
				cfg.Temperature = defaultTemperature

				cfg.IncludeExtensions = []string{}
				cfg.IgnorePatterns = []string{}
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
			wantCfg: func(home string) *ai.Config {
				cfg := &ai.Config{}
				cfg.ProjectRoot = home
				cfg.StoreDir = filepath.Join(home, configDirName, storeDirName)

				cfg.Model = "custom-model"
				cfg.EmbeddingModel = "custom-embed"
				cfg.Temperature = 0.5

				cfg.IncludeExtensions = []string{".rs"}
				cfg.IgnorePatterns = []string{"node_modules/"}
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

			// Skip ProjectRoot check validated by projectRoot func
			assert.NotEmpty(t, cfg.ProjectRoot)
			assert.Equal(t, want.StoreDir, cfg.StoreDir)
			assert.Equal(t, want.Model, cfg.Model)
			assert.Equal(t, want.EmbeddingModel, cfg.EmbeddingModel)
			assert.Equal(t, want.Temperature, cfg.Temperature)
			assert.Equal(t, want.IncludeExtensions, cfg.IncludeExtensions)
			assert.Equal(t, want.IgnorePatterns, cfg.IgnorePatterns)
		})
	}
}

func Test_projectRoot(t *testing.T) {
	root := t.TempDir()

	// create nested structure
	nested := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0755))

	// create .git at root
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0755))

	// change working dir
	old, _ := os.Getwd()
	defer os.Chdir(old)
	require.NoError(t, os.Chdir(nested))

	got, err := projectRoot()
	require.NoError(t, err)

	expected, err := filepath.EvalSymlinks(root)
	require.NoError(t, err)

	actual, err := filepath.EvalSymlinks(got)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}
