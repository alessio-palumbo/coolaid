package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	attemptTimeout  = 10 * time.Second
	backoffPeriod   = 200 * time.Millisecond
	embedMaxRetries = 3
)

var (
	errRetryableNetwork = errors.New("retryable: network error")
	errRetryableOllama  = errors.New("retryable: ollama unavailable")
)

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Embed generates an embedding for the given text.
//
// The request is bounded by the provided context for timeout and
// cancellation. Each attempt may use a derived context with its own
// timeout.
//
// It retries on transient failures (network errors and 5xx responses)
// using a simple backoff strategy. Non-retryable errors (e.g. 4xx)
// are returned immediately.
func (c *Client) Embed(ctx context.Context, text string) ([]float64, error) {
	reqBody := embeddingRequest{
		Prompt: text,
		Model:  c.EmbeddingsModel,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	for i := range embedMaxRetries {
		res, err := c.embedAttempt(ctx, data)
		if err == nil {
			return res.Embedding, nil
		}

		switch err {
		case errRetryableNetwork, errRetryableOllama:
			time.Sleep(time.Duration(i+1) * backoffPeriod)
			continue
		default:
			return nil, err
		}
	}

	return nil, fmt.Errorf("embed failed after retries")
}

// embedAttempt performs a single embedding request with a per-attempt timeout.
// It returns the decoded response or an error without applying retry logic.
func (c *Client) embedAttempt(ctx context.Context, data []byte) (*embeddingResponse, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
	defer cancel()

	resp, err := c.post(attemptCtx, embeddingsEndpoint, data)
	if err != nil {
		return nil, errRetryableNetwork
	}
	defer resp.Body.Close()

	// Retryable server-side failure
	if resp.StatusCode >= 500 {
		return nil, errRetryableOllama
	}

	// Non-retryable client / unexpected error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d", resp.StatusCode)
	}

	var res embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res, nil
}
