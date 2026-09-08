package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"tiny-harness/ai"
	"tiny-harness/tools"
)

// ErrMaxTurns is returned when Run hits Config.MaxTurns with tool calls still pending.
var ErrMaxTurns = errors.New("max turns exceeded")

// Config bounds a Run. Zero values mean no extra limits (use the caller's ctx only).
type Config struct {
	// MaxTurns is the maximum provider Complete calls per Run. Zero means unlimited.
	MaxTurns int
	// Timeout, if > 0, is applied to the whole Run together with ctx.
	Timeout time.Duration
	// SystemPrompt, if set, is prepended as a system message on each model call.
	// It is not stored in conversation history.
	SystemPrompt string
}

// Agent runs turns against a Provider and keeps the conversation.
type Agent struct {
	provider ai.Provider
	messages []ai.Message
	tools    []tools.Tool
	cfg      Config
	OnDelta  func(ai.Delta)
	OnTurn   func(*ai.Output)
}

// New creates an Agent that calls provider.
func New(provider ai.Provider, tools []tools.Tool, cfg Config) *Agent {
	return &Agent{provider: provider, tools: tools, cfg: cfg}
}

// Messages returns a copy of the conversation so far.
func (a *Agent) Messages() []ai.Message {
	return slices.Clone(a.messages)
}

// Run appends a user message, completes one model turn, and records the reply.
func (a *Agent) Run(ctx context.Context, userMsg string) (*ai.Output, error) {
	if a.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.Timeout)
		defer cancel()
	}

	a.messages = append(a.messages, ai.Message{Role: "user", Content: userMsg})

	var out *ai.Output
	var err error
	turns := 0

	for {
		if a.cfg.MaxTurns > 0 && turns >= a.cfg.MaxTurns {
			return out, fmt.Errorf("%w: %d", ErrMaxTurns, a.cfg.MaxTurns)
		}
		turns++

		out, err = a.turn(ctx)
		if err != nil {
			return nil, err
		}

		a.messages = append(a.messages, ai.Message{
			Role:      "assistant",
			Content:   out.Content,
			ToolCalls: out.ToolCalls,
		})

		for _, call := range out.ToolCalls {
			a.messages = append(a.messages, ai.Message{
				Role:       "tool",
				Content:    a.runTool(ctx, call),
				ToolCallID: call.ID,
			})
		}

		if len(out.ToolCalls) == 0 {
			break
		}
	}

	return out, nil
}

func (a *Agent) turn(ctx context.Context) (*ai.Output, error) {
	stream, err := a.provider.Stream(ctx, &ai.Input{
		Messages: a.modelMessages(),
		Tools:    toolsToAiTools(a.tools),
	})
	if err != nil {
		return nil, err
	}
	return ai.CollectFunc(stream, a.OnDelta)
}

func (a *Agent) modelMessages() []ai.Message {
	if a.cfg.SystemPrompt == "" {
		return slices.Clone(a.messages)
	}
	out := make([]ai.Message, 0, 1+len(a.messages))
	out = append(out, ai.Message{Role: "system", Content: a.cfg.SystemPrompt})
	return append(out, a.messages...)
}

func (a *Agent) runTool(ctx context.Context, call ai.ToolCall) string {
	exe, ok := tools.Find(a.tools, call.Name)
	if !ok {
		return fmt.Sprintf("tool %q not found", call.Name)
	}
	result, err := exe.Run(ctx, call.Arguments)
	if err != nil {
		return err.Error()
	}
	return result
}

func toolsToAiTools(list []tools.Tool) []ai.Tool {
	result := make([]ai.Tool, len(list))
	for i, tool := range list {
		result[i] = ai.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		}
	}
	return result
}
