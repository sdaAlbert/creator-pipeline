package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"creator-pipeline/internal/app"
	creator "creator-pipeline/internal/eino"
	"creator-pipeline/internal/idempotency"
	"creator-pipeline/internal/metrics"
	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/task"
)

func TestScriptRouteRejectsPendingTask(t *testing.T) {
	h := newRouteTestHandler(t, task.StatusPending)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task1/script", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "script_unavailable") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestScriptRouteReturnsNotFoundForMissingTask(t *testing.T) {
	h := newRouteTestHandler(t, task.StatusSucceeded)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/missing/script", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestRewriteShotRouteRejectsInvalidIndex(t *testing.T) {
	h := newRouteTestHandler(t, task.StatusSucceeded)
	body := bytes.NewBufferString(`{"shot_index":99,"instruction":"change it"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task1/rewrite-shot", body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "rewrite_failed") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func newRouteTestHandler(t *testing.T, status task.Status) http.Handler {
	t.Helper()
	repo := task.NewMemoryRepository()
	q := queue.NewMemoryQueue()
	svc := app.NewService(nil, repo, q, idempotency.NewMemoryStore(0), metrics.NewRegistry(q))
	plan := creator.CreationPlan{
		RunID:        "run_test",
		TraceVersion: "planning_trace.v1",
		Prompt:       "Create a commercial ad for a night riding safety light in the city",
		PromptType:   "commercial",
		Roles:        []creator.Role{{Name: "Rider", Description: "Urban cyclist"}},
		Scenes:       []creator.Scene{{Name: "Night city", Mood: "premium"}},
		Shots:        []creator.Shot{{Index: 1, Description: "Cyclist rides through the city", DurationMS: 3000}},
		Dialogues:    []creator.Dialogue{{ShotIndex: 1, Text: "Own the night ride."}},
		QualityScore: 90,
	}
	taskObj := task.New("task1", "u1", "idem", plan, 2)
	taskObj.Status = status
	if err := repo.Create(context.Background(), taskObj); err != nil {
		t.Fatal(err)
	}
	return routes(svc, repo, metrics.NewRegistry(q))
}
