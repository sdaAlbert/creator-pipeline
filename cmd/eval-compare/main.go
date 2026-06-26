package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	evalh "creator-pipeline/internal/eval"
)

func main() {
	basePath := flag.String("base", "", "base report.json path")
	candidatePath := flag.String("candidate", "", "candidate report.json path")
	jsonOut := flag.Bool("json", false, "print JSON output")
	flag.Parse()
	if *basePath == "" || *candidatePath == "" {
		log.Fatal("--base and --candidate are required")
	}
	base, err := evalh.ReadReport(*basePath)
	if err != nil {
		log.Fatalf("read base report: %v", err)
	}
	candidate, err := evalh.ReadReport(*candidatePath)
	if err != nil {
		log.Fatalf("read candidate report: %v", err)
	}
	cmp := evalh.Compare(base, candidate)
	if *jsonOut {
		b, _ := json.MarshalIndent(cmp, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Printf("base=%s candidate=%s\n", cmp.BaseRunID, cmp.CandidateRunID)
	fmt.Printf("score_delta=%.4f fallback_rate_delta=%.4f repair_success_delta=%.4f p95_latency_delta_ms=%d\n", cmp.ScoreDelta, cmp.FallbackRateDelta, cmp.RepairSuccessDelta, cmp.P95LatencyDeltaMS)
	fmt.Printf("regressed_cases=%d improved_cases=%d\n", len(cmp.RegressedCases), len(cmp.ImprovedCases))
	for _, c := range cmp.RegressedCases {
		fmt.Printf("REGRESSED %s %.4f -> %.4f delta=%.4f\n", c.CaseID, c.BaseScore, c.CandidateScore, c.Delta)
	}
	for _, c := range cmp.ImprovedCases {
		fmt.Printf("IMPROVED %s %.4f -> %.4f delta=%.4f\n", c.CaseID, c.BaseScore, c.CandidateScore, c.Delta)
	}
}
