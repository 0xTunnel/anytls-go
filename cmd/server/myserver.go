package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"crypto/tls"
	"errors"
	"sync"
	"time"
)

var errSnapshotUnavailable = errors.New("node snapshot is unavailable")
var errAuthNotMatched = errors.New("auth not matched")

type networkTimeouts struct {
	TCP time.Duration
	UDP time.Duration
}

type myServer struct {
	tlsConfig        *tls.Config
	snapshotStore    *state.Store
	deviceTracker    *state.DeviceTracker
	tcpTracker       *state.TCPTracker
	traffic          *state.TrafficAggregator
	panelClient      *ppanel.Client
	userSnapshotPath string
	networkTimeouts  networkTimeouts
	tcpLimit         int64

	snapshotLogMu         sync.Mutex
	lastSnapshotSignature string
}

type userAccess struct {
	User    state.User
	ConnTag string
	Release func()
}

func NewNodeServer(tlsConfig *tls.Config, snapshotStore *state.Store, deviceTracker *state.DeviceTracker, tcpTracker *state.TCPTracker, traffic *state.TrafficAggregator, panelClient *ppanel.Client, userSnapshotPath string, timeouts networkTimeouts, tcpLimit int64) *myServer {
	server := &myServer{
		tlsConfig:        tlsConfig,
		snapshotStore:    snapshotStore,
		deviceTracker:    deviceTracker,
		tcpTracker:       tcpTracker,
		traffic:          traffic,
		panelClient:      panelClient,
		userSnapshotPath: userSnapshotPath,
		networkTimeouts:  timeouts,
		tcpLimit:         tcpLimit,
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

func (s *myServer) TLSConfig() *tls.Config {
	if s == nil {
		return nil
	}
	return s.tlsConfig
}

func (s *myServer) TrafficRecorder() *state.TrafficAggregator {
	if s == nil {
		return nil
	}
	return s.traffic
}

func (s *myServer) NetworkTimeouts() networkTimeouts {
	if s == nil {
		return networkTimeouts{}
	}
	return s.networkTimeouts
}

func (s *myServer) currentSnapshot() *state.Snapshot {
	if s == nil || s.snapshotStore == nil {
		return nil
	}
	return s.snapshotStore.Load()
}

func (s *myServer) currentProtocolAdapter() (protocolAdapter, error) {
	snapshot := s.currentSnapshot()
	if snapshot == nil {
		return nil, errSnapshotUnavailable
	}
	return selectProtocolAdapter(snapshot.Protocol)
}

func (s *myServer) authenticateByHash(authHash []byte, remoteAddr string, remoteIP string) (*userAccess, error) {
	snapshot := s.currentSnapshot()
	if snapshot == nil {
		return nil, errSnapshotUnavailable
	}
	user, ok := snapshot.LookupAuthHash(authHash)
	if !ok {
		return nil, errAuthNotMatched
	}
	access := &userAccess{
		User:    user,
		ConnTag: buildConnectionTag(user.ID, remoteAddr),
		Release: func() {},
	}
	if err := s.deviceTracker.Acquire(user.ID, remoteIP, user.DeviceLimit); err != nil {
		return access, err
	}
	if err := s.tcpTracker.Acquire(user.ID, s.tcpLimit); err != nil {
		s.deviceTracker.Release(user.ID, remoteIP)
		return access, err
	}
	access.Release = func() {
		s.deviceTracker.Release(user.ID, remoteIP)
		s.tcpTracker.Release(user.ID)
	}
	return access, nil
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
