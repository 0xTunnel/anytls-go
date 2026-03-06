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
			Port:          443,
			PaddingScheme: "stop=1",
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
