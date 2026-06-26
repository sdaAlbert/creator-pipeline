package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	creator "creator-pipeline/internal/eino"
)

func NewRunID() string {
	return fmt.Sprintf("eval_%s", time.Now().UTC().Format("20060102T150405Z"))
}

func WriteJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func WriteReportMarkdown(path string, report Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# CreatorWorkflow Eval Report\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", report.RunID)
	fmt.Fprintf(&b, "- Mode: `%s`\n", report.Mode)
	if report.Model != "" {
		fmt.Fprintf(&b, "- Model: `%s`\n", report.Model)
	}
	fmt.Fprintf(&b, "- Cases: `%d` passed / `%d` total\n", report.Summary.PassedCases, report.Summary.TotalCases)
	fmt.Fprintf(&b, "- Average score: `%.4f`\n", report.Summary.AverageScore)
	fmt.Fprintf(&b, "- P95 latency: `%d ms`\n\n", report.Summary.P95LatencyMS)

	fmt.Fprintf(&b, "## Summary Metrics\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n| --- | ---: |\n")
	fmt.Fprintf(&b, "| prompt_type_accuracy | %.4f |\n", report.Summary.PromptTypeAccuracy)
	fmt.Fprintf(&b, "| keyword_hit_rate | %.4f |\n", report.Summary.KeywordHitRate)
	fmt.Fprintf(&b, "| language_match_rate | %.4f |\n", report.Summary.LanguageMatchRate)
	fmt.Fprintf(&b, "| shot_count_valid_rate | %.4f |\n", report.Summary.ShotCountValidRate)
	fmt.Fprintf(&b, "| repair_success_rate | %.4f |\n", report.Summary.RepairSuccessRate)
	fmt.Fprintf(&b, "| fallback_rate | %.4f |\n", report.Summary.FallbackRate)
	fmt.Fprintf(&b, "| llm_node_success_rate | %.4f |\n", report.Summary.LLMNodeSuccessRate)
	fmt.Fprintf(&b, "| avg_latency_ms | %d |\n", report.Summary.AverageLatencyMS)
	fmt.Fprintf(&b, "| p95_latency_ms | %d |\n\n", report.Summary.P95LatencyMS)

	fmt.Fprintf(&b, "## Cases\n\n")
	fmt.Fprintf(&b, "| Case | Pass | Score | Keyword | Fallback | LLM Nodes | Latency | Failure |\n")
	fmt.Fprintf(&b, "| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "| `%s` | %t | %.4f | %.4f | %.4f | %.4f | %d | %s |\n", c.CaseID, c.Passed, c.Score, c.KeywordHitRate, c.FallbackRate, c.LLMNodeSuccessRate, c.LatencyMS, c.FailureCategory)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func BuildArtifact(runID string, c Case, plan creator.CreationPlan, result CaseResult, err error) Artifact {
	artifact := Artifact{
		ArtifactVersion: ArtifactVersion,
		RunID:           runID,
		TraceVersion:    plan.TraceVersion,
		CaseID:          c.ID,
		Input:           c,
		Plan:            plan,
		PlanningTrace:   plan.PlanningTrace,
		CallbackEvents:  plan.CallbackEvents,
		EvalResult:      result,
		CreatedAt:       time.Now().UTC(),
	}
	if err != nil {
		artifact.Error = err.Error()
	}
	return artifact
}

func ReadReport(path string) (Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var r Report
	if err := json.Unmarshal(b, &r); err != nil {
		return Report{}, err
	}
	return r, nil
}

func ReadArtifact(path string) (Artifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	var a Artifact
	if err := json.Unmarshal(b, &a); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

func Compare(base Report, candidate Report) CompareReport {
	out := CompareReport{
		BaseRunID:          base.RunID,
		CandidateRunID:     candidate.RunID,
		ScoreDelta:         round(candidate.Summary.AverageScore - base.Summary.AverageScore),
		FallbackRateDelta:  round(candidate.Summary.FallbackRate - base.Summary.FallbackRate),
		RepairSuccessDelta: round(candidate.Summary.RepairSuccessRate - base.Summary.RepairSuccessRate),
		P95LatencyDeltaMS:  candidate.Summary.P95LatencyMS - base.Summary.P95LatencyMS,
	}
	baseCases := map[string]CaseResult{}
	for _, c := range base.Cases {
		baseCases[c.CaseID] = c
	}
	for _, c := range candidate.Cases {
		b, ok := baseCases[c.CaseID]
		if !ok {
			continue
		}
		delta := round(c.Score - b.Score)
		cd := CaseDelta{CaseID: c.CaseID, BaseScore: b.Score, CandidateScore: c.Score, Delta: delta}
		if delta <= -0.05 {
			out.RegressedCases = append(out.RegressedCases, cd)
		}
		if delta >= 0.05 {
			out.ImprovedCases = append(out.ImprovedCases, cd)
		}
	}
	sort.Slice(out.RegressedCases, func(i, j int) bool { return out.RegressedCases[i].Delta < out.RegressedCases[j].Delta })
	sort.Slice(out.ImprovedCases, func(i, j int) bool { return out.ImprovedCases[i].Delta > out.ImprovedCases[j].Delta })
	return out
}

func InspectArtifact(a Artifact) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Trace artifact %s / case %s\n", a.RunID, a.CaseID)
	fmt.Fprintf(&b, "Score %.4f passed=%t failure=%s\n", a.EvalResult.Score, a.EvalResult.Passed, a.EvalResult.FailureCategory)
	fmt.Fprintf(&b, "Quality score=%d repair_attempts=%d issues=%s\n", a.Plan.QualityScore, a.Plan.RepairAttempts, strings.Join(a.Plan.Quality.Issues, ","))

	trace := append([]creator.PlanningTrace(nil), a.PlanningTrace...)
	sort.Slice(trace, func(i, j int) bool { return trace[i].DurationMS > trace[j].DurationMS })
	fmt.Fprintf(&b, "\nSlowest nodes:\n")
	for i, t := range trace {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&b, "- %s source=%s status=%s duration_ms=%d error=%s\n", t.Node, t.Source, t.Status, t.DurationMS, t.Error)
	}

	fmt.Fprintf(&b, "\nFallback/error nodes:\n")
	found := false
	for _, t := range a.PlanningTrace {
		if t.Source == "fallback" || t.Status == "error" || t.Error != "" {
			found = true
			fmt.Fprintf(&b, "- step=%d node=%s source=%s status=%s error=%s\n", t.Step, t.Node, t.Source, t.Status, t.Error)
		}
	}
	if !found {
		fmt.Fprintf(&b, "- none\n")
	}
	return b.String()
}
