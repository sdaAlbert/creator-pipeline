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
	artifactPath := flag.String("artifact", "", "trace artifact JSON path")
	mode := flag.String("mode", "inspect", "inspect or full")
	fromNode := flag.String("from-node", "", "documentary marker for the node under investigation")
	llmConfigPath := flag.String("llm-config", "", "optional MiniMax config path for full replay")
	llmRequired := flag.Bool("llm-required", false, "require MiniMax during full replay")
	flag.Parse()
	if *artifactPath == "" {
		log.Fatal("--artifact is required")
	}
	a, err := evalh.ReadArtifact(*artifactPath)
	if err != nil {
		log.Fatalf("read artifact: %v", err)
	}
	if *fromNode != "" {
		fmt.Printf("focused node: %s\n", *fromNode)
	}
	if *mode == "inspect" {
		fmt.Print(evalh.InspectArtifact(a))
		return
	}
	if *mode != "full" {
		log.Fatalf("unsupported --mode %q", *mode)
	}

	ctx := context.Background()
	var filler creator.JSONFiller
	if *llmConfigPath != "" {
		cfg, err := llm.LoadConfig(*llmConfigPath)
		if err != nil {
			log.Fatalf("load llm config: %v", err)
		}
		if err := cfg.ValidateMiniMax(*llmRequired); err != nil {
			log.Fatalf("validate minimax config: %v", err)
		}
		filler = llm.NewMiniMaxClient(cfg)
	}
	planner, err := creator.NewPlanner(ctx, filler, creator.WithRequiredLLM(*llmRequired))
	if err != nil {
		log.Fatalf("build planner: %v", err)
	}
	plan, err := planner.Plan(ctx, creator.PromptInput{UserID: "trace-replay", Prompt: a.Input.Prompt, Assets: a.Input.Assets})
	result := evalh.ScoreCase(a.Input, plan, 0, err)
	if err != nil {
		log.Fatalf("full replay failed: %v", err)
	}
	fmt.Printf("full replay complete: case=%s new_run_id=%s score=%.4f passed=%t\n", a.CaseID, plan.RunID, result.Score, result.Passed)
	fmt.Print(evalh.InspectArtifact(evalh.BuildArtifact(plan.RunID, a.Input, plan, result, nil)))
}
