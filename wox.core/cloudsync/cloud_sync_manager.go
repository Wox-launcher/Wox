package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
	"wox/database"
	"wox/util"

	"github.com/google/uuid"
)

type CloudSyncConfig struct {
	DebounceMs    int64
	MaxBatchCount int
	MaxBatchBytes int
	PullInterval  time.Duration
	PullLimit     int
}

func DefaultCloudSyncConfig() CloudSyncConfig {
	return CloudSyncConfig{
		DebounceMs:    2000,
		MaxBatchCount: 100,
		MaxBatchBytes: 1 * 1024 * 1024,
		PullInterval:  5 * time.Minute,
		PullLimit:     200,
	}
}

type CloudSyncDependencies struct {
	Client            CloudSyncClient
	Crypto            CloudSyncCrypto
	DeviceProvider    CloudSyncDeviceProvider
	Applier           CloudSyncApplier
	OplogStore        CloudSyncOplogStore
	Notifier          CloudSyncChangeNotifier
	ExclusionProvider CloudSyncPluginExclusionProvider
}

type CloudSyncManager struct {
	config         CloudSyncConfig
	client         CloudSyncClient
	crypto         CloudSyncCrypto
	deviceProvider CloudSyncDeviceProvider
	applier        CloudSyncApplier
	oplogStore     CloudSyncOplogStore
	notifier       CloudSyncChangeNotifier
	exclusions     CloudSyncPluginExclusionProvider

	mu        sync.Mutex
	pushMu    sync.Mutex
	pullMu    sync.Mutex
	randMu    sync.Mutex
	rand      *rand.Rand
	debouncer *util.Debouncer[struct{}]
	cancel    context.CancelFunc
	started   bool
}

func NewCloudSyncManager(config CloudSyncConfig, deps CloudSyncDependencies) *CloudSyncManager {
	normalized := normalizeCloudSyncConfig(config)
	return &CloudSyncManager{
		config:         normalized,
		client:         deps.Client,
		crypto:         deps.Crypto,
		deviceProvider: deps.DeviceProvider,
		applier:        deps.Applier,
		oplogStore:     deps.OplogStore,
		notifier:       deps.Notifier,
		exclusions:     deps.ExclusionProvider,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *CloudSyncManager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.debouncer = util.NewDebouncer[struct{}](m.config.DebounceMs, m.config.DebounceMs, func(_ []struct{}, reason string) {
		m.PushPending(runCtx, reason)
	})
	m.debouncer.Start(runCtx)
	m.mu.Unlock()

	if m.notifier != nil {
		util.Go(runCtx, "cloud sync oplog watcher", func() {
			for {
				select {
				case <-runCtx.Done():
					return
				case <-m.notifier.Changes():
					m.debouncer.Add(runCtx, []struct{}{{}})
				}
			}
		})
	}

	util.Go(runCtx, "cloud sync initial pull", func() {
		m.Pull(runCtx, "startup")
	})

	util.Go(runCtx, "cloud sync pull ticker", func() {
		ticker := time.NewTicker(m.config.PullInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				m.Pull(runCtx, "periodic")
			}
		}
	})
}

func (m *CloudSyncManager) Stop(ctx context.Context) {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return
	}
	m.started = false
	cancel := m.cancel
	m.cancel = nil
	debouncer := m.debouncer
	m.debouncer = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if debouncer != nil {
		debouncer.Done(ctx)
	}
}

func (m *CloudSyncManager) PushPending(ctx context.Context, reason string) {
	m.pushMu.Lock()
	defer m.pushMu.Unlock()

	if err := m.ensureConfigured(); err != nil {
		m.recordFailure(ctx, err)
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	for {
		if ctx.Err() != nil {
			return
		}

		pending, err := m.oplogStore.LoadPending(ctx, m.config.MaxBatchCount*4)
		if err != nil {
			m.recordFailure(ctx, fmt.Errorf("failed to load pending oplogs: %w", err))
			return
		}
		if len(pending) == 0 {
			return
		}

		eligible, dropped := m.filterOplogsByDisabledPlugins(ctx, pending)
		if len(dropped) > 0 {
			if err := m.oplogStore.MarkSynced(ctx, dropped); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to drop disabled plugin oplogs: %v", err))
			}
		}

		if len(eligible) == 0 {
			if len(dropped) > 0 {
				continue
			}
			return
		}

		changes, oplogIds, err := m.buildPushBatch(ctx, eligible)
		if err != nil {
			m.recordFailure(ctx, fmt.Errorf("failed to build push batch: %w", err))
			return
		}
		if len(changes) == 0 {
			return
		}

		deviceId, err := m.deviceProvider.DeviceID(ctx)
		if err != nil {
			m.recordFailure(ctx, fmt.Errorf("failed to get device id: %w", err))
			return
		}

		resp, err := m.client.Push(ctx, CloudSyncPushRequest{
			DeviceID: deviceId,
			Changes:  changes,
		})
		if err != nil {
			m.recordFailure(ctx, fmt.Errorf("cloud sync push failed: %w", err))
			return
		}

		if err := m.oplogStore.MarkSynced(ctx, oplogIds); err != nil {
			m.recordFailure(ctx, fmt.Errorf("failed to mark oplogs synced: %w", err))
			return
		}

		m.recordPushSuccess(ctx, resp)

		if len(eligible) <= len(oplogIds) {
			return
		}
	}
}

func (m *CloudSyncManager) Pull(ctx context.Context, reason string) {
	m.pullMu.Lock()
	defer m.pullMu.Unlock()

	if err := m.ensureConfigured(); err != nil {
		m.recordFailure(ctx, err)
		return
	}
	if m.applier == nil {
		m.recordFailure(ctx, fmt.Errorf("cloud sync applier not configured"))
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		m.recordFailure(ctx, fmt.Errorf("failed to load cloud sync state: %w", err))
		return
	}

	deviceId, err := m.deviceProvider.DeviceID(ctx)
	if err != nil {
		m.recordFailure(ctx, fmt.Errorf("failed to get device id: %w", err))
		return
	}

	cursor := state.Cursor
	for {
		if ctx.Err() != nil {
			return
		}

		resp, err := m.client.Pull(ctx, CloudSyncPullRequest{
			DeviceID: deviceId,
			Cursor:   cursor,
			Limit:    m.config.PullLimit,
		})
		if err != nil {
			m.recordFailure(ctx, fmt.Errorf("cloud sync pull failed: %w", err))
			return
		}

		if len(resp.Records) > 0 {
			if err := m.applyRecords(ctx, resp.Records); err != nil {
				m.recordFailure(ctx, fmt.Errorf("failed to apply remote records: %w", err))
				return
			}
		}

		cursor = resp.NextCursor
		if _, err := UpdateCloudSyncState(ctx, func(s *database.CloudSyncState) {
			s.Cursor = cursor
			s.LastPullTs = util.GetSystemTimestamp()
			s.LastError = ""
			s.BackoffUntil = 0
			s.RetryCount = 0
		}); err != nil {
			m.recordFailure(ctx, fmt.Errorf("failed to update cloud sync state: %w", err))
			return
		}

		if !resp.HasMore {
			return
		}
	}
}

func (m *CloudSyncManager) applyRecords(ctx context.Context, records []CloudSyncRecord) error {
	disabled := m.disabledPluginSet(ctx)
	for _, record := range records {
		if record.EntityType == EntityPluginSetting {
			if _, blocked := disabled[record.PluginID]; blocked {
				continue
			}
		}

		var rawValue string
		if record.Op == OpUpsert {
			if record.Value == nil {
				return fmt.Errorf("missing encrypted value for upsert")
			}
			aad := buildCloudSyncAAD(record.EntityType, record.PluginID, record.Key, record.Op)
			plaintext, err := m.crypto.Decrypt(ctx, *record.Value, aad)
			if err != nil {
				return err
			}
			rawValue = plaintext
		}

		switch record.EntityType {
		case EntityWoxSetting:
			if err := m.applier.ApplyWoxSetting(ctx, record.Key, record.Op, rawValue); err != nil {
				return err
			}
		case EntityPluginSetting:
			if err := m.applier.ApplyPluginSetting(ctx, record.PluginID, record.Key, record.Op, rawValue); err != nil {
				return err
			}
		default:
			util.GetLogger().Warn(ctx, fmt.Sprintf("unknown cloud sync entity type: %s", record.EntityType))
		}
	}

	return nil
}

func (m *CloudSyncManager) buildPushBatch(ctx context.Context, oplogs []database.Oplog) ([]CloudSyncChange, []uint, error) {
	var changes []CloudSyncChange
	var oplogIds []uint
	var totalBytes int

	for _, oplog := range oplogs {
		change, err := m.oplogToChange(ctx, oplog)
		if err != nil {
			return nil, nil, err
		}

		encoded, err := json.Marshal(change)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encode change: %w", err)
		}

		if len(changes) > 0 && totalBytes+len(encoded) > m.config.MaxBatchBytes {
			break
		}

		changes = append(changes, change)
		oplogIds = append(oplogIds, oplog.ID)
		totalBytes += len(encoded)

		if len(changes) >= m.config.MaxBatchCount {
			break
		}
	}

	return changes, oplogIds, nil
}

func (m *CloudSyncManager) oplogToChange(ctx context.Context, oplog database.Oplog) (CloudSyncChange, error) {
	pluginId := ""
	if oplog.EntityType == EntityPluginSetting {
		pluginId = oplog.EntityID
	}

	var encrypted *CloudSyncEncryptedValue
	if oplog.Operation == OpUpsert {
		aad := buildCloudSyncAAD(oplog.EntityType, pluginId, oplog.Key, oplog.Operation)
		value, err := m.crypto.Encrypt(ctx, oplog.Value, aad)
		if err != nil {
			return CloudSyncChange{}, err
		}
		encrypted = value
	}

	return CloudSyncChange{
		ChangeID:   uuid.NewString(),
		EntityType: oplog.EntityType,
		PluginID:   pluginId,
		Key:        oplog.Key,
		Op:         oplog.Operation,
		ClientTs:   oplog.Timestamp,
		Value:      encrypted,
	}, nil
}

func (m *CloudSyncManager) filterOplogsByDisabledPlugins(ctx context.Context, oplogs []database.Oplog) ([]database.Oplog, []uint) {
	disabled := m.disabledPluginSet(ctx)
	if len(disabled) == 0 {
		return oplogs, nil
	}

	eligible := make([]database.Oplog, 0, len(oplogs))
	var dropped []uint
	for _, oplog := range oplogs {
		if oplog.EntityType == EntityPluginSetting {
			if _, blocked := disabled[oplog.EntityID]; blocked {
				dropped = append(dropped, oplog.ID)
				continue
			}
		}
		eligible = append(eligible, oplog)
	}

	return eligible, dropped
}

func (m *CloudSyncManager) disabledPluginSet(ctx context.Context) map[string]struct{} {
	if m.exclusions == nil {
		return nil
	}

	disabledList := m.exclusions.DisabledPluginIDs(ctx)
	if len(disabledList) == 0 {
		return nil
	}

	disabled := make(map[string]struct{}, len(disabledList))
	for _, pluginId := range disabledList {
		if pluginId == "" {
			continue
		}
		disabled[pluginId] = struct{}{}
	}

	return disabled
}

func (m *CloudSyncManager) ensureConfigured() error {
	if m.client == nil {
		return fmt.Errorf("cloud sync client not configured")
	}
	if m.crypto == nil {
		return fmt.Errorf("cloud sync crypto not configured")
	}
	if m.deviceProvider == nil {
		return fmt.Errorf("cloud sync device provider not configured")
	}
	if m.oplogStore == nil {
		return fmt.Errorf("cloud sync oplog store not configured")
	}
	return nil
}

func (m *CloudSyncManager) isBackoffActive(ctx context.Context) bool {
	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to load cloud sync state: %v", err))
		return false
	}

	now := util.GetSystemTimestamp()
	return state.BackoffUntil > now
}

func (m *CloudSyncManager) recordFailure(ctx context.Context, err error) {
	if err == nil {
		return
	}

	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.LastError = err.Error()
		state.RetryCount++
		state.BackoffUntil = util.GetSystemTimestamp() + m.nextBackoffMs(state.RetryCount)
	})
}

func (m *CloudSyncManager) recordPushSuccess(ctx context.Context, resp *CloudSyncPushResponse) {
	_, _ = UpdateCloudSyncState(ctx, func(state *database.CloudSyncState) {
		state.LastError = ""
		state.RetryCount = 0
		state.BackoffUntil = 0
		state.LastPushTs = util.GetSystemTimestamp()
		if resp != nil {
			if resp.ServerTs > 0 {
				state.LastPushTs = resp.ServerTs
			}
			if resp.NextCursor != "" {
				state.Cursor = resp.NextCursor
			}
		}
	})
}

func (m *CloudSyncManager) nextBackoffMs(retryCount int) int64 {
	if retryCount < 1 {
		retryCount = 1
	}

	base := int64(1000)
	max := int64(120000)
	exp := int64(1) << minInt(retryCount-1, 7)
	delay := base * exp
	if delay > max {
		delay = max
	}

	jitterRange := delay / 4
	if jitterRange <= 0 {
		return delay
	}

	return delay + m.randInt63n(jitterRange+1)
}

func (m *CloudSyncManager) randInt63n(n int64) int64 {
	m.randMu.Lock()
	defer m.randMu.Unlock()
	if m.rand == nil {
		m.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return m.rand.Int63n(n)
}

func buildCloudSyncAAD(entityType string, pluginId string, key string, op string) string {
	return fmt.Sprintf("%s:%s:%s:%s", entityType, pluginId, key, op)
}

func normalizeCloudSyncConfig(config CloudSyncConfig) CloudSyncConfig {
	normalized := config
	defaults := DefaultCloudSyncConfig()

	if normalized.DebounceMs <= 0 {
		normalized.DebounceMs = defaults.DebounceMs
	}
	if normalized.MaxBatchCount <= 0 {
		normalized.MaxBatchCount = defaults.MaxBatchCount
	}
	if normalized.MaxBatchBytes <= 0 {
		normalized.MaxBatchBytes = defaults.MaxBatchBytes
	}
	if normalized.PullInterval <= 0 {
		normalized.PullInterval = defaults.PullInterval
	}
	if normalized.PullLimit <= 0 {
		normalized.PullLimit = defaults.PullLimit
	}

	return normalized
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
