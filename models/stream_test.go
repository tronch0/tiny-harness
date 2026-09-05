package models

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
