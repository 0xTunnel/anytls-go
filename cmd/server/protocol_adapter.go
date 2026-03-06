package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"anytls/proxy/padding"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strings"
)

const defaultNodeProtocol = "anytls"

type protocolAdapter interface {
	Name() string
	BuildSnapshot(config *ppanel.ServerConfigResponse, users []ppanel.ServerUser) (*state.Snapshot, error)
	HandleConn(ctx context.Context, conn net.Conn, runtime protocolRuntime)
}

type protocolRuntime interface {
	TLSConfig() *tls.Config
	IsNodeMode() bool
	authenticateByHash(authHash []byte, remoteAddr string, remoteIP string) (*userAccess, error)
	TrafficRecorder() *state.TrafficAggregator
	NetworkTimeouts() networkTimeouts
}

type anyTLSProtocolAdapter struct{}

var protocolAdapters = map[string]protocolAdapter{
	defaultNodeProtocol: anyTLSProtocolAdapter{},
}

func normalizeProtocolName(protocol string) string {
	normalized := strings.ToLower(strings.TrimSpace(protocol))
	if normalized == "" {
		return defaultNodeProtocol
	}
	return normalized
}

func supportedProtocolNames() []string {
	names := make([]string, 0, len(protocolAdapters))
	for name := range protocolAdapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func selectProtocolAdapter(protocol string) (protocolAdapter, error) {
	normalized := normalizeProtocolName(protocol)
	adapter, ok := protocolAdapters[normalized]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol %q, supported protocols: %s", normalized, strings.Join(supportedProtocolNames(), ", "))
	}
	return adapter, nil
}

func (anyTLSProtocolAdapter) Name() string {
	return defaultNodeProtocol
}

func (anyTLSProtocolAdapter) BuildSnapshot(config *ppanel.ServerConfigResponse, users []ppanel.ServerUser) (*state.Snapshot, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if normalizeProtocolName(config.Protocol) != defaultNodeProtocol {
		return nil, fmt.Errorf("unsupported protocol %q for anytls adapter", config.Protocol)
	}
	snapshot, err := state.BuildSnapshot(config, users)
	if err != nil {
		return nil, err
	}
	if snapshot.Port <= 0 {
		return nil, fmt.Errorf("invalid anytls port %d", snapshot.Port)
	}
	if snapshot.PaddingScheme != "" && !padding.UpdatePaddingScheme([]byte(snapshot.PaddingScheme)) {
		return nil, fmt.Errorf("invalid padding scheme from panel")
	}
	return snapshot, nil
}

func (adapter anyTLSProtocolAdapter) HandleConn(ctx context.Context, conn net.Conn, runtime protocolRuntime) {
	handleAnyTLSConnection(ctx, conn, runtime)
}
