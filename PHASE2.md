# Statechart Integration: Phase 2 (Guards & Actions)

## Current Status (Post-MVP)

- YAML parsing → `statechartx.Machine` complete (`registry/statechart/spec.go`).
- Registry detects/parses statecharts (`registry/registry.go` List()).
- API MVP: list machines, create instances (in-memory), send events (stubbed responses) (`api/v1/statecharts.go`).
- E2E tests pass (`e2e_tests.sh`).
- Stubs: Guards/actions always-true/noop (log only). No real Runtime.Current/Send (unexported/missing in shims).
- `yaml/trafficlight.yaml` demo works (curls list/create/send).

## Phase 2 Goal

Replace stubs with **real guards/actions**:

- **Guards**: `github.com/antonmedv/expr` VM eval vs `ctx` (state data) + `evt.Data`.
- **Actions**: LLM calls via `config.LLM` (prompt: \"Execute action '{name}' in {from}→{to} on {evt} with ctx {ctx}\").
- Fix Runtime integration: Export/access `Current()`, `Send(Event{ID, Data})`.
- Persistence: In-memory → per-instance JSON file (`instances/{machine}/{iid}.json`).
- Defer: Timers (`time.AfterFunc`), parallel regions, history states.

## Implementation Steps

1. **Add deps** (`go.mod`):

   ```
   go get github.com/antonmedv/expr/v2
   ```

2. **Fix Runtime shims** (`registry/statechart/spec.go` / new `runtime.go`):
   - Export `rt.current` → `Current() StateID`.
   - `Send(evt *Event)` → process event, guards/actions → transition.
   - `Event{ID string, Data any}` (ID=type).
   - Store history in ctx.

3. **Real Guards** (`spec.go:resolveGuard`):

   ```
   expr.Compile(expr, expr.Env{ctx: map[string]any, evt: map[string]any})
   env := map[string]any{"ctx": statechartx.FromContext(ctx), "evt": evt.Data}
   out, err := prog.Run(env)
   return out.(bool), err
   ```

4. **LLM Actions** (new `internal/llm/client.go`):
   - HTTP client to `config.LLM.Endpoint` (OpenAI/Anthropic).
   - Prompt template: `actions/{name}.prompt` or inline.
   - Exec: `POST /chat/completions` → parse response → `ctx` updates.

5. **Persistence** (`api/v1/statecharts.go`):
   - Instance dir: `instances/{machineID}/{instID}.json` (marshal ctx + history).
   - Load on create (if exists), Save on Send (after transition).

6. **Update E2E** (`e2e_tests.sh`):
   - Test guard fail (e.g. `guard: ctx.count > 3`), action log/output.
   - Restart server → instances persist.

## Critical Files

- `go.mod` (expr dep).
- `registry/statechart/spec.go` (guards/actions → real).
- New: `registry/statechart/runtime.go` (Runtime impl/shims).
- New: `internal/llm/client.go`.
- `api/v1/statecharts.go` (persistence, LLM config pass).
- `config/config.go` (LLM struct: Endpoint, APIKey, Model).
- `e2e_tests.sh` (guard/action/persist tests).
- Demo YAML: `yaml/counter.yaml` (guards: `ctx.count < 5`), `yaml/llm-action.yaml`.

## Verification

- Build/test: `go build ./cmd/server && go test ./...`.
- Run: `LISTEN_ADDR=:8080 LLM_ENDPOINT=... go run ./cmd/server`.
- Curl:

  ```
  # List/create/send (real transition)
  curl :8080/api/v1/statecharts/counter/instances -d'{"initialContext":{"count":0}}'
  curl -XPOST :8080/api/v1/statecharts/counter/instances/i1/events -d'{"id":"inc"}'  # Succeeds (guard true)
  curl ... -d'{"id":"inc"}' x6  # Fails (guard false)
  # Restart server → state persists
  ```

- Logs: Expr eval, LLM prompts/responses.

## Risks/Alternatives

- Risk: Expr unsafe (sandboxed, whitelist vars). LLM latency/cost (cache?).
- Alt Guards: Custom DSL < expr (secure/fast).
- Alt Actions: JS VM (otto) < LLM (flexible).
- Defer: Distributed persist (Redis), timers (channel queue).

Approve Phase 2?
