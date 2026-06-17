package cloudsync

import (
	"context"
	"encoding/json"
	"wox/database"
)

const (
	EntityWoxSetting      = "wox_setting"
	EntityPluginSetting   = "plugin_setting"
	EntityInstalledPlugin = "installed_plugin"
	EntityInstalledTheme  = "installed_theme"
	OpUpsert              = "upsert"
	OpDelete              = "delete"

	InstallSyncSourceStore = "store"
	InstallSyncSourceUser  = "user"

	CloudSyncProgressOperationSnapshot = "snapshot"
	CloudSyncProgressOperationPush     = "push"
	CloudSyncProgressOperationPull     = "pull"
	CloudSyncProgressOperationRestore  = "restore"

	CloudSyncHistoryStatusSucceeded = "succeeded"
	CloudSyncHistoryStatusFailed    = "failed"
)

type CloudSyncProgress struct {
	Active     bool   `json:"active"`
	Operation  string `json:"operation,omitempty"`
	EntityType string `json:"entity_type,omitempty"`
	PluginID   string `json:"plugin_id,omitempty"`
	Key        string `json:"key,omitempty"`
	Current    int    `json:"current"`
	Total      int    `json:"total,omitempty"`
}

// CloudSyncHistoryRecord is a local-only summary of one push or pull attempt.
type CloudSyncHistoryRecord struct {
	ID           uint
	Operation    string
	Reason       string
	Status       string
	StartedAt    int64
	FinishedAt   int64
	DurationMs   int64
	ItemCount    int
	EntityCounts map[string]int
	Error        string
}

type CloudSyncEncryptedValue struct {
	KeyVersion int    `json:"key_version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type CloudSyncChange struct {
	ChangeID   string                   `json:"change_id"`
	EntityType string                   `json:"entity_type"`
	PluginID   string                   `json:"plugin_id,omitempty"`
	Key        string                   `json:"key"`
	Op         string                   `json:"op"`
	ClientTs   int64                    `json:"client_ts"`
	Value      *CloudSyncEncryptedValue `json:"value,omitempty"`
}

type CloudSyncAppliedChange struct {
	ChangeID string `json:"change_id"`
	Status   string `json:"status"`
	ServerTs int64  `json:"server_ts"`
}

type CloudSyncPushRequest struct {
	DeviceID string            `json:"device_id"`
	Platform string            `json:"platform"`
	Changes  []CloudSyncChange `json:"changes"`
}

type CloudSyncPushResponse struct {
	ServerTs   int64                    `json:"server_ts"`
	Applied    []CloudSyncAppliedChange `json:"applied"`
	NextCursor string                   `json:"next_cursor"`
}

type CloudSyncPullRequest struct {
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
	Cursor   string `json:"cursor"`
	Limit    int    `json:"limit"`
}

type CloudSyncRecord struct {
	EntityType string                   `json:"entity_type"`
	PluginID   string                   `json:"plugin_id,omitempty"`
	Key        string                   `json:"key"`
	Op         string                   `json:"op"`
	ServerTs   int64                    `json:"server_ts"`
	ClientTs   int64                    `json:"client_ts"`
	Value      *CloudSyncEncryptedValue `json:"value,omitempty"`
}

type CloudSyncPullResponse struct {
	Records    []CloudSyncRecord `json:"records"`
	NextCursor string            `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type CloudSyncRecordKey struct {
	EntityType string `json:"entity_type"`
	PluginID   string `json:"plugin_id"`
	Key        string `json:"key"`
	Op         string `json:"op"`
}

type CloudSyncRecordKeyListRequest struct {
	DeviceID string `json:"device_id"`
	Platform string `json:"platform"`
}

type CloudSyncRecordKeyListResponse struct {
	Keys []CloudSyncRecordKey `json:"keys"`
}

type CloudSyncClient interface {
	Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error)
	Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error)
	Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error)
	ListRecordKeys(ctx context.Context, req CloudSyncRecordKeyListRequest) (*CloudSyncRecordKeyListResponse, error)
}

type CloudSyncKeyClient interface {
	Status(ctx context.Context) (CloudSyncKeyStatus, error)
	InitKey(ctx context.Context, req CloudSyncKeyInitRequest) (*CloudSyncKeyInitResponse, error)
	FetchKey(ctx context.Context, req CloudSyncKeyFetchRequest) (*CloudSyncKeyFetchResponse, error)
	PrepareKeyReset(ctx context.Context) (*CloudSyncKeyResetPrepareResponse, error)
	ResetKey(ctx context.Context, req CloudSyncKeyResetRequest) (*CloudSyncKeyResetResponse, error)
}

type CloudSyncDeviceProvider interface {
	DeviceID(ctx context.Context) (string, error)
}

type CloudSyncCrypto interface {
	Encrypt(ctx context.Context, plaintext string, aad string) (*CloudSyncEncryptedValue, error)
	Decrypt(ctx context.Context, value CloudSyncEncryptedValue, aad string) (string, error)
}

type CloudSyncApplier interface {
	ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error
	ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error
	ApplyInstalledPlugin(ctx context.Context, pluginID string, op string, rawValue string) error
	ApplyInstalledTheme(ctx context.Context, themeID string, op string, rawValue string) error
}

// CloudSyncSettingReloader lets the sync manager refresh UI-side cached settings
// after remote records have been applied locally.
type CloudSyncSettingReloader interface {
	ReloadSetting(ctx context.Context)
	ReloadSettingPlugins(ctx context.Context)
	ReloadSettingThemes(ctx context.Context)
}

// CloudSyncCurrentThemeApplier applies the current ThemeId after remote setting changes.
type CloudSyncCurrentThemeApplier interface {
	ApplyCurrentTheme(ctx context.Context)
}

// InstalledPluginValue stores enough source data to reproduce a store plugin
// installation on another device.
type InstalledPluginValue struct {
	ID       string          `json:"id"`
	Version  string          `json:"version,omitempty"`
	Source   string          `json:"source,omitempty"`
	Manifest json.RawMessage `json:"manifest,omitempty"`
}

// InstalledThemeValue stores the full theme payload so user-edited themes can
// be restored without depending on the remote theme store.
type InstalledThemeValue struct {
	ID      string          `json:"id"`
	Version string          `json:"version,omitempty"`
	Source  string          `json:"source,omitempty"`
	Theme   json.RawMessage `json:"theme,omitempty"`
}

type CloudSyncKDF struct {
	Alg         string `json:"alg"`
	Version     int    `json:"version"`
	Salt        string `json:"salt"`
	Iter        int    `json:"iter"`
	MemKiB      int    `json:"mem_kib"`
	Parallelism int    `json:"parallelism"`
	HashLen     int    `json:"hash_len"`
}

type CloudSyncKeyInitRequest struct {
	DeviceID     string       `json:"device_id"`
	DeviceName   string       `json:"device_name,omitempty"`
	KDF          CloudSyncKDF `json:"kdf"`
	EncryptedDEK string       `json:"encrypted_dek"`
	KeyVersion   int          `json:"key_version"`
}

type CloudSyncKeyInitResponse struct {
	KeyVersion int   `json:"key_version"`
	CreatedAt  int64 `json:"created_at"`
}

type CloudSyncKeyFetchRequest struct {
	DeviceID string `json:"device_id"`
}

type CloudSyncKeyFetchResponse struct {
	KeyVersion   int          `json:"key_version"`
	KDF          CloudSyncKDF `json:"kdf"`
	EncryptedDEK string       `json:"encrypted_dek"`
}

type CloudSyncKeyResetPrepareResponse struct {
	ResetToken string `json:"reset_token"`
	ExpiresAt  int64  `json:"expires_at"`
}

type CloudSyncKeyResetRequest struct {
	ResetToken string `json:"reset_token"`
	Confirm    bool   `json:"confirm"`
}

type CloudSyncKeyResetResponse struct {
	ResetAt int64 `json:"reset_at"`
}

type CloudSyncOplogStore interface {
	LoadPending(ctx context.Context, limit int) ([]database.Oplog, error)
	MarkSynced(ctx context.Context, ids []uint) error
}

type CloudSyncPendingCounter interface {
	CountPending(ctx context.Context) (int, error)
}

type CloudSyncLocalSnapshotter interface {
	EnqueueLocalSnapshot(ctx context.Context) error
	EnqueueMissingLocalSnapshot(ctx context.Context, remoteKeys []CloudSyncRecordKey) error
}

type CloudSyncChangeNotifier interface {
	Changes() <-chan struct{}
}

type CloudSyncProgressNotifier interface {
	CloudSyncProgressChanged(ctx context.Context, progress CloudSyncProgress)
}

// CloudSyncHistoryStore persists local sync attempt summaries for diagnostic UI surfaces.
type CloudSyncHistoryStore interface {
	Record(ctx context.Context, record CloudSyncHistoryRecord) error
	ListRecent(ctx context.Context, limit int) ([]CloudSyncHistoryRecord, error)
}

type CloudSyncPluginExclusionProvider interface {
	DisabledPluginIDs(ctx context.Context) []string
}
