package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

const aiWorkerURL = "http://localhost:8000"

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

func NewClient() *Client {
	return &Client{BaseURL: aiWorkerURL}
}

func (c *Client) GenerateStream(prompt string) error {
	reqBody := GenerateRequest{Prompt: prompt}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		c.BaseURL+generateEndpoint,
		contentTypeJSON,
		bytes.NewBuffer(data),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	buffer := make([]byte, 1024)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			print(string(buffer[:n]))
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}
