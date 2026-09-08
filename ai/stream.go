package ai

import "strings"

// Delta is a single incremental token from a streaming inference call.
type Delta struct {
	ReasoningDelta string
	ContentDelta   string
}

// Stream delivers incremental deltas from a streaming inference call.
// The provider sends deltas via Send and must call Finish when done.
// Callers read deltas from Deltas and check Err after the channel closes.
type Stream struct {
	deltas    chan Delta
	err       error
	toolCalls []ToolCall
	usage     Usage
}

// NewStream creates a stream handle for a provider to populate.
func NewStream() *Stream {
	return &Stream{deltas: make(chan Delta)}
}

// Deltas returns the read-only channel of incremental tokens.
func (s *Stream) Deltas() <-chan Delta {
	return s.deltas
}

// Err returns the error set by Finish, if any. Call after Deltas is closed.
func (s *Stream) Err() error {
	return s.err
}

// SetToolCalls records assembled tool calls. For use by providers only, before Finish.
func (s *Stream) SetToolCalls(calls []ToolCall) {
	s.toolCalls = calls
}

// SetUsage records provider-reported token counts. For use by providers only, before Finish.
func (s *Stream) SetUsage(u Usage) {
	s.usage = u
}

// Send emits one delta. For use by providers only.
func (s *Stream) Send(delta Delta) {
	s.deltas <- delta
}

// Finish closes the stream and records the final error, if any.
func (s *Stream) Finish(err error) {
	s.err = err
	close(s.deltas)
}

// Collect reads all deltas and returns the assembled output.
func Collect(stream *Stream) (*Output, error) {
	return CollectFunc(stream, nil)
}

// CollectFunc is Collect plus a callback for each delta (live tokens).
func CollectFunc(stream *Stream, onDelta func(Delta)) (*Output, error) {
	var reasoning, content strings.Builder

	for delta := range stream.Deltas() {
		if onDelta != nil {
			onDelta(delta)
		}
		reasoning.WriteString(delta.ReasoningDelta)
		content.WriteString(delta.ContentDelta)
	}

	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &Output{
		Content:   content.String(),
		Reasoning: reasoning.String(),
		ToolCalls: stream.toolCalls,
		Usage:     stream.usage,
	}, nil
}
