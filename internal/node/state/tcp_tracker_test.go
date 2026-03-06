package state

import (
	"errors"
	"testing"
)

func TestTCPTrackerRespectsLimit(t *testing.T) {
	tracker := NewTCPTracker()
	if err := tracker.Acquire(1, 2); err != nil {
		t.Fatalf("Acquire() first connection error = %v", err)
	}
	if err := tracker.Acquire(1, 2); err != nil {
		t.Fatalf("Acquire() second connection error = %v", err)
	}
	err := tracker.Acquire(1, 2)
	if !errors.Is(err, ErrTCPLimitExceeded) {
		t.Fatalf("Acquire() error = %v, want %v", err, ErrTCPLimitExceeded)
	}
}

func TestTCPTrackerReleasesSlots(t *testing.T) {
	tracker := NewTCPTracker()
	if err := tracker.Acquire(7, 1); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	tracker.Release(7)
	if err := tracker.Acquire(7, 1); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
}

func TestTCPTrackerZeroLimitDisablesTracking(t *testing.T) {
	tracker := NewTCPTracker()
	for i := 0; i < 10; i++ {
		if err := tracker.Acquire(9, 0); err != nil {
			t.Fatalf("Acquire() iteration %d error = %v", i, err)
		}
	}
}
