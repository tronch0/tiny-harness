package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"tiny-harness/models"
)

// OpenAIProvider is a struct that implements the Provider interface.
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
	Stream   bool             `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

type chatStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

// Ensure OpenAIProvider implements Provider.
var _ models.Provider = (*OpenAIProvider)(nil)

// Complete sends the request to the chat completions API and returns the assistant output.
func (p *OpenAIProvider) Complete(ctx context.Context, in *models.Input) (*models.Output, error) {
	res, err := p.doChatRequest(ctx, in, false)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return extractOutput(res)
}

// Stream streams tokens from the chat completions API.
func (p *OpenAIProvider) Stream(ctx context.Context, in *models.Input) (*models.Stream, error) {
	res, err := p.doChatRequest(ctx, in, true)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("unexpected status %d", res.StatusCode)
	}

	stream := models.NewStream()
	go func() {
		defer res.Body.Close()
		stream.Finish(readSSE(ctx, res.Body, stream))
	}()

	return stream, nil
}

func (p *OpenAIProvider) doChatRequest(ctx context.Context, in *models.Input, stream bool) (*http.Response, error) {
	reqBody := chatRequest{
		Model:    p.model,
		Messages: in.Messages,
		Stream:   stream,
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

	return res, nil
}

func extractOutput(res *http.Response) (*models.Output, error) {
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

	return &models.Output{
		Content:   parsed.Choices[0].Message.Content,
		Reasoning: parsed.Choices[0].Message.ReasoningContent,
	}, nil
}

func readSSE(ctx context.Context, body io.Reader, stream *models.Stream) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return nil
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("decode stream chunk: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		if delta.Content == "" && delta.ReasoningContent == "" {
			continue
		}

		stream.Send(models.Delta{
			ReasoningDelta: delta.ReasoningContent,
			ContentDelta:   delta.Content,
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}
