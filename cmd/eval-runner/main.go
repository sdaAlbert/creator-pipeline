package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	creator "creator-pipeline/internal/eino"
	evalh "creator-pipeline/internal/eval"
	"creator-pipeline/internal/llm"
)

func main() {
	datasetPath := flag.String("dataset", "tests/evals/prompts.jsonl", "JSONL eval dataset path")
	outDir := flag.String("out", "artifacts/eval-runs", "output directory for eval artifacts")
	llmConfigPath := flag.String("llm-config", "", "optional MiniMax config path")
	llmRequired := flag.Bool("llm-required", false, "require all LLM planning nodes to use MiniMax without fallback")
	runID := flag.String("run-id", "", "optional stable eval run id")
	flag.Parse()

	ctx := context.Background()
	cases, err := evalh.LoadDataset(*datasetPath)
	if err != nil {
		log.Fatalf("load dataset: %v", err)
	}

	if *llmRequired && *llmConfigPath == "" {
		log.Fatal("--llm-required requires --llm-config")
	}

	var filler creator.JSONFiller
	mode := "memory"
	model := "deterministic-fallback"
	if *llmConfigPath != "" {
		cfg, err := llm.LoadConfig(*llmConfigPath)
		if err != nil {
			log.Fatalf("load llm config: %v", err)
		}
		if err := cfg.ValidateMiniMax(*llmRequired); err != nil {
			log.Fatalf("validate minimax config: %v", err)
		}
		filler = llm.NewMiniMaxClient(cfg)
		mode = "minimax"
		if *llmRequired {
			mode = "minimax-strict"
		}
		model = cfg.Model
	}

	planner, err := creator.NewPlanner(ctx, filler, creator.WithRequiredLLM(*llmRequired))
	if err != nil {
		log.Fatalf("build planner: %v", err)
	}
	report, err := evalh.RunSuite(ctx, planner, cases, evalh.RunConfig{RunID: *runID, Mode: mode, Model: model, OutDir: *outDir})
	if err != nil {
		log.Fatalf("run eval suite: %v", err)
	}
	fmt.Printf("eval run %s complete: passed=%d/%d score=%.4f report=%s/%s/report.md\n", report.RunID, report.Summary.PassedCases, report.Summary.TotalCases, report.Summary.AverageScore, *outDir, report.RunID)
}
