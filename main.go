package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tiny-harness/agent"
	"tiny-harness/ai"
	"tiny-harness/ai/providers/openai"
	"tiny-harness/tools"
)

func main() {
	provider := openai.NewOpenAIProvider(
		"http://192.168.50.65:11434",
		`C:\Users\User\models\qwen3.8\Qwen3.8-27B-UD-Q5_K_M.gguf`,
	)

	a := agent.New(provider, demoTools(), agent.Config{
		MaxTurns:     8,
		Timeout:      2 * time.Minute,
		SystemPrompt: "You are a concise assistant. Use tools when they help answer the user.",
	})

	fmt.Println("tiny-harness  (exit / quit / Ctrl+D to leave, Ctrl+C to cancel a turn)")
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		userMsg := strings.TrimSpace(scanner.Text())
		if userMsg == "" {
			continue
		}
		if userMsg == "exit" || userMsg == "quit" {
			break
		}

		printer := newDeltaPrinter()
		a.OnDelta = printer.print
		a.OnTurn = printer.printUsage

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		_, err := a.Run(ctx, userMsg)
		stop()
		printer.finish()
		if err != nil {
			printRunErr(err)
			continue
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		log.Fatal(err)
	}
}

type deltaPrinter struct {
	reasoning bool
	content   bool
}

func newDeltaPrinter() *deltaPrinter {
	return &deltaPrinter{}
}

func (p *deltaPrinter) print(d ai.Delta) {
	if strings.TrimSpace(d.ReasoningDelta) != "" {
		if p.content {
			fmt.Println()
			p.content = false
		}
		if !p.reasoning {
			fmt.Println("reasoning:")
			p.reasoning = true
			fmt.Print(strings.TrimLeft(d.ReasoningDelta, " \t\r\n"))
		} else {
			fmt.Print(d.ReasoningDelta)
		}
	}
	if strings.TrimSpace(d.ContentDelta) != "" {
		if p.reasoning {
			fmt.Println()
			p.reasoning = false
		}
		if !p.content {
			fmt.Println("assistant:")
			p.content = true
			fmt.Print(strings.TrimLeft(d.ContentDelta, " \t\r\n"))
		} else {
			fmt.Print(d.ContentDelta)
		}
	}
}

func (p *deltaPrinter) printUsage(out *ai.Output) {
	p.endLine()
	u := out.Usage
	fmt.Println("--------------------------------")
	fmt.Println("Usage Tokens")
	fmt.Println("PromptTokens", u.PromptTokens)
	fmt.Println("CompletionTokens", u.CompletionTokens)
	fmt.Println("TotalTokens", u.PromptTokens+u.CompletionTokens)
	fmt.Println("--------------------------------")
}

func (p *deltaPrinter) finish() {
	p.endLine()
	fmt.Println()
}

func (p *deltaPrinter) endLine() {
	if p.reasoning || p.content {
		fmt.Println()
	}
	p.reasoning = false
	p.content = false
}

func demoTools() []tools.Tool {
	return []tools.Tool{{
		Name:        "get_weather",
		Description: "Get the weather for a given location",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"location": {
					"type": "string",
					"description": "The location to get the weather for"
				}
			}
		}`),
		Execute: func(ctx context.Context, input json.RawMessage) (string, error) {
			fmt.Println("tool: get_weather")
			return "The weather in Tokyo is sunny.", nil
		},
	}}
}

func printRunErr(err error) {
	switch {
	case errors.Is(err, context.Canceled):
		fmt.Println("cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		fmt.Println("timed out")
	case errors.Is(err, agent.ErrMaxTurns):
		fmt.Println("max turns exceeded")
	default:
		fmt.Println("error:", err)
	}
}
