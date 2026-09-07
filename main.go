package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tiny-harness/agent"
	"tiny-harness/ai/providers/openai"
	"tiny-harness/tools"
)

func main() {
	provider := openai.NewOpenAIProvider(
		"http://192.168.50.65:11434",
		`C:\Users\User\models\qwen3.8\Qwen3.8-27B-UD-Q5_K_M.gguf`,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tools := []tools.Tool{
		tools.Tool{
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
				fmt.Println("Executing get_weather tool...")
				return "The weather in Tokyo is sunny.", nil
			},
		},
	}

	a := agent.New(provider, tools, agent.Config{
		MaxTurns: 8,
		Timeout:  30 * time.Second,
	})
	fmt.Println("Running agent...")
	fmt.Println("--------------------------------")

	userMsg := "can you recomened a 5 day trip to japan. based on the weather in tokyo."
	out, err := a.Run(ctx, userMsg)
	if err != nil {
		fatalInferenceErr(err)
		return
	}

	if out.Reasoning != "" {
		fmt.Println("reasoning:")
		fmt.Println(out.Reasoning)
	}
	fmt.Println("user message:")
	fmt.Println(userMsg)

	fmt.Println("--------------------------------")
	fmt.Println("assistant message:")
	fmt.Println(out.Content)
}

func fatalInferenceErr(err error) {
	if errors.Is(err, context.Canceled) {
		fmt.Println("cancelled")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("timed out")
		return
	}
	if errors.Is(err, agent.ErrMaxTurns) {
		fmt.Println("max turns exceeded")
		return
	}
	log.Fatal(err)
}
