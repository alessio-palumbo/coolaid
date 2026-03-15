package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultHTTPTimeout = 60 * time.Second
	ollamaURL          = "http://localhost:11434"
	llmModel           = "llama3"
	embeddingsModel    = "nomic-embed-text"
)

var (
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

func NewClient() *Client {
	return &Client{
		client:          &http.Client{Timeout: defaultHTTPTimeout},
		BaseURL:         ollamaURL,
		LLMModel:        llmModel,
		EmbeddingsModel: embeddingsModel,
	}
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
