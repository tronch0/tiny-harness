# Tiny Harness

Educational, minimal agent harness. Follow [Pi](https://github.com/badlogic/pi-mono) layering, takes insperation from the minimal design of Pi.

Push features down only when a higher layer needs them. Do not put the agent loop, tools, or session state in a provider.

## Current contract (`ai`)

- `Provider`: `Complete` and `Stream`
- `Input` / `Output` / `Message` — not HTTP `Request` / `Response`
- `Input.Tools` — schema-only `Tool` (name, description, parameters); no Execute
- `Output.ToolCalls` — model-requested calls (`ID`, `Name`, `Arguments`)
- `Output.Usage` — `PromptTokens` / `CompletionTokens` for that call (zero if the provider omitted them)
- `Message` may carry `ToolCalls` (assistant) or `ToolCallID` (role=`tool` result)
- `Stream` + `Delta` (reasoning and content tokens); `Collect` / `CollectFunc` drain a stream
- Streaming assembles `ToolCalls` (OpenAI-style incremental `delta.tool_calls`) and `Usage` from a final chunk (`stream_options.include_usage`)
- `context.Context` is cancel/timeout only. Never put it on `Input`. Never name a type `Context`.

Executable tools live in `tools` (`Tool` + `Execute`). Providers map schemas/calls only.

`agent.Config` bounds a `Run`: `MaxTurns` (0 = unlimited), `Timeout` (0 = caller `ctx` only), `SystemPrompt` (prepended each `Complete`, not stored in history). `ErrMaxTurns` if the loop still has tool calls after the cap.

Provider config (`address`, `model`, HTTP client) lives on the provider struct. 

## Style

- Slim Go. Stdlib only unless a dependency is clearly needed.
- One concern per package. No speculative abstractions.
- Tests next to the code they cover.
