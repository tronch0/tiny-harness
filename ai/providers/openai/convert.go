package openai

import (
	"encoding/json"

	"tiny-harness/ai"
)

func toWireMessages(msgs []ai.Message) []wireMessage {
	out := make([]wireMessage, len(msgs))
	for i, m := range msgs {
		out[i] = wireMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  toWireToolCalls(m.ToolCalls),
			ToolCallID: m.ToolCallID,
		}
	}
	return out
}

func toWireTools(tools []ai.Tool) []wireTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]wireTool, len(tools))
	for i, t := range tools {
		params := t.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{}`)
		}
		out[i] = wireTool{Type: "function"}
		out[i].Function.Name = t.Name
		out[i].Function.Description = t.Description
		out[i].Function.Parameters = params
	}
	return out
}

func toWireToolCalls(calls []ai.ToolCall) []wireToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]wireToolCall, len(calls))
	for i, c := range calls {
		args := string(c.Arguments)
		if args == "" {
			args = "{}"
		}
		out[i] = wireToolCall{ID: c.ID, Type: "function"}
		out[i].Function.Name = c.Name
		out[i].Function.Arguments = args
	}
	return out
}

func fromWireUsage(u wireUsage) ai.Usage {
	return ai.Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
	}
}

func fromWireToolCalls(calls []wireToolCall) []ai.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ai.ToolCall, len(calls))
	for i, c := range calls {
		args := json.RawMessage(c.Function.Arguments)
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		out[i] = ai.ToolCall{
			ID:        c.ID,
			Name:      c.Function.Name,
			Arguments: args,
		}
	}
	return out
}
