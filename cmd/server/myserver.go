package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"crypto/tls"
	"sync"
)

type myServer struct {
	tlsConfig        *tls.Config
	snapshotStore    *state.Store
	deviceTracker    *state.DeviceTracker
	traffic          *state.TrafficAggregator
	panelClient      *ppanel.Client
	userSnapshotPath string

	snapshotLogMu         sync.Mutex
	lastSnapshotSignature string
}

func NewNodeServer(tlsConfig *tls.Config, snapshotStore *state.Store, deviceTracker *state.DeviceTracker, traffic *state.TrafficAggregator, panelClient *ppanel.Client, userSnapshotPath string) *myServer {
	server := &myServer{
		tlsConfig:        tlsConfig,
		snapshotStore:    snapshotStore,
		deviceTracker:    deviceTracker,
		traffic:          traffic,
		panelClient:      panelClient,
		userSnapshotPath: userSnapshotPath,
	}
	if snapshotStore != nil {
		if snapshot := snapshotStore.Load(); snapshot != nil {
			server.lastSnapshotSignature = snapshotSignature(snapshot)
		}
	}
	return server
}

func (s *myServer) IsNodeMode() bool {
	return s != nil && s.snapshotStore != nil && s.deviceTracker != nil && s.traffic != nil && s.panelClient != nil
}

func (s *myServer) rememberSnapshotSignature(signature string) bool {
	if s == nil {
		return false
	}
	s.snapshotLogMu.Lock()
	defer s.snapshotLogMu.Unlock()
	if s.lastSnapshotSignature == signature {
		return false
	}
	s.lastSnapshotSignature = signature
	return true
}
