package settingadapter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"wox/cloudsync"
	"wox/common"
	"wox/database"
	"wox/plugin/system/notes"
	"wox/setting"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type conflictPullClient struct {
	cloudsync.CloudSyncClient
	record cloudsync.CloudSyncRecord
}

func (c conflictPullClient) Pull(context.Context, cloudsync.CloudSyncPullRequest) (*cloudsync.CloudSyncPullResponse, error) {
	return &cloudsync.CloudSyncPullResponse{Records: []cloudsync.CloudSyncRecord{c.record}, NextCursor: "next"}, nil
}

type conflictPlaintextCrypto struct{ cloudsync.CloudSyncCrypto }

func (conflictPlaintextCrypto) Decrypt(_ context.Context, value cloudsync.CloudSyncEncryptedValue, _ string) (string, error) {
	return value.Ciphertext, nil
}

func TestCloudNotesPreservesLocalDraftAndArchivesBothVersions(t *testing.T) {
	initSnapshotterTestDatabase(t)
	ctx := context.Background()
	store := setting.NewPluginSettingStore(database.GetDB(), common.NotesPluginID)
	local := common.NoteRecord{ID: "original", Revision: "local", SchemaVersion: 1, Document: notes.ParseMarkdown("# Project\n\nLocal **draft**")}
	remote := common.NoteRecord{ID: local.ID, Revision: "remote", SchemaVersion: 1, Document: notes.ParseMarkdown("# Project\n\nRemote winner")}
	localJSON, err := json.Marshal(local)
	require.NoError(t, err)
	remoteJSON, err := json.Marshal(remote)
	require.NoError(t, err)
	require.NoError(t, store.SetWithSync("note:"+local.ID, string(localJSON), true))
	require.NoError(t, store.Set("placement:"+local.ID, common.NotePlacement{NoteID: local.ID, ParentID: "group-1"}))
	manager := cloudsync.NewCloudSyncManager(cloudsync.DefaultCloudSyncConfig(), cloudsync.CloudSyncDependencies{
		Client: conflictPullClient{record: cloudsync.CloudSyncRecord{
			EntityType: cloudsync.EntityPluginSetting, PluginID: common.NotesPluginID, Key: "note:" + local.ID,
			Op: cloudsync.OpUpsert, ClientTs: util.GetSystemTimestamp() + 100,
			Value: &cloudsync.CloudSyncEncryptedValue{KeyVersion: 1, Ciphertext: string(remoteJSON)},
		}},
		Crypto: conflictPlaintextCrypto{}, DeviceProvider: cloudsync.NewDatabaseDeviceProvider(),
		Applier: NewLocalSettingApplier(), OplogStore: cloudsync.NewDefaultOplogStore(), ConflictPreserver: LocalConflictPreserver{},
	})

	manager.Pull(ctx, "conflict-test")
	manager.Pull(ctx, "retry")
	repository := notes.NewRepository(store)
	winner, err := repository.Get(local.ID)
	require.NoError(t, err)
	require.Equal(t, "remote", winner.Revision)
	all, err := repository.List(false)
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, note := range all {
		if note.ID == local.ID {
			continue
		}
		require.Contains(t, notes.ToMarkdown(note.Document), "Local **draft**")
		require.Contains(t, notes.NoteTitle(note.Document), "Sync conflict")
		var placement common.NotePlacement
		require.NoError(t, store.Get("placement:"+note.ID, &placement))
		require.Equal(t, "group-1", placement.ParentID)
	}
	root := util.GetLocation().GetBackupDirectory()
	total, copies := cloudsync.ConflictCounts(root)
	require.Equal(t, 1, total)
	require.Equal(t, 1, copies)
	files, err := filepath.Glob(filepath.Join(root, "sync_conflicts", "note-*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	raw, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var archived cloudsync.Conflict
	require.NoError(t, json.Unmarshal(raw, &archived))
	require.Equal(t, string(localJSON), archived.LocalValue)
	require.Equal(t, string(remoteJSON), archived.RemoteValue)
}

func TestCustomPluginConflictHandlerRunsWithoutSettingAdapterCoupling(t *testing.T) {
	initSnapshotterTestDatabase(t)
	ctx := context.Background()
	const pluginID = "com.wox.test.custom-conflict"

	calls := 0
	returnValue := "preserved-copy"
	cloudsync.RegisterPluginConflictHandler(pluginID, func(_ context.Context, conflict cloudsync.Conflict) (cloudsync.PluginConflictResult, error) {
		calls++
		return cloudsync.PluginConflictResult{
			ArchiveKind:  cloudsync.ConflictArchiveGeneric,
			SettingKey:   conflict.Key + ":copy",
			SettingValue: returnValue,
		}, nil
	})
	t.Cleanup(func() {
		cloudsync.RegisterPluginConflictHandler(pluginID, nil)
	})

	preserver := LocalConflictPreserver{}
	notify, err := preserver.PreserveConflict(ctx, cloudsync.Conflict{
		EntityType:  cloudsync.EntityPluginSetting,
		PluginID:    pluginID,
		Key:         "item:1",
		LocalValue:  "local-1",
		RemoteValue: "remote-1",
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.NotNil(t, notify)

	root := util.GetLocation().GetBackupDirectory()
	total, noteCopies := cloudsync.ConflictCounts(root)
	require.Equal(t, 1, total)
	require.Equal(t, 0, noteCopies)
	files, err := filepath.Glob(filepath.Join(root, "sync_conflicts", "conflict-*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)

	returnValue = ""
	notifyRetry, err := preserver.PreserveConflict(ctx, cloudsync.Conflict{
		EntityType:  cloudsync.EntityPluginSetting,
		PluginID:    pluginID,
		Key:         "item:2",
		LocalValue:  "local-2",
		RemoteValue: "remote-2",
	})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Nil(t, notifyRetry)
}
