package ui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/account"
	"wox/cloudsync"
	"wox/database"
	"wox/util"
)

func TestCloudBootstrapStatusRequiresLoggedInEligibleAccount(t *testing.T) {
	initSyncBootstrapServiceTest(t, database.AccountState{})

	_, err := NewCoreServices().CloudBootstrapStatus(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "account is not logged in") {
		t.Fatalf("error = %v, want login error", err)
	}
}

func TestCloudBootstrapStatusReportsRemoteDataAndKey(t *testing.T) {
	client := &testCloudSyncClient{
		snapshotResponse: &cloudsync.CloudSyncPullResponse{
			Records: []cloudsync.CloudSyncRecord{{EntityType: cloudsync.EntityWoxSetting, Key: "ThemeId", Op: cloudsync.OpUpsert}},
		},
	}
	keyClient := &testCloudSyncKeyClient{status: cloudsync.CloudSyncKeyStatus{Available: true, Version: 1}}
	initSyncBootstrapServiceTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, client, keyClient)

	status, err := NewCoreServices().CloudBootstrapStatus(context.Background(), "")
	if err != nil {
		t.Fatalf("CloudBootstrapStatus: %v", err)
	}
	if !status.HasRemoteData || !status.HasRemoteKey {
		t.Fatalf("status = %#v, want remote data and key", status)
	}
	if len(client.snapshotRequests) != 1 || client.snapshotRequests[0].Limit != 1 {
		t.Fatalf("snapshot requests = %#v, want one limit=1 request", client.snapshotRequests)
	}
}

func TestStartCloudBootstrapInitializesKeyAndEnablesSync(t *testing.T) {
	keyClient := &testCloudSyncKeyClient{status: cloudsync.CloudSyncKeyStatus{Available: false}}
	initSyncBootstrapServiceTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, &testCloudSyncClient{}, keyClient)

	if err := NewCoreServices().StartCloudBootstrap(context.Background(), "", "test recovery code"); err != nil {
		t.Fatalf("StartCloudBootstrap: %v", err)
	}
	if keyClient.initCalls != 1 {
		t.Fatalf("init key calls = %d, want 1", keyClient.initCalls)
	}
	status := account.GetService().Status(context.Background())
	if !status.SyncEnabled {
		t.Fatal("sync enabled = false, want true")
	}
}

func TestCloudDevicesUpdatesCurrentDeviceBeforeListing(t *testing.T) {
	client := &testCloudSyncClient{}
	initSyncBootstrapServiceTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true}, client)

	if _, err := NewCoreServices().CloudDevices(context.Background(), ""); err != nil {
		t.Fatalf("CloudDevices: %v", err)
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

func TestJoinCloudDeviceUsesCurrentDeviceAndStartsManager(t *testing.T) {
	client := &testCloudSyncClient{deviceUpdated: make(chan struct{}, 1)}
	initSyncBootstrapServiceTest(t, database.AccountState{LoggedIn: true, Email: "u@example.com", SyncEligible: true, SyncPlan: "pro", SyncEnabled: true}, client)
	cloudsync.MarkCloudSyncBootstrapComplete(context.Background())

	err := NewCoreServices().JoinCloudDevice(context.Background(), "")
	select {
	case <-client.deviceUpdated:
	case <-time.After(time.Second):
	}
	if service := cloudsync.GetService(); service != nil && service.Manager != nil {
		service.Manager.Stop(context.Background())
	}
	if err != nil {
		t.Fatalf("JoinCloudDevice: %v", err)
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

func initSyncBootstrapServiceTest(t *testing.T, accountState database.AccountState, clientAndKey ...any) {
	t.Helper()
	woxDataDir, err := os.MkdirTemp("", "wox-sync-service-test-*")
	if err != nil {
		t.Fatalf("create wox data directory: %v", err)
	}
	userDataDir, err := os.MkdirTemp("", "wox-sync-service-user-test-*")
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

	var client cloudsync.CloudSyncClient = &testCloudSyncClient{}
	var deviceClient cloudsync.CloudSyncDeviceClient
	keyClient := &testCloudSyncKeyClient{}
	for _, item := range clientAndKey {
		switch typed := item.(type) {
		case cloudsync.CloudSyncClient:
			client = typed
			if typedDeviceClient, ok := item.(cloudsync.CloudSyncDeviceClient); ok {
				deviceClient = typedDeviceClient
			}
		case *testCloudSyncKeyClient:
			keyClient = typed
		}
	}
	if deviceClient == nil {
		deviceClient = client.(cloudsync.CloudSyncDeviceClient)
	}
	deviceProvider := testCloudSyncDeviceProvider{}
	keyring := &testCloudSyncKeyring{values: map[string]string{"dek": `{"dek":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","version":1}`}}
	keyManager := cloudsync.NewKeyManager(cloudsync.KeyManagerConfig{
		Keyring:        keyring,
		KeyClient:      keyClient,
		DeviceProvider: deviceProvider,
	})
	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: deviceProvider,
		Applier:        &testCloudSyncApplier{},
		OplogStore:     &testCloudSyncOplogStore{},
		Snapshotter:    testCloudSyncSnapshotter{},
	})
	cloudsync.SetService(&cloudsync.Service{Manager: manager, DeviceClient: deviceClient, KeyManager: keyManager, DeviceProvider: deviceProvider})
}

type testCloudSyncClient struct {
	snapshotResponse     *cloudsync.CloudSyncPullResponse
	snapshotRequests     []cloudsync.CloudSyncPullRequest
	deviceUpdateRequests []cloudsync.CloudSyncDeviceUpdateRequest
	deviceUpdated        chan struct{}
	deviceListRequests   []cloudsync.CloudSyncDeviceListRequest
	deviceJoinRequests   []cloudsync.CloudSyncDeviceJoinRequest
}

func (c *testCloudSyncClient) Push(ctx context.Context, req cloudsync.CloudSyncPushRequest) (*cloudsync.CloudSyncPushResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncPushResponse{}, nil
}

func (c *testCloudSyncClient) Pull(ctx context.Context, req cloudsync.CloudSyncPullRequest) (*cloudsync.CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncPullResponse{}, nil
}

func (c *testCloudSyncClient) Snapshot(ctx context.Context, req cloudsync.CloudSyncPullRequest) (*cloudsync.CloudSyncPullResponse, error) {
	_ = ctx
	c.snapshotRequests = append(c.snapshotRequests, req)
	if c.snapshotResponse != nil {
		return c.snapshotResponse, nil
	}
	return &cloudsync.CloudSyncPullResponse{}, nil
}

func (c *testCloudSyncClient) ListRecordKeys(ctx context.Context, req cloudsync.CloudSyncRecordKeyListRequest) (*cloudsync.CloudSyncRecordKeyListResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncRecordKeyListResponse{}, nil
}

func (c *testCloudSyncClient) UpdateDevice(ctx context.Context, req cloudsync.CloudSyncDeviceUpdateRequest) (*cloudsync.CloudSyncDeviceUpdateResponse, error) {
	_ = ctx
	c.deviceUpdateRequests = append(c.deviceUpdateRequests, req)
	if c.deviceUpdated != nil {
		c.deviceUpdated <- struct{}{}
	}
	return &cloudsync.CloudSyncDeviceUpdateResponse{DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform}, nil
}

func (c *testCloudSyncClient) ListDevices(ctx context.Context, req cloudsync.CloudSyncDeviceListRequest) (*cloudsync.CloudSyncDeviceListResponse, error) {
	_ = ctx
	c.deviceListRequests = append(c.deviceListRequests, req)
	return &cloudsync.CloudSyncDeviceListResponse{}, nil
}

func (c *testCloudSyncClient) RevokeDevice(ctx context.Context, req cloudsync.CloudSyncDeviceRevokeRequest) (*cloudsync.CloudSyncDeviceRevokeResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncDeviceRevokeResponse{OK: true}, nil
}

func (c *testCloudSyncClient) JoinDevice(ctx context.Context, req cloudsync.CloudSyncDeviceJoinRequest) (*cloudsync.CloudSyncDeviceJoinResponse, error) {
	_ = ctx
	c.deviceJoinRequests = append(c.deviceJoinRequests, req)
	return &cloudsync.CloudSyncDeviceJoinResponse{DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform}, nil
}

type testCloudSyncKeyClient struct {
	status    cloudsync.CloudSyncKeyStatus
	initCalls int
}

func (c *testCloudSyncKeyClient) Status(ctx context.Context) (cloudsync.CloudSyncKeyStatus, error) {
	_ = ctx
	return c.status, nil
}

func (c *testCloudSyncKeyClient) InitKey(ctx context.Context, req cloudsync.CloudSyncKeyInitRequest) (*cloudsync.CloudSyncKeyInitResponse, error) {
	_ = ctx
	_ = req
	c.initCalls++
	return &cloudsync.CloudSyncKeyInitResponse{KeyVersion: 1}, nil
}

func (c *testCloudSyncKeyClient) FetchKey(ctx context.Context, req cloudsync.CloudSyncKeyFetchRequest) (*cloudsync.CloudSyncKeyFetchResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncKeyFetchResponse{}, nil
}

func (c *testCloudSyncKeyClient) PrepareKeyReset(ctx context.Context) (*cloudsync.CloudSyncKeyResetPrepareResponse, error) {
	_ = ctx
	return &cloudsync.CloudSyncKeyResetPrepareResponse{}, nil
}

func (c *testCloudSyncKeyClient) ResetKey(ctx context.Context, req cloudsync.CloudSyncKeyResetRequest) (*cloudsync.CloudSyncKeyResetResponse, error) {
	_ = ctx
	_ = req
	return &cloudsync.CloudSyncKeyResetResponse{}, nil
}

type testCloudSyncDeviceProvider struct{}

func (testCloudSyncDeviceProvider) DeviceID(ctx context.Context) (string, error) {
	_ = ctx
	return "device-a", nil
}

type testCloudSyncCrypto struct{}

func (testCloudSyncCrypto) Encrypt(ctx context.Context, plaintext string, aad string) (*cloudsync.CloudSyncEncryptedValue, error) {
	_ = ctx
	_ = aad
	return &cloudsync.CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: plaintext}, nil
}

func (testCloudSyncCrypto) Decrypt(ctx context.Context, value cloudsync.CloudSyncEncryptedValue, aad string) (string, error) {
	_ = ctx
	_ = aad
	return value.Ciphertext, nil
}

type testCloudSyncApplier struct{}

func (a *testCloudSyncApplier) ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error {
	_ = ctx
	_ = key
	_ = op
	_ = rawValue
	return nil
}

func (a *testCloudSyncApplier) ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error {
	_ = ctx
	_ = pluginID
	_ = key
	_ = op
	_ = rawValue
	return nil
}

func (a *testCloudSyncApplier) ApplyInstalledPlugin(ctx context.Context, pluginID string, op string, rawValue string) error {
	_ = ctx
	_ = pluginID
	_ = op
	_ = rawValue
	return nil
}

func (a *testCloudSyncApplier) ApplyInstalledTheme(ctx context.Context, themeID string, op string, rawValue string) error {
	_ = ctx
	_ = themeID
	_ = op
	_ = rawValue
	return nil
}

type testCloudSyncOplogStore struct{}

func (s *testCloudSyncOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	_ = ctx
	_ = limit
	return nil, nil
}

func (s *testCloudSyncOplogStore) MarkSynced(ctx context.Context, ids []uint) error {
	_ = ctx
	_ = ids
	return nil
}

func (s *testCloudSyncOplogStore) MarkPushFailed(ctx context.Context, failures []cloudsync.CloudSyncOplogPushFailure) error {
	_ = ctx
	_ = failures
	return nil
}

type testCloudSyncSnapshotter struct{}

func (testCloudSyncSnapshotter) EnqueueLocalSnapshot(ctx context.Context) error {
	_ = ctx
	return nil
}

func (testCloudSyncSnapshotter) EnqueueMissingLocalSnapshot(ctx context.Context, remoteKeys []cloudsync.CloudSyncRecordKey) error {
	_ = ctx
	_ = remoteKeys
	return nil
}

type testCloudSyncKeyring struct {
	values map[string]string
}

func (k *testCloudSyncKeyring) Get(ctx context.Context, key string) (string, error) {
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

func (k *testCloudSyncKeyring) Set(ctx context.Context, key string, value string) error {
	_ = ctx
	if k.values == nil {
		k.values = map[string]string{}
	}
	k.values[key] = value
	return nil
}

func (k *testCloudSyncKeyring) Delete(ctx context.Context, key string) error {
	_ = ctx
	delete(k.values, key)
	return nil
}
