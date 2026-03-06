package ppanel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const defaultProtocol = "anytls"

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	protocol   string
	serverID   int64
	secretKey  string
}

func NewClient(rawURL string, serverID int64, secretKey string) (*Client, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("panel url is required")
	}
	parsed, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse panel url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid panel url: %s", rawURL)
	}
	if serverID <= 0 {
		return nil, fmt.Errorf("server id must be greater than zero")
	}
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("secret key is required")
	}
	return &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		protocol:  defaultProtocol,
		serverID:  serverID,
		secretKey: secretKey,
	}, nil
}

func (c *Client) FetchConfig(ctx context.Context) (*ServerConfigResponse, error) {
	var out ServerConfigResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/server/config", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) FetchUsers(ctx context.Context) ([]ServerUser, error) {
	var out UserListResponse
	if err := c.doRequest(ctx, http.MethodGet, "/v1/server/user", nil, &out); err != nil {
		return nil, err
	}
	return out.Users, nil
}

func (c *Client) PushOnlineUsers(ctx context.Context, users []OnlineUser) error {
	return c.doRequest(ctx, http.MethodPost, "/v1/server/online", OnlineUsersRequest{Users: users}, nil)
}

func (c *Client) PushUserTraffic(ctx context.Context, traffic []UserTraffic) error {
	return c.doRequest(ctx, http.MethodPost, "/v1/server/push", PushTrafficRequest{Traffic: traffic}, nil)
}

func (c *Client) PushStatus(ctx context.Context, status ServerStatusRequest) error {
	return c.doRequest(ctx, http.MethodPost, "/v1/server/status", status, nil)
}

func (c *Client) doRequest(ctx context.Context, method, requestPath string, body any, out any) error {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(endpoint.Path, requestPath)
	query := endpoint.Query()
	query.Set("protocol", c.protocol)
	query.Set("server_id", fmt.Sprintf("%d", c.serverID))
	query.Set("secret_key", c.secretKey)
	endpoint.RawQuery = query.Encode()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request %s %s: %w", method, endpoint.String(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("panel api %s %s returned %d: %s", method, endpoint.Path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	if out == nil {
		var envelope ResponseEnvelope[json.RawMessage]
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			return fmt.Errorf("decode response envelope: %w", err)
		}
		if envelope.Code != 0 && envelope.Code != http.StatusOK {
			return fmt.Errorf("panel api %s %s failed: code=%d msg=%s", method, endpoint.Path, envelope.Code, envelope.Msg)
		}
		return nil
	}

	var envelope ResponseEnvelope[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode response envelope: %w", err)
	}
	if envelope.Code != 0 && envelope.Code != http.StatusOK {
		return fmt.Errorf("panel api %s %s failed: code=%d msg=%s", method, endpoint.Path, envelope.Code, envelope.Msg)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode response data: %w", err)
	}
	return nil
}
