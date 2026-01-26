package cloudsync

import (
	"context"
	"wox/database"
)

const (
	EntityWoxSetting    = "wox_setting"
	EntityPluginSetting = "plugin_setting"
	OpUpsert            = "upsert"
	OpDelete            = "delete"
)

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
	Changes  []CloudSyncChange `json:"changes"`
}

type CloudSyncPushResponse struct {
	ServerTs   int64                    `json:"server_ts"`
	Applied    []CloudSyncAppliedChange `json:"applied"`
	NextCursor string                   `json:"next_cursor"`
}

type CloudSyncPullRequest struct {
	DeviceID string `json:"device_id"`
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

type CloudSyncClient interface {
	Push(ctx context.Context, req CloudSyncPushRequest) (*CloudSyncPushResponse, error)
	Pull(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error)
	Snapshot(ctx context.Context, req CloudSyncPullRequest) (*CloudSyncPullResponse, error)
}

type CloudSyncKeyClient interface {
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

type CloudSyncChangeNotifier interface {
	Changes() <-chan struct{}
}

type CloudSyncPluginExclusionProvider interface {
	DisabledPluginIDs(ctx context.Context) []string
}
