package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	creator "creator-pipeline/internal/eino"
	"creator-pipeline/internal/idempotency"
	"creator-pipeline/internal/metrics"
	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/task"
)

const defaultMaxRetries = 2

type Planner interface {
	Plan(context.Context, creator.PromptInput) (creator.CreationPlan, error)
}

type Service struct {
	planner Planner
	repo    task.Repository
	queue   queue.Queue
	idem    idempotency.Store
	metrics *metrics.Registry
}

type CreateRequest struct {
	UserID         string             `json:"user_id"`
	Prompt         string             `json:"prompt"`
	IdempotencyKey string             `json:"idempotency_key"`
	Assets         []creator.AssetRef `json:"assets,omitempty"`
}

type CreateResponse struct {
	TaskID     string               `json:"task_id"`
	Status     task.Status          `json:"status"`
	Deduped    bool                 `json:"deduped"`
	Plan       creator.CreationPlan `json:"plan"`
	MetricsURL string               `json:"metrics_url"`
}

func NewService(planner Planner, repo task.Repository, q queue.Queue, idem idempotency.Store, m *metrics.Registry) *Service {
	return &Service{planner: planner, repo: repo, queue: q, idem: idem, metrics: m}
}

func (s *Service) CreateCreation(ctx context.Context, req CreateRequest) (CreateResponse, error) {
	if req.IdempotencyKey != "" {
		if taskID, ok := s.idem.Get(req.IdempotencyKey); ok {
			t, err := s.repo.Get(ctx, taskID)
			if err != nil {
				return CreateResponse{}, err
			}
			return CreateResponse{TaskID: t.ID, Status: t.Status, Deduped: true, Plan: t.Plan, MetricsURL: "/metrics"}, nil
		}
	}

	plan, err := s.planner.Plan(ctx, creator.PromptInput{
		UserID: req.UserID,
		Prompt: req.Prompt,
		Assets: req.Assets,
	})
	if err != nil {
		return CreateResponse{}, err
	}

	t := task.New(newID(), req.UserID, req.IdempotencyKey, plan, defaultMaxRetries)
	if err := s.repo.Create(ctx, t); err != nil {
		return CreateResponse{}, err
	}
	if err := s.queue.Publish(queue.Message{TaskID: t.ID}); err != nil {
		return CreateResponse{}, err
	}
	s.idem.Set(req.IdempotencyKey, t.ID)
	s.metrics.TaskCreated()

	return CreateResponse{TaskID: t.ID, Status: t.Status, Plan: plan, MetricsURL: "/metrics"}, nil
}

func (s *Service) Retry(ctx context.Context, taskID string) (*task.Task, error) {
	t, err := s.repo.Update(ctx, taskID, task.Retry)
	if err != nil {
		return nil, err
	}
	if err := s.queue.Publish(queue.Message{TaskID: t.ID}); err != nil {
		return nil, err
	}
	s.metrics.Retry()
	return t, nil
}

func (s *Service) Cancel(ctx context.Context, taskID string) (*task.Task, error) {
	return s.repo.Update(ctx, taskID, task.Cancel)
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("task-%x", b)
	}
	return hex.EncodeToString(b[:])
}
