# Tiny Harness

Educational, minimal agent harness. Follow [Pi](https://github.com/badlogic/pi-mono) layering, takes insperation from the minimal design of Pi.

Push features down only when a higher layer needs them. Do not put the agent loop, tools, or session state in a provider.

## Current contract (`ai`)

- `Provider`: `Complete` and `Stream`
- `Input` / `Output` / `Message` — not HTTP `Request` / `Response`
- `Stream` + `Delta` (reasoning and content tokens); `Collect` drains a stream
- `context.Context` is cancel/timeout only. Never put it on `Input`. Never name a type `Context`.

Provider config (`address`, `model`, HTTP client) lives on the provider struct. 

## Style

- Slim Go. Stdlib only unless a dependency is clearly needed.
- One concern per package. No speculative abstractions.
- Tests next to the code they cover.
