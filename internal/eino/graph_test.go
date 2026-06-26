package eino

import (
	"context"
	"testing"
)

func TestCommercialBranchWithAssetTools(t *testing.T) {
	planner, err := NewPlanner(context.Background(), nil)
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	plan, err := planner.Plan(context.Background(), PromptInput{
		UserID: "u1",
		Prompt: "Create a commercial ad for a night riding safety light in the city",
		Assets: []AssetRef{
			{ObjectKey: "uploads/light.png", Kind: "image"},
			{ObjectKey: "uploads/night.mp4", Kind: "video"},
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.PromptType != "commercial" {
		t.Fatalf("PromptType = %q, want commercial", plan.PromptType)
	}
	if plan.AssetMetadata == nil || plan.AssetMetadata.AssetCount != 2 || !plan.AssetMetadata.HasUserMaterial {
		t.Fatalf("AssetMetadata = %+v, want two user assets", plan.AssetMetadata)
	}
	assertTraceHas(t, plan, nodeAssetTools)
	assertTraceHas(t, plan, nodeCommercial)
	assertCallbackHas(t, plan, nodeCommercial, "start")
}

func TestTutorialBranch(t *testing.T) {
	planner, err := NewPlanner(context.Background(), nil)
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	plan, err := planner.Plan(context.Background(), PromptInput{
		UserID: "u1",
		Prompt: "Create a tutorial video for setting up a smart coffee machine",
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.PromptType != "tutorial" {
		t.Fatalf("PromptType = %q, want tutorial", plan.PromptType)
	}
	assertTraceHas(t, plan, nodeTutorial)
}

func TestRepairBranchAddsSemanticAnchor(t *testing.T) {
	planner, err := NewPlanner(context.Background(), nil)
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	plan, err := planner.Plan(context.Background(), PromptInput{
		UserID: "u1",
		Prompt: "Create a commercial ad for a night riding safety light",
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.RepairAttempts == 0 {
		t.Fatalf("RepairAttempts = 0, want repair loop to run")
	}
	assertTraceHas(t, plan, nodeRepair)
	if len(plan.Shots) == 0 || !containsAnyKeyword(plan.Shots[0].Description, []string{"night", "riding", "safety", "light"}) {
		t.Fatalf("first repaired shot does not contain semantic anchor: %+v", plan.Shots)
	}
}

func TestPlanningTraceHasStableRunMetadata(t *testing.T) {
	planner, err := NewPlanner(context.Background(), nil)
	if err != nil {
		t.Fatalf("NewPlanner() error = %v", err)
	}

	plan, err := planner.Plan(context.Background(), PromptInput{
		UserID: "u1",
		Prompt: "Create a tutorial video for brewing coffee",
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.RunID == "" {
		t.Fatal("RunID is empty")
	}
	if plan.TraceVersion != traceVersion {
		t.Fatalf("TraceVersion = %q, want %q", plan.TraceVersion, traceVersion)
	}
	if len(plan.PlanningTrace) == 0 {
		t.Fatal("PlanningTrace is empty")
	}
	for i, item := range plan.PlanningTrace {
		if item.Step != i+1 {
			t.Fatalf("trace step at index %d = %d, want %d", i, item.Step, i+1)
		}
		if item.Node == "" || item.Source == "" || item.Status == "" {
			t.Fatalf("trace item missing stable fields: %+v", item)
		}
		if item.Error != "" && item.Status != "error" {
			t.Fatalf("trace item with error has status %q: %+v", item.Status, item)
		}
	}
}
func TestSemanticQualityDetectsMissingKeywords(t *testing.T) {
	quality := evaluateQuality("Create a commercial ad for a night riding safety light", CreationPlan{
		PromptType: "commercial",
		Scenes:     fallbackScenes("commercial"),
		Shots: []Shot{
			{Index: 1, Description: "A beach scene with unrelated fashion poses", DurationMS: 3000},
			{Index: 2, Description: "A food close-up with no product context", DurationMS: 3000},
			{Index: 3, Description: "A generic end card", DurationMS: 3000},
		},
		Duration: &DurationEstimate{ShotCount: 3, TotalDurationMS: 9000, TargetDurationMS: 10000, DeltaMS: -1000},
	})

	if quality.SemanticScore >= 15 {
		t.Fatalf("SemanticScore = %d, want low score for unrelated output", quality.SemanticScore)
	}
	if len(quality.MissingKeywords) == 0 {
		t.Fatalf("MissingKeywords is empty, want prompt keywords to be flagged")
	}
}

func assertTraceHas(t *testing.T, plan CreationPlan, node string) {
	t.Helper()
	for _, item := range plan.PlanningTrace {
		if item.Node == node {
			return
		}
	}
	t.Fatalf("planning trace missing node %q: %+v", node, plan.PlanningTrace)
}

func assertCallbackHas(t *testing.T, plan CreationPlan, node string, event string) {
	t.Helper()
	for _, item := range plan.CallbackEvents {
		if item.Node == node && item.Event == event {
			return
		}
	}
	t.Fatalf("callback events missing %s/%s: %+v", node, event, plan.CallbackEvents)
}
