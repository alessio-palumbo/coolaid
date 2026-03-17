package llm

import (
	"ai-cli/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout = 60 * time.Second
	ollamaURL          = "http://localhost:11434"
)

var (
	tagsEndpoint       = "/api/tags"
	generateEndpoint   = "/api/generate"
	embeddingsEndpoint = "/api/embeddings"
	contentTypeJSON    = "application/json"
)

type Client struct {
	client          *http.Client
	BaseURL         string
	LLMModel        string
	EmbeddingsModel string
}

type generateRequest struct {
	Prompt string `json:"prompt"`
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		client:          &http.Client{Timeout: defaultHTTPTimeout},
		BaseURL:         ollamaURL,
		LLMModel:        cfg.LLM.Model,
		EmbeddingsModel: cfg.LLM.EmbeddingModel,
	}

	if err := client.ping(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) ping() error {
	resp, err := c.client.Get(c.BaseURL + tagsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to contact Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to decode Ollama tags: %w", err)
	}

	available := map[string]bool{}
	for _, m := range tags.Models {
		available[normalizeModel(m.Name)] = true
	}
	if !available[c.LLMModel] {
		return fmt.Errorf("LLM model '%s' not installed", c.LLMModel)
	}
	if !available[c.EmbeddingsModel] {
		return fmt.Errorf("embedding model '%s' not installed", c.EmbeddingsModel)
	}

	return nil
}

func normalizeModel(name string) string {
	if i := strings.Index(name, ":"); i != -1 {
		return name[:i]
	}
	return name
}
func (t *tagsResponse) contains(s string) bool {
	for _, v := range t.Models {
		if v.Name == s {
			return true
		}
	}
	return false
}

func (c *Client) GenerateStream(prompt string, writer io.Writer) error {
	reqBody := generateRequest{
		Prompt: prompt,
		Model:  c.LLMModel,
		Stream: true,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := c.post(generateEndpoint, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	for {
		var chunk generateResponse
		err := decoder.Decode(&chunk)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if chunk.Response != "" {
			_, _ = writer.Write([]byte(chunk.Response))
		}
		if chunk.Done {
			break
		}
	}

	return nil
}

func (c *Client) post(endpoint string, data []byte) (*http.Response, error) {
	resp, err := c.client.Post(c.BaseURL+endpoint, contentTypeJSON, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d", resp.StatusCode)
	}
	return resp, err
}
