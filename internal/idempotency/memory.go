package idempotency

import (
	"sync"
	"time"
)

type Store interface {
	Get(key string) (string, bool)
	Set(key string, taskID string)
}

type MemoryStore struct {
	mu   sync.Mutex
	ttl  time.Duration
	data map[string]entry
}

type entry struct {
	taskID    string
	expiresAt time.Time
}

func NewMemoryStore(ttl time.Duration) *MemoryStore {
	return &MemoryStore{
		ttl:  ttl,
		data: make(map[string]entry),
	}
}

func (s *MemoryStore) Get(key string) (string, bool) {
	if key == "" {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expiresAt) {
		delete(s.data, key)
		return "", false
	}
	return e.taskID, true
}

func (s *MemoryStore) Set(key string, taskID string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{taskID: taskID, expiresAt: time.Now().Add(s.ttl)}
}
