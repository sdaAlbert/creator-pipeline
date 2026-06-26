package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"creator-pipeline/internal/script"
	"creator-pipeline/internal/task"
)

type RewriteRequest struct {
	ShotIndex   int    `json:"shot_index"`
	Instruction string `json:"instruction"`
}

func (s *Service) ScriptDocument(ctx context.Context, taskID string) (script.Document, error) {
	t, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return script.Document{}, fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	if t.Status != task.StatusSucceeded {
		return script.Document{}, fmt.Errorf("%w: current status=%s", ErrScriptNotReady, t.Status)
	}
	return script.FromPlan(t.Plan), nil
}

func (s *Service) ScriptMarkdown(ctx context.Context, taskID string) (string, error) {
	doc, err := s.ScriptDocument(ctx, taskID)
	if err != nil {
		return "", err
	}
	return script.Markdown(doc), nil
}

func (s *Service) RewriteShot(ctx context.Context, taskID string, req RewriteRequest) (script.Document, error) {
	t, err := s.repo.Update(ctx, taskID, func(t *task.Task) error {
		if t.Status != task.StatusSucceeded {
			return fmt.Errorf("%w: current status=%s", ErrScriptNotReady, t.Status)
		}
		if err := script.RewriteShot(&t.Plan, req.ShotIndex, req.Instruction); err != nil {
			return fmt.Errorf("%w: %s", ErrBadRewrite, err.Error())
		}
		t.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return script.Document{}, classifyUpdateError(taskID, err)
	}
	return script.FromPlan(t.Plan), nil
}

func (s *Service) RewriteDialogue(ctx context.Context, taskID string, req RewriteRequest) (script.Document, error) {
	t, err := s.repo.Update(ctx, taskID, func(t *task.Task) error {
		if t.Status != task.StatusSucceeded {
			return fmt.Errorf("%w: current status=%s", ErrScriptNotReady, t.Status)
		}
		if err := script.RewriteDialogue(&t.Plan, req.ShotIndex, req.Instruction); err != nil {
			return fmt.Errorf("%w: %s", ErrBadRewrite, err.Error())
		}
		t.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return script.Document{}, classifyUpdateError(taskID, err)
	}
	return script.FromPlan(t.Plan), nil
}

func classifyUpdateError(taskID string, err error) error {
	if strings.Contains(err.Error(), "task "+taskID+" not found") {
		return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
	}
	return err
}
