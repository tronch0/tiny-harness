package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"tiny-harness/models"
)

// OpenAIProvider is a struct that implements the InferenceProvider interface.
type OpenAIProvider struct {
	address string // e.g. "http://192.168.50.65:11434"
	model   string // e.g. "llama3.2"
	client  *http.Client
}

// NewOpenAIProvider creates a new OpenAIProvider.
func NewOpenAIProvider(address, model string) *OpenAIProvider {
	return &OpenAIProvider{
		address: strings.TrimSuffix(address, "/"),
		model:   model,
		client:  &http.Client{},
	}
}

type chatRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message models.Message `json:"message"`
	} `json:"choices"`
}

// Ensure OpenAIProvider implements InferenceProvider.
var _ models.InferenceProvider = (*OpenAIProvider)(nil)

// Execute sends the request to the chat completions API and returns the assistant response.
func (p *OpenAIProvider) Execute(ctx context.Context, req *models.Request) (*models.Response, error) {
	reqBody := chatRequest{
		Model:    p.model,
		Messages: req.Messages,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := p.address + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	return extractResponse(res)
}

func extractResponse(res *http.Response) (*models.Response, error) {
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", res.StatusCode)
	}

	var parsed chatResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &models.Response{Content: parsed.Choices[0].Message.Content}, nil
}
