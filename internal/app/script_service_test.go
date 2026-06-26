package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	creator "creator-pipeline/internal/eino"
	"creator-pipeline/internal/idempotency"
	"creator-pipeline/internal/metrics"
	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/task"
)

func TestScriptDocumentRequiresSucceededTask(t *testing.T) {
	svc, repo := newScriptTestService(t, task.StatusPending)
	if _, err := svc.ScriptDocument(context.Background(), "task1"); !errors.Is(err, ErrScriptNotReady) {
		t.Fatalf("expected ErrScriptNotReady, got %v", err)
	}
	if _, err := repo.Get(context.Background(), "task1"); err != nil {
		t.Fatal(err)
	}
}

func TestRewriteShotPersistsOnlySelectedShot(t *testing.T) {
	svc, repo := newScriptTestService(t, task.StatusSucceeded)
	before, _ := repo.Get(context.Background(), "task1")
	beforeShot1 := before.Plan.Shots[0].Description
	beforeShot2 := before.Plan.Shots[1].Description
	doc, err := svc.RewriteShot(context.Background(), "task1", RewriteRequest{ShotIndex: 2, Instruction: "Make this more emotional"})
	if err != nil {
		t.Fatalf("RewriteShot() error = %v", err)
	}
	if len(doc.StoryboardRows) != 2 {
		t.Fatalf("unexpected doc rows: %+v", doc.StoryboardRows)
	}
	after, _ := repo.Get(context.Background(), "task1")
	if after.Plan.Shots[0].Description != beforeShot1 {
		t.Fatalf("shot 1 changed")
	}
	if after.Plan.Shots[1].Description == beforeShot2 || !strings.Contains(after.Plan.Shots[1].Description, "Make this more emotional") {
		t.Fatalf("shot 2 not rewritten: %q", after.Plan.Shots[1].Description)
	}
}

func TestRewriteDialoguePersistsOnlySelectedDialogue(t *testing.T) {
	svc, repo := newScriptTestService(t, task.StatusSucceeded)
	before, _ := repo.Get(context.Background(), "task1")
	beforeDialogue1 := before.Plan.Dialogues[0].Text
	beforeDialogue2 := before.Plan.Dialogues[1].Text
	_, err := svc.RewriteDialogue(context.Background(), "task1", RewriteRequest{ShotIndex: 2, Instruction: "Make this shorter"})
	if err != nil {
		t.Fatalf("RewriteDialogue() error = %v", err)
	}
	after, _ := repo.Get(context.Background(), "task1")
	if after.Plan.Dialogues[0].Text != beforeDialogue1 {
		t.Fatalf("dialogue 1 changed")
	}
	if after.Plan.Dialogues[1].Text == beforeDialogue2 || !strings.Contains(after.Plan.Dialogues[1].Text, "Make this shorter") {
		t.Fatalf("dialogue 2 not rewritten: %q", after.Plan.Dialogues[1].Text)
	}
}

func TestRewriteInvalidShotIndexReturnsError(t *testing.T) {
	svc, _ := newScriptTestService(t, task.StatusSucceeded)
	if _, err := svc.RewriteShot(context.Background(), "task1", RewriteRequest{ShotIndex: 99, Instruction: "x"}); !errors.Is(err, ErrBadRewrite) {
		t.Fatalf("expected ErrBadRewrite, got %v", err)
	}
}

func newScriptTestService(t *testing.T, status task.Status) (*Service, *task.MemoryRepository) {
	t.Helper()
	repo := task.NewMemoryRepository()
	q := queue.NewMemoryQueue()
	svc := NewService(nil, repo, q, idempotency.NewMemoryStore(0), metrics.NewRegistry(q))
	plan := creator.CreationPlan{
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
		Dialogues:    []creator.Dialogue{{ShotIndex: 1, Text: "Own the night ride."}, {ShotIndex: 2, Text: "Be seen before every turn."}},
		QualityScore: 90,
	}
	taskObj := task.New("task1", "u1", "idem", plan, 2)
	taskObj.Status = status
	if err := repo.Create(context.Background(), taskObj); err != nil {
		t.Fatal(err)
	}
	return svc, repo
}
