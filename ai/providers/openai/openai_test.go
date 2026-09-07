package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tiny-harness/ai"
)

const (
	testServerAddress = "http://192.168.50.65:11434"
	testModelID       = `C:\Users\User\models\qwen3.8\Qwen3.8-27B-UD-Q5_K_M.gguf`
)

func TestComplete_happyPath(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := provider.Complete(ctx, &ai.Input{
		Messages: []ai.Message{
			{Role: "user", Content: "Say hello in one short sentence."},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if out.Content == "" {
		t.Fatal("Complete() returned empty content")
	}

	t.Logf("output: %q", out.Content)
}

func TestComplete_timeout(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := provider.Complete(ctx, &ai.Input{
		Messages: []ai.Message{
			{Role: "user", Content: "Write a very long essay about the history of computing."},
		},
	})
	if err == nil {
		t.Fatal("Complete() expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Complete() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestComplete_sendsToolsAndParsesToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Tools) != 1 {
			t.Fatalf("tools len = %d, want 1", len(req.Tools))
		}
		if req.Tools[0].Type != "function" || req.Tools[0].Function.Name != "echo" {
			t.Fatalf("tool = %+v, want function echo", req.Tools[0])
		}
		if string(req.Tools[0].Function.Parameters) != `{"type":"object"}` {
			t.Fatalf("parameters = %s", req.Tools[0].Function.Parameters)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"echo","arguments":"{\"message\":\"hi\"}"}}]}}]}`)
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	out, err := provider.Complete(context.Background(), &ai.Input{
		Messages: []ai.Message{{Role: "user", Content: "say hi via echo"}},
		Tools: []ai.Tool{{
			Name:        "echo",
			Description: "Echo a message",
			Parameters:  json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(out.ToolCalls))
	}
	tc := out.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "echo" {
		t.Fatalf("ToolCall = %+v", tc)
	}
	if string(tc.Arguments) != `{"message":"hi"}` {
		t.Fatalf("Arguments = %s", tc.Arguments)
	}
}

func TestComplete_roundTripsToolMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var req chatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Messages) != 3 {
			t.Fatalf("messages len = %d, want 3", len(req.Messages))
		}
		asst := req.Messages[1]
		if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].Function.Name != "echo" {
			t.Fatalf("assistant tool_calls = %+v", asst.ToolCalls)
		}
		tool := req.Messages[2]
		if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content != "hi" {
			t.Fatalf("tool message = %+v", tool)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}]}`)
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	out, err := provider.Complete(context.Background(), &ai.Input{
		Messages: []ai.Message{
			{Role: "user", Content: "echo hi"},
			{
				Role: "assistant",
				ToolCalls: []ai.ToolCall{{
					ID:        "call_1",
					Name:      "echo",
					Arguments: json.RawMessage(`{"message":"hi"}`),
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if out.Content != "done" {
		t.Fatalf("Content = %q, want done", out.Content)
	}
}

func TestStream_parsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	stream, err := provider.Stream(context.Background(), &ai.Input{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	out, err := ai.Collect(stream)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if out.Reasoning != "think " {
		t.Fatalf("Reasoning = %q, want %q", out.Reasoning, "think ")
	}
	if out.Content != "Hello!" {
		t.Fatalf("Content = %q, want %q", out.Content, "Hello!")
	}
}

func TestStream_unexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	_, err := provider.Stream(context.Background(), &ai.Input{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("Stream() expected status error, got nil")
	}
}

func TestStream_cancelMidStream(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not support flushing")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hi\"}}]}\n\n")
		flusher.Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	stream, err := provider.Stream(ctx, &ai.Input{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	delta, ok := <-stream.Deltas()
	if !ok {
		t.Fatal("expected at least one delta")
	}
	if delta.ContentDelta != "Hi" {
		t.Fatalf("ContentDelta = %q, want %q", delta.ContentDelta, "Hi")
	}

	<-started
	cancel()

	for range stream.Deltas() {
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("stream.Err() = %v, want context.Canceled", stream.Err())
	}
}

func TestStream_timeout(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := provider.Stream(ctx, &ai.Input{
		Messages: []ai.Message{
			{Role: "user", Content: "Write a very long essay about the history of computing."},
		},
	})
	if err == nil {
		t.Fatal("Stream() expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stream() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStream_emitsDeltas(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := provider.Stream(ctx, &ai.Input{
		Messages: []ai.Message{
			{Role: "user", Content: "Say hello in one short sentence."},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	out, err := ai.Collect(stream)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if out.Content == "" {
		t.Fatal("Collect() returned empty content")
	}

	t.Logf("reasoning: %q", out.Reasoning)
	t.Logf("content: %q", out.Content)
}
