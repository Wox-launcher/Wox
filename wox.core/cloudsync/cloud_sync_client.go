package cloudsync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"wox/util"
)

type CloudSyncAuthProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type StaticAuthProvider struct {
	Token string
}

func (p StaticAuthProvider) AccessToken(ctx context.Context) (string, error) {
	_ = ctx
	return p.Token, nil
}

type CloudSyncHTTPClientConfig struct {
	BaseURL        string
	AuthProvider   CloudSyncAuthProvider
	DeviceProvider CloudSyncDeviceProvider
	AppVersion     string
	Platform       string
	HTTPClient     *http.Client
}

type CloudSyncHTTPClient struct {
	baseURL        string
	authProvider   CloudSyncAuthProvider
	deviceProvider CloudSyncDeviceProvider
	appVersion     string
	platform       string
	httpClient     *http.Client
}

func NewCloudSyncHTTPClient(config CloudSyncHTTPClientConfig) (*CloudSyncHTTPClient, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("cloud sync base url is empty")
	}

	return &CloudSyncHTTPClient{
		baseURL:        baseURL,
		authProvider:   config.AuthProvider,
		deviceProvider: config.DeviceProvider,
		appVersion:     strings.TrimSpace(config.AppVersion),
		platform:       strings.TrimSpace(config.Platform),
		httpClient:     config.HTTPClient,
	}, nil
}

func (c *CloudSyncHTTPClient) Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error) {
	var resp CloudSyncPushResponse
	if err := c.post(ctx, "/v1/sync/push", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	var resp CloudSyncPullResponse
	if err := c.post(ctx, "/v1/sync/pull", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	var resp CloudSyncPullResponse
	if err := c.post(ctx, "/v1/sync/snapshot", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) InitKey(ctx context.Context, req CloudSyncKeyInitRequest) (*CloudSyncKeyInitResponse, error) {
	var resp CloudSyncKeyInitResponse
	if err := c.post(ctx, "/v1/sync/key/init", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) FetchKey(ctx context.Context, req CloudSyncKeyFetchRequest) (*CloudSyncKeyFetchResponse, error) {
	var resp CloudSyncKeyFetchResponse
	if err := c.post(ctx, "/v1/sync/key/fetch", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) PrepareKeyReset(ctx context.Context) (*CloudSyncKeyResetPrepareResponse, error) {
	var resp CloudSyncKeyResetPrepareResponse
	if err := c.post(ctx, "/v1/sync/key/reset/prepare", map[string]any{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) ResetKey(ctx context.Context, req CloudSyncKeyResetRequest) (*CloudSyncKeyResetResponse, error) {
	var resp CloudSyncKeyResetResponse
	if err := c.post(ctx, "/v1/sync/key/reset", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) post(ctx context.Context, path string, body any, target any) error {
	url := c.baseURL + path
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if token, err := c.resolveAccessToken(ctx); err != nil {
		return err
	} else if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if deviceID, err := c.resolveDeviceID(ctx); err != nil {
		return err
	} else if deviceID != "" {
		req.Header.Set("X-Device-Id", deviceID)
	}

	if c.appVersion != "" {
		req.Header.Set("X-App-Version", c.appVersion)
	}

	platform := c.platform
	if platform == "" {
		platform = util.GetCurrentPlatform()
	}
	if platform != "" {
		req.Header.Set("X-Platform", platform)
	}

	if traceId := util.GetContextTraceId(ctx); traceId != "" {
		req.Header.Set("X-Trace-Id", traceId)
	}

	client := c.httpClient
	if client == nil {
		client = util.GetHTTPClient(ctx)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cloud sync request failed (%d): %s", resp.StatusCode, readResponseBody(resp.Body))
	}

	if target == nil {
		return nil
	}

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (c *CloudSyncHTTPClient) resolveAccessToken(ctx context.Context) (string, error) {
	if c.authProvider == nil {
		return "", nil
	}
	return c.authProvider.AccessToken(ctx)
}

func (c *CloudSyncHTTPClient) resolveDeviceID(ctx context.Context) (string, error) {
	if c.deviceProvider == nil {
		return "", nil
	}
	return c.deviceProvider.DeviceID(ctx)
}

func readResponseBody(reader io.Reader) string {
	payload, err := io.ReadAll(reader)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(payload))
}
