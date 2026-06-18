package cloudsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"
	"wox/database"
	"wox/util"

	"github.com/google/uuid"
)

const cloudSyncMaxOplogPushFailures = 3

type CloudSyncConfig struct {
	MaxBatchCount int
	MaxBatchBytes int
	PullLimit     int
	SyncInterval  time.Duration
}

func DefaultCloudSyncConfig() CloudSyncConfig {
	return CloudSyncConfig{
		MaxBatchCount: 100,
		MaxBatchBytes: 1 * 1024 * 1024,
		PullLimit:     200,
		SyncInterval:  5 * time.Minute,
	}
}

type CloudSyncDependencies struct {
	Client            CloudSyncClient
	Crypto            CloudSyncCrypto
	DeviceProvider    CloudSyncDeviceProvider
	Applier           CloudSyncApplier
	OplogStore        CloudSyncOplogStore
	Snapshotter       CloudSyncLocalSnapshotter
	ProgressNotifier  CloudSyncProgressNotifier
	ExclusionProvider CloudSyncPluginExclusionProvider
	SettingReloader   CloudSyncSettingReloader
	HistoryStore      CloudSyncHistoryStore
}

type CloudSyncManager struct {
	config           CloudSyncConfig
	client           CloudSyncClient
	crypto           CloudSyncCrypto
	deviceProvider   CloudSyncDeviceProvider
	applier          CloudSyncApplier
	oplogStore       CloudSyncOplogStore
	snapshotter      CloudSyncLocalSnapshotter
	progressNotifier CloudSyncProgressNotifier
	exclusions       CloudSyncPluginExclusionProvider
	settingReloader  CloudSyncSettingReloader
	historyStore     CloudSyncHistoryStore

	mu         sync.Mutex
	pushMu     sync.Mutex
	pullMu     sync.Mutex
	progressMu sync.RWMutex
	progress   CloudSyncProgress
	randMu     sync.Mutex
	rand       *rand.Rand
	cancel     context.CancelFunc
	started    bool
}

func NewCloudSyncManager(config CloudSyncConfig, deps CloudSyncDependencies) *CloudSyncManager {
	normalized := normalizeCloudSyncConfig(config)
	return &CloudSyncManager{
		config:           normalized,
		client:           deps.Client,
		crypto:           deps.Crypto,
		deviceProvider:   deps.DeviceProvider,
		applier:          deps.Applier,
		oplogStore:       deps.OplogStore,
		snapshotter:      deps.Snapshotter,
		progressNotifier: deps.ProgressNotifier,
		exclusions:       deps.ExclusionProvider,
		rand:             rand.New(rand.NewSource(time.Now().UnixNano())),
		settingReloader:  deps.SettingReloader,
		historyStore:     deps.HistoryStore,
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
	m.mu.Unlock()

	util.Go(runCtx, "cloud sync loop", func() {
		m.runSyncLoop(runCtx)
	})
}

// runSyncLoop keeps scheduled pull and push work serialized so shared sync state
// is updated in one predictable order.
func (m *CloudSyncManager) runSyncLoop(ctx context.Context) {
	m.syncOnce(ctx, "startup", "startup-missing-keys", true)

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.syncOnce(ctx, "periodic-pull", "periodic-push", false)
		}
	}
}

// syncOnce performs one ordered sync pass and preserves the startup missing-key
// snapshot behavior separately from normal periodic pushes.
func (m *CloudSyncManager) syncOnce(ctx context.Context, pullReason string, pushReason string, pushMissingSnapshot bool) {
	if ctx.Err() != nil {
		return
	}

	m.Pull(ctx, pullReason)
	if ctx.Err() != nil {
		return
	}

	if pushMissingSnapshot {
		m.PushMissingLocalSnapshot(ctx, pushReason)
		return
	}
	m.PushPending(ctx, pushReason)
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
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Progress returns the in-flight sync operation progress without persisting transient UI state.
func (m *CloudSyncManager) Progress() CloudSyncProgress {
	m.progressMu.RLock()
	defer m.progressMu.RUnlock()
	return m.progress
}

func (m *CloudSyncManager) PushPending(ctx context.Context, reason string) {
	m.pushMu.Lock()
	defer m.pushMu.Unlock()

	m.pushPendingLocked(ctx, reason)
}

// PushLocalSnapshot queues the current local settings before reusing the normal pending-oplog push path.
func (m *CloudSyncManager) PushLocalSnapshot(ctx context.Context, reason string) {
	m.pushMu.Lock()
	defer m.pushMu.Unlock()

	if err := m.ensureConfigured(); err != nil {
		m.recordFailure(ctx, err)
		return
	}
	if m.snapshotter == nil {
		m.recordFailure(ctx, fmt.Errorf("cloud sync local snapshotter not configured"))
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationSnapshot})
	if err := m.snapshotter.EnqueueLocalSnapshot(ctx); err != nil {
		m.recordFailure(ctx, fmt.Errorf("failed to enqueue local snapshot: %w", err))
		m.clearProgress(CloudSyncProgressOperationSnapshot)
		return
	}

	m.clearProgress(CloudSyncProgressOperationSnapshot)
	m.pushPendingLocked(ctx, reason)
}

// PushMissingLocalSnapshot uploads persisted local records whose identities do not exist on the server yet.
func (m *CloudSyncManager) PushMissingLocalSnapshot(ctx context.Context, reason string) {
	m.pushMu.Lock()
	defer m.pushMu.Unlock()

	if err := m.ensureConfigured(); err != nil {
		m.recordFailure(ctx, err)
		return
	}
	if m.snapshotter == nil {
		m.recordFailure(ctx, fmt.Errorf("cloud sync local snapshotter not configured"))
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	deviceId, err := m.deviceProvider.DeviceID(ctx)
	if err != nil {
		m.recordFailure(ctx, fmt.Errorf("failed to get device id: %w", err))
		return
	}

	resp, err := m.client.ListRecordKeys(ctx, CloudSyncRecordKeyListRequest{DeviceID: deviceId, Platform: util.GetCurrentPlatform()})
	if err != nil {
		m.recordFailure(ctx, fmt.Errorf("cloud sync record key list failed: %w", err))
		return
	}
	var remoteKeys []CloudSyncRecordKey
	if resp != nil {
		remoteKeys = resp.Keys
	}

	m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationSnapshot})
	if err := m.snapshotter.EnqueueMissingLocalSnapshot(ctx, remoteKeys); err != nil {
		m.recordFailure(ctx, fmt.Errorf("failed to enqueue missing local snapshot: %w", err))
		m.clearProgress(CloudSyncProgressOperationSnapshot)
		return
	}

	m.clearProgress(CloudSyncProgressOperationSnapshot)
	m.pushPendingLocked(ctx, reason)
}

func (m *CloudSyncManager) pushPendingLocked(ctx context.Context, reason string) {
	startedAt := util.GetSystemTimestamp()
	historyItemCount := 0
	entityCounts := map[string]int{}
	historyKeys := []CloudSyncRecordKey{}
	historyDetails := []CloudSyncHistoryRecordDetail{}
	historyStatus := ""
	var historyErr error
	defer func() {
		m.recordOperationHistory(ctx, CloudSyncProgressOperationPush, reason, startedAt, historyItemCount, entityCounts, historyKeys, historyDetails, historyStatus, historyErr)
	}()
	fail := func(err error) {
		historyStatus = CloudSyncHistoryStatusFailed
		historyErr = err
		m.recordFailure(ctx, err)
	}

	if err := m.ensureConfigured(); err != nil {
		fail(err)
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	total := m.countPendingOplogs(ctx)
	processed := 0
	progressStarted := false
	defer func() {
		if progressStarted {
			m.clearProgress(CloudSyncProgressOperationPush)
		}
	}()

	for {
		if ctx.Err() != nil {
			historyStatus = "cancelled"
			return
		}

		pending, err := m.oplogStore.LoadPending(ctx, m.config.MaxBatchCount*4)
		if err != nil {
			fail(fmt.Errorf("failed to load pending oplogs: %w", err))
			return
		}
		if len(pending) == 0 {
			return
		}

		if !progressStarted {
			progressStarted = true
			m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationPush, Current: processed, Total: total})
		}
		m.setProgressFromOplog(CloudSyncProgressOperationPush, pending[0], processed, total)
		eligible, dropped := m.filterOplogsByDisabledPlugins(ctx, pending)
		if len(dropped) > 0 {
			if err := m.oplogStore.MarkSynced(ctx, dropped); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to drop disabled plugin oplogs: %v", err))
			}
			processed += len(dropped)
		}

		if len(eligible) == 0 {
			if len(dropped) > 0 {
				m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationPush, Current: processed, Total: total})
				continue
			}
			return
		}

		changes, oplogIds, err := m.buildPushBatch(ctx, eligible)
		if err != nil {
			fail(fmt.Errorf("failed to build push batch: %w", err))
			return
		}
		if len(changes) == 0 {
			return
		}

		m.setProgressFromOplog(CloudSyncProgressOperationPush, eligible[0], processed, total)
		deviceId, err := m.deviceProvider.DeviceID(ctx)
		if err != nil {
			fail(fmt.Errorf("failed to get device id: %w", err))
			return
		}

		resp, err := m.client.Push(ctx, CloudSyncPushRequest{
			DeviceID: deviceId,
			Platform: util.GetCurrentPlatform(),
			Changes:  changes,
		})
		if err != nil {
			fail(fmt.Errorf("cloud sync push failed: %w", err))
			return
		}

		syncedIds, rejectedFailures, err := m.resolvePushResults(resp, changes, oplogIds, eligible)
		if err != nil {
			fail(err)
			return
		}
		if len(syncedIds) > 0 {
			if err := m.oplogStore.MarkSynced(ctx, syncedIds); err != nil {
				fail(fmt.Errorf("failed to mark oplogs synced: %w", err))
				return
			}
			syncedChanges := cloudSyncChangesForIDs(changes, oplogIds, syncedIds)
			addCloudSyncChangeEntityCounts(entityCounts, syncedChanges)
			historyKeys = append(historyKeys, cloudSyncChangeRecordKeys(syncedChanges)...)
		}
		if len(rejectedFailures) > 0 {
			if err := m.oplogStore.MarkPushFailed(ctx, rejectedFailures); err != nil {
				fail(fmt.Errorf("failed to mark oplogs failed: %w", err))
				return
			}
			historyStatus = CloudSyncHistoryStatusFailed
			if len(syncedIds) > 0 {
				historyStatus = CloudSyncHistoryStatusPartialSucceeded
			}
			historyErr = errors.New(lastCloudSyncOplogPushFailureError(rejectedFailures))
		}
		if len(syncedIds) > 0 || len(rejectedFailures) > 0 {
			historyDetails = append(historyDetails, cloudSyncPushHistoryDetails(changes, oplogIds, syncedIds, rejectedFailures)...)
			historyItemCount = len(historyDetails)
		}

		processed += len(syncedIds) + countDiscardedOplogPushFailures(rejectedFailures)
		if processed > 0 {
			progressIndex := min(processed-1, len(eligible)-1)
			m.setProgressFromOplog(CloudSyncProgressOperationPush, eligible[progressIndex], processed, total)
		}
		m.recordPushSuccess(ctx, resp)

		if len(rejectedFailures) > 0 {
			return
		}
		if len(eligible) <= len(oplogIds) {
			return
		}
	}
}

func (m *CloudSyncManager) Pull(ctx context.Context, reason string) {
	m.pullMu.Lock()
	defer m.pullMu.Unlock()

	startedAt := util.GetSystemTimestamp()
	pulled := 0
	entityCounts := map[string]int{}
	historyKeys := []CloudSyncRecordKey{}
	historyStatus := ""
	var historyErr error
	defer func() {
		m.recordOperationHistory(ctx, CloudSyncProgressOperationPull, reason, startedAt, pulled, entityCounts, historyKeys, nil, historyStatus, historyErr)
	}()
	fail := func(err error) {
		historyStatus = CloudSyncHistoryStatusFailed
		historyErr = err
		m.recordFailure(ctx, err)
	}

	if err := m.ensureConfigured(); err != nil {
		fail(err)
		return
	}
	if m.applier == nil {
		fail(fmt.Errorf("cloud sync applier not configured"))
		return
	}

	if m.isBackoffActive(ctx) {
		return
	}

	defer m.clearProgress(CloudSyncProgressOperationPull)
	m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationPull, Current: pulled})

	state, err := LoadCloudSyncState(ctx)
	if err != nil {
		fail(fmt.Errorf("failed to load cloud sync state: %w", err))
		return
	}

	deviceId, err := m.deviceProvider.DeviceID(ctx)
	if err != nil {
		fail(fmt.Errorf("failed to get device id: %w", err))
		return
	}

	cursor := state.Cursor
	for {
		if ctx.Err() != nil {
			historyStatus = "cancelled"
			return
		}

		resp, err := m.client.Pull(ctx, CloudSyncPullRequest{
			DeviceID: deviceId,
			Platform: util.GetCurrentPlatform(),
			Cursor:   cursor,
			Limit:    m.config.PullLimit,
		})
		if err != nil {
			fail(fmt.Errorf("cloud sync pull failed: %w", err))
			return
		}

		if len(resp.Records) > 0 {
			m.setProgressFromRecord(CloudSyncProgressOperationPull, resp.Records[0], pulled, 0)
			pulled += len(resp.Records)
			addCloudSyncRecordEntityCounts(entityCounts, resp.Records)
			historyKeys = append(historyKeys, cloudSyncRecordKeys(resp.Records)...)
			if err := m.applyRecords(ctx, resp.Records); err != nil {
				fail(fmt.Errorf("failed to apply remote records: %w", err))
				return
			}
			m.setProgressFromRecord(CloudSyncProgressOperationPull, resp.Records[len(resp.Records)-1], pulled, 0)
		}

		cursor = resp.NextCursor
		if _, err := UpdateCloudSyncState(ctx, func(s *database.CloudSyncState) {
			s.Cursor = cursor
			s.LastPullTs = util.GetSystemTimestamp()
			s.LastError = ""
			s.BackoffUntil = 0
			s.RetryCount = 0
		}); err != nil {
			fail(fmt.Errorf("failed to update cloud sync state: %w", err))
			return
		}

		if !resp.HasMore {
			return
		}
	}
}

// HasRemoteSnapshotData checks for any server-side sync records without applying them locally.
func (m *CloudSyncManager) HasRemoteSnapshotData(ctx context.Context) (bool, error) {
	if m.client == nil {
		return false, fmt.Errorf("cloud sync client not configured")
	}
	if m.deviceProvider == nil {
		return false, fmt.Errorf("cloud sync device provider not configured")
	}
	deviceId, err := m.deviceProvider.DeviceID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get device id: %w", err)
	}
	resp, err := m.client.Snapshot(ctx, CloudSyncPullRequest{DeviceID: deviceId, Platform: util.GetCurrentPlatform(), Cursor: "", Limit: 1})
	if err != nil {
		return false, fmt.Errorf("cloud sync snapshot check failed: %w", err)
	}
	return len(resp.Records) > 0, nil
}

// RestoreSnapshot applies the full server snapshot locally without treating snapshot offsets as incremental pull cursors.
func (m *CloudSyncManager) RestoreSnapshot(ctx context.Context) error {
	if m.client == nil {
		return fmt.Errorf("cloud sync client not configured")
	}
	if m.crypto == nil {
		return fmt.Errorf("cloud sync crypto not configured")
	}
	if m.deviceProvider == nil {
		return fmt.Errorf("cloud sync device provider not configured")
	}
	if m.applier == nil {
		return fmt.Errorf("cloud sync applier not configured")
	}

	deviceId, err := m.deviceProvider.DeviceID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get device id: %w", err)
	}

	cursor := ""
	defer m.clearProgress(CloudSyncProgressOperationRestore)
	restored := 0
	m.setProgress(CloudSyncProgress{Operation: CloudSyncProgressOperationRestore, Current: restored})
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resp, err := m.client.Snapshot(ctx, CloudSyncPullRequest{
			DeviceID: deviceId,
			Platform: util.GetCurrentPlatform(),
			Cursor:   cursor,
			Limit:    m.config.PullLimit,
		})
		if err != nil {
			return fmt.Errorf("cloud sync snapshot failed: %w", err)
		}
		if len(resp.Records) > 0 {
			m.setProgressFromRecord(CloudSyncProgressOperationRestore, resp.Records[0], restored, 0)
			if err := m.applyRecords(ctx, resp.Records); err != nil {
				return fmt.Errorf("failed to apply remote snapshot: %w", err)
			}
			restored += len(resp.Records)
			m.setProgressFromRecord(CloudSyncProgressOperationRestore, resp.Records[len(resp.Records)-1], restored, 0)
		}
		cursor = resp.NextCursor
		if !resp.HasMore {
			break
		}
	}

	_, err = UpdateCloudSyncState(ctx, func(s *database.CloudSyncState) {
		s.LastPullTs = util.GetSystemTimestamp()
		s.LastError = ""
		s.BackoffUntil = 0
		s.RetryCount = 0
		s.Bootstrapped = true
	})
	if err != nil {
		return fmt.Errorf("failed to update cloud sync state: %w", err)
	}
	return nil
}

func (m *CloudSyncManager) applyRecords(ctx context.Context, records []CloudSyncRecord) error {
	disabled := m.disabledPluginSet(ctx)
	appliedWoxSetting := false
	appliedPluginSetting := false
	appliedInstalledPlugin := false
	appliedInstalledTheme := false
	themeSettingChanged := false
	for _, record := range records {
		if record.EntityType == EntityPluginSetting || record.EntityType == EntityInstalledPlugin {
			pluginID := record.PluginID
			if record.EntityType == EntityInstalledPlugin && pluginID == "" {
				pluginID = record.Key
			}
			if _, blocked := disabled[pluginID]; blocked {
				continue
			}
		}
		if !isSyncRecordForCurrentPlatform(record) {
			continue
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
			willChangeCurrentTheme := themeSettingWillChange(record, rawValue)
			if err := m.applier.ApplyWoxSetting(ctx, record.Key, record.Op, rawValue); err != nil {
				return err
			}
			appliedWoxSetting = true
			themeSettingChanged = themeSettingChanged || willChangeCurrentTheme
		case EntityPluginSetting:
			if err := m.applier.ApplyPluginSetting(ctx, record.PluginID, record.Key, record.Op, rawValue); err != nil {
				return err
			}
			appliedPluginSetting = true
		case EntityInstalledPlugin:
			if err := m.applier.ApplyInstalledPlugin(ctx, record.Key, record.Op, rawValue); err != nil {
				return err
			}
			appliedInstalledPlugin = true
		case EntityInstalledTheme:
			if err := m.applier.ApplyInstalledTheme(ctx, record.Key, record.Op, rawValue); err != nil {
				return err
			}
			appliedInstalledTheme = true
		default:
			util.GetLogger().Warn(ctx, fmt.Sprintf("unknown cloud sync entity type: %s", record.EntityType))
		}
	}

	m.reloadAppliedSettings(ctx, appliedWoxSetting, appliedPluginSetting, appliedInstalledPlugin, appliedInstalledTheme, themeSettingChanged)
	return nil
}

func isSyncRecordForCurrentPlatform(record CloudSyncRecord) bool {
	switch record.EntityType {
	case EntityWoxSetting, EntityPluginSetting:
		platform, ok := splitSyncPlatformKey(record.Key)
		return !ok || platform == util.GetCurrentPlatform()
	default:
		return true
	}
}

func splitSyncPlatformKey(key string) (string, bool) {
	index := strings.LastIndex(key, "@")
	if index <= 0 || index == len(key)-1 {
		return "", false
	}

	platform := strings.ToLower(strings.TrimSpace(key[index+1:]))
	if !util.IsSupportedPlatform(platform) {
		return "", false
	}
	return platform, true
}

func themeSettingWillChange(record CloudSyncRecord, rawValue string) bool {
	if record.EntityType != EntityWoxSetting || record.Key != "ThemeId" {
		return false
	}

	db := database.GetDB()
	if db == nil {
		return true
	}

	var stored database.WoxSetting
	err := db.Where("key = ?", record.Key).First(&stored).Error
	switch record.Op {
	case OpUpsert:
		return err != nil || stored.Value != rawValue
	case OpDelete:
		return err == nil
	default:
		return false
	}
}

// reloadAppliedSettings refreshes UI caches once per applied remote batch
// instead of once per individual record.
func (m *CloudSyncManager) reloadAppliedSettings(ctx context.Context, reloadWoxSettings bool, reloadPluginSettings bool, reloadInstalledPlugins bool, reloadInstalledThemes bool, applyCurrentTheme bool) {
	if m.settingReloader == nil {
		return
	}
	if reloadWoxSettings {
		m.settingReloader.ReloadSetting(ctx)
	}
	if reloadPluginSettings || reloadInstalledPlugins {
		m.settingReloader.ReloadSettingPlugins(ctx)
	}
	if reloadInstalledThemes {
		m.settingReloader.ReloadSettingThemes(ctx)
	}
	if applyCurrentTheme {
		if themeApplier, ok := m.settingReloader.(CloudSyncCurrentThemeApplier); ok {
			themeApplier.ApplyCurrentTheme(ctx)
		}
	}
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

// resolvePushResults maps server per-change results back to local oplog IDs and advances local failure counters.
func (m *CloudSyncManager) resolvePushResults(resp *CloudSyncPushResponse, changes []CloudSyncChange, oplogIds []uint, oplogs []database.Oplog) ([]uint, []CloudSyncOplogPushFailure, error) {
	if resp == nil {
		return nil, nil, errors.New("cloud sync push response is empty")
	}
	if len(resp.Applied) == 0 && len(changes) > 0 {
		return nil, nil, errors.New("cloud sync push response has no per-change results")
	}

	oplogByID := map[uint]database.Oplog{}
	for _, oplog := range oplogs {
		oplogByID[oplog.ID] = oplog
	}
	changeIDToOplogID := map[string]uint{}
	for i, change := range changes {
		if i >= len(oplogIds) {
			return nil, nil, errors.New("cloud sync push batch result mapping is invalid")
		}
		changeIDToOplogID[change.ChangeID] = oplogIds[i]
	}

	seen := map[string]struct{}{}
	var synced []uint
	var failures []CloudSyncOplogPushFailure
	for _, result := range resp.Applied {
		oplogID, ok := changeIDToOplogID[result.ChangeID]
		if !ok {
			return nil, nil, fmt.Errorf("cloud sync push response referenced unknown change %s", result.ChangeID)
		}
		seen[result.ChangeID] = struct{}{}
		switch result.Status {
		case "ok":
			synced = append(synced, oplogID)
		case "rejected":
			oplog, ok := oplogByID[oplogID]
			if !ok {
				return nil, nil, fmt.Errorf("cloud sync push response referenced unknown oplog %d", oplogID)
			}
			failedCount := oplog.CloudSyncPushFailedCount + 1
			failures = append(failures, CloudSyncOplogPushFailure{
				ID:          oplogID,
				FailedCount: failedCount,
				LastError:   cloudSyncAppliedChangeError(result),
				Discarded:   failedCount >= cloudSyncMaxOplogPushFailures,
			})
		default:
			return nil, nil, fmt.Errorf("cloud sync push response has unsupported change status %q", result.Status)
		}
	}
	for _, change := range changes {
		if _, ok := seen[change.ChangeID]; !ok {
			return nil, nil, fmt.Errorf("cloud sync push response missing result for change %s", change.ChangeID)
		}
	}
	return synced, failures, nil
}

// cloudSyncAppliedChangeError uses the server-localized message for user-facing diagnostics.
func cloudSyncAppliedChangeError(result CloudSyncAppliedChange) string {
	if result.Message != "" {
		return result.Message
	}
	if result.Code != "" {
		return result.Code
	}
	return "cloud sync change rejected"
}

// cloudSyncChangesForIDs preserves batch order when recording counts and history for successful rows only.
func cloudSyncChangesForIDs(changes []CloudSyncChange, oplogIds []uint, selectedIds []uint) []CloudSyncChange {
	if len(selectedIds) == 0 {
		return nil
	}
	selected := make([]CloudSyncChange, 0, len(selectedIds))
	for i, oplogID := range oplogIds {
		if i < len(changes) && slices.Contains(selectedIds, oplogID) {
			selected = append(selected, changes[i])
		}
	}
	return selected
}

// cloudSyncPushHistoryDetails records per-item outcomes in original batch order for the detail view.
func cloudSyncPushHistoryDetails(changes []CloudSyncChange, oplogIds []uint, syncedIds []uint, failures []CloudSyncOplogPushFailure) []CloudSyncHistoryRecordDetail {
	failureByID := map[uint]CloudSyncOplogPushFailure{}
	for _, failure := range failures {
		failureByID[failure.ID] = failure
	}

	details := make([]CloudSyncHistoryRecordDetail, 0, len(syncedIds)+len(failures))
	for i, oplogID := range oplogIds {
		if i >= len(changes) {
			continue
		}
		status := ""
		errorMessage := ""
		if slices.Contains(syncedIds, oplogID) {
			status = CloudSyncHistoryStatusSucceeded
		}
		if failure, ok := failureByID[oplogID]; ok {
			status = CloudSyncHistoryStatusFailed
			errorMessage = failure.LastError
		}
		if status == "" {
			continue
		}
		details = append(details, CloudSyncHistoryRecordDetail{
			EntityType: changes[i].EntityType,
			PluginID:   changes[i].PluginID,
			Key:        changes[i].Key,
			Op:         changes[i].Op,
			Status:     status,
			Error:      errorMessage,
		})
	}
	return details
}

func countDiscardedOplogPushFailures(failures []CloudSyncOplogPushFailure) int {
	count := 0
	for _, failure := range failures {
		if failure.Discarded {
			count++
		}
	}
	return count
}

// lastCloudSyncOplogPushFailureError returns the most recent rejection reason for operation history.
func lastCloudSyncOplogPushFailureError(failures []CloudSyncOplogPushFailure) string {
	for i := len(failures) - 1; i >= 0; i-- {
		if failures[i].LastError != "" {
			return failures[i].LastError
		}
	}
	return "cloud sync change rejected"
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
		if oplog.EntityType == EntityPluginSetting || oplog.EntityType == EntityInstalledPlugin {
			pluginID := oplog.EntityID
			if pluginID == "" {
				pluginID = oplog.Key
			}
			if _, blocked := disabled[pluginID]; blocked {
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

func (m *CloudSyncManager) countPendingOplogs(ctx context.Context) int {
	counter, ok := m.oplogStore.(CloudSyncPendingCounter)
	if !ok {
		return 0
	}

	total, err := counter.CountPending(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to count pending cloud sync oplogs: %v", err))
		return 0
	}
	return total
}

func (m *CloudSyncManager) setProgress(progress CloudSyncProgress) {
	progress.Active = true
	m.progressMu.Lock()
	m.progress = progress
	m.progressMu.Unlock()
	m.notifyProgressChanged(progress)
}

func (m *CloudSyncManager) clearProgress(operation string) {
	m.progressMu.Lock()
	if operation == "" || m.progress.Operation == operation {
		m.progress = CloudSyncProgress{}
		m.progressMu.Unlock()
		m.notifyProgressChanged(CloudSyncProgress{})
		return
	}
	m.progressMu.Unlock()
}

func (m *CloudSyncManager) setProgressFromOplog(operation string, oplog database.Oplog, current int, total int) {
	pluginId := ""
	if oplog.EntityType == EntityPluginSetting {
		pluginId = oplog.EntityID
	}
	m.setProgress(CloudSyncProgress{
		Operation:  operation,
		EntityType: oplog.EntityType,
		PluginID:   pluginId,
		Key:        oplog.Key,
		Current:    current,
		Total:      total,
	})
}

func (m *CloudSyncManager) setProgressFromRecord(operation string, record CloudSyncRecord, current int, total int) {
	m.setProgress(CloudSyncProgress{
		Operation:  operation,
		EntityType: record.EntityType,
		PluginID:   record.PluginID,
		Key:        record.Key,
		Current:    current,
		Total:      total,
	})
}

func (m *CloudSyncManager) notifyProgressChanged(progress CloudSyncProgress) {
	if m.progressNotifier == nil {
		return
	}

	util.Go(context.Background(), "cloud sync progress changed", func() {
		m.progressNotifier.CloudSyncProgressChanged(context.Background(), progress)
	})
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

// recordOperationHistory keeps user-visible sync history separate from sync cursor/state semantics.
func (m *CloudSyncManager) recordOperationHistory(ctx context.Context, operation string, reason string, startedAt int64, itemCount int, entityCounts map[string]int, keys []CloudSyncRecordKey, details []CloudSyncHistoryRecordDetail, status string, err error) {
	if m.historyStore == nil {
		return
	}
	if status == "" {
		status = CloudSyncHistoryStatusSucceeded
	}
	if status == CloudSyncHistoryStatusSucceeded && itemCount == 0 {
		return
	}
	if status != CloudSyncHistoryStatusSucceeded && status != CloudSyncHistoryStatusPartialSucceeded && status != CloudSyncHistoryStatusFailed {
		return
	}

	finishedAt := util.GetSystemTimestamp()
	record := CloudSyncHistoryRecord{
		Operation:    operation,
		Reason:       reason,
		Status:       status,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		DurationMs:   finishedAt - startedAt,
		ItemCount:    itemCount,
		EntityCounts: copyCloudSyncEntityCounts(entityCounts),
		Keys:         append([]CloudSyncRecordKey(nil), keys...),
		Details:      append([]CloudSyncHistoryRecordDetail(nil), details...),
	}
	if err != nil {
		record.Error = err.Error()
	}

	if recordErr := m.historyStore.Record(ctx, record); recordErr != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to record cloud sync history: %v", recordErr))
	}
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
		}
		// Push revisions are not pull acknowledgements; pull advances the cursor after applying remote records.
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

func addCloudSyncChangeEntityCounts(counts map[string]int, changes []CloudSyncChange) {
	for _, change := range changes {
		if change.EntityType != "" {
			counts[change.EntityType]++
		}
	}
}

func addCloudSyncRecordEntityCounts(counts map[string]int, records []CloudSyncRecord) {
	for _, record := range records {
		if record.EntityType != "" {
			counts[record.EntityType]++
		}
	}
}

func cloudSyncChangeRecordKeys(changes []CloudSyncChange) []CloudSyncRecordKey {
	keys := make([]CloudSyncRecordKey, 0, len(changes))
	for _, change := range changes {
		keys = append(keys, CloudSyncRecordKey{
			EntityType: change.EntityType,
			PluginID:   change.PluginID,
			Key:        change.Key,
			Op:         change.Op,
		})
	}
	return keys
}

func cloudSyncRecordKeys(records []CloudSyncRecord) []CloudSyncRecordKey {
	keys := make([]CloudSyncRecordKey, 0, len(records))
	for _, record := range records {
		keys = append(keys, CloudSyncRecordKey{
			EntityType: record.EntityType,
			PluginID:   record.PluginID,
			Key:        record.Key,
			Op:         record.Op,
		})
	}
	return keys
}

func copyCloudSyncEntityCounts(counts map[string]int) map[string]int {
	copied := map[string]int{}
	for entityType, count := range counts {
		if entityType != "" && count > 0 {
			copied[entityType] = count
		}
	}
	return copied
}

func normalizeCloudSyncConfig(config CloudSyncConfig) CloudSyncConfig {
	normalized := config
	defaults := DefaultCloudSyncConfig()

	if normalized.MaxBatchCount <= 0 {
		normalized.MaxBatchCount = defaults.MaxBatchCount
	}
	if normalized.MaxBatchBytes <= 0 {
		normalized.MaxBatchBytes = defaults.MaxBatchBytes
	}
	if normalized.PullLimit <= 0 {
		normalized.PullLimit = defaults.PullLimit
	}
	if normalized.SyncInterval <= 0 {
		normalized.SyncInterval = defaults.SyncInterval
	}

	return normalized
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
