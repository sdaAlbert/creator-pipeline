# CreatorPipeline

CreatorPipeline is a production-style AI creation workflow backend built with Go and Eino. It turns a user prompt and optional media assets into a structured creation plan, persists the task, dispatches it asynchronously, stores the generated result, and exposes task state plus operational metrics.

The project focuses on the hard parts of AI backend systems: stateful LLM workflow orchestration, branchable planning, tool-backed generation, semantic quality checks, repair loops, long-running task state management, idempotency, retries, cancellation, and observability.

## Why This Project Exists

AI creation is rarely a single model call in production. A real service has to split a vague prompt into structured intermediate results, call backend tools, validate whether the output is usable, repair bad plans, persist state, execute long-running generation jobs, and expose enough trace data to debug failures.

This repository demonstrates that flow end to end:

- Eino builds a stateful AI planning graph instead of a one-shot prompt.
- The API turns the plan into a durable creation task.
- The worker executes the task asynchronously with retry, timeout, and cancellation support.
- MySQL, RabbitMQ, Redis, MinIO, and Prometheus are wired in real infrastructure mode.
- The final media generation service is intentionally mocked, so the repository stays runnable without a private video model.

## Architecture

```text
Client
  -> POST /api/v1/creations
  -> Go API
  -> Eino StateGraph planner
       -> prompt classification
       -> asset metadata tools
       -> role planning
       -> scene planning
       -> storyboard branch
       -> duration estimation tool
       -> semantic quality check
       -> repair loop
       -> dialogue generation
  -> MySQL creation_tasks
  -> RabbitMQ creator.generation
  -> Worker
  -> mock video generation service
  -> MinIO result object
  -> CDN-style result_url

Prometheus scrapes /metrics for task throughput, status counts, P95 latency,
worker activity, model failures, and queue backlog.
```

## Eino Workflow

The core workflow is implemented as an Eino graph in `internal/eino/graph.go`.

```text
init_state
  -> classify_prompt
  -> Branch: has assets ? asset_tools : roles
  -> roles
  -> scenes
  -> Branch: commercial/story/tutorial storyboard
  -> duration_tools
  -> semantic_quality_check
  -> Branch: pass ? dialogues : repair_shots
  -> final_plan
```

Implemented Eino capabilities:

- `StateGraph`: stores prompt type, assets, roles, scenes, shots, tool results, quality signals, repair attempts, and trace data in structured graph state.
- `Branch`: chooses different paths based on asset availability, prompt type, and quality result.
- `ToolsNode`: wraps backend-like tools such as asset metadata inspection and video duration estimation.
- `Callbacks`: records node lifecycle events for observability.
- `Repair loop`: fixes weak storyboards before the plan is accepted.
- Real `ChatModel`: MiniMax is used for role, scene, storyboard, and dialogue planning when `LLM_CONFIG_PATH` is provided. Set `LLM_REQUIRED=true` to fail fast instead of falling back to local deterministic planning.

The semantic quality check validates more than shape:

- storyboard count and duration distribution
- whether the plan still matches the original prompt
- whether important prompt keywords appear in the plan
- whether the generated language matches the prompt language

If the plan fails, `repair_shots` adjusts duration and injects missing prompt keywords as semantic anchors.

## Planning Trace Example

The API response includes `plan.planning_trace` and `plan.callback_events`, so a bad result can be located by graph node.

```json
{
  "task_id": "task-123",
  "status": "pending",
  "plan": {
    "prompt_type": "commercial",
    "quality": {
      "passed": true,
      "score": 88,
      "missing_keywords": ["light", "city"],
      "repair_attempts": 1
    },
    "planning_trace": [
      {"node": "init_state", "source": "state_graph", "duration_ms": 0},
      {"node": "classify_prompt", "source": "rule", "duration_ms": 0},
      {"node": "asset_tools", "source": "tools_node", "duration_ms": 3},
      {"node": "roles", "source": "llm", "duration_ms": 10340},
      {"node": "scenes", "source": "llm", "duration_ms": 30311},
      {"node": "commercial_storyboard", "source": "llm", "duration_ms": 15870},
      {"node": "duration_tools", "source": "tools_node", "duration_ms": 4},
      {"node": "semantic_quality_check", "source": "semantic_rule", "duration_ms": 0},
      {"node": "repair_shots", "source": "repair_loop", "duration_ms": 0},
      {"node": "dialogues", "source": "llm", "duration_ms": 19927}
    ],
    "callback_events": [
      {"node": "commercial_storyboard", "component": "Lambda", "event": "start"},
      {"node": "commercial_storyboard", "component": "Lambda", "event": "end", "duration_ms": 15870}
    ]
  }
}
```

## API

Create a task:

```http
POST /api/v1/creations
Content-Type: application/json

{
  "user_id": "u1",
  "prompt": "Create a commercial ad for a night riding safety light in the city",
  "idempotency_key": "demo-1",
  "assets": [
    {"object_key": "uploads/light.png", "kind": "image"},
    {"object_key": "uploads/night-ride.mp4", "kind": "video"}
  ]
}
```

Query a task:

```http
GET /api/v1/tasks/{task_id}
```

Retry a failed task:

```http
POST /api/v1/tasks/{task_id}/retry
```

Cancel a pending or running task:

```http
POST /api/v1/tasks/{task_id}/cancel
```

Metrics:

```http
GET /metrics
```

## Run Locally

Memory mode is the default. It uses in-memory repository, queue, idempotency store, and mock object storage.

```bash
go run ./cmd/api
```

Then create a task:

```bash
curl -X POST http://localhost:8080/api/v1/creations \
  -H "Content-Type: application/json" \
  -d '{"user_id":"u1","prompt":"Create a commercial ad for a night riding safety light in the city","idempotency_key":"demo-1"}'
```

## Run With Real Infrastructure

Start MySQL, RabbitMQ, Redis, MinIO, and Prometheus:

```bash
docker compose -f deployments/docker-compose.yml up -d mysql rabbitmq redis minio prometheus
```

Start the API:

```bash
INFRA_MODE=real go run ./cmd/api
```

PowerShell:

```powershell
$env:INFRA_MODE = "real"
go run ./cmd/api
```

Default local endpoints:

| Service | Endpoint |
| --- | --- |
| API | `http://localhost:8080` |
| MySQL | `localhost:3306` |
| RabbitMQ | `amqp://guest:guest@localhost:35672/` |
| RabbitMQ UI | `http://localhost:15672` |
| Redis | `localhost:6379` |
| MinIO API | `http://localhost:9000` |
| MinIO Console | `http://localhost:9001` |
| Prometheus | `http://localhost:9090` |

RabbitMQ still listens on `5672` inside the container. The host port is published as `35672` to avoid common local port reservations on Windows. Set `RABBITMQ_URL` if you want to use a different local port.

Real infrastructure mode uses:

- MySQL table `creation_tasks` with task state, retry count, error message, timestamps, deadline, and `plan_json`.
- Unique key `(user_id, idempotency_key)` as the durable idempotency guard.
- Transaction plus `SELECT ... FOR UPDATE` when worker code claims a task.
- Index `(status, updated_at)` for timeout scans and compensation jobs.
- Redis key `creator:idempotency:{key}` as a fast idempotency cache.
- RabbitMQ queue `creator.generation` for async execution.
- MinIO bucket `creator-results` for generated task output.

## MiniMax Planner Mode

Without an LLM config, the planner can use deterministic local generation so the project remains runnable in CI and local demos. For a real AI workflow run, provide `LLM_CONFIG_PATH` and set `LLM_REQUIRED=true`.

In strict mode, the API fails task creation if any required MiniMax planning node fails. This prevents a demo from silently passing through fallback logic.

PowerShell:

```powershell
$env:INFRA_MODE = "real"
$env:LLM_CONFIG_PATH = "C:\path\to\config.local.json"
$env:LLM_REQUIRED = "true"
go run ./cmd/api
```

Expected config shape:

```json
{
  "provider": "minimax",
  "base_url": "https://api.minimaxi.com/anthropic",
  "model": "MiniMax-M2.7",
  "api_key": "..."
}
```

The API key is read from the local file only. It is not logged and is not returned in API responses.

Validated strict MiniMax planning trace:

```text
roles=llm
scenes=llm
commercial_storyboard=llm
duration_tools=tools_node
semantic_quality_check=semantic_rule
dialogues=llm
```

Example strict-mode E2E result:

```text
FinalStatus: succeeded
PromptType: commercial
QualityScore: 97
ResultHTTP: 200
MySQL status: succeeded
Redis idempotency: hit
RabbitMQ creator.generation messages: 0
```

## Tests

Run unit tests:

```bash
go test ./...
```

Current graph tests cover:

- commercial branch
- tutorial branch
- asset tools branch
- repair branch
- semantic quality check for missing prompt keywords

Run a simple load test after starting the API:

```bash
k6 run tests/k6/submit_tasks.js
```

## Project Structure

```text
cmd/api                 HTTP API and embedded worker startup
cmd/worker              Worker entry placeholder
deployments             Docker Compose and Prometheus config
internal/app            Creation API service
internal/config         Environment config
internal/eino           Eino planning graph, schema, and tests
internal/idempotency    Memory and Redis idempotency stores
internal/llm            MiniMax config and chat client adapter
internal/metrics        Prometheus text metrics registry
internal/queue          Memory and RabbitMQ queues
internal/storage        Mock and MinIO result storage
internal/task           Task model, state machine, memory/MySQL repositories
internal/worker         Async generation worker
migrations              MySQL schema
tests/k6                Concurrent task submission script
```

## Production Boundaries

This project intentionally keeps the final video model service as a mock implementation. The purpose is to show how an AI creation backend coordinates planning, task state, failure recovery, object storage, and observability. The mock worker can be replaced by a real HTTP or gRPC video generation service without changing the API contract or task state machine.
