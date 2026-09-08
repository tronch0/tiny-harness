package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"tiny-harness/ai"
)

func readSSE(ctx context.Context, body io.Reader, stream *ai.Stream) error {
	var acc []ai.ToolCall
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			stream.SetToolCalls(acc)
			return err
		}

		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			stream.SetToolCalls(acc)
			return nil
		}

		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return fmt.Errorf("decode stream chunk: %w", err)
		}
		if chunk.Usage != nil {
			stream.SetUsage(fromWireUsage(*chunk.Usage))
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		acc = mergeStreamToolCalls(acc, delta.ToolCalls)

		if delta.Content == "" && delta.ReasoningContent == "" {
			continue
		}

		stream.Send(ai.Delta{
			ReasoningDelta: delta.ReasoningContent,
			ContentDelta:   delta.Content,
		})
	}
	stream.SetToolCalls(acc)
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return nil
}

func mergeStreamToolCalls(acc []ai.ToolCall, deltas []wireStreamToolCall) []ai.ToolCall {
	for _, d := range deltas {
		for len(acc) <= d.Index {
			acc = append(acc, ai.ToolCall{})
		}
		call := &acc[d.Index]
		if d.ID != "" {
			call.ID = d.ID
		}
		if d.Function.Name != "" {
			call.Name = d.Function.Name
		}
		if d.Function.Arguments != "" {
			call.Arguments = append(call.Arguments, d.Function.Arguments...)
		}
	}
	return acc
}
