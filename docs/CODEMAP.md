# CODEMAP — "To change X, edit Y"

> For the AI pipeline flow (phases, agent tool-loop, when RAG is called, sequence diagram) see
> [`AI_PIPELINE.md`](AI_PIPELINE.md).


A navigation index so you can jump straight to the right file instead of searching. Pair with the
package map in the root `CLAUDE.md`. Keep this current as new packages land.

## Backend (Go)

| I want to change… | Edit |
|---|---|
| The review pipeline / phase orchestration | `internal/review/engine.go` (+ `phases.go`, `mapping.go`, `filters.go`) |
| The agentic review loop / step budget / dedup | `internal/agent/agent.go` |
| Tools the agent can call (schemas) | `internal/agent/tools.go` |
| The agent system prompt / initial context | `internal/agent/prompt.go` |
| RAG chunking / embeddings / Qdrant / retrieval | `internal/rag/{chunk,embed,qdrant,service}.go` |
| Where changed files get indexed for RAG | `internal/review/engine.go` (`indexForRAG`) |
| Confidence/severity filtering, dedup, reflection application | `internal/review/severity.go`, `internal/review/filters.go` |
| LLM request/response, retries, providers | `internal/llm/client.go` |
| Tool-calling wire format (OpenAI + Anthropic) | `internal/llm/tools.go` |
| Model pricing / per-review cost (`CostUSD`, `Usage`) | `internal/llm/pricing.go` |
| Per-phase model tiering resolution | `internal/review/engine.go` (`resolveModels`, `SetModelTiers`) |
| Prompt text (triage/review/reflection) & injection defense | `internal/llm/prompt.go` |
| LLM data types (findings, triage, validation, tool types) | `internal/llm/types.go`, `internal/llm/tools.go` |
| MCP tool calls (get PR, files, diff, post review, search) | `internal/mcp/client.go` |
| Our own MCP server (protocol loop) | `internal/mcpserver/server.go` |
| Tools our MCP server exposes | `internal/mcpserver/tools.go` |
| MCP server binary / wiring | `cmd/mcp-server/main.go` |
| Eval scoring (precision/recall/F1, matching) | `internal/eval/score.go` |
| Eval dataset types + loader | `internal/eval/dataset.go` |
| Eval runner / report / threshold gate | `internal/eval/runner.go` |
| Agent-backed eval reviewer (fixture files) | `internal/eval/agentreviewer.go` |
| Golden eval cases | `evals/*.json` |
| Eval CLI | `cmd/eval/main.go` (`make eval`) |
| Static analysis rules | `internal/analyzer/` |
| Repo context passed to the reviewer | `internal/repository/` |
| HTTP routes | `internal/api/router.go` |
| Metrics registry / Prometheus exposition | `internal/metrics/metrics.go` |
| App metric definitions + record helpers | `internal/metrics/app.go` |
| HTTP metrics / rate-limit / webhook-dedup middleware | `internal/api/middleware/{metrics,ratelimit}.go` |
| CI pipeline | `.github/workflows/ci.yml` |
| An HTTP handler's behavior | `internal/api/handlers/*.go` |
| Agent-trace / execution-log read API | `internal/services/review_service.go` (`ListExecutionLogs`) + `GET /api/reviews/:id/logs` |
| Live activity feed API | `internal/services/dashboard_service.go` (`ListRecentActivity`) + `GET /api/dashboard/activity` |
| How a handler gets its dependencies (closures/DI) | `cmd/server/main.go` |
| Async job execution / worker count / retries | `internal/jobs/` |
| Webhook parsing / HMAC verification | `internal/webhook/` |
| DB queries / persistence | `internal/services/*.go` |
| Domain models & DTOs | `internal/models/` |
| Config / env vars / validation | `internal/config/` |
| DB schema | `migrations/*.sql` (add a new numbered pair) |
| Error types / HTTP status mapping | `pkg/errors/errors.go` |
| Logging setup | `pkg/logger/logger.go` |

## Frontend (React + TS) — `frontend/src/`

| I want to change… | Edit |
|---|---|
| Routing / top-level shell | `App.tsx` |
| A page | `pages/` |
| A reusable component | `components/` |
| Data fetching hooks | `hooks/useApi.ts` (mock is fallback-only) |
| API client + TS types | `lib/api.ts`, `lib/types.ts` |
| Agent-trace timeline component | `components/Review/AgentTrace.tsx` (icons, status colors, collapsible) |
| Live activity feed (dashboard sidebar, 5s poll) | `components/Dashboard/ActivityFeed.tsx` |
| Pipeline page (rendered Mermaid flowcharts) | `pages/Pipeline.tsx`, `lib/pipelineDiagrams.ts`, `components/common/MermaidDiagram.tsx` (lazy-loaded route) |
| Shared formatters (time-ago, step labels) | `lib/format.ts` |
| Cost display | `pages/PullRequestDetail.tsx`, `pages/Reviews.tsx`, `pages/Dashboard.tsx` (cost StatsCard) |

## Ops

| Task | File |
|---|---|
| Add/adjust a service (db, redis, qdrant, embeddings) | `docker-compose.yml` |
| Backend image build | `Dockerfile` |
| Dev commands | `Makefile` |
| Runtime config template | `.env.example` |

## Status

All planned workstreams (0, A–F) are implemented. Future ideas: prompt-cache instrumentation,
multi-tenant auth, distributed (Redis-backed) rate limiting, frontend code-splitting.

_Done: `internal/agent/` (WS-A) · `internal/rag/` (WS-B) · `internal/llm/pricing.go` + tiering + cost
(WS-C) · `internal/mcpserver/` + `cmd/mcp-server/` (WS-D) · `internal/eval/` + `evals/` + `cmd/eval`
(WS-E)._
