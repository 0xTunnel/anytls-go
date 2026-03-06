package main

import (
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"errors"
	"strings"
	"testing"
)

func TestSelectProtocolAdapterDefaultsToAnyTLS(t *testing.T) {
	t.Parallel()

	adapter, err := selectProtocolAdapter("")
	if err != nil {
		t.Fatalf("selectProtocolAdapter() error = %v", err)
	}
	if got := adapter.Name(); got != defaultNodeProtocol {
		t.Fatalf("adapter.Name() = %q, want %q", got, defaultNodeProtocol)
	}
}

func TestSelectProtocolAdapterRejectsUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	_, err := selectProtocolAdapter("trojan")
	if err == nil {
		t.Fatal("selectProtocolAdapter() expected error")
	}
	if !strings.Contains(err.Error(), "supported protocols") {
		t.Fatalf("error = %v", err)
	}
}

func TestCurrentProtocolAdapterUsesSnapshotProtocol(t *testing.T) {
	t.Parallel()

	server := &myServer{snapshotStore: state.NewStore(&state.Snapshot{Protocol: defaultNodeProtocol})}
	adapter, err := server.currentProtocolAdapter()
	if err != nil {
		t.Fatalf("currentProtocolAdapter() error = %v", err)
	}
	if adapter.Name() != defaultNodeProtocol {
		t.Fatalf("adapter.Name() = %q", adapter.Name())
	}
}

func TestCurrentProtocolAdapterRejectsUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	server := &myServer{snapshotStore: state.NewStore(&state.Snapshot{Protocol: "trojan"})}
	_, err := server.currentProtocolAdapter()
	if err == nil {
		t.Fatal("currentProtocolAdapter() expected error")
	}
}

func TestAnyTLSProtocolAdapterRejectsInvalidPort(t *testing.T) {
	t.Parallel()

	adapter := anyTLSProtocolAdapter{}
	_, err := adapter.BuildSnapshot(&ppanel.ServerConfigResponse{Protocol: defaultNodeProtocol}, []ppanel.ServerUser{{ID: 1, UUID: "user-1"}})
	if err == nil {
		t.Fatal("BuildSnapshot() expected invalid port error")
	}
}

func TestAuthenticateByHashReturnsSnapshotUnavailable(t *testing.T) {
	t.Parallel()

	server := &myServer{}
	_, err := server.authenticateByHash(make([]byte, 32), "127.0.0.1:1234", "127.0.0.1")
	if !errors.Is(err, errSnapshotUnavailable) {
		t.Fatalf("authenticateByHash() error = %v, want %v", err, errSnapshotUnavailable)
	}
}
