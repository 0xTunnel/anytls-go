package state

import (
	"errors"
	"sync"
)

var ErrTCPLimitExceeded = errors.New("tcp limit exceeded")

type TCPTracker struct {
	mu     sync.Mutex
	counts map[int64]int64
}

func NewTCPTracker() *TCPTracker {
	return &TCPTracker{
		counts: make(map[int64]int64),
	}
}

func (t *TCPTracker) Acquire(userID int64, limit int64) error {
	if t == nil || userID <= 0 || limit == 0 {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.counts[userID] >= limit {
		return ErrTCPLimitExceeded
	}
	t.counts[userID]++
	return nil
}

func (t *TCPTracker) Release(userID int64) {
	if t == nil || userID <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	current := t.counts[userID]
	if current <= 1 {
		delete(t.counts, userID)
		return
	}
	t.counts[userID] = current - 1
}
