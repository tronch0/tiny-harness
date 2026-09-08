# Tiny Harness roadmap

Educational, minimal agent harness. Push features down only when a higher layer needs them. Do not put the agent loop, tools, or session state in a provider.

## Done

```text
ai/        Complete / Stream, schemas, tool_calls
tools/     Tool + Find + Run
agent/     history, execute, inner loop, Stream + OnDelta, Config
main.go    stdin REPL + live tokens + demo tool
```

Inner loop: user → model → tools → model → stop.
Outer loop: stdin → `Run` on the same `Agent` → print → repeat.

---

## Phase 1 — what’s left

Thin slices, in this order. Each should stay usable without the later ones.

### 1. Outer loop (REPL) — done

`main` reads stdin and calls `Run` again on the **same** `Agent`. History already persists. No new package.

### 2. System prompt — done

One string on `Config`, prepended as `role=system` on each `Complete`. Not stored in `Messages()`.

### 3. Streaming on the agent — done

`Run` uses `Stream` + `CollectFunc`. `OnDelta` prints tokens. Streamed `tool_calls` are assembled so the inner loop still works. `Complete` remains on the provider for tests / non-stream callers.

### 4. Real tools

`get_weather` / echo proved the contract. Next tools are app-defined, Pi-style:

- `read` / `write` / `edit` / `bash` (or a subset)
- Stay in `tools` or `main` as values of `tools.Tool` — no Manager
- Safety (cwd, allowlists) only as much as a tool needs to run

### 5. Sessions

Persist `messages` (JSONL is enough) so a chat can restart.

- Harness concern: load/save around the agent, not inside the provider
- After the REPL is real enough that you care about losing history

---

## Phase 2 — skipped for now

Do these only after Phase 1 is in daily use. They are easy to over-build.

### Events

Pi emits `turn_start`, `message_update`, `tool_execution_*`. Useful for a TUI. Until then, `onDelta` + printing in `main` is enough. Do not add an event bus to `agent` first.

### Tool Manager

A registry type is extra structure. The agent already holds `[]tools.Tool` and `Find`. Add a Manager only if you need permissions, enable/disable, or dynamic loading.

### Compaction

Prune or summarize old messages when the context window fills. Needs a working multi-turn REPL (and maybe sessions) so you can see the problem. Lives on the agent (or a `transform` hook), not the provider.

### Parallelism

Run several `tool_calls` from one assistant message concurrently. Sequential is correct and simpler. Parallel only if tools are slow and independent.

### Extra providers

Anthropic / others implement `ai.Provider` the same way OpenAI does (`Complete` / `Stream`, map schemas and `tool_calls`). Add a vendor when you have a second backend to talk to — not as an abstraction exercise.

---

## Out of scope unless needed

Retries, per-tool timeouts, token/usage accounting, temperature and sampling on `Input`, branching session trees, a TUI package. Same rule: a higher layer must need them first.
