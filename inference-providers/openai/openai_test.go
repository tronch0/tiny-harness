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

func TestComplete_happyPath(t *testing.T) {
	provider := NewOpenAIProvider(testServerAddress, testModelID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := provider.Complete(ctx, &models.Input{
		Messages: []models.Message{
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

	_, err := provider.Complete(ctx, &models.Input{
		Messages: []models.Message{
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
	stream, err := provider.Stream(context.Background(), &models.Input{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	out, err := models.Collect(stream)
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
	_, err := provider.Stream(context.Background(), &models.Input{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
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
	stream, err := provider.Stream(ctx, &models.Input{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
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

	_, err := provider.Stream(ctx, &models.Input{
		Messages: []models.Message{
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

	stream, err := provider.Stream(ctx, &models.Input{
		Messages: []models.Message{
			{Role: "user", Content: "Say hello in one short sentence."},
		},
	})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	out, err := models.Collect(stream)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if out.Content == "" {
		t.Fatal("Collect() returned empty content")
	}

	t.Logf("reasoning: %q", out.Reasoning)
	t.Logf("content: %q", out.Content)
}
