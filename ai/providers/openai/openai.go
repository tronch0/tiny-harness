package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tiny-harness/ai"
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

// Ensure OpenAIProvider implements Provider.
var _ ai.Provider = (*OpenAIProvider)(nil)

// Complete sends the request to the chat completions API and returns the assistant output.
func (p *OpenAIProvider) Complete(ctx context.Context, in *ai.Input) (*ai.Output, error) {
	res, err := p.doChatRequest(ctx, in, false)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return extractOutput(res)
}

// Stream streams tokens from the chat completions API.
func (p *OpenAIProvider) Stream(ctx context.Context, in *ai.Input) (*ai.Stream, error) {
	res, err := p.doChatRequest(ctx, in, true)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("unexpected status %d", res.StatusCode)
	}

	stream := ai.NewStream()
	go func() {
		defer res.Body.Close()
		stream.Finish(readSSE(ctx, res.Body, stream))
	}()

	return stream, nil
}

func (p *OpenAIProvider) doChatRequest(ctx context.Context, in *ai.Input, stream bool) (*http.Response, error) {
	reqBody := chatRequest{
		Model:    p.model,
		Messages: toWireMessages(in.Messages),
		Tools:    toWireTools(in.Tools),
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

func extractOutput(res *http.Response) (*ai.Output, error) {
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

	msg := parsed.Choices[0].Message
	return &ai.Output{
		Content:   msg.Content,
		Reasoning: msg.ReasoningContent,
		ToolCalls: fromWireToolCalls(msg.ToolCalls),
	}, nil
}
