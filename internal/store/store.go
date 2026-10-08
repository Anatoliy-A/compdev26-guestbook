package store

import (
	"context"
	"sync"
	"time"
)

type Entry struct {
	ID          string    `json:"id"`
	GuestbookID string    `json:"guestbookId"`
	Name        string    `json:"name"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Store interface {
	Add(ctx context.Context, e Entry) error
	// List returns the newest entries first.
	List(ctx context.Context, guestbookID string, limit int) ([]Entry, error)
}

// Memory is a non-persistent store for local runs and tests.
type Memory struct {
	mu      sync.Mutex
	entries []Entry
}

func NewMemory() *Memory {
	return &Memory{}
}

func (m *Memory) Add(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

func (m *Memory) List(_ context.Context, guestbookID string, limit int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Entry, 0, limit)
	for i := len(m.entries) - 1; i >= 0 && len(out) < limit; i-- {
		if m.entries[i].GuestbookID == guestbookID {
			out = append(out, m.entries[i])
		}
	}
	return out, nil
}
