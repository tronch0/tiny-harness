package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tiny-harness/models"
)

const (
	testServerAddress = "http://192.168.50.65:11434"
	testModelID       = `C:\Users\User\models\qwen3.8\Qwen3.8-27B-UD-Q5_K_M.gguf`
)

func TestExecute_happyPath(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := provider.Execute(ctx, &models.Request{
		Messages: []models.Message{
			{Role: "user", Content: "Say hello in one short sentence."},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.Content == "" {
		t.Fatal("Execute() returned empty content")
	}

	t.Logf("response: %q", res.Content)
}

func TestExecute_timeout(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := provider.Execute(ctx, &models.Request{
		Messages: []models.Message{
			{Role: "user", Content: "Write a very long essay about the history of computing."},
		},
	})
	if err == nil {
		t.Fatal("Execute() expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Execute() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStreamExecute_parsesSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"think \"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"!\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	stream, err := provider.StreamExecute(context.Background(), &models.Request{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("StreamExecute() error = %v", err)
	}

	res, err := models.CollectStream(stream)
	if err != nil {
		t.Fatalf("CollectStream() error = %v", err)
	}
	if res.Reasoning != "think " {
		t.Fatalf("Reasoning = %q, want %q", res.Reasoning, "think ")
	}
	if res.Content != "Hello!" {
		t.Fatalf("Content = %q, want %q", res.Content, "Hello!")
	}
}

func TestStreamExecute_unexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	provider := NewOpenAIProvider(srv.URL, "test-model")
	_, err := provider.StreamExecute(context.Background(), &models.Request{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("StreamExecute() expected status error, got nil")
	}
}

func TestStreamExecute_cancelMidStream(t *testing.T) {
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
	stream, err := provider.StreamExecute(ctx, &models.Request{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("StreamExecute() error = %v", err)
	}

	chunk, ok := <-stream.Chunks()
	if !ok {
		t.Fatal("expected at least one chunk")
	}
	if chunk.ContentDelta != "Hi" {
		t.Fatalf("ContentDelta = %q, want %q", chunk.ContentDelta, "Hi")
	}

	<-started
	cancel()

	for range stream.Chunks() {
	}
	if !errors.Is(stream.Err(), context.Canceled) {
		t.Fatalf("stream.Err() = %v, want context.Canceled", stream.Err())
	}
}

func TestStreamExecute_timeout(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(1 * time.Millisecond)

	_, err := provider.StreamExecute(ctx, &models.Request{
		Messages: []models.Message{
			{Role: "user", Content: "Write a very long essay about the history of computing."},
		},
	})
	if err == nil {
		t.Fatal("StreamExecute() expected timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("StreamExecute() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestStreamExecute_emitsChunks(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := provider.StreamExecute(ctx, &models.Request{
		Messages: []models.Message{
			{Role: "user", Content: "Say hello in one short sentence."},
		},
	})
	if err != nil {
		t.Fatalf("StreamExecute() error = %v", err)
	}

	res, err := models.CollectStream(stream)
	if err != nil {
		t.Fatalf("CollectStream() error = %v", err)
	}
	if res.Content == "" {
		t.Fatal("CollectStream() returned empty content")
	}

	t.Logf("reasoning: %q", res.Reasoning)
	t.Logf("content: %q", res.Content)
}
