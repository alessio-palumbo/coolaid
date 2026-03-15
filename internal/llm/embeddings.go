package llm

import (
	"encoding/json"
)

type embeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

func (c *Client) Embed(text string) ([]float64, error) {
	reqBody := embeddingRequest{
		Prompt: text,
		Model:  c.EmbeddingsModel,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := c.post(embeddingsEndpoint, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res embeddingResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	return res.Embedding, nil
}
