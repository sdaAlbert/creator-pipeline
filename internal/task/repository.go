package task

import "context"

type Repository interface {
	Create(ctx context.Context, t *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, id string, mutate func(*Task) error) (*Task, error)
}
