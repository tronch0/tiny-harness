package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tiny-harness/inference-providers/openai"
	"tiny-harness/models"
)

func main() {
	provider := openai.NewOpenAIProvider(
		"http://192.168.50.65:11434",
		`C:\Users\User\models\qwen3.8\Qwen3.8-27B-UD-Q5_K_M.gguf`,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stream, err := provider.Stream(ctx, &models.Input{
		Messages: []models.Message{
			// {Role: "user", Content: "Say hello in one short sentence."},
			{Role: "user", Content: "can you recomened a 5 day trip to japan."},
		},
	})
	if err != nil {
		fatalInferenceErr(err)
		return
	}

	printedReasoning := false

	for delta := range stream.Deltas() {
		if delta.ReasoningDelta != "" {
			if !printedReasoning {
				fmt.Println("reasoning:")
				printedReasoning = true
			}
			fmt.Print(delta.ReasoningDelta)
		}
		if delta.ContentDelta != "" {
			if printedReasoning {
				fmt.Println()
				fmt.Println("content:")
				printedReasoning = false
			}
			fmt.Print(delta.ContentDelta)
		}
	}
	fmt.Println()

	if err := stream.Err(); err != nil {
		fatalInferenceErr(err)
	}
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
	log.Fatal(err)
}
