package state

import (
	"anytls/internal/ppanel"
	"crypto/sha256"
	"testing"
	"time"
)

func TestBuildSnapshotCreatesAuthIndex(t *testing.T) {
	config := &ppanel.ServerConfigResponse{
		Basic: ppanel.ServerBasic{
			PullInterval: 30,
			PushInterval: 60,
		},
		Protocol: "anytls",
		Config: ppanel.AnyTLSConfig{
			Port: 443,
			SecurityConfig: &ppanel.SecurityConfig{
				PaddingScheme: "stop=1",
			},
		},
	}
	users := []ppanel.ServerUser{
		{ID: 1, UUID: "uuid-1", DeviceLimit: 2, SpeedLimit: 10},
		{ID: 2, UUID: "uuid-2", DeviceLimit: 3, SpeedLimit: 20},
	}

	snapshot, err := BuildSnapshot(config, users)
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.PullInterval != 30*time.Second {
		t.Fatalf("unexpected pull interval: %v", snapshot.PullInterval)
	}
	if snapshot.PushInterval != 60*time.Second {
		t.Fatalf("unexpected push interval: %v", snapshot.PushInterval)
	}
	if snapshot.PaddingScheme != "stop=1" {
		t.Fatalf("unexpected padding scheme: %q", snapshot.PaddingScheme)
	}
	sum := sha256.Sum256([]byte("uuid-1"))
	user, ok := snapshot.UsersByHash[sum]
	if !ok {
		t.Fatalf("missing auth hash entry")
	}
	if user.ID != 1 || user.DeviceLimit != 2 {
		t.Fatalf("unexpected user data: %+v", user)
	}
	lookupUser, ok := snapshot.LookupAuthHash(sum[:])
	if !ok || lookupUser.ID != 1 {
		t.Fatalf("LookupAuthHash() returned (%+v, %v)", lookupUser, ok)
	}
}

func TestBuildSnapshotRejectsDuplicateAuthHash(t *testing.T) {
	config := &ppanel.ServerConfigResponse{
		Protocol: "anytls",
		Config:   ppanel.AnyTLSConfig{Port: 443},
	}
	_, err := BuildSnapshot(config, []ppanel.ServerUser{{ID: 1, UUID: "same"}, {ID: 2, UUID: "same"}})
	if err == nil {
		t.Fatal("BuildSnapshot() expected duplicate auth hash error")
	}
}

func TestBuildSnapshotUsesNestedPaddingScheme(t *testing.T) {
	config := &ppanel.ServerConfigResponse{
		Protocol: "anytls",
		Config: ppanel.AnyTLSConfig{
			Port: 1110,
			SecurityConfig: &ppanel.SecurityConfig{
				PaddingScheme: "stop=1",
			},
		},
	}

	snapshot, err := BuildSnapshot(config, []ppanel.ServerUser{{ID: 1, UUID: "uuid-1"}})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.PaddingScheme != "stop=1" {
		t.Fatalf("BuildSnapshot() padding scheme = %q, want %q", snapshot.PaddingScheme, "stop=1")
	}
}

func TestBuildSnapshotIgnoresTopLevelPaddingScheme(t *testing.T) {
	config := &ppanel.ServerConfigResponse{
		Protocol: "anytls",
		Config: ppanel.AnyTLSConfig{
			Port:          1110,
			PaddingScheme: "top-level-should-be-ignored",
		},
	}

	snapshot, err := BuildSnapshot(config, []ppanel.ServerUser{{ID: 1, UUID: "uuid-1"}})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.PaddingScheme != "" {
		t.Fatalf("BuildSnapshot() padding scheme = %q, want empty string", snapshot.PaddingScheme)
	}
}

func TestBuildSnapshotKeepsProtocolForAdapterSelection(t *testing.T) {
	config := &ppanel.ServerConfigResponse{
		Protocol: "trojan",
		Config: ppanel.AnyTLSConfig{
			Port: 0,
		},
	}

	snapshot, err := BuildSnapshot(config, []ppanel.ServerUser{{ID: 1, UUID: "uuid-1"}})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.Protocol != "trojan" {
		t.Fatalf("BuildSnapshot() protocol = %q, want %q", snapshot.Protocol, "trojan")
	}
}
