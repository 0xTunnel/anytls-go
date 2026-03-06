package state

import (
	"errors"
	"testing"
)

func TestDeviceTrackerRespectsLimit(t *testing.T) {
	tracker := NewDeviceTracker()
	if err := tracker.Acquire(1, "10.0.0.1", 2); err != nil {
		t.Fatalf("Acquire() first ip error = %v", err)
	}
	if err := tracker.Acquire(1, "10.0.0.2", 2); err != nil {
		t.Fatalf("Acquire() second ip error = %v", err)
	}
	err := tracker.Acquire(1, "10.0.0.3", 2)
	if !errors.Is(err, ErrDeviceLimitExceeded) {
		t.Fatalf("Acquire() error = %v, want %v", err, ErrDeviceLimitExceeded)
	}
}

func TestDeviceTrackerReleasesSlots(t *testing.T) {
	tracker := NewDeviceTracker()
	if err := tracker.Acquire(7, "10.0.0.1", 1); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	tracker.Release(7, "10.0.0.1")
	if err := tracker.Acquire(7, "10.0.0.2", 1); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	entries := tracker.OnlineEntries()
	if len(entries) != 1 || entries[0].IP != "10.0.0.2" {
		t.Fatalf("unexpected online entries: %+v", entries)
	}
}
