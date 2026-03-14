package ai

import (
	"bytes"
	"encoding/json"
	"net/http"
)

var (
	generateEndpoint = "/generate"
	contentTypeJSON  = "application/json"
)

type Client struct {
	BaseURL string
}

type GenerateRequest struct {
	Prompt string `json:"prompt"`
}

type GenerateResponse struct {
	Response string `json:"response"`
}

func (c *Client) Generate(prompt string) (string, error) {
	reqBody := GenerateRequest{Prompt: prompt}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	resp, err := http.Post(
		c.BaseURL+generateEndpoint,
		contentTypeJSON,
		bytes.NewBuffer(data),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result GenerateResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	return result.Response, nil
}
