package ai

import (
	"context"
	"encoding/json"
)

// Provider is an interface that defines the methods for an external AI provider.
type Provider interface {
	Complete(ctx context.Context, in *Input) (*Output, error)
	Stream(ctx context.Context, in *Input) (*Stream, error)
}

// Message is one turn in the conversation sent to or returned from a model.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall // assistant messages that request tools
	ToolCallID string     // role=tool: which call this result answers
}

// Tool is a schema-only tool description for the model (no Execute).
type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema object
}

// ToolCall is a model request to run a tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage // JSON object (or raw JSON text from the provider)
}

type Input struct {
	Messages []Message
	Tools    []Tool
}

type Output struct {
	Content   string
	Reasoning string
	ToolCalls []ToolCall
}
