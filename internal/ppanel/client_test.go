package ppanel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
