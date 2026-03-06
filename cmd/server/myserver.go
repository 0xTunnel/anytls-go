package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"crypto/tls"
)

type myServer struct {
	tlsConfig     *tls.Config
	snapshotStore *state.Store
	deviceTracker *state.DeviceTracker
	traffic       *state.TrafficAggregator
	panelClient   *ppanel.Client
}

func NewNodeServer(tlsConfig *tls.Config, snapshotStore *state.Store, deviceTracker *state.DeviceTracker, traffic *state.TrafficAggregator, panelClient *ppanel.Client) *myServer {
	return &myServer{
		tlsConfig:     tlsConfig,
		snapshotStore: snapshotStore,
		deviceTracker: deviceTracker,
		traffic:       traffic,
		panelClient:   panelClient,
	}
}

func (s *myServer) IsNodeMode() bool {
	return s != nil && s.snapshotStore != nil && s.deviceTracker != nil && s.traffic != nil && s.panelClient != nil
}
