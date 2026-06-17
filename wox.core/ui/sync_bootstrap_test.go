package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"wox/account"
	"wox/cloudsync"
	"wox/database"
	"wox/util"
)

func TestSyncBootstrapRoutesRegistered(t *testing.T) {
	if routers["/sync/bootstrap/status"] == nil {
		t.Fatal("sync bootstrap status route is not registered")
	}
	if routers["/sync/bootstrap/start"] == nil {
		t.Fatal("sync bootstrap start route is not registered")
	}
}

func TestHandleSyncBootstrapStatusRequiresLoggedInEligibleAccount(t *testing.T) {
	initSyncBootstrapRouterTest(t, database.AccountState{})

	response := postSyncBootstrapStatus()

	if response.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d", response.Code, http.StatusOK)
	}
	if !bytes.Contains(response.Body.Bytes(), []byte("account is not logged in")) {
		t.Fatalf("body = %s, want login error", response.Body.String())
	}
}

func TestHandleSyncBootstrapStatusReportsRemoteDataAndKey(t *testing.T) {
	client := &routerCloudSyncClient{
		snapshotResponse: &cloudsync.CloudSyncPullResponse{
			Records: []cloudsync.CloudSyncRecord{{EntityType: cloudsync.EntityWoxSetting, Key: "ThemeId", Op: cloudsync.OpUpsert}},
		},
	}
	keyClient := &routerCloudSyncKeyClient{status: cloudsync.CloudSyncKeyStatus{Available: true, Version: 1}}
	initSyncBootstrapRouterTest(t, database.AccountState{UserID: "user-1", Email: "u@example.com", SyncEligible: true}, client, keyClient)

	response := postSyncBootstrapStatus()

	if response.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d, body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	var envelope struct {
		Data struct {
			HasRemoteData bool `json:"has_remote_data"`
			HasRemoteKey  bool `json:"has_remote_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !envelope.Data.HasRemoteData || !envelope.Data.HasRemoteKey {
		t.Fatalf("data = %#v, want remote data and key", envelope.Data)
	}
	if len(client.snapshotRequests) != 1 || client.snapshotRequests[0].Limit != 1 {
		t.Fatalf("snapshot requests = %#v, want one limit=1 request", client.snapshotRequests)
	}
}

func TestHandleSyncBootstrapStartInitializesKeyAndEnablesSync(t *testing.T) {
	keyClient := &routerCloudSyncKeyClient{status: cloudsync.CloudSyncKeyStatus{Available: false}}
	initSyncBootstrapRouterTest(t, database.AccountState{UserID: "user-1", Email: "u@example.com", SyncEligible: true}, &routerCloudSyncClient{}, keyClient)

	request := httptest.NewRequest(http.MethodPost, "/sync/bootstrap/start", bytes.NewReader([]byte(`{"recovery_code":"test recovery code"}`)))
	response := httptest.NewRecorder()
	routers["/sync/bootstrap/start"](response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d, body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if keyClient.initCalls != 1 {
		t.Fatalf("init key calls = %d, want 1", keyClient.initCalls)
	}
	status := account.GetService().Status(context.Background())
	if !status.SyncEnabled {
		t.Fatal("sync enabled = false, want true")
	}
}

func initSyncBootstrapRouterTest(t *testing.T, accountState database.AccountState, clientAndKey ...any) {
	t.Helper()
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(t.TempDir(), "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(t.TempDir(), "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("init database: %v", err)
	}

	if accountState.ID == 0 {
		accountState.ID = 1
	}
	if err := database.GetDB().Save(&accountState).Error; err != nil {
		t.Fatalf("seed account state: %v", err)
	}
	account.SetService(account.NewService("http://sync.test"))
	t.Cleanup(func() {
		account.SetService(nil)
		cloudsync.SetService(nil)
	})

	var client cloudsync.CloudSyncClient = &routerCloudSyncClient{}
	keyClient := &routerCloudSyncKeyClient{}
	for _, item := range clientAndKey {
		switch typed := item.(type) {
		case cloudsync.CloudSyncClient:
			client = typed
		case *routerCloudSyncKeyClient:
			keyClient = typed
		}
	}
	deviceProvider := routerCloudSyncDeviceProvider{}
	keyManager := cloudsync.NewKeyManager(cloudsync.KeyManagerConfig{
		Keyring:        &routerCloudSyncKeyring{},
		KeyClient:      keyClient,
		DeviceProvider: deviceProvider,
	})
	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client:         client,
		Crypto:         routerCloudSyncCrypto{},
		DeviceProvider: deviceProvider,
		Applier:        &routerCloudSyncApplier{},
		OplogStore:     &routerCloudSyncOplogStore{},
	})
	cloudsync.SetService(&cloudsync.Service{Manager: manager, Client: nil, KeyManager: keyManager, DeviceProvider: deviceProvider})
}

func postSyncBootstrapStatus() *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/sync/bootstrap/status", nil)
	response := httptest.NewRecorder()
	routers["/sync/bootstrap/status"](response, request)
	return response
}

type routerCloudSyncClient struct {
	snapshotResponse *cloudsync.CloudSyncPullResponse
	snapshotRequests []cloudsync.CloudSyncPullRequest
}

func (c *routerCloudSyncClient) Push(ctx context.Context, req cloudsync.CloudSyncPushRequest) (*cloudsync.CloudSyncPushResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncPushResponse{}, nil
}

func (c *routerCloudSyncClient) Pull(ctx context.Context, req cloudsync.CloudSyncPullRequest) (*cloudsync.CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncPullResponse{}, nil
}

func (c *routerCloudSyncClient) Snapshot(ctx context.Context, req cloudsync.CloudSyncPullRequest) (*cloudsync.CloudSyncPullResponse, error) {
	_ = ctx
	c.snapshotRequests = append(c.snapshotRequests, req)
	if c.snapshotResponse != nil {
		return c.snapshotResponse, nil
	}
	return &cloudsync.CloudSyncPullResponse{}, nil
}

type routerCloudSyncKeyClient struct {
	status    cloudsync.CloudSyncKeyStatus
	initCalls int
}

func (c *routerCloudSyncKeyClient) Status(ctx context.Context) (cloudsync.CloudSyncKeyStatus, error) {
	_ = ctx
	return c.status, nil
}

func (c *routerCloudSyncKeyClient) InitKey(ctx context.Context, req cloudsync.CloudSyncKeyInitRequest) (*cloudsync.CloudSyncKeyInitResponse, error) {
	_ = ctx
	_ = req
	c.initCalls++
	return &cloudsync.CloudSyncKeyInitResponse{KeyVersion: 1}, nil
}

func (c *routerCloudSyncKeyClient) FetchKey(ctx context.Context, req cloudsync.CloudSyncKeyFetchRequest) (*cloudsync.CloudSyncKeyFetchResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncKeyFetchResponse{}, nil
}

func (c *routerCloudSyncKeyClient) PrepareKeyReset(ctx context.Context) (*cloudsync.CloudSyncKeyResetPrepareResponse, error) {
	_ = ctx
	return &cloudsync.CloudSyncKeyResetPrepareResponse{}, nil
}

func (c *routerCloudSyncKeyClient) ResetKey(ctx context.Context, req cloudsync.CloudSyncKeyResetRequest) (*cloudsync.CloudSyncKeyResetResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncKeyResetResponse{}, nil
}

type routerCloudSyncDeviceProvider struct{}

func (routerCloudSyncDeviceProvider) DeviceID(ctx context.Context) (string, error) {
	_ = ctx
	return "device-a", nil
}

type routerCloudSyncCrypto struct{}

func (routerCloudSyncCrypto) Encrypt(ctx context.Context, plaintext string, aad string) (*cloudsync.CloudSyncEncryptedValue, error) {
	_ = ctx
	_ = aad
	return &cloudsync.CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: plaintext}, nil
}

func (routerCloudSyncCrypto) Decrypt(ctx context.Context, value cloudsync.CloudSyncEncryptedValue, aad string) (string, error) {
	_ = ctx
	_ = aad
	return value.Ciphertext, nil
}

type routerCloudSyncApplier struct{}

func (a *routerCloudSyncApplier) ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error {
	_ = ctx
	_ = key
	_ = op
	_ = rawValue
	return nil
}

func (a *routerCloudSyncApplier) ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error {
	_ = ctx
	_ = pluginID
	_ = key
	_ = op
	_ = rawValue
	return nil
}

func (a *routerCloudSyncApplier) ApplyInstalledPlugin(ctx context.Context, pluginID string, op string, rawValue string) error {
	_ = ctx
	_ = pluginID
	_ = op
	_ = rawValue
	return nil
}

func (a *routerCloudSyncApplier) ApplyInstalledTheme(ctx context.Context, themeID string, op string, rawValue string) error {
	_ = ctx
	_ = themeID
	_ = op
	_ = rawValue
	return nil
}

type routerCloudSyncOplogStore struct{}

func (s *routerCloudSyncOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	_ = ctx
	_ = limit
	return nil, nil
}

func (s *routerCloudSyncOplogStore) MarkSynced(ctx context.Context, ids []uint) error {
	_ = ctx
	_ = ids
	return nil
}

type routerCloudSyncKeyring struct {
	values map[string]string
}

func (k *routerCloudSyncKeyring) Get(ctx context.Context, key string) (string, error) {
	_ = ctx
	if k.values == nil {
		return "", cloudsync.ErrKeyNotFound
	}
	value, ok := k.values[key]
	if !ok {
		return "", cloudsync.ErrKeyNotFound
	}
	return value, nil
}

func (k *routerCloudSyncKeyring) Set(ctx context.Context, key string, value string) error {
	_ = ctx
	if k.values == nil {
		k.values = map[string]string{}
	}
	k.values[key] = value
	return nil
}

func (k *routerCloudSyncKeyring) Delete(ctx context.Context, key string) error {
	_ = ctx
	delete(k.values, key)
	return nil
}
