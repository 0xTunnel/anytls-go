package state

import (
	"errors"
	"sort"
	"sync"
)

var ErrDeviceLimitExceeded = errors.New("device limit exceeded")

type OnlineEntry struct {
	UserID int64
	IP     string
}

type DeviceTracker struct {
	mu    sync.Mutex
	users map[int64]map[string]int
}

func NewDeviceTracker() *DeviceTracker {
	return &DeviceTracker{
		users: make(map[int64]map[string]int),
	}
}

func (t *DeviceTracker) Acquire(userID int64, ip string, limit int64) error {
	if t == nil || userID <= 0 || ip == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.users[userID]; !ok {
		t.users[userID] = make(map[string]int)
	}
	entries := t.users[userID]
	if current, ok := entries[ip]; ok {
		entries[ip] = current + 1
		return nil
	}
	if limit > 0 && int64(len(entries)) >= limit {
		return ErrDeviceLimitExceeded
	}
	entries[ip] = 1
	return nil
}

func (t *DeviceTracker) Release(userID int64, ip string) {
	if t == nil || userID <= 0 || ip == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entries, ok := t.users[userID]
	if !ok {
		return
	}
	current, ok := entries[ip]
	if !ok {
		return
	}
	if current <= 1 {
		delete(entries, ip)
	} else {
		entries[ip] = current - 1
	}
	if len(entries) == 0 {
		delete(t.users, userID)
	}
}

func (t *DeviceTracker) OnlineEntries() []OnlineEntry {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	entries := make([]OnlineEntry, 0, len(t.users))
	for userID, ips := range t.users {
		if len(ips) == 0 {
			continue
		}
		ipList := make([]string, 0, len(ips))
		for ip := range ips {
			ipList = append(ipList, ip)
		}
		sort.Strings(ipList)
		entries = append(entries, OnlineEntry{UserID: userID, IP: ipList[0]})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UserID < entries[j].UserID
	})
	return entries
}
