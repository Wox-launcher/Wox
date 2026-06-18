package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
	if routers["/sync/devices/join"] == nil {
		t.Fatal("sync device join route is not registered")
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
	initSyncBootstrapRouterTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, client, keyClient)

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
	initSyncBootstrapRouterTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, &routerCloudSyncClient{}, keyClient)

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

func TestHandleSyncDevicesListUpdatesCurrentDeviceBeforeListing(t *testing.T) {
	client := &routerCloudSyncClient{}
	initSyncBootstrapRouterTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, client)

	request := httptest.NewRequest(http.MethodPost, "/sync/devices/list", nil)
	response := httptest.NewRecorder()
	routers["/sync/devices/list"](response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d, body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if len(client.deviceUpdateRequests) != 1 {
		t.Fatalf("device update requests = %#v, want one request", client.deviceUpdateRequests)
	}
	updateReq := client.deviceUpdateRequests[0]
	if updateReq.DeviceID != "device-a" {
		t.Fatalf("device update id = %q, want device-a", updateReq.DeviceID)
	}
	if updateReq.DeviceName == "" {
		t.Fatal("device update name is empty")
	}
	if updateReq.Platform != util.GetCurrentPlatform() {
		t.Fatalf("device update platform = %q, want %q", updateReq.Platform, util.GetCurrentPlatform())
	}
	if len(client.deviceListRequests) != 1 || client.deviceListRequests[0].DeviceID != "device-a" {
		t.Fatalf("device list requests = %#v, want one request for current device", client.deviceListRequests)
	}
}

func TestHandleSyncDeviceJoinUsesCurrentDeviceAndStartsManager(t *testing.T) {
	client := &routerCloudSyncClient{}
	initSyncBootstrapRouterTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true, SyncPlan: "pro", SyncEnabled: true}, client)

	request := httptest.NewRequest(http.MethodPost, "/sync/devices/join", nil)
	response := httptest.NewRecorder()
	routers["/sync/devices/join"](response, request)
	if service := cloudsync.GetService(); service != nil && service.Manager != nil {
		service.Manager.Stop(context.Background())
	}

	if response.Code != http.StatusOK {
		t.Fatalf("http status = %d, want %d, body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if len(client.deviceJoinRequests) != 1 {
		t.Fatalf("device join requests = %#v, want one request", client.deviceJoinRequests)
	}
	joinReq := client.deviceJoinRequests[0]
	if joinReq.DeviceID != "device-a" {
		t.Fatalf("device join id = %q, want device-a", joinReq.DeviceID)
	}
	if joinReq.DeviceName == "" {
		t.Fatal("device join name is empty")
	}
	if joinReq.Platform != util.GetCurrentPlatform() {
		t.Fatalf("device join platform = %q, want %q", joinReq.Platform, util.GetCurrentPlatform())
	}
	if len(client.deviceUpdateRequests) != 1 {
		t.Fatalf("device update requests after join = %#v, want manager restart metadata update", client.deviceUpdateRequests)
	}
}

func initSyncBootstrapRouterTest(t *testing.T, accountState database.AccountState, clientAndKey ...any) {
	t.Helper()
	woxDataDir, err := os.MkdirTemp("", "wox-sync-router-test-*")
	if err != nil {
		t.Fatalf("create wox data directory: %v", err)
	}
	userDataDir, err := os.MkdirTemp("", "wox-sync-router-user-test-*")
	if err != nil {
		t.Fatalf("create user data directory: %v", err)
	}
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(woxDataDir, "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(userDataDir, "user"))
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
	var deviceClient cloudsync.CloudSyncDeviceClient
	keyClient := &routerCloudSyncKeyClient{}
	for _, item := range clientAndKey {
		switch typed := item.(type) {
		case cloudsync.CloudSyncClient:
			client = typed
			if typedDeviceClient, ok := item.(cloudsync.CloudSyncDeviceClient); ok {
				deviceClient = typedDeviceClient
			}
		case *routerCloudSyncKeyClient:
			keyClient = typed
		}
	}
	if deviceClient == nil {
		deviceClient = client.(cloudsync.CloudSyncDeviceClient)
	}
	deviceProvider := routerCloudSyncDeviceProvider{}
	keyring := &routerCloudSyncKeyring{values: map[string]string{"dek": `{"dek":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","version":1}`}}
	keyManager := cloudsync.NewKeyManager(cloudsync.KeyManagerConfig{
		Keyring:        keyring,
		KeyClient:      keyClient,
		DeviceProvider: deviceProvider,
	})
	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client:         client,
		Crypto:         routerCloudSyncCrypto{},
		DeviceProvider: deviceProvider,
		Applier:        &routerCloudSyncApplier{},
		OplogStore:     &routerCloudSyncOplogStore{},
		Snapshotter:    routerCloudSyncSnapshotter{},
	})
	cloudsync.SetService(&cloudsync.Service{Manager: manager, DeviceClient: deviceClient, KeyManager: keyManager, DeviceProvider: deviceProvider})
}

func postSyncBootstrapStatus() *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/sync/bootstrap/status", nil)
	response := httptest.NewRecorder()
	routers["/sync/bootstrap/status"](response, request)
	return response
}

type routerCloudSyncClient struct {
	snapshotResponse     *cloudsync.CloudSyncPullResponse
	snapshotRequests     []cloudsync.CloudSyncPullRequest
	deviceUpdateRequests []cloudsync.CloudSyncDeviceUpdateRequest
	deviceListRequests   []cloudsync.CloudSyncDeviceListRequest
	deviceJoinRequests   []cloudsync.CloudSyncDeviceJoinRequest
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

func (c *routerCloudSyncClient) ListRecordKeys(ctx context.Context, req cloudsync.CloudSyncRecordKeyListRequest) (*cloudsync.CloudSyncRecordKeyListResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncRecordKeyListResponse{}, nil
}

func (c *routerCloudSyncClient) UpdateDevice(ctx context.Context, req cloudsync.CloudSyncDeviceUpdateRequest) (*cloudsync.CloudSyncDeviceUpdateResponse, error) {
	_ = ctx
	c.deviceUpdateRequests = append(c.deviceUpdateRequests, req)
	return &cloudsync.CloudSyncDeviceUpdateResponse{DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform}, nil
}

func (c *routerCloudSyncClient) ListDevices(ctx context.Context, req cloudsync.CloudSyncDeviceListRequest) (*cloudsync.CloudSyncDeviceListResponse, error) {
	_ = ctx
	c.deviceListRequests = append(c.deviceListRequests, req)
	return &cloudsync.CloudSyncDeviceListResponse{}, nil
}

func (c *routerCloudSyncClient) RevokeDevice(ctx context.Context, req cloudsync.CloudSyncDeviceRevokeRequest) (*cloudsync.CloudSyncDeviceRevokeResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncDeviceRevokeResponse{OK: true}, nil
}

func (c *routerCloudSyncClient) JoinDevice(ctx context.Context, req cloudsync.CloudSyncDeviceJoinRequest) (*cloudsync.CloudSyncDeviceJoinResponse, error) {
	_ = ctx
	c.deviceJoinRequests = append(c.deviceJoinRequests, req)
	return &cloudsync.CloudSyncDeviceJoinResponse{DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform}, nil
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

func (s *routerCloudSyncOplogStore) MarkPushFailed(ctx context.Context, failures []cloudsync.CloudSyncOplogPushFailure) error {
	_ = ctx
	_ = failures
	return nil
}

type routerCloudSyncSnapshotter struct{}

func (routerCloudSyncSnapshotter) EnqueueLocalSnapshot(ctx context.Context) error {
	_ = ctx
	return nil
}

func (routerCloudSyncSnapshotter) EnqueueMissingLocalSnapshot(ctx context.Context, remoteKeys []cloudsync.CloudSyncRecordKey) error {
	_ = ctx
	_ = remoteKeys
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
