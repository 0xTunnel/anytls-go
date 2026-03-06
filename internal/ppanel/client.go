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

	"github.com/sirupsen/logrus"
)

const defaultProtocol = "anytls"

const maxResponseBodyBytes = 1 << 20

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	protocol   string
	serverID   int64
	secretKey  string
}

func NewClient(rawURL string, serverID int64, secretKey string) (*Client, error) {
	return NewClientWithProtocol(rawURL, serverID, secretKey, defaultProtocol)
}

func NewClientWithProtocol(rawURL string, serverID int64, secretKey string, protocol string) (*Client, error) {
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
	normalizedProtocol := strings.ToLower(strings.TrimSpace(protocol))
	if normalizedProtocol == "" {
		normalizedProtocol = defaultProtocol
	}
	return &Client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		protocol:  normalizedProtocol,
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
	requestBytes := 0
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		requestBytes = len(payload)
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	startedAt := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logRequest(method, endpoint.Path, 0, time.Since(startedAt), requestBytes, 0, err)
		return fmt.Errorf("perform request %s %s: %w", method, endpoint.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		if readErr != nil {
			requestErr := fmt.Errorf("panel api %s %s returned %d and response body could not be read: %w", method, endpoint.Path, resp.StatusCode, readErr)
			c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, 0, requestErr)
			return requestErr
		}
		requestErr := fmt.Errorf("panel api %s %s returned %d: %s", method, endpoint.Path, resp.StatusCode, strings.TrimSpace(string(payload)))
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, len(payload), requestErr)
		return requestErr
	}

	if out == nil {
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, 0, nil)
		return nil
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		requestErr := fmt.Errorf("read response body: %w", err)
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, 0, requestErr)
		return requestErr
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) > maxResponseBodyBytes {
		requestErr := fmt.Errorf("response body exceeds %d bytes", maxResponseBodyBytes)
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, len(payload), requestErr)
		return requestErr
	}
	if len(payload) == 0 || string(payload) == "null" {
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, len(payload), nil)
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		requestErr := fmt.Errorf("decode response body: %w", err)
		c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, len(payload), requestErr)
		return requestErr
	}
	c.logRequest(method, endpoint.Path, resp.StatusCode, time.Since(startedAt), requestBytes, len(payload), nil)
	return nil
}

func (c *Client) logRequest(method string, requestPath string, statusCode int, duration time.Duration, requestBytes int, responseBytes int, err error) {
	fields := logrus.Fields{
		"component":      "panel",
		"event":          "api_request",
		"method":         method,
		"path":           requestPath,
		"server_id":      c.serverID,
		"duration_ms":    duration.Milliseconds(),
		"request_bytes":  requestBytes,
		"response_bytes": responseBytes,
	}
	if statusCode > 0 {
		fields["status_code"] = statusCode
	}
	entry := logrus.WithFields(fields)
	if err != nil {
		entry.WithError(err).Warn("panel api request failed")
		return
	}
	entry.Debug("panel api request completed")
}
