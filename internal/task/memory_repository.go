package task

import (
	"context"
	"fmt"
	"sync"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{tasks: make(map[string]*Task)}
}

func (r *MemoryRepository) Create(_ context.Context, t *Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[t.ID]; ok {
		return fmt.Errorf("task %s already exists", t.ID)
	}
	cp := *t
	r.tasks[t.ID] = &cp
	return nil
}

func (r *MemoryRepository) Get(_ context.Context, id string) (*Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	cp := *t
	return &cp, nil
}

func (r *MemoryRepository) Update(_ context.Context, id string, mutate func(*Task) error) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	if err := mutate(t); err != nil {
		return nil, err
	}
	cp := *t
	return &cp, nil
}
