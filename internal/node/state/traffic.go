package state

import "sync"

type TrafficStats struct {
	Upload   int64
	Download int64
}

type TrafficAggregator struct {
	mu      sync.Mutex
	pending map[int64]TrafficStats
}

func NewTrafficAggregator() *TrafficAggregator {
	return &TrafficAggregator{pending: make(map[int64]TrafficStats)}
}

func (a *TrafficAggregator) AddUpload(userID int64, n int64) {
	a.add(userID, n, 0)
}

func (a *TrafficAggregator) AddDownload(userID int64, n int64) {
	a.add(userID, 0, n)
}

func (a *TrafficAggregator) add(userID int64, upload, download int64) {
	if a == nil || userID <= 0 || (upload <= 0 && download <= 0) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	current := a.pending[userID]
	current.Upload += upload
	current.Download += download
	a.pending[userID] = current
}

func (a *TrafficAggregator) DrainAll() map[int64]TrafficStats {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.pending) == 0 {
		return nil
	}
	result := a.pending
	a.pending = make(map[int64]TrafficStats)
	return result
}

func (a *TrafficAggregator) Restore(pending map[int64]TrafficStats) {
	if a == nil || len(pending) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for userID, stats := range pending {
		current := a.pending[userID]
		current.Upload += stats.Upload
		current.Download += stats.Download
		a.pending[userID] = current
	}
}
