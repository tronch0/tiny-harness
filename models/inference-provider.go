package models

import "context"

// InferenceProvider is an interface that defines the methods for an external AI provider.
type InferenceProvider interface {
	Execute(ctx context.Context, req *Request) (*Response, error)
}

// Message is a struct that represents a message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Messages []Message `json:"messages"`
}

type Response struct {
	Content string `json:"content"`
}
