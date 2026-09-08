package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"tiny-harness/ai"
	"tiny-harness/tools"
)

type fakeProvider struct {
	inputs []*ai.Input
	outs   []*ai.Output
	err    error
	i      int
	block  bool
}

func (f *fakeProvider) Complete(ctx context.Context, in *ai.Input) (*ai.Output, error) {
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return nil, f.err
	}
	if f.i >= len(f.outs) {
		return &ai.Output{}, nil
	}
	out := f.outs[f.i]
	f.i++
	return out, nil
}

func (f *fakeProvider) Stream(ctx context.Context, in *ai.Input) (*ai.Stream, error) {
	out, err := f.Complete(ctx, in)
	if err != nil {
		return nil, err
	}
	stream := ai.NewStream()
	go func() {
		if out.Reasoning != "" {
			stream.Send(ai.Delta{ReasoningDelta: out.Reasoning})
		}
		if out.Content != "" {
			stream.Send(ai.Delta{ContentDelta: out.Content})
		}
		stream.SetToolCalls(out.ToolCalls)
		stream.SetUsage(out.Usage)
		stream.Finish(nil)
	}()
	return stream, nil
}

func TestRun_appendsTurn(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{{Content: "hi", Reasoning: "think"}}}
	a := New(p, nil, Config{})

	out, err := a.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Content != "hi" {
		t.Fatalf("Content = %q, want %q", out.Content, "hi")
	}

	got := a.Messages()
	want := []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	if !messagesEqual(got, want) {
		t.Fatalf("Messages() = %#v, want %#v", got, want)
	}
	if len(p.inputs) != 1 || len(p.inputs[0].Messages) != 1 {
		t.Fatalf("provider saw %#v, want one user message", p.inputs)
	}
}

func TestRun_sendsHistory(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{{Content: "second"}}}
	a := New(p, nil, Config{})
	a.messages = []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	if _, err := a.Run(context.Background(), "again"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := p.inputs[0].Messages
	want := []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
		{Role: "user", Content: "again"},
	}
	if !messagesEqual(got, want) {
		t.Fatalf("provider messages = %#v, want %#v", got, want)
	}
}

func TestRun_prependsSystemPrompt(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{{Content: "hi"}}}
	a := New(p, nil, Config{SystemPrompt: "be brief"})

	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := p.inputs[0].Messages
	want := []ai.Message{
		{Role: "system", Content: "be brief"},
		{Role: "user", Content: "hello"},
	}
	if !messagesEqual(got, want) {
		t.Fatalf("provider messages = %#v, want %#v", got, want)
	}
	hist := a.Messages()
	if len(hist) != 2 || hist[0].Role != "user" {
		t.Fatalf("Messages() = %#v, want conversation without system", hist)
	}
}

func TestRun_systemPromptOnEveryComplete(t *testing.T) {
	call := ai.ToolCall{ID: "call_1", Name: "echo"}
	p := &fakeProvider{outs: []*ai.Output{
		{ToolCalls: []ai.ToolCall{call}},
		{Content: "done"},
	}}
	a := New(p, []tools.Tool{{
		Name: "echo",
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "ok", nil
		},
	}}, Config{SystemPrompt: "use tools"})

	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(p.inputs) != 2 {
		t.Fatalf("Complete calls = %d, want 2", len(p.inputs))
	}
	for i, in := range p.inputs {
		if len(in.Messages) == 0 || in.Messages[0].Role != "system" || in.Messages[0].Content != "use tools" {
			t.Fatalf("Complete %d messages = %#v, want system first", i, in.Messages)
		}
	}
}

func TestRun_errorKeepsUserMessage(t *testing.T) {
	wantErr := errors.New("provider failed")
	p := &fakeProvider{err: wantErr}
	a := New(p, nil, Config{})

	_, err := a.Run(context.Background(), "hello")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}

	got := a.Messages()
	want := []ai.Message{{Role: "user", Content: "hello"}}
	if !messagesEqual(got, want) {
		t.Fatalf("Messages() = %#v, want %#v", got, want)
	}
}

func TestRun_canceledContext(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{{Content: "hi"}}}
	a := New(p, nil, Config{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.Run(ctx, "hello")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRun_sendsToolSchemas(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{{Content: "ok"}}}
	params := json.RawMessage(`{"type":"object"}`)
	a := New(p, []tools.Tool{{
		Name:        "echo",
		Description: "Echoes input",
		Parameters:  params,
	}}, Config{})

	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(p.inputs) != 1 {
		t.Fatalf("provider saw %d inputs, want 1", len(p.inputs))
	}
	got := p.inputs[0].Tools
	if len(got) != 1 || got[0].Name != "echo" || got[0].Description != "Echoes input" || string(got[0].Parameters) != string(params) {
		t.Fatalf("Input.Tools = %#v, want echo schema", got)
	}
}

func TestRun_loopsUntilNoToolCalls(t *testing.T) {
	call := ai.ToolCall{
		ID:        "call_1",
		Name:      "echo",
		Arguments: json.RawMessage(`{"message":"hi"}`),
	}
	p := &fakeProvider{outs: []*ai.Output{
		{Content: "calling echo", ToolCalls: []ai.ToolCall{call}},
		{Content: "done"},
	}}
	a := New(p, []tools.Tool{{
		Name: "echo",
		Execute: func(_ context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}}, Config{})

	out, err := a.Run(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out.Content != "done" {
		t.Fatalf("Content = %q, want done", out.Content)
	}
	if len(p.inputs) != 2 {
		t.Fatalf("Complete calls = %d, want 2", len(p.inputs))
	}

	second := p.inputs[1].Messages
	wantSecond := []ai.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "calling echo", ToolCalls: []ai.ToolCall{call}},
		{Role: "tool", Content: `{"message":"hi"}`, ToolCallID: "call_1"},
	}
	if !messagesEqual(second, wantSecond) {
		t.Fatalf("second Complete messages = %#v, want %#v", second, wantSecond)
	}

	got := a.Messages()
	want := append(wantSecond, ai.Message{Role: "assistant", Content: "done"})
	if !messagesEqual(got, want) {
		t.Fatalf("Messages() = %#v, want %#v", got, want)
	}
}

func TestRun_missingToolAppendsErrorResult(t *testing.T) {
	p := &fakeProvider{outs: []*ai.Output{
		{ToolCalls: []ai.ToolCall{{ID: "call_1", Name: "missing"}}},
		{Content: "gave up"},
	}}
	a := New(p, nil, Config{})

	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := a.Messages()
	if len(got) != 4 || got[2].Role != "tool" || got[2].ToolCallID != "call_1" {
		t.Fatalf("Messages() = %#v, want user, assistant, tool, assistant", got)
	}
	if got[2].Content != `tool "missing" not found` {
		t.Fatalf("tool Content = %q", got[2].Content)
	}
}

func TestRun_toolErrorAppendsErrorResult(t *testing.T) {
	wantErr := errors.New("boom")
	p := &fakeProvider{outs: []*ai.Output{
		{ToolCalls: []ai.ToolCall{{ID: "call_1", Name: "fail"}}},
		{Content: "gave up"},
	}}
	a := New(p, []tools.Tool{{
		Name: "fail",
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "", wantErr
		},
	}}, Config{})

	if _, err := a.Run(context.Background(), "hello"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := a.Messages()
	if len(got) != 4 || got[2].Role != "tool" || got[2].Content != "boom" {
		t.Fatalf("Messages() = %#v, want tool error result then final assistant", got)
	}
}

func TestRun_maxTurns(t *testing.T) {
	call := ai.ToolCall{ID: "call_1", Name: "echo"}
	p := &fakeProvider{outs: []*ai.Output{
		{Content: "one", ToolCalls: []ai.ToolCall{call}},
		{Content: "two", ToolCalls: []ai.ToolCall{call}},
		{Content: "should not run"},
	}}
	a := New(p, []tools.Tool{{
		Name: "echo",
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "ok", nil
		},
	}}, Config{MaxTurns: 2})

	out, err := a.Run(context.Background(), "hello")
	if !errors.Is(err, ErrMaxTurns) {
		t.Fatalf("Run() error = %v, want ErrMaxTurns", err)
	}
	if out == nil || out.Content != "two" {
		t.Fatalf("last Output = %#v, want Content two", out)
	}
	if len(p.inputs) != 2 {
		t.Fatalf("Complete calls = %d, want 2", len(p.inputs))
	}
}

func TestRun_timeout(t *testing.T) {
	p := &fakeProvider{block: true}
	a := New(p, nil, Config{Timeout: 5 * time.Millisecond})

	_, err := a.Run(context.Background(), "hello")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMessages_isCopy(t *testing.T) {
	a := New(&fakeProvider{}, nil, Config{})
	a.messages = []ai.Message{{Role: "user", Content: "hello"}}

	got := a.Messages()
	got[0].Content = "mutated"

	if a.messages[0].Content != "hello" {
		t.Fatal("Messages() allowed mutation of agent history")
	}
}

func messagesEqual(a, b []ai.Message) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Role != b[i].Role || a[i].Content != b[i].Content || a[i].ToolCallID != b[i].ToolCallID {
			return false
		}
		if len(a[i].ToolCalls) != len(b[i].ToolCalls) {
			return false
		}
		for j := range a[i].ToolCalls {
			ac, bc := a[i].ToolCalls[j], b[i].ToolCalls[j]
			if ac.ID != bc.ID || ac.Name != bc.Name || string(ac.Arguments) != string(bc.Arguments) {
				return false
			}
		}
	}
	return true
}
