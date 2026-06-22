package cloudsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"wox/database"
	"wox/util"
)

func TestRestoreSnapshotAppliesRemoteRecordsAndMarksBootstrapped(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	client := &testCloudSyncClient{
		snapshotResponses: []*CloudSyncPullResponse{
			{
				Records: []CloudSyncRecord{
					{
						EntityType: EntityWoxSetting,
						Key:        "ThemeId",
						Op:         OpUpsert,
						Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-theme"},
					},
				},
				NextCursor: "1",
				HasMore:    true,
			},
			{
				Records: []CloudSyncRecord{
					{
						EntityType: EntityPluginSetting,
						PluginID:   "plugin-a",
						Key:        "enabled",
						Op:         OpUpsert,
						Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "true"},
					},
				},
				NextCursor: "2",
			},
		},
	}
	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(CloudSyncConfig{PullLimit: 1}, CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		Applier:        applier,
	})

	if err := manager.RestoreSnapshot(ctx); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	if len(client.snapshotRequests) != 2 {
		t.Fatalf("snapshot calls = %d, want 2", len(client.snapshotRequests))
	}
	if client.snapshotRequests[0].Cursor != "" || client.snapshotRequests[1].Cursor != "1" {
		t.Fatalf("snapshot cursors = %#v, want empty then 1", client.snapshotRequests)
	}
	if got := applier.wox["ThemeId"]; got != "remote-theme" {
		t.Fatalf("applied wox setting = %q, want remote-theme", got)
	}
	if got := applier.plugins["plugin-a:enabled"]; got != "true" {
		t.Fatalf("applied plugin setting = %q, want true", got)
	}
	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if !state.Bootstrapped {
		t.Fatal("state bootstrapped = false, want true")
	}
	if state.LastPullTs == 0 {
		t.Fatal("state last pull timestamp was not updated")
	}
	if state.Cursor != "" {
		t.Fatalf("state cursor = %q, want unchanged empty cursor", state.Cursor)
	}
}

func TestRestoreSnapshotDoesNotMarkBootstrappedWhenApplyFails(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	manager := NewCloudSyncManager(CloudSyncConfig{PullLimit: 1}, CloudSyncDependencies{
		Client: &testCloudSyncClient{
			snapshotResponses: []*CloudSyncPullResponse{
				{
					Records: []CloudSyncRecord{
						{EntityType: EntityWoxSetting, Key: "ThemeId", Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-theme"}},
					},
				},
			},
		},
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		Applier:        &testCloudSyncApplier{err: errors.New("apply failed")},
	})

	if err := manager.RestoreSnapshot(ctx); err == nil {
		t.Fatal("RestoreSnapshot succeeded, want apply error")
	}
	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Bootstrapped {
		t.Fatal("state bootstrapped = true, want false")
	}
}

func TestCloudSyncPushPendingDoesNotAdvancePullRevisionCursor(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	if err := SaveCloudSyncState(ctx, &database.CloudSyncState{ID: cloudSyncStateID, Cursor: "10"}); err != nil {
		t.Fatalf("save state: %v", err)
	}
	store := &testCloudSyncOplogStore{
		pending: []database.Oplog{
			{
				ID:         1,
				EntityType: EntityWoxSetting,
				Operation:  OpUpsert,
				Key:        "ThemeId",
				Value:      "dark",
				Timestamp:  123,
			},
		},
	}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client:         &testCloudSyncClient{},
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     store,
	})

	manager.PushPending(ctx, "test")

	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Cursor != "10" {
		t.Fatalf("state cursor = %q, want unchanged revision cursor 10", state.Cursor)
	}
	if state.LastPushTs == 0 {
		t.Fatal("state last push timestamp was not updated")
	}
	if len(store.synced) != 1 || store.synced[0] != 1 {
		t.Fatalf("synced oplogs = %#v, want [1]", store.synced)
	}
}

func TestCloudSyncRequestsIncludeCurrentPlatform(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	store := &testCloudSyncOplogStore{
		pending: []database.Oplog{
			{
				ID:         1,
				EntityType: EntityWoxSetting,
				Operation:  OpUpsert,
				Key:        "ThemeId",
				Value:      "dark",
				Timestamp:  123,
			},
		},
	}
	client := &testCloudSyncClient{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     store,
		Snapshotter:    &testCloudSyncSnapshotter{},
		Applier:        &testCloudSyncApplier{},
	})

	manager.PushPending(ctx, "test")
	manager.Pull(ctx, "test")
	if _, err := manager.HasRemoteSnapshotData(ctx); err != nil {
		t.Fatalf("HasRemoteSnapshotData failed: %v", err)
	}
	manager.PushMissingLocalSnapshot(ctx, "test")

	currentPlatform := util.GetCurrentPlatform()
	if len(client.pushRequests) == 0 || !allPushRequestsUsePlatform(client.pushRequests, currentPlatform) {
		t.Fatalf("push platform = %#v, want %s", client.pushRequests, currentPlatform)
	}
	if len(client.pullRequests) == 0 || !allPullRequestsUsePlatform(client.pullRequests, currentPlatform) {
		t.Fatalf("pull platform = %#v, want %s", client.pullRequests, currentPlatform)
	}
	if len(client.snapshotRequests) == 0 || !allPullRequestsUsePlatform(client.snapshotRequests, currentPlatform) {
		t.Fatalf("snapshot platform = %#v, want %s", client.snapshotRequests, currentPlatform)
	}
	if len(client.recordKeyRequests) == 0 || !allRecordKeyRequestsUsePlatform(client.recordKeyRequests, currentPlatform) {
		t.Fatalf("record-key platform = %#v, want %s", client.recordKeyRequests, currentPlatform)
	}
}

func TestCloudSyncStartUpdatesCurrentDeviceMetadata(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	client := &testCloudSyncClient{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     &testCloudSyncOplogStore{},
		Snapshotter:    &testCloudSyncSnapshotter{},
		Applier:        &testCloudSyncApplier{},
	})

	manager.Start(ctx)
	defer manager.Stop(ctx)

	if len(client.deviceUpdateRequests) != 1 {
		t.Fatalf("device update calls = %d, want 1", len(client.deviceUpdateRequests))
	}
	got := client.deviceUpdateRequests[0]
	if got.DeviceID != "device-a" {
		t.Fatalf("device update id = %q, want device-a", got.DeviceID)
	}
	if got.DeviceName != resolveDeviceName() {
		t.Fatalf("device update name = %q, want %q", got.DeviceName, resolveDeviceName())
	}
	if got.Platform != util.GetCurrentPlatform() {
		t.Fatalf("device update platform = %q, want %q", got.Platform, util.GetCurrentPlatform())
	}
}

func TestCloudSyncStartRunsSingleOrderedSyncLoop(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	client := &loopCloudSyncClient{}
	store := &loopCloudSyncOplogStore{}
	history := &loopCloudSyncHistoryStore{}
	manager := NewCloudSyncManager(CloudSyncConfig{
		SyncInterval: 100 * time.Millisecond,
		PullLimit:    1,
	}, CloudSyncDependencies{
		Client:         client,
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     store,
		Snapshotter:    &testCloudSyncSnapshotter{},
		Applier:        &testCloudSyncApplier{},
		HistoryStore:   history,
	})

	manager.Start(ctx)
	manager.Start(ctx)
	defer manager.Stop(ctx)

	records := history.waitForRecords(t, 4, time.Second)
	expected := []loopCloudSyncHistoryEntry{
		{operation: CloudSyncProgressOperationPull, reason: "startup"},
		{operation: CloudSyncProgressOperationPush, reason: "startup-missing-keys"},
		{operation: CloudSyncProgressOperationPull, reason: "periodic-pull"},
		{operation: CloudSyncProgressOperationPush, reason: "periodic-push"},
	}
	for i, want := range expected {
		if records[i] != want {
			t.Fatalf("history[%d] = %#v, want %#v; all records = %#v", i, records[i], want, records)
		}
	}

	manager.Stop(ctx)
	recordCountAfterStop := history.count()
	time.Sleep(120 * time.Millisecond)
	if got := history.count(); got != recordCountAfterStop {
		t.Fatalf("history count after Stop = %d, want %d", got, recordCountAfterStop)
	}
}

func TestCloudSyncManagerStopsWhenDeviceRevoked(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	manager := NewCloudSyncManager(CloudSyncConfig{SyncInterval: time.Hour}, CloudSyncDependencies{
		Client:         &revokedCloudSyncClient{},
		Crypto:         testCloudSyncCrypto{},
		DeviceProvider: testCloudSyncDeviceProvider{deviceID: "device-a"},
		OplogStore:     &testCloudSyncOplogStore{},
		Snapshotter:    &testCloudSyncSnapshotter{},
		Applier:        &testCloudSyncApplier{},
	})

	manager.Start(ctx)
	defer manager.Stop(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state, err := LoadCloudSyncState(ctx)
		if err != nil {
			t.Fatalf("load state: %v", err)
		}
		manager.mu.Lock()
		started := manager.started
		manager.mu.Unlock()
		if state.LastError == "revoked" && !started {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	state, _ := LoadCloudSyncState(ctx)
	manager.mu.Lock()
	started := manager.started
	manager.mu.Unlock()
	t.Fatalf("last error = %q, started = %v, want revoked and stopped", state.LastError, started)
}

func TestApplyRecordsSkipsSettingsForOtherPlatforms(t *testing.T) {
	ctx := context.Background()
	initCloudSyncTestDatabase(t)

	currentPlatform := util.GetCurrentPlatform()
	otherPlatform := testOtherPlatform()
	applier := &testCloudSyncApplier{}
	manager := NewCloudSyncManager(DefaultCloudSyncConfig(), CloudSyncDependencies{
		Crypto:  testCloudSyncCrypto{},
		Applier: applier,
	})

	records := []CloudSyncRecord{
		{EntityType: EntityWoxSetting, Key: "ThemeId", Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "theme"}},
		{EntityType: EntityWoxSetting, Key: "MainHotkey@" + currentPlatform, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "current-hotkey"}},
		{EntityType: EntityWoxSetting, Key: "MainHotkey@" + otherPlatform, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "other-hotkey"}},
		{EntityType: EntityPluginSetting, PluginID: "browser", Key: "defaultBrowser@" + currentPlatform, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "current-browser"}},
		{EntityType: EntityPluginSetting, PluginID: "browser", Key: "defaultBrowser@" + otherPlatform, Op: OpUpsert, Value: &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "other-browser"}},
	}

	if err := manager.applyRecords(ctx, records); err != nil {
		t.Fatalf("apply records: %v", err)
	}

	if got := applier.wox["ThemeId"]; got != "theme" {
		t.Fatalf("common wox setting = %q, want theme", got)
	}
	if got := applier.wox["MainHotkey@"+currentPlatform]; got != "current-hotkey" {
		t.Fatalf("current wox setting = %q, want current-hotkey", got)
	}
	if _, exists := applier.wox["MainHotkey@"+otherPlatform]; exists {
		t.Fatalf("other platform wox setting was applied: %#v", applier.wox)
	}
	if got := applier.plugins["browser:defaultBrowser@"+currentPlatform]; got != "current-browser" {
		t.Fatalf("current plugin setting = %q, want current-browser", got)
	}
	if _, exists := applier.plugins["browser:defaultBrowser@"+otherPlatform]; exists {
		t.Fatalf("other platform plugin setting was applied: %#v", applier.plugins)
	}
}

func initCloudSyncTestDatabase(t *testing.T) {
	t.Helper()
	woxDataDir, err := os.MkdirTemp("", "wox-cloudsync-test-*")
	if err != nil {
		t.Fatalf("create wox data directory: %v", err)
	}
	t.Setenv(util.TestWoxDataDirEnv, woxDataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(t.TempDir(), "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("init database: %v", err)
	}
	t.Cleanup(func() {
		db := database.GetDB()
		if db == nil {
			return
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

type testCloudSyncClient struct {
	snapshotResponses    []*CloudSyncPullResponse
	snapshotRequests     []CloudSyncPullRequest
	pushRequests         []CloudSyncPushRequest
	pullRequests         []CloudSyncPullRequest
	recordKeyRequests    []CloudSyncRecordKeyListRequest
	deviceUpdateRequests []CloudSyncDeviceUpdateRequest
}

func (c *testCloudSyncClient) Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error) {
	_ = ctx
	c.pushRequests = append(c.pushRequests, req)
	applied := make([]CloudSyncAppliedChange, 0, len(req.Changes))
	for _, change := range req.Changes {
		applied = append(applied, CloudSyncAppliedChange{ChangeID: change.ChangeID, Status: "ok"})
	}
	return &CloudSyncPushResponse{Applied: applied, NextCursor: "pushed"}, nil
}

func (c *testCloudSyncClient) Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	c.pullRequests = append(c.pullRequests, req)
	return &CloudSyncPullResponse{NextCursor: "pulled"}, nil
}

func (c *testCloudSyncClient) Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	c.snapshotRequests = append(c.snapshotRequests, req)
	if len(c.snapshotResponses) == 0 {
		return &CloudSyncPullResponse{}, nil
	}
	resp := c.snapshotResponses[0]
	c.snapshotResponses = c.snapshotResponses[1:]
	return resp, nil
}

func (c *testCloudSyncClient) ListRecordKeys(ctx context.Context, req CloudSyncRecordKeyListRequest) (*CloudSyncRecordKeyListResponse, error) {
	_ = ctx
	c.recordKeyRequests = append(c.recordKeyRequests, req)
	return &CloudSyncRecordKeyListResponse{}, nil
}

func (c *testCloudSyncClient) UpdateDevice(ctx context.Context, req CloudSyncDeviceUpdateRequest) (*CloudSyncDeviceUpdateResponse, error) {
	_ = ctx
	c.deviceUpdateRequests = append(c.deviceUpdateRequests, req)
	return &CloudSyncDeviceUpdateResponse{DeviceID: req.DeviceID, DeviceName: req.DeviceName, Platform: req.Platform}, nil
}

type testCloudSyncCrypto struct{}

func (testCloudSyncCrypto) Encrypt(ctx context.Context, plaintext string, aad string) (*CloudSyncEncryptedValue, error) {
	_ = ctx
	_ = aad
	return &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: plaintext}, nil
}

func (testCloudSyncCrypto) Decrypt(ctx context.Context, value CloudSyncEncryptedValue, aad string) (string, error) {
	_ = ctx
	_ = aad
	return value.Ciphertext, nil
}

type testCloudSyncDeviceProvider struct {
	deviceID string
}

func (p testCloudSyncDeviceProvider) DeviceID(ctx context.Context) (string, error) {
	_ = ctx
	return p.deviceID, nil
}

type testCloudSyncApplier struct {
	wox     map[string]string
	plugins map[string]string
	err     error
}

func (a *testCloudSyncApplier) ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error {
	_ = ctx
	_ = op
	if a.err != nil {
		return a.err
	}
	if a.wox == nil {
		a.wox = map[string]string{}
	}
	a.wox[key] = rawValue
	return nil
}

func (a *testCloudSyncApplier) ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error {
	_ = ctx
	_ = op
	if a.err != nil {
		return a.err
	}
	if a.plugins == nil {
		a.plugins = map[string]string{}
	}
	a.plugins[pluginID+":"+key] = rawValue
	return nil
}

func (a *testCloudSyncApplier) ApplyInstalledPlugin(ctx context.Context, pluginID string, op string, rawValue string) error {
	_ = ctx
	_ = pluginID
	_ = op
	_ = rawValue
	return a.err
}

func (a *testCloudSyncApplier) ApplyInstalledTheme(ctx context.Context, themeID string, op string, rawValue string) error {
	_ = ctx
	_ = themeID
	_ = op
	_ = rawValue
	return a.err
}

type testCloudSyncOplogStore struct {
	pending  []database.Oplog
	synced   []uint
	failures []CloudSyncOplogPushFailure
}

type testCloudSyncSnapshotter struct{}

func (s *testCloudSyncSnapshotter) EnqueueLocalSnapshot(ctx context.Context) error {
	_ = ctx
	return nil
}

func (s *testCloudSyncSnapshotter) EnqueueMissingLocalSnapshot(ctx context.Context, remoteKeys []CloudSyncRecordKey) error {
	_ = ctx
	_ = remoteKeys
	return nil
}

func testOtherPlatform() string {
	switch util.GetCurrentPlatform() {
	case util.PlatformWindows:
		return util.PlatformMacOS
	case util.PlatformMacOS:
		return util.PlatformWindows
	default:
		return util.PlatformMacOS
	}
}

func allPushRequestsUsePlatform(requests []CloudSyncPushRequest, platform string) bool {
	for _, request := range requests {
		if request.Platform != platform {
			return false
		}
	}
	return true
}

func allPullRequestsUsePlatform(requests []CloudSyncPullRequest, platform string) bool {
	for _, request := range requests {
		if request.Platform != platform {
			return false
		}
	}
	return true
}

func allRecordKeyRequestsUsePlatform(requests []CloudSyncRecordKeyListRequest, platform string) bool {
	for _, request := range requests {
		if request.Platform != platform {
			return false
		}
	}
	return true
}

func (s *testCloudSyncOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	_ = ctx
	if len(s.pending) > limit {
		return s.pending[:limit], nil
	}
	return s.pending, nil
}

func (s *testCloudSyncOplogStore) MarkSynced(ctx context.Context, ids []uint) error {
	_ = ctx
	s.synced = append(s.synced, ids...)
	return nil
}

func (s *testCloudSyncOplogStore) MarkPushFailed(ctx context.Context, failures []CloudSyncOplogPushFailure) error {
	_ = ctx
	s.failures = append(s.failures, failures...)
	return nil
}

type testCloudSyncKeyring struct {
	values map[string]string
}

type loopCloudSyncClient struct {
	mu sync.Mutex
}

func (c *loopCloudSyncClient) Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error) {
	_ = ctx
	c.mu.Lock()
	defer c.mu.Unlock()

	applied := make([]CloudSyncAppliedChange, 0, len(req.Changes))
	for _, change := range req.Changes {
		applied = append(applied, CloudSyncAppliedChange{ChangeID: change.ChangeID, Status: "ok"})
	}
	return &CloudSyncPushResponse{Applied: applied}, nil
}

func (c *loopCloudSyncClient) Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	c.mu.Lock()
	defer c.mu.Unlock()

	return &CloudSyncPullResponse{
		Records: []CloudSyncRecord{
			{
				EntityType: EntityWoxSetting,
				Key:        "ThemeId",
				Op:         OpUpsert,
				Value:      &CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: "remote-theme"},
			},
		},
		NextCursor: "pulled",
	}, nil
}

func (c *loopCloudSyncClient) Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	return &CloudSyncPullResponse{}, nil
}

func (c *loopCloudSyncClient) ListRecordKeys(ctx context.Context, req CloudSyncRecordKeyListRequest) (*CloudSyncRecordKeyListResponse, error) {
	_ = ctx
	_ = req
	return &CloudSyncRecordKeyListResponse{}, nil
}

func (c *loopCloudSyncClient) UpdateDevice(ctx context.Context, req CloudSyncDeviceUpdateRequest) (*CloudSyncDeviceUpdateResponse, error) {
	_ = ctx
	_ = req
	return &CloudSyncDeviceUpdateResponse{}, nil
}

type revokedCloudSyncClient struct{}

func (c *revokedCloudSyncClient) Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error) {
	_ = ctx
	_ = req
	return nil, &CloudSyncRequestError{Code: "device_revoked", Message: "revoked"}
}

func (c *revokedCloudSyncClient) Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	return nil, &CloudSyncRequestError{Code: "device_revoked", Message: "revoked"}
}

func (c *revokedCloudSyncClient) Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error) {
	_ = ctx
	_ = req
	return nil, &CloudSyncRequestError{Code: "device_revoked", Message: "revoked"}
}

func (c *revokedCloudSyncClient) ListRecordKeys(ctx context.Context, req CloudSyncRecordKeyListRequest) (*CloudSyncRecordKeyListResponse, error) {
	_ = ctx
	_ = req
	return nil, &CloudSyncRequestError{Code: "device_revoked", Message: "revoked"}
}

func (c *revokedCloudSyncClient) UpdateDevice(ctx context.Context, req CloudSyncDeviceUpdateRequest) (*CloudSyncDeviceUpdateResponse, error) {
	_ = ctx
	_ = req
	return &CloudSyncDeviceUpdateResponse{}, nil
}

type loopCloudSyncOplogStore struct {
	mu     sync.Mutex
	nextID uint
}

func (s *loopCloudSyncOplogStore) LoadPending(ctx context.Context, limit int) ([]database.Oplog, error) {
	_ = ctx
	_ = limit
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	return []database.Oplog{
		{
			ID:         s.nextID,
			EntityType: EntityWoxSetting,
			Operation:  OpUpsert,
			Key:        "ThemeId",
			Value:      "dark",
			Timestamp:  util.GetSystemTimestamp(),
		},
	}, nil
}

func (s *loopCloudSyncOplogStore) MarkSynced(ctx context.Context, ids []uint) error {
	_ = ctx
	_ = ids
	return nil
}

func (s *loopCloudSyncOplogStore) MarkPushFailed(ctx context.Context, failures []CloudSyncOplogPushFailure) error {
	_ = ctx
	_ = failures
	return nil
}

type loopCloudSyncHistoryEntry struct {
	operation string
	reason    string
}

type loopCloudSyncHistoryStore struct {
	mu      sync.Mutex
	records []loopCloudSyncHistoryEntry
}

func (s *loopCloudSyncHistoryStore) Record(ctx context.Context, record CloudSyncHistoryRecord) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, loopCloudSyncHistoryEntry{
		operation: record.Operation,
		reason:    record.Reason,
	})
	return nil
}

func (s *loopCloudSyncHistoryStore) ListRecent(ctx context.Context, limit int) ([]CloudSyncHistoryRecord, error) {
	_ = ctx
	_ = limit
	return nil, nil
}

func (s *loopCloudSyncHistoryStore) Get(ctx context.Context, id uint) (*CloudSyncHistoryRecord, error) {
	_ = ctx
	_ = id
	return nil, nil
}

func (s *loopCloudSyncHistoryStore) waitForRecords(t *testing.T, count int, timeout time.Duration) []loopCloudSyncHistoryEntry {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		if len(s.records) >= count {
			records := append([]loopCloudSyncHistoryEntry(nil), s.records...)
			s.mu.Unlock()
			return records
		}
		s.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	t.Fatalf("timed out waiting for %d history records; got %#v", count, s.records)
	return nil
}

func (s *loopCloudSyncHistoryStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (k *testCloudSyncKeyring) Get(ctx context.Context, key string) (string, error) {
	_ = ctx
	if k.values == nil {
		return "", ErrKeyNotFound
	}
	value, ok := k.values[key]
	if !ok {
		return "", ErrKeyNotFound
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
