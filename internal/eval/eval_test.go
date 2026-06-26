package eval

import (
	"strings"
	"testing"

	creator "creator-pipeline/internal/eino"
)

func TestDecodeDataset(t *testing.T) {
	input := `{"id":"c1","prompt":"Create a commercial ad for a light","expected_type":"commercial","required_keywords":["light"],"language":"en","min_shots":3,"target_duration_ms":10000}` + "\n"
	cases, err := DecodeDataset(strings.NewReader(input))
	if err != nil {
		t.Fatalf("DecodeDataset() error = %v", err)
	}
	if len(cases) != 1 || cases[0].ID != "c1" {
		t.Fatalf("unexpected cases: %+v", cases)
	}
}

func TestScoreCase(t *testing.T) {
	c := Case{ID: "c1", Prompt: "Create a commercial ad for a night riding safety light in the city", ExpectedType: "commercial", RequiredKeywords: []string{"night", "riding", "safety", "light", "city"}, Language: "en", MinShots: 3, TargetDurationMS: 10000}
	plan := creator.CreationPlan{
		Prompt:       c.Prompt,
		PromptType:   "commercial",
		TraceVersion: "planning_trace.v1",
		Roles:        []creator.Role{{Name: "rider", Description: "night city safety light"}},
		Scenes:       []creator.Scene{{Name: "night city", Mood: "premium"}, {Name: "riding", Mood: "safe"}, {Name: "callout", Mood: "clear"}},
		Shots: []creator.Shot{
			{Index: 1, Description: "night riding safety light in city", DurationMS: 3333},
			{Index: 2, Description: "safety light product", DurationMS: 3333},
			{Index: 3, Description: "city call to action", DurationMS: 3334},
		},
		Duration: &creator.DurationEstimate{TotalDurationMS: 10000, TargetDurationMS: 10000},
		PlanningTrace: []creator.PlanningTrace{
			{Node: "roles", Source: "llm", Status: "success"},
			{Node: "scenes", Source: "llm", Status: "success"},
			{Node: "commercial_storyboard", Source: "llm", Status: "success"},
			{Node: "dialogues", Source: "llm", Status: "success"},
		},
	}
	res := ScoreCase(c, plan, 12, nil)
	if !res.Passed {
		t.Fatalf("expected pass, got %+v", res)
	}
	if res.KeywordHitRate != 1 || res.LLMNodeSuccessRate != 1 {
		t.Fatalf("unexpected rates: %+v", res)
	}
}

func TestInspectArtifact(t *testing.T) {
	a := Artifact{RunID: "run1", CaseID: "case1", EvalResult: CaseResult{Score: 0.5}, PlanningTrace: []creator.PlanningTrace{{Step: 1, Node: "roles", Source: "fallback", Status: "error", Error: "boom"}}}
	out := InspectArtifact(a)
	if !strings.Contains(out, "roles") || !strings.Contains(out, "fallback") {
		t.Fatalf("inspect output missing trace details: %s", out)
	}
}
