package task

import (
	"encoding/json"
	"time"

	creator "creator-pipeline/internal/eino"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
	StatusTimeout   Status = "timeout"
)

type Task struct {
	ID             string               `json:"id"`
	UserID         string               `json:"user_id"`
	IdempotencyKey string               `json:"idempotency_key,omitempty"`
	Prompt         string               `json:"prompt"`
	Plan           creator.CreationPlan `json:"plan"`
	PlanJSON       json.RawMessage      `json:"-"`
	Status         Status               `json:"status"`
	Attempt        int                  `json:"attempt"`
	MaxRetries     int                  `json:"max_retries"`
	ErrorCode      string               `json:"error_code,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	ResultURL      string               `json:"result_url,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
	StartedAt      *time.Time           `json:"started_at,omitempty"`
	FinishedAt     *time.Time           `json:"finished_at,omitempty"`
	DeadlineAt     *time.Time           `json:"deadline_at,omitempty"`
}

func New(id string, userID string, idempotencyKey string, plan creator.CreationPlan, maxRetries int) *Task {
	now := time.Now().UTC()
	planJSON, _ := json.Marshal(plan)
	return &Task{
		ID:             id,
		UserID:         userID,
		IdempotencyKey: idempotencyKey,
		Prompt:         plan.Prompt,
		Plan:           plan,
		PlanJSON:       planJSON,
		Status:         StatusPending,
		MaxRetries:     maxRetries,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (t *Task) Terminal() bool {
	return t.Status == StatusSucceeded || t.Status == StatusFailed || t.Status == StatusCanceled
}
