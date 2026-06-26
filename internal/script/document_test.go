package script

import (
	"strings"
	"testing"

	creator "creator-pipeline/internal/eino"
)

func TestFromPlanAndMarkdown(t *testing.T) {
	plan := samplePlan()
	doc := FromPlan(plan)
	if doc.Title == "" || doc.Logline == "" || doc.Synopsis == "" {
		t.Fatalf("document missing user-facing summary fields: %+v", doc)
	}
	if doc.PlannerPathSummary == "" || doc.PlannerPathSummary == "unknown" {
		t.Fatalf("planner path summary missing: %+v", doc)
	}
	if len(doc.StoryboardRows) != 2 {
		t.Fatalf("storyboard rows = %d", len(doc.StoryboardRows))
	}
	md := Markdown(doc)
	for _, want := range []string{"# ", "## Logline", "## Storyboard", "| Time | Shot | Visual | Voice-over | Asset | Purpose |", "## Quality Review", "Planner path:"} {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func TestRewriteShotOnlyChangesSelectedShot(t *testing.T) {
	plan := samplePlan()
	beforeOther := plan.Shots[0].Description
	beforeSelected := plan.Shots[1].Description
	if err := RewriteShot(&plan, 2, "Make it more emotional"); err != nil {
		t.Fatalf("RewriteShot() error = %v", err)
	}
	if plan.Shots[0].Description != beforeOther {
		t.Fatalf("unselected shot changed: %q", plan.Shots[0].Description)
	}
	if plan.Shots[1].Description == beforeSelected || !strings.Contains(plan.Shots[1].Description, "Make it more emotional") {
		t.Fatalf("selected shot not rewritten: %q", plan.Shots[1].Description)
	}
}

func TestRewriteDialogueOnlyChangesSelectedDialogue(t *testing.T) {
	plan := samplePlan()
	beforeOther := plan.Dialogues[0].Text
	beforeSelected := plan.Dialogues[1].Text
	if err := RewriteDialogue(&plan, 2, "Shorter voice-over"); err != nil {
		t.Fatalf("RewriteDialogue() error = %v", err)
	}
	if plan.Dialogues[0].Text != beforeOther {
		t.Fatalf("unselected dialogue changed: %q", plan.Dialogues[0].Text)
	}
	if plan.Dialogues[1].Text == beforeSelected || !strings.Contains(plan.Dialogues[1].Text, "Shorter voice-over") {
		t.Fatalf("selected dialogue not rewritten: %q", plan.Dialogues[1].Text)
	}
}

func TestRewriteInvalidShotIndex(t *testing.T) {
	plan := samplePlan()
	if err := RewriteShot(&plan, 9, "change"); err == nil {
		t.Fatal("expected invalid shot index error")
	}
	if err := RewriteDialogue(&plan, 9, "change"); err == nil {
		t.Fatal("expected invalid dialogue index error")
	}
}

func samplePlan() creator.CreationPlan {
	return creator.CreationPlan{
		RunID:        "run_test",
		TraceVersion: "planning_trace.v1",
		Prompt:       "Create a commercial ad for a night riding safety light in the city",
		PromptType:   "commercial",
		Roles:        []creator.Role{{Name: "Rider", Description: "Urban cyclist"}},
		Scenes:       []creator.Scene{{Name: "Night city", Mood: "premium"}},
		Shots: []creator.Shot{
			{Index: 1, Description: "Cyclist rides through the city", DurationMS: 3000},
			{Index: 2, Description: "Safety light illuminates the road", DurationMS: 4000},
		},
		Dialogues: []creator.Dialogue{
			{ShotIndex: 1, Text: "Own the night ride."},
			{ShotIndex: 2, Text: "Be seen before every turn."},
		},
		Assets:       []creator.AssetRef{{ObjectKey: "uploads/light.png", Kind: "image"}},
		QualityScore: 90,
		PlanningTrace: []creator.PlanningTrace{
			{Step: 1, Node: "classify_prompt", Source: "branch", Status: "ok"},
			{Step: 2, Node: "storyboard", Source: "llm", Status: "ok"},
		},
		Duration: &creator.DurationEstimate{TotalDurationMS: 7000, TargetDurationMS: 7000},
	}
}
