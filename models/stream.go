package models

import "strings"

// StreamChunk is a single incremental token from a streaming inference call.
type StreamChunk struct {
	ReasoningDelta string
	ContentDelta   string
}

// Stream delivers incremental chunks from a streaming inference call.
// The provider sends chunks via Send and must call Finish when done.
// Callers read chunks from Chunks and check Err after the channel closes.
type Stream struct {
	chunks chan StreamChunk
	err    error
}

// NewStream creates a stream handle for a provider to populate.
func NewStream() *Stream {
	return &Stream{chunks: make(chan StreamChunk)}
}

// Chunks returns the read-only channel of incremental tokens.
func (s *Stream) Chunks() <-chan StreamChunk {
	return s.chunks
}

// Err returns the error set by Finish, if any. Call after Chunks is closed.
func (s *Stream) Err() error {
	return s.err
}

// Send emits one chunk. For use by inference providers only.
func (s *Stream) Send(chunk StreamChunk) {
	s.chunks <- chunk
}

// Finish closes the stream and records the final error, if any.
func (s *Stream) Finish(err error) {
	s.err = err
	close(s.chunks)
}

// CollectStream reads all chunks and returns the assembled response.
func CollectStream(stream *Stream) (*Response, error) {
	var reasoning, content strings.Builder

	for chunk := range stream.Chunks() {
		reasoning.WriteString(chunk.ReasoningDelta)
		content.WriteString(chunk.ContentDelta)
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &Response{
		Content:   content.String(),
		Reasoning: reasoning.String(),
	}, nil
}
