# CreatorWorkflow Harness Interview Notes

## One-line positioning

CreatorWorkflow Harness is a Go + Eino + MiniMax planning workflow harness for AI video creation. It does not claim to own the final video model; it proves that the planning layer can be evaluated, traced, replayed, repaired, and compared across prompt/model versions.

## Why this is more than a demo agent

A 60-point agent can call a model and a tool once. This project focuses on the 90-point engineering questions:

- How do we know a planning output is better?
- Which node caused a bad result?
- Did a prompt/model change regress older cases?
- Can we replay the same input and inspect trace artifacts?
- Can we quantify fallback, repair, latency, and LLM node success?

## Harness design

- Dataset: `tests/evals/prompts.jsonl` contains commercial/story/tutorial cases in English and Chinese, with and without assets.
- Runner: `cmd/eval-runner` executes the Eino planner over the dataset.
- Evaluator: `internal/eval` scores prompt type, keywords, language, shots, duration, repair, fallback, LLM node success, and latency.
- Artifact: each case stores input, plan, planning trace, callback events, score, and error.
- Replay: `cmd/trace-replay` inspects artifacts or reruns the original input.
- Compare: `cmd/eval-compare` reports score, fallback, repair, latency, regressed cases, and improved cases.

## How to explain missing real video generation

The project boundary is planning workflow reliability, not video model quality. The worker calls `VideoGenerator`, and the default implementation is `MockGenerator`. This is deliberate: the final video model can be replaced by HTTP/gRPC provider code without changing planning, state machine, queue, idempotency, storage, or eval harness.

## Demo commands

```bash
go test ./...
go run ./cmd/eval-runner --dataset tests/evals/prompts.jsonl --out artifacts/eval-runs --run-id local_memory
go run ./cmd/trace-replay --artifact artifacts/eval-runs/local_memory/cases/commercial_001.json --mode inspect
go run ./cmd/eval-compare --base artifacts/eval-runs/run_a/report.json --candidate artifacts/eval-runs/run_b/report.json
```

Strict MiniMax demo:

```powershell
go run ./cmd/eval-runner `
  --dataset tests/evals/prompts.jsonl `
  --out artifacts/eval-runs `
  --llm-config C:\path\to\config.local.json `
  --llm-required
```

## Resume bullets

```latex
\item 基于 Go + Eino StateGraph 构建 AI 视频创作 planning workflow，接入 MiniMax 真实生成角色、场景、分镜和台词，并通过 Branch/ToolsNode 将素材状态、创作类型和时长估算纳入规划链路。
\item 设计轻量级 Agent Harness，基于 prompt dataset 批量评测关键词命中、语言一致性、分镜结构、时长偏差、repair 成功率、fallback 率和节点耗时，支持 prompt/model 版本回归对比。
\item 基于 planning trace、Callback events 和 trace artifact 记录 LLM 节点、工具调用、错误、耗时和模型来源，结合 replay 与 repair loop 定位并修复偏题、缺关键词和时长异常问题。
```
