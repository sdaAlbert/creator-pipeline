package eval

import (
	"context"
	"path/filepath"
	"time"

	creator "creator-pipeline/internal/eino"
)

type Planner interface {
	Plan(ctx context.Context, in creator.PromptInput) (creator.CreationPlan, error)
}

type RunConfig struct {
	RunID  string
	Mode   string
	Model  string
	OutDir string
}

func RunSuite(ctx context.Context, planner Planner, cases []Case, cfg RunConfig) (Report, error) {
	if cfg.RunID == "" {
		cfg.RunID = NewRunID()
	}
	started := time.Now().UTC()
	caseResults := make([]CaseResult, 0, len(cases))
	caseDir := filepath.Join(cfg.OutDir, cfg.RunID, "cases")

	for _, c := range cases {
		caseStarted := time.Now()
		plan, err := planner.Plan(ctx, creator.PromptInput{UserID: "eval-runner", Prompt: c.Prompt, Assets: c.Assets})
		latencyMS := time.Since(caseStarted).Milliseconds()
		result := ScoreCase(c, plan, latencyMS, err)
		caseResults = append(caseResults, result)
		artifact := BuildArtifact(cfg.RunID, c, plan, result, err)
		if writeErr := WriteJSON(filepath.Join(caseDir, c.ID+".json"), artifact); writeErr != nil {
			return Report{}, writeErr
		}
	}

	ended := time.Now().UTC()
	report := Report{
		RunID:     cfg.RunID,
		Mode:      cfg.Mode,
		Model:     cfg.Model,
		StartedAt: started,
		EndedAt:   ended,
		Cases:     caseResults,
		Summary:   Aggregate(caseResults),
	}
	base := filepath.Join(cfg.OutDir, cfg.RunID)
	if err := WriteJSON(filepath.Join(base, "report.json"), report); err != nil {
		return Report{}, err
	}
	if err := WriteReportMarkdown(filepath.Join(base, "report.md"), report); err != nil {
		return Report{}, err
	}
	return report, nil
}
