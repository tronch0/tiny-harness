package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func echoTool() Tool {
	return Tool{
		Name:        "echo",
		Description: "Echoes the message argument",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`),
		Execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			var parsed struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &parsed); err != nil {
				return "", err
			}
			return parsed.Message, nil
		},
	}
}

func TestRun_happyPath(t *testing.T) {
	tool := echoTool()
	got, err := tool.Run(context.Background(), json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != "hello" {
		t.Fatalf("Run() = %q, want %q", got, "hello")
	}
}

func TestRun_executeError(t *testing.T) {
	wantErr := errors.New("boom")
	tool := Tool{
		Name: "fail",
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "", wantErr
		},
	}
	_, err := tool.Run(context.Background(), nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRun_nilExecute(t *testing.T) {
	tool := Tool{Name: "noop"}
	_, err := tool.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("Run() expected error for nil Execute")
	}
}

func TestRun_canceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := echoTool().Run(ctx, json.RawMessage(`{"message":"hello"}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestFind(t *testing.T) {
	list := []Tool{echoTool(), {Name: "other"}}

	got, ok := Find(list, "echo")
	if !ok {
		t.Fatal("Find(echo) = false, want true")
	}
	if got.Name != "echo" {
		t.Fatalf("Find(echo).Name = %q, want echo", got.Name)
	}

	if _, ok := Find(list, "missing"); ok {
		t.Fatal("Find(missing) = true, want false")
	}
}
