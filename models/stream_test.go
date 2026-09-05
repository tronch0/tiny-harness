package models

import (
	"errors"
	"testing"
)

func TestCollectStream(t *testing.T) {
	stream := NewStream()

	go func() {
		stream.Send(StreamChunk{ReasoningDelta: "think "})
		stream.Send(StreamChunk{ContentDelta: "Hello"})
		stream.Send(StreamChunk{ContentDelta: "!"})
		stream.Finish(nil)
	}()

	res, err := CollectStream(stream)
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

func TestCollectStream_error(t *testing.T) {
	stream := NewStream()
	wantErr := errors.New("stream failed")

	go func() {
		stream.Send(StreamChunk{ContentDelta: "partial"})
		stream.Finish(wantErr)
	}()

	_, err := CollectStream(stream)
	if !errors.Is(err, wantErr) {
		t.Fatalf("CollectStream() error = %v, want %v", err, wantErr)
	}
}

func TestStream_ErrAfterFinish(t *testing.T) {
	stream := NewStream()
	wantErr := errors.New("cancelled")

	go func() {
		stream.Finish(wantErr)
	}()

	for range stream.Chunks() {
	}

	if !errors.Is(stream.Err(), wantErr) {
		t.Fatalf("Err() = %v, want %v", stream.Err(), wantErr)
	}
}
