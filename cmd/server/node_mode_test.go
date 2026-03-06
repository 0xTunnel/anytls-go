package main

import (
	"anytls/internal/config"
	"anytls/internal/ppanel"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRememberSnapshotSignature(t *testing.T) {
	server := &myServer{}

	if changed := server.rememberSnapshotSignature("first"); !changed {
		t.Fatal("rememberSnapshotSignature(first) = false, want true")
	}
	if changed := server.rememberSnapshotSignature("first"); changed {
		t.Fatal("rememberSnapshotSignature(first) = true, want false for unchanged snapshot")
	}
	if changed := server.rememberSnapshotSignature("second"); !changed {
		t.Fatal("rememberSnapshotSignature(second) = false, want true for changed snapshot")
	}
}

func TestResolveUserSnapshotPathUsesConfigDirEvenWhenLogDirConfigured(t *testing.T) {
	t.Parallel()

	nodeConfig := &config.NodeConfig{
		LogFileDir: "/tmp/anytls/logs",
		Path:       "/tmp/anytls/node.toml",
	}

	if got := resolveUserSnapshotPath(nodeConfig); got != "/tmp/anytls/users.json" {
		t.Fatalf("resolveUserSnapshotPath() = %q", got)
	}
}

func TestResolveUserSnapshotPathFallsBackToConfigDir(t *testing.T) {
	t.Parallel()

	nodeConfig := &config.NodeConfig{Path: "/tmp/anytls/node.toml"}

	if got := resolveUserSnapshotPath(nodeConfig); got != "/tmp/anytls/users.json" {
		t.Fatalf("resolveUserSnapshotPath() = %q", got)
	}
}

func TestFetchNodeSnapshotPersistsUsers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/server/config":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"basic": map[string]any{
					"pull_interval": 30,
					"push_interval": 60,
				},
				"protocol": "anytls",
				"config": map[string]any{
					"port": 8443,
				},
			}); err != nil {
				t.Fatalf("Encode(config) error = %v", err)
			}
		case "/v1/server/user":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"users": []map[string]any{{
					"id":           1,
					"uuid":         "a3de8552-ba4f-4493-ad71-0611869ecd89",
					"speed_limit":  1073741824,
					"device_limit": 3,
				}},
			}); err != nil {
				t.Fatalf("Encode(users) error = %v", err)
			}
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := ppanel.NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "cache", "users.json")
	snapshot, err := fetchNodeSnapshot(context.Background(), client, outputPath)
	if err != nil {
		t.Fatalf("fetchNodeSnapshot() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("fetchNodeSnapshot() returned nil snapshot")
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var users ppanel.UserListResponse
	if err := json.Unmarshal(data, &users); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(users.Users) != 1 {
		t.Fatalf("len(users.Users) = %d", len(users.Users))
	}
	if users.Users[0].UUID != "a3de8552-ba4f-4493-ad71-0611869ecd89" {
		t.Fatalf("users.Users[0].UUID = %q", users.Users[0].UUID)
	}
}

func TestFetchNodeSnapshotRejectsUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/server/config":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"basic":    map[string]any{},
				"protocol": "trojan",
				"config":   map[string]any{},
			}); err != nil {
				t.Fatalf("Encode(config) error = %v", err)
			}
		case "/v1/server/user":
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"users": []map[string]any{}}); err != nil {
				t.Fatalf("Encode(users) error = %v", err)
			}
		default:
			t.Fatalf("path = %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := ppanel.NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = fetchNodeSnapshot(context.Background(), client, "")
	if err == nil {
		t.Fatal("fetchNodeSnapshot() expected error")
	}
}
