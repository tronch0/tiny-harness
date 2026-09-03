package openai

import (
	"context"
	"errors"
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
