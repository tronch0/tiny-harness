package ai

import (
	"errors"
	"testing"
)

func TestCollect(t *testing.T) {
	stream := NewStream()

	go func() {
		stream.Send(Delta{ReasoningDelta: "think "})
		stream.Send(Delta{ContentDelta: "Hello"})
		stream.Send(Delta{ContentDelta: "!"})
		stream.Finish(nil)
	}()

	out, err := Collect(stream)
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

func TestCollect_error(t *testing.T) {
	stream := NewStream()
	wantErr := errors.New("stream failed")

	go func() {
		stream.Send(Delta{ContentDelta: "partial"})
		stream.Finish(wantErr)
	}()

	_, err := Collect(stream)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Collect() error = %v, want %v", err, wantErr)
	}
}

func TestCollect_usage(t *testing.T) {
	stream := NewStream()

	go func() {
		stream.Send(Delta{ContentDelta: "hi"})
		stream.SetUsage(Usage{PromptTokens: 12, CompletionTokens: 3})
		stream.Finish(nil)
	}()

	out, err := Collect(stream)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if out.Usage.PromptTokens != 12 || out.Usage.CompletionTokens != 3 {
		t.Fatalf("Usage = %+v", out.Usage)
	}
}

func TestCollect_toolCalls(t *testing.T) {
	stream := NewStream()
	calls := []ToolCall{{ID: "call_1", Name: "echo"}}

	go func() {
		stream.Send(Delta{ContentDelta: "hi"})
		stream.SetToolCalls(calls)
		stream.Finish(nil)
	}()

	out, err := Collect(stream)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if out.Content != "hi" {
		t.Fatalf("Content = %q", out.Content)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Name != "echo" {
		t.Fatalf("ToolCalls = %#v", out.ToolCalls)
	}
}

func TestCollectFunc_onDelta(t *testing.T) {
	stream := NewStream()
	var got []Delta

	go func() {
		stream.Send(Delta{ContentDelta: "A"})
		stream.Send(Delta{ContentDelta: "B"})
		stream.Finish(nil)
	}()

	out, err := CollectFunc(stream, func(d Delta) { got = append(got, d) })
	if err != nil {
		t.Fatalf("CollectFunc() error = %v", err)
	}
	if out.Content != "AB" || len(got) != 2 {
		t.Fatalf("Content = %q deltas = %#v", out.Content, got)
	}
}

func TestStream_ErrAfterFinish(t *testing.T) {
	stream := NewStream()
	wantErr := errors.New("cancelled")

	go func() {
		stream.Finish(wantErr)
	}()

	for range stream.Deltas() {
	}

	if !errors.Is(stream.Err(), wantErr) {
		t.Fatalf("Err() = %v, want %v", stream.Err(), wantErr)
	}
}
