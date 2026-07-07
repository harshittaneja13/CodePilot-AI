# CLAUDE.md — CodePilot AI

> Read this first. It is the map of the repo so you don't have to re-explore each session.
> For deeper detail see `docs/CODEMAP.md` (navigation), `docs/AI_PIPELINE.md` (pipeline flowcharts +
> tool/RAG blueprint), and `docs/architecture.md` (design).
> Keep this file **short** — it loads every turn.

## What this is

**CodePilot AI** — an automated GitHub PR-review agent. A GitHub webhook triggers a review; the
backend gathers PR context via the **GitHub MCP server**, runs an LLM review pipeline, posts inline
comments back to the PR, and stores results for a React dashboard.

Stack: **Go 1.23 (Gin)** backend · **React 18 + Vite + TS** frontend · **PostgreSQL** (data + durable
job queue) · **Redis** (provisioned, not yet used) · **Docker Compose**. LLM is provider-agnostic via
`LLM_*` env (currently **Groq**, `llama-3.3-70b-versatile`, 12k TPM free tier).

## Package map (`internal/`)

| Package | Responsibility |
|---|---|
| `api/` + `api/handlers/` | Gin router and HTTP handlers |
| `config/` | Load + validate env config (`config.Load`) |
| `database/` | Postgres connection + migration runner |
| `jobs/` | Durable Postgres-backed job queue (2 workers) — runs reviews async |
| `llm/` | LLM client (`client.go`), tool-calling (`tools.go`), prompts (`prompt.go`), types (`types.go`) |
| `mcp/` | MCP **client** over stdio JSON-RPC to the GitHub MCP server; also direct GitHub REST fallbacks |
| `mcpserver/` | MCP **server** (stdio) exposing CodePilot's own tools (get_review, list_findings, search_reviews, retrieve_code_context); binary at `cmd/mcp-server` |
| `agent/` | Autonomous tool-use review loop (`agent.go`, `tools.go`) — the preferred Phase-2 reviewer |
| `rag/` | Semantic code retrieval — chunk/embed/index into Qdrant, retrieve (`service.go`, `embed.go`, `qdrant.go`, `chunk.go`); optional, gated by `RAG_ENABLED` |
| `eval/` | Offline eval harness — scores agent findings vs a golden set (`evals/*.json`) by precision/recall/F1; runner at `cmd/eval` (`make eval`) |
| `review/` | **Review engine** — orchestrates the pipeline (`engine.go`, `phases.go`, `mapping.go`, `filters.go`) |
| `analyzer/` | Deterministic static analysis (patterns/complexity) |
| `repository/` | Builds repo context for a review |
| `services/` | Data-access layer (repository / PR / review / dashboard) |
| `models/` | Domain models + request/response DTOs |
| `webhook/` | GitHub webhook parsing + HMAC-SHA256 verification |
| `metrics/` | Dependency-free Prometheus-text metrics registry; served at `/api/metrics` |
| `pkg/errors` | Typed, HTTP-mappable errors (`apperrors`) |
| `pkg/logger` | Structured logging (zerolog), `logger.Log` |

## Entrypoint & flow

`cmd/server/main.go` wires everything (config → db+migrations → services → MCP client → LLM client →
review engine → job queue → webhook processor → Gin router → HTTP server w/ graceful shutdown).
Handlers get dependencies as **closures** defined in `main.go` (e.g. `triggerReview`, `syncPRs`).

Review pipeline (`internal/review/engine.go`, `ProcessPullRequest`):
1. **Triage** — LLM picks ≤8 priority files (skipped if PR < 4 files).
2. **Review** — the **agent** (`internal/agent`) runs a bounded tool-use loop (read files,
   search code, read diffs, and — when `RAG_ENABLED` — `retrieve_context` semantic search)
   then submits findings. Falls back to a single-shot LLM review (`runReview`) when the model
   lacks tool support or the agent fails. Gate: `llmClient.SupportsTools()`.
3. **Reflection** — 2nd LLM call validates; drops findings with confidence < 0.7 or `is_valid=false`.

**Cost & tiering:** each phase can use a different model (`LLM_TRIAGE/REVIEW/REFLECTION_MODEL`,
resolved in `engine.resolveModels`). Per-review input/output tokens and USD cost (`internal/llm`
`Usage` + `CostUSD`) are accumulated per phase and stored on the review (`cost_usd`, migration 004).

Key routes: `POST /api/webhooks/github` (HMAC-verified, delivery-ID deduped), `GET /api/health`,
`GET /api/metrics` (Prometheus), `GET /api/reviews/:id/logs` (agent trace),
`GET /api/dashboard/activity` (live feed), plus repos/PRs/reviews/analytics REST under `/api`
(see `internal/api`).
Global middleware: recovery, logging, metrics, per-IP rate limiting, CORS (`internal/api/middleware`).

## Conventions

- **Errors:** construct via `pkg/errors` (`apperrors.NewNotFound/NewValidation/NewInternal/…`); map to
  HTTP with `apperrors.ToHTTPStatus`. Wrap internal errors with `%w`.
- **Logging:** zerolog via `pkg/logger`; use request-scoped `log.With().Str(...).Logger()`.
- **LLM structured output:** JSON mode via `ChatWithModelJSON`; parse with the `llm.ParseXxxResponse`
  helpers (they tolerate text via `extractJSON`). Prompts defend against injection with XML delimiters.
- **DB access** goes through `internal/services`, not raw SQL in handlers.
- **Config** is env-only (`.env` / `docker-compose`); never hardcode secrets.

## Commands

```bash
make run            # run backend (needs Postgres+Redis)   make test   # go test -race -cover ./...
make lint           # golangci-lint                        make build  # compile ./cmd/server
make docker-up      # full stack via compose               make migrate-up
make frontend-dev   # Vite dev server                      make build-mcp  # MCP server binary
make eval           # offline review-quality eval (needs LLM_API_KEY)
```

## Working here efficiently

- Prefer editing the **smallest relevant file**; consult `docs/CODEMAP.md` to find it instead of grepping.
- For broad/unfamiliar searches, use an **Explore subagent** so exploration stays out of the main context.
- Ongoing effort + decisions: `/Users/harshittaneja/.claude/plans/plan-all-the-improvements-fuzzy-waterfall.md`
  (adds: tool-use agent loop, RAG via Qdrant, cost tracking, own MCP server, evals, CI/observability).
