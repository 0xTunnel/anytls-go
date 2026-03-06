package ppanel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientFetchConfigDecodesRawJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/server/config" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("protocol"); got != "anytls" {
			t.Fatalf("protocol = %q", got)
		}
		if got := r.URL.Query().Get("server_id"); got != "1" {
			t.Fatalf("server_id = %q", got)
		}
		if got := r.URL.Query().Get("secret_key"); got != "secret" {
			t.Fatalf("secret_key = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"basic": map[string]any{
				"push_interval": 60,
				"pull_interval": 30,
			},
			"protocol": "anytls",
			"config": map[string]any{
				"port": 1110,
				"security_config": map[string]any{
					"padding_scheme": "stop=1",
				},
			},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	config, err := client.FetchConfig(context.Background())
	if err != nil {
		t.Fatalf("FetchConfig() error = %v", err)
	}
	if config.Config.Port != 1110 {
		t.Fatalf("port = %d", config.Config.Port)
	}
	if config.Protocol != "anytls" {
		t.Fatalf("protocol = %q", config.Protocol)
	}
	if config.Basic.PullInterval != 30 {
		t.Fatalf("pull interval = %d", config.Basic.PullInterval)
	}
	if config.Basic.PushInterval != 60 {
		t.Fatalf("push interval = %d", config.Basic.PushInterval)
	}
	if got := config.Config.EffectivePaddingScheme(); got != "stop=1" {
		t.Fatalf("padding scheme = %q", got)
	}
}

func TestClientFetchUsersDecodesRawJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/server/user" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"users": []map[string]any{{
				"id":           1,
				"uuid":         "user-uuid",
				"speed_limit":  1024,
				"device_limit": 3,
			}},
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	users, err := client.FetchUsers(context.Background())
	if err != nil {
		t.Fatalf("FetchUsers() error = %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("len(users) = %d", len(users))
	}
	if users[0].UUID != "user-uuid" {
		t.Fatalf("uuid = %q", users[0].UUID)
	}
	if users[0].SpeedLimit != 1024 {
		t.Fatalf("speed_limit = %d", users[0].SpeedLimit)
	}
}

func TestClientPushStatusAcceptsNoContent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/v1/server/status" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.PushStatus(context.Background(), ServerStatusRequest{CPU: 1.5})
	if err != nil {
		t.Fatalf("PushStatus() error = %v", err)
	}
}

func TestClientReturnsHTTPErrorBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, 1, "secret")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.FetchConfig(context.Background())
	if err == nil {
		t.Fatal("FetchConfig() expected error")
	}
	if !strings.Contains(err.Error(), "returned 400") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("error = %v", err)
	}
}

func TestWriteUserListWritesPrettyJSON(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "cache", "ppanel-users.json")
	users := []ServerUser{{ID: 1, UUID: "a3de8552-ba4f-4493-ad71-0611869ecd89", SpeedLimit: 1073741824, DeviceLimit: 3}}

	if err := WriteUserList(outputPath, users); err != nil {
		t.Fatalf("WriteUserList() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "\n  \"users\": [\n") {
		t.Fatalf("WriteUserList() output = %q, want indented json", string(data))
	}

	var decoded UserListResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(decoded.Users) != 1 {
		t.Fatalf("len(decoded.Users) = %d", len(decoded.Users))
	}
	if decoded.Users[0].UUID != users[0].UUID {
		t.Fatalf("decoded uuid = %q, want %q", decoded.Users[0].UUID, users[0].UUID)
	}
	if decoded.Users[0].SpeedLimit != users[0].SpeedLimit {
		t.Fatalf("decoded speed_limit = %d, want %d", decoded.Users[0].SpeedLimit, users[0].SpeedLimit)
	}
	if decoded.Users[0].DeviceLimit != users[0].DeviceLimit {
		t.Fatalf("decoded device_limit = %d, want %d", decoded.Users[0].DeviceLimit, users[0].DeviceLimit)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %#o, want %#o", info.Mode().Perm(), os.FileMode(0600))
	}
}

func TestWriteUserListOverwritesExistingFile(t *testing.T) {
	t.Parallel()

	outputPath := filepath.Join(t.TempDir(), "ppanel-users.json")
	if err := os.WriteFile(outputPath, []byte("stale"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	users := []ServerUser{{ID: 2, UUID: "user-2", SpeedLimit: 2048, DeviceLimit: 4}}
	if err := WriteUserList(outputPath, users); err != nil {
		t.Fatalf("WriteUserList() error = %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "stale") {
		t.Fatalf("WriteUserList() output = %q, want stale content removed", string(data))
	}
}
