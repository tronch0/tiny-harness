package ai

import "context"

// Provider is an interface that defines the methods for an external AI provider.
type Provider interface {
	Complete(ctx context.Context, in *Input) (*Output, error)
	Stream(ctx context.Context, in *Input) (*Stream, error)
}

// Message is a struct that represents a message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Input struct {
	Messages []Message `json:"messages"`
}

type Output struct {
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
}
