package cloudsync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"wox/i18n"
	"wox/util"
)

type CloudSyncAuthProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

type CloudSyncRefreshAuthProvider interface {
	CloudSyncAuthProvider
	RefreshAccessToken(ctx context.Context) (string, error)
	MarkSessionExpired(ctx context.Context)
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
	baseURLMu      sync.RWMutex
	authProvider   CloudSyncAuthProvider
	deviceProvider CloudSyncDeviceProvider
	appVersion     string
	platform       string
	httpClient     *http.Client
}

type responseEnvelope struct {
	Status  int             `json:"status"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type CloudSyncRequestError struct {
	Code          string
	Message       string
	NextSyncAfter int64
}

func (e *CloudSyncRequestError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

func NewCloudSyncHTTPClient(config CloudSyncHTTPClientConfig) (*CloudSyncHTTPClient, error) {
	baseURL := normalizeBaseURL(config.BaseURL)
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

func (c *CloudSyncHTTPClient) SetBaseURL(baseURL string) {
	if c == nil {
		return
	}
	c.baseURLMu.Lock()
	defer c.baseURLMu.Unlock()
	c.baseURL = normalizeBaseURL(baseURL)
}

func (c *CloudSyncHTTPClient) BaseURL() string {
	if c == nil {
		return ""
	}
	c.baseURLMu.RLock()
	defer c.baseURLMu.RUnlock()
	return c.baseURL
}

func normalizeBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
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

func (c *CloudSyncHTTPClient) ListRecordKeys(ctx context.Context, req CloudSyncRecordKeyListRequest) (*CloudSyncRecordKeyListResponse, error) {
	var resp CloudSyncRecordKeyListResponse
	if err := c.post(ctx, "/v1/sync/record-keys", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) ListDevices(ctx context.Context, req CloudSyncDeviceListRequest) (*CloudSyncDeviceListResponse, error) {
	var resp CloudSyncDeviceListResponse
	if err := c.post(ctx, "/v1/sync/devices/list", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) RevokeDevice(ctx context.Context, req CloudSyncDeviceRevokeRequest) (*CloudSyncDeviceRevokeResponse, error) {
	var resp CloudSyncDeviceRevokeResponse
	if err := c.post(ctx, "/v1/sync/devices/revoke", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) UpdateDevice(ctx context.Context, req CloudSyncDeviceUpdateRequest) (*CloudSyncDeviceUpdateResponse, error) {
	var resp CloudSyncDeviceUpdateResponse
	if err := c.post(ctx, "/v1/sync/devices/update", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *CloudSyncHTTPClient) Status(ctx context.Context) (CloudSyncKeyStatus, error) {
	var resp CloudSyncKeyStatus
	if err := c.post(ctx, "/v1/sync/key/status", map[string]any{}, &resp); err != nil {
		return CloudSyncKeyStatus{}, err
	}
	return resp, nil
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
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	if err := c.postWithToken(ctx, path, payload, target, ""); err != nil {
		if errors.Is(err, errCloudSyncUnauthorized) {
			if refreshProvider, ok := c.authProvider.(CloudSyncRefreshAuthProvider); ok {
				token, refreshErr := refreshProvider.RefreshAccessToken(ctx)
				if refreshErr != nil {
					refreshProvider.MarkSessionExpired(ctx)
					return err
				}
				return c.postWithToken(ctx, path, payload, target, token)
			}
		}
		return err
	}
	return nil
}

var errCloudSyncUnauthorized = fmt.Errorf("cloud sync unauthorized")

func (c *CloudSyncHTTPClient) postWithToken(ctx context.Context, path string, payload []byte, target any, tokenOverride string) error {
	url := c.BaseURL() + path

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", strings.ReplaceAll(string(i18n.GetI18nManager().GetCurrentLangCode()), "_", "-"))

	token := tokenOverride
	if token == "" {
		resolvedToken, err := c.resolveAccessToken(ctx)
		if err != nil {
			return err
		}
		token = resolvedToken
	}
	if token != "" {
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

	responsePayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var envelope responseEnvelope
	envelopeErr := json.Unmarshal(responsePayload, &envelope)
	if resp.StatusCode == http.StatusUnauthorized {
		return errCloudSyncUnauthorized
	}
	if resp.StatusCode >= 400 {
		if envelopeErr == nil && envelope.Code != "" {
			return cloudSyncRequestErrorFromEnvelope(envelope)
		}
		if envelopeErr == nil && envelope.Message != "" {
			return errors.New(envelope.Message)
		}
		return fmt.Errorf("cloud sync request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responsePayload)))
	}

	if target == nil {
		return nil
	}

	if envelopeErr == nil && envelope.Code != "" {
		if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}

	if err := json.Unmarshal(responsePayload, target); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func cloudSyncRequestErrorFromEnvelope(envelope responseEnvelope) error {
	requestErr := &CloudSyncRequestError{Code: envelope.Code, Message: envelope.Message}
	var details struct {
		NextSyncAfter int64 `json:"next_sync_after"`
	}
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		_ = json.Unmarshal(envelope.Data, &details)
		requestErr.NextSyncAfter = details.NextSyncAfter
	}
	return requestErr
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
