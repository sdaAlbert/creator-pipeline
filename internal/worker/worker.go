package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"creator-pipeline/internal/metrics"
	"creator-pipeline/internal/queue"
	"creator-pipeline/internal/storage"
	"creator-pipeline/internal/task"
	"creator-pipeline/internal/video"
)

type Config struct {
	Concurrency  int
	JobTimeout   time.Duration
	MaxRetries   int
	PollInterval time.Duration
}

type Worker struct {
	repo      task.Repository
	queue     queue.Queue
	store     storage.Store
	generator video.VideoGenerator
	metrics   *metrics.Registry
	config    Config
}

func New(repo task.Repository, q queue.Queue, store storage.Store, generator video.VideoGenerator, m *metrics.Registry, cfg Config) *Worker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 5 * time.Second
	}
	if generator == nil {
		generator = video.NewMockGenerator()
	}
	return &Worker{repo: repo, queue: q, store: store, generator: generator, metrics: m, config: cfg}
}

func (w *Worker) Run(ctx context.Context) {
	ch, err := w.queue.Consume()
	if err != nil {
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < w.config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg := <-ch:
					w.handle(ctx, msg)
				}
			}
		}()
	}
	wg.Wait()
}

func (w *Worker) handle(ctx context.Context, msg queue.Message) {
	started := time.Now()
	deadline := started.Add(w.config.JobTimeout)

	t, err := w.repo.Update(ctx, msg.TaskID, func(t *task.Task) error {
		if t.Status == task.StatusCanceled {
			return errors.New("task already canceled")
		}
		if t.Status != task.StatusPending {
			return errors.New("task is not pending")
		}
		return task.Start(t, deadline)
	})
	if err != nil {
		return
	}

	w.metrics.WorkerRunning(1)
	defer w.metrics.WorkerRunning(-1)

	jobCtx, cancel := context.WithTimeout(ctx, w.config.JobTimeout)
	defer cancel()

	result, err := w.generator.Generate(jobCtx, video.GenerationRequest{
		TaskID:  t.ID,
		Prompt:  t.Prompt,
		Plan:    t.Plan,
		Attempt: t.Attempt,
	})
	if err != nil {
		w.finishFailed(ctx, t.ID, err, time.Since(started))
		return
	}

	resultURL, err := w.store.WriteResult(ctx, t.ID, result.Payload)
	if err != nil {
		w.finishFailed(ctx, t.ID, err, time.Since(started))
		return
	}

	updated, err := w.repo.Update(ctx, t.ID, func(t *task.Task) error {
		if t.Status == task.StatusCanceled {
			return nil
		}
		return task.Succeed(t, resultURL)
	})
	if err == nil && updated.Status == task.StatusSucceeded {
		w.metrics.TaskFinished(task.StatusSucceeded, time.Since(started))
	}
}

func (w *Worker) finishFailed(ctx context.Context, taskID string, cause error, duration time.Duration) {
	if errors.Is(cause, context.DeadlineExceeded) {
		updated, err := w.repo.Update(ctx, taskID, func(t *task.Task) error {
			return task.Timeout(t, cause.Error())
		})
		if err == nil {
			w.metrics.TaskFinished(task.StatusTimeout, duration)
			w.requeueIfPossible(ctx, updated)
		}
		return
	}

	w.metrics.ModelFailure()
	updated, err := w.repo.Update(ctx, taskID, func(t *task.Task) error {
		return task.Fail(t, "model_failed", cause.Error())
	})
	if err == nil {
		w.metrics.TaskFinished(task.StatusFailed, duration)
		w.requeueIfPossible(ctx, updated)
	}
}

func (w *Worker) requeueIfPossible(ctx context.Context, t *task.Task) {
	if t.Attempt > t.MaxRetries {
		return
	}
	updated, err := w.repo.Update(ctx, t.ID, task.Retry)
	if err != nil {
		return
	}
	_ = w.queue.Publish(queue.Message{TaskID: updated.ID})
	w.metrics.Retry()
}
