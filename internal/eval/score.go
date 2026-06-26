package eval

import (
	"math"
	"sort"
	"strings"

	creator "creator-pipeline/internal/eino"
)

var requiredLLMNodes = map[string]bool{
	"roles":                 true,
	"scenes":                true,
	"commercial_storyboard": true,
	"story_storyboard":      true,
	"tutorial_storyboard":   true,
	"dialogues":             true,
}

func ScoreCase(c Case, plan creator.CreationPlan, latencyMS int64, runErr error) CaseResult {
	res := CaseResult{CaseID: c.ID, LatencyMS: latencyMS}
	if runErr != nil {
		res.FailureCategory = "llm_error"
		res.Issues = append(res.Issues, runErr.Error())
		return res
	}

	res.PromptTypeAccuracy = strings.EqualFold(plan.PromptType, c.ExpectedType)
	if !res.PromptTypeAccuracy {
		res.Issues = append(res.Issues, "prompt_type_mismatch")
	}

	res.KeywordHitRate = keywordHitRate(plan, c.RequiredKeywords)
	if res.KeywordHitRate < 1 {
		res.Issues = append(res.Issues, "keyword_missing")
	}

	res.LanguageMatch = languageMatch(c.Language, plan)
	if !res.LanguageMatch {
		res.Issues = append(res.Issues, "language_mismatch")
	}

	res.ShotCountValid = len(plan.Shots) >= c.MinShots
	if !res.ShotCountValid {
		res.Issues = append(res.Issues, "shot_count_invalid")
	}

	actualDuration := totalDuration(plan)
	res.DurationErrorMS = abs(actualDuration - c.TargetDurationMS)
	if res.DurationErrorMS > 2500 {
		res.Issues = append(res.Issues, "duration_invalid")
	}

	res.RepairSuccess = !c.ExpectRepair || plan.RepairAttempts > 0
	if c.ExpectRepair && !res.RepairSuccess {
		res.Issues = append(res.Issues, "repair_not_triggered")
	}

	res.FallbackRate = fallbackRate(plan.PlanningTrace)
	res.LLMNodeSuccessRate = llmNodeSuccessRate(plan.PlanningTrace)
	res.Score = weightedScore(res)
	res.Passed = res.Score >= 0.75
	res.FailureCategory = classifyFailure(res)
	return res
}

func Summarize(runID string, mode string, model string, started, endedTime interface{}, results []CaseResult) Report {
	// Kept for compatibility with callers that construct Report manually.
	return Report{RunID: runID, Mode: mode, Model: model, Cases: results}
}

func Aggregate(results []CaseResult) SuiteSummary {
	s := SuiteSummary{TotalCases: len(results), FailureCategoriesCount: map[string]int{}}
	if len(results) == 0 {
		return s
	}
	latencies := make([]int64, 0, len(results))
	var score, typeOK, keyword, langOK, shotOK, repairOK, fallback, llmOK, latencySum float64
	for _, r := range results {
		if r.Passed {
			s.PassedCases++
		}
		if r.PromptTypeAccuracy {
			typeOK++
		}
		if r.LanguageMatch {
			langOK++
		}
		if r.ShotCountValid {
			shotOK++
		}
		if r.RepairSuccess {
			repairOK++
		}
		if r.FailureCategory != "" {
			s.FailureCategoriesCount[r.FailureCategory]++
		}
		score += r.Score
		keyword += r.KeywordHitRate
		fallback += r.FallbackRate
		llmOK += r.LLMNodeSuccessRate
		latencySum += float64(r.LatencyMS)
		latencies = append(latencies, r.LatencyMS)
	}
	s.FailedCases = len(results) - s.PassedCases
	den := float64(len(results))
	s.AverageScore = round(score / den)
	s.PromptTypeAccuracy = round(typeOK / den)
	s.KeywordHitRate = round(keyword / den)
	s.LanguageMatchRate = round(langOK / den)
	s.ShotCountValidRate = round(shotOK / den)
	s.RepairSuccessRate = round(repairOK / den)
	s.FallbackRate = round(fallback / den)
	s.LLMNodeSuccessRate = round(llmOK / den)
	s.AverageLatencyMS = int64(latencySum / den)
	s.P95LatencyMS = percentileLatency(latencies, 0.95)
	return s
}

func keywordHitRate(plan creator.CreationPlan, keywords []string) float64 {
	if len(keywords) == 0 {
		return 1
	}
	text := strings.ToLower(planText(plan))
	hits := 0
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(kw))) {
			hits++
		}
	}
	return round(float64(hits) / float64(len(keywords)))
}

func languageMatch(lang string, plan creator.CreationPlan) bool {
	text := planText(plan)
	if strings.EqualFold(lang, "zh") {
		return hasCJK(text)
	}
	return true
}

func fallbackRate(trace []creator.PlanningTrace) float64 {
	if len(trace) == 0 {
		return 0
	}
	fallback := 0
	for _, item := range trace {
		if item.Source == "fallback" {
			fallback++
		}
	}
	return round(float64(fallback) / float64(len(trace)))
}

func llmNodeSuccessRate(trace []creator.PlanningTrace) float64 {
	want := 0
	ok := 0
	for _, item := range trace {
		if requiredLLMNodes[item.Node] {
			want++
			if item.Source == "llm" && item.Status == "success" {
				ok++
			}
		}
	}
	if want == 0 {
		return 1
	}
	return round(float64(ok) / float64(want))
}

func weightedScore(r CaseResult) float64 {
	score := 0.0
	if r.PromptTypeAccuracy {
		score += 0.18
	}
	score += 0.24 * r.KeywordHitRate
	if r.LanguageMatch {
		score += 0.14
	}
	if r.ShotCountValid {
		score += 0.14
	}
	if r.DurationErrorMS <= 2500 {
		score += 0.12
	}
	if r.RepairSuccess {
		score += 0.08
	}
	score += 0.10 * r.LLMNodeSuccessRate
	return round(score)
}

func classifyFailure(r CaseResult) string {
	if r.Passed {
		return ""
	}
	for _, issue := range r.Issues {
		switch issue {
		case "keyword_missing":
			return "keyword_missing"
		case "language_mismatch":
			return "language_mismatch"
		case "duration_invalid":
			return "duration_invalid"
		case "shot_count_invalid", "repair_not_triggered":
			return "quality_failed"
		}
	}
	if r.LLMNodeSuccessRate < 0.99 || r.FallbackRate > 0 {
		return "llm_error"
	}
	return "quality_failed"
}

func totalDuration(plan creator.CreationPlan) int {
	if plan.Duration != nil && plan.Duration.TotalDurationMS > 0 {
		return plan.Duration.TotalDurationMS
	}
	total := 0
	for _, shot := range plan.Shots {
		total += shot.DurationMS
	}
	return total
}

func planText(plan creator.CreationPlan) string {
	var b strings.Builder
	for _, role := range plan.Roles {
		b.WriteString(role.Name + " " + role.Description + " ")
	}
	for _, scene := range plan.Scenes {
		b.WriteString(scene.Name + " " + scene.Mood + " ")
	}
	for _, shot := range plan.Shots {
		b.WriteString(shot.Description + " ")
	}
	for _, dialogue := range plan.Dialogues {
		b.WriteString(dialogue.Text + " ")
	}
	return b.String()
}

func hasCJK(s string) bool {
	for _, r := range s {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

func percentileLatency(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	idx := int(math.Ceil(float64(len(values))*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func round(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
