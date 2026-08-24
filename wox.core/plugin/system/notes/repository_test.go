package notes

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wox/common"
	"wox/database"
	"wox/setting"
)

func newRepositoryForTest(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return NewRepository(setting.NewPluginSettingStore(db, common.NotesPluginID)), db
}

func TestRepositorySaveConflictPreservesRemoteAndCreatesCopy(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	record, err := repository.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	remoteDocument := common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Remote"}}}
	remote, conflict, err := repository.Save(record.ID, record.Revision, remoteDocument)
	if err != nil || conflict {
		t.Fatalf("remote save conflict=%t err=%v", conflict, err)
	}
	localDocument := common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Local"}}}
	copyRecord, conflict, err := repository.Save(record.ID, record.Revision, localDocument)
	if err != nil || !conflict {
		t.Fatalf("stale save conflict=%t err=%v", conflict, err)
	}
	current, err := repository.Get(record.ID)
	if err != nil || NoteTitle(current.Document) != "Remote" || current.Revision != remote.Revision {
		t.Fatalf("remote record overwritten: %#v err=%v", current, err)
	}
	if NoteTitle(copyRecord.Document) != "Local (Sync Conflict)" {
		t.Fatalf("unexpected conflict title: %q", NoteTitle(copyRecord.Document))
	}
}

func TestRepositoryTrashRestoreAndPurge(t *testing.T) {
	repository, db := newRepositoryForTest(t)
	record, err := repository.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	deleted, err := repository.Delete(record.ID)
	if err != nil || deleted.DeletedAt == 0 {
		t.Fatalf("delete: %#v err=%v", deleted, err)
	}
	restored, err := repository.Restore(record.ID)
	if err != nil || restored.DeletedAt != 0 {
		t.Fatalf("restore: %#v err=%v", restored, err)
	}
	deleted, err = repository.Delete(record.ID)
	if err != nil {
		t.Fatalf("delete again: %v", err)
	}
	deleted.DeletedAt = time.Now().Add(-61 * 24 * time.Hour).UnixMilli()
	if err := repository.write(deleted); err != nil {
		t.Fatalf("backdate trash: %v", err)
	}
	count, err := repository.PurgeDeletedBefore(time.Now().Add(-60 * 24 * time.Hour))
	if err != nil || count != 1 {
		t.Fatalf("purge count=%d err=%v", count, err)
	}
	var remaining int64
	if err := db.Model(&database.PluginSetting{}).Where("plugin_id = ? AND key = ?", common.NotesPluginID, noteSettingPrefix+record.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		t.Fatalf("purged row remains=%d err=%v", remaining, err)
	}
}

func TestRepositoryLocalPreferencesDoNotSyncAndTrashHonorsBoundary(t *testing.T) {
	repository, db := newRepositoryForTest(t)
	if err := repository.SetLocal("windowBounds", `{"x":-120}`); err != nil {
		t.Fatalf("set local preference: %v", err)
	}
	var local database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", common.NotesPluginID, "windowBounds").First(&local).Error; err != nil || !local.IsLocal {
		t.Fatalf("preference was not local-only: %#v err=%v", local, err)
	}
	var oplogs int64
	if err := db.Model(&database.Oplog{}).Where("entity_id = ? AND key = ?", common.NotesPluginID, "windowBounds").Count(&oplogs).Error; err != nil || oplogs != 0 {
		t.Fatalf("local preference generated sync oplog: count=%d err=%v", oplogs, err)
	}

	record, err := repository.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	record, err = repository.Delete(record.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	cutoff := time.Now().Add(-60 * 24 * time.Hour).Truncate(time.Millisecond)
	record.DeletedAt = cutoff.Add(time.Millisecond).UnixMilli()
	if err := repository.write(record); err != nil {
		t.Fatalf("write boundary record: %v", err)
	}
	if count, err := repository.PurgeDeletedBefore(cutoff); err != nil || count != 0 {
		t.Fatalf("newer-than-boundary trash purged: count=%d err=%v", count, err)
	}
	record.DeletedAt = cutoff.UnixMilli()
	if err := repository.write(record); err != nil {
		t.Fatalf("write exact boundary record: %v", err)
	}
	if count, err := repository.PurgeDeletedBefore(cutoff); err != nil || count != 1 {
		t.Fatalf("exact-boundary trash not purged: count=%d err=%v", count, err)
	}
}

func TestRepositoryCoalescesDeferredSyncPerNoteKey(t *testing.T) {
	repository, db := newRepositoryForTest(t)
	first, err := repository.Create()
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	for _, title := range []string{"one", "two", "three"} {
		first, _, err = repository.Save(first.ID, first.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: title}}})
		if err != nil {
			t.Fatalf("save first: %v", err)
		}
	}
	second, err := repository.Create()
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	var oplogs []database.Oplog
	if err := db.Where("entity_id = ? AND synced_to_cloud = ? AND cloud_sync_discarded = ?", common.NotesPluginID, false, false).Order("key").Find(&oplogs).Error; err != nil {
		t.Fatalf("list oplogs: %v", err)
	}
	keys := map[string]bool{}
	for _, oplog := range oplogs {
		keys[oplog.Key] = true
	}
	if len(oplogs) != 2 || !keys[noteSettingPrefix+first.ID] || !keys[noteSettingPrefix+second.ID] {
		t.Fatalf("note oplogs were not coalesced per key: %#v", oplogs)
	}
}
