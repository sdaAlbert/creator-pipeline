package storage

import (
	"context"
	"fmt"
	"strings"
)

type Store interface {
	WriteResult(ctx context.Context, taskID string, payload []byte) (string, error)
}

type MockStore struct {
	cdnBase string
}

func NewMockStore(cdnBase string) *MockStore {
	return &MockStore{cdnBase: strings.TrimRight(cdnBase, "/")}
}

func (s *MockStore) WriteResult(_ context.Context, taskID string, _ []byte) (string, error) {
	return fmt.Sprintf("%s/generated/%s.mp4", s.cdnBase, taskID), nil
}
