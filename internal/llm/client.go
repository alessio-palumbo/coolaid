package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)
const (
	responseHeaderTimeout  = 60 * time.Second
	idleConnTimeout        = 90 * time.Second
	generateContextTimeout = 60 * time.Second
	ollamaURL              = "http://localhost:11434"
)

var (
	tagsEndpoint       = "/api/tags"
	chatEndpoint       = "/api/chat"
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

type Message struct {
	Role    Role
	Content string
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

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
}

type chatMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type tagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// NewClient creates a new Ollama client configured with the given LLM and embedding models.
//
// It validates connectivity to the Ollama server via a ping before returning the client.
// The underlying HTTP client is configured with transport-level timeouts to guard against
// stalled connections, while request-level timeouts are controlled via context.
//
// The returned client is safe for concurrent use.
func NewClient(model, embeddingModel string) (*Client, error) {
	client := &Client{
		client: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: responseHeaderTimeout,
				IdleConnTimeout:       idleConnTimeout,
			},
		},
		BaseURL:         ollamaURL,
		LLMModel:        model,
		EmbeddingsModel: embeddingModel,
	}

	if err := client.ping(); err != nil {
		return nil, err
	}
	return client, nil
}

// Generate performs a non-streaming LLM completion request.
//
// It sends the prompt to the configured model and blocks until the full response is received.
// The request is bounded by the provided context for timeout/cancellation control.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, generateContextTimeout)
	defer cancel()

	resp, err := c.doGenerateRequest(ctx, prompt, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var chunk generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&chunk); err != nil {
		return "", err
	}
	return chunk.Response, nil
}

// GenerateStream performs a streaming LLM completion request.
//
// Tokens are written incrementally to the provided writer as they are received.
// The call respects context cancellation and will stop immediately if the context is done.
// Intended for real-time CLI output or interactive sessions.
func (c *Client) GenerateStream(ctx context.Context, prompt string, writer io.Writer) error {
	resp, err := c.doGenerateRequest(ctx, prompt, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var chunk generateResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
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

// ChatStream performs a streaming chat completion request.
//
// It streams assistant tokens to the provided writer in real time
// while also accumulating the full response for history tracking.
//
// The request respects context cancellation and will stop immediately
// if the context is done (useful for user interruption or shutdown).
func (c *Client) ChatStream(ctx context.Context, messages []Message, writer io.Writer) (string, error) {
	reqBody := chatRequest{
		Model:    c.LLMModel,
		Messages: toChatMessages(messages),
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := c.post(ctx, chatEndpoint, data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(resp.Body)
	var fullResponse strings.Builder

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		var chunk chatResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		if content := chunk.Message.Content; content != "" {
			// stream to user
			_, _ = writer.Write([]byte(content))
			// capture for history
			fullResponse.WriteString(content)
		}

		if chunk.Done {
			break
		}
	}

	return fullResponse.String(), nil
}

// doGenerateRequest prepares and executes a generate request against the LLM.
//
// It builds the request payload and performs the HTTP call using the provided
// context for timeout and cancellation control. The caller is responsible for
// consuming and closing the response body.
//
// Non-200 responses are treated as errors and the response body is closed
// before returning.
func (c *Client) doGenerateRequest(ctx context.Context, prompt string, stream bool) (*http.Response, error) {
	reqBody := generateRequest{
		Prompt: prompt,
		Model:  c.LLMModel,
		Stream: stream,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.post(ctx, generateEndpoint, data)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("llm returned status %d", resp.StatusCode)
	}

	return resp, nil
}

// post performs a POST request to the given endpoint and returns the response.
// It assume Content-Type is application/json.
func (c *Client) post(ctx context.Context, endpoint string, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentTypeJSON)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}

// ping checks that ollama server is running and that the configured models are available.
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

func toChatMessages(msgs []Message) []chatMessage {
	out := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = chatMessage(m)
	}
	return out
}
