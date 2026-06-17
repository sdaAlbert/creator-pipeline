package task

import (
	"fmt"
	"time"
)

func CanTransition(from Status, to Status) bool {
	switch from {
	case StatusPending:
		return to == StatusRunning || to == StatusCanceled
	case StatusRunning:
		return to == StatusSucceeded || to == StatusFailed || to == StatusTimeout || to == StatusCanceled
	case StatusTimeout:
		return to == StatusPending || to == StatusFailed
	case StatusFailed:
		return to == StatusPending
	default:
		return false
	}
}

func Start(t *Task, deadline time.Time) error {
	if !CanTransition(t.Status, StatusRunning) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusRunning)
	}
	now := time.Now().UTC()
	t.Status = StatusRunning
	t.Attempt++
	t.StartedAt = &now
	t.DeadlineAt = &deadline
	t.UpdatedAt = now
	t.ErrorCode = ""
	t.ErrorMessage = ""
	return nil
}

func Succeed(t *Task, resultURL string) error {
	if !CanTransition(t.Status, StatusSucceeded) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusSucceeded)
	}
	now := time.Now().UTC()
	t.Status = StatusSucceeded
	t.ResultURL = resultURL
	t.FinishedAt = &now
	t.UpdatedAt = now
	return nil
}

func Fail(t *Task, code string, message string) error {
	if !CanTransition(t.Status, StatusFailed) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusFailed)
	}
	now := time.Now().UTC()
	t.Status = StatusFailed
	t.ErrorCode = code
	t.ErrorMessage = message
	t.FinishedAt = &now
	t.UpdatedAt = now
	return nil
}

func Timeout(t *Task, message string) error {
	if !CanTransition(t.Status, StatusTimeout) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusTimeout)
	}
	now := time.Now().UTC()
	t.Status = StatusTimeout
	t.ErrorCode = "timeout"
	t.ErrorMessage = message
	t.FinishedAt = &now
	t.UpdatedAt = now
	return nil
}

func Cancel(t *Task) error {
	if !CanTransition(t.Status, StatusCanceled) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusCanceled)
	}
	now := time.Now().UTC()
	t.Status = StatusCanceled
	t.FinishedAt = &now
	t.UpdatedAt = now
	return nil
}

func Retry(t *Task) error {
	if !CanTransition(t.Status, StatusPending) {
		return fmt.Errorf("cannot transition %s to %s", t.Status, StatusPending)
	}
	if t.Attempt > t.MaxRetries {
		return fmt.Errorf("max retries exceeded")
	}
	now := time.Now().UTC()
	t.Status = StatusPending
	t.ErrorCode = ""
	t.ErrorMessage = ""
	t.FinishedAt = nil
	t.UpdatedAt = now
	return nil
}
