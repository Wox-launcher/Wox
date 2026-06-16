package cloudsync

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
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

func initCloudSyncTestDatabase(t *testing.T) {
	t.Helper()
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(t.TempDir(), "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(t.TempDir(), "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

type testCloudSyncClient struct {
	snapshotResponses []*CloudSyncPullResponse
	snapshotRequests  []CloudSyncPullRequest
	pushRequests      []CloudSyncPushRequest
	pullRequests      []CloudSyncPullRequest
}

func (c *testCloudSyncClient) Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error) {
	_ = ctx
	c.pushRequests = append(c.pushRequests, req)
	return &CloudSyncPushResponse{NextCursor: "pushed"}, nil
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

type testCloudSyncOplogStore struct {
	pending []database.Oplog
	synced  []uint
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

type testCloudSyncKeyring struct {
	values map[string]string
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
