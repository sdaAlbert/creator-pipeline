package eval

import (
	"time"

	creator "creator-pipeline/internal/eino"
)

const ArtifactVersion = "creator_eval_artifact.v1"

type Case struct {
	ID               string             `json:"id"`
	Prompt           string             `json:"prompt"`
	Assets           []creator.AssetRef `json:"assets,omitempty"`
	ExpectedType     string             `json:"expected_type"`
	RequiredKeywords []string           `json:"required_keywords"`
	Language         string             `json:"language"`
	MinShots         int                `json:"min_shots"`
	TargetDurationMS int                `json:"target_duration_ms"`
	ExpectRepair     bool               `json:"expect_repair,omitempty"`
}

type CaseResult struct {
	CaseID             string   `json:"case_id"`
	Passed             bool     `json:"passed"`
	Score              float64  `json:"score"`
	PromptTypeAccuracy bool     `json:"prompt_type_accuracy"`
	KeywordHitRate     float64  `json:"keyword_hit_rate"`
	LanguageMatch      bool     `json:"language_match"`
	ShotCountValid     bool     `json:"shot_count_valid"`
	DurationErrorMS    int      `json:"duration_error_ms"`
	RepairSuccess      bool     `json:"repair_success"`
	FallbackRate       float64  `json:"fallback_rate"`
	LLMNodeSuccessRate float64  `json:"llm_node_success_rate"`
	LatencyMS          int64    `json:"latency_ms"`
	FailureCategory    string   `json:"failure_category,omitempty"`
	Issues             []string `json:"issues,omitempty"`
}

type SuiteSummary struct {
	TotalCases             int            `json:"total_cases"`
	PassedCases            int            `json:"passed_cases"`
	FailedCases            int            `json:"failed_cases"`
	AverageScore           float64        `json:"average_score"`
	PromptTypeAccuracy     float64        `json:"prompt_type_accuracy"`
	KeywordHitRate         float64        `json:"keyword_hit_rate"`
	LanguageMatchRate      float64        `json:"language_match_rate"`
	ShotCountValidRate     float64        `json:"shot_count_valid_rate"`
	RepairSuccessRate      float64        `json:"repair_success_rate"`
	FallbackRate           float64        `json:"fallback_rate"`
	LLMNodeSuccessRate     float64        `json:"llm_node_success_rate"`
	P95LatencyMS           int64          `json:"p95_latency_ms"`
	AverageLatencyMS       int64          `json:"avg_latency_ms"`
	RegressedCases         int            `json:"regressed_cases,omitempty"`
	ImprovedCases          int            `json:"improved_cases,omitempty"`
	FailureCategoriesCount map[string]int `json:"failure_categories_count,omitempty"`
}

type Report struct {
	RunID     string       `json:"run_id"`
	Mode      string       `json:"mode"`
	Model     string       `json:"model,omitempty"`
	StartedAt time.Time    `json:"started_at"`
	EndedAt   time.Time    `json:"ended_at"`
	Summary   SuiteSummary `json:"summary"`
	Cases     []CaseResult `json:"cases"`
}

type Artifact struct {
	ArtifactVersion string                  `json:"artifact_version"`
	RunID           string                  `json:"run_id"`
	TraceVersion    string                  `json:"trace_version"`
	CaseID          string                  `json:"case_id"`
	Input           Case                    `json:"input"`
	Plan            creator.CreationPlan    `json:"plan"`
	PlanningTrace   []creator.PlanningTrace `json:"planning_trace"`
	CallbackEvents  []creator.CallbackEvent `json:"callback_events"`
	EvalResult      CaseResult              `json:"eval_result"`
	CreatedAt       time.Time               `json:"created_at"`
	Error           string                  `json:"error,omitempty"`
}

type CompareReport struct {
	BaseRunID          string      `json:"base_run_id"`
	CandidateRunID     string      `json:"candidate_run_id"`
	ScoreDelta         float64     `json:"score_delta"`
	FallbackRateDelta  float64     `json:"fallback_rate_delta"`
	RepairSuccessDelta float64     `json:"repair_success_delta"`
	P95LatencyDeltaMS  int64       `json:"p95_latency_delta_ms"`
	RegressedCases     []CaseDelta `json:"regressed_cases"`
	ImprovedCases      []CaseDelta `json:"improved_cases"`
}

type CaseDelta struct {
	CaseID         string  `json:"case_id"`
	BaseScore      float64 `json:"base_score"`
	CandidateScore float64 `json:"candidate_score"`
	Delta          float64 `json:"delta"`
}
