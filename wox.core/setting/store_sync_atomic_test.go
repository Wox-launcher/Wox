package setting

import (
	"errors"
	"path/filepath"
	"testing"

	"wox/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestSetWithSyncRollsBackSettingWhenOplogFails(t *testing.T) {
	db := openSettingSyncDB(t)
	if err := db.Exec(`CREATE TRIGGER reject_query_histories_oplog
		BEFORE INSERT ON oplogs
		WHEN NEW.key = 'QueryHistories'
		BEGIN
			SELECT RAISE(ABORT, 'oplog rejected');
		END`).Error; err != nil {
		t.Fatal(err)
	}

	store := NewWoxSettingStore(db)
	value := NewWoxSettingValue(store, "QueryHistories", "[]")
	if err := value.Set(`[{"QueryText":"1+1"}]`); err == nil {
		t.Fatal("expected oplog failure")
	}
	if got := value.Get(); got != "[]" {
		t.Fatalf("memory = %q, want the previous value", got)
	}
	var saved database.WoxSetting
	err := db.Where("key = ?", "QueryHistories").First(&saved).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("setting row after failed oplog: %v, value=%q", err, saved.Value)
	}

	if err := db.Exec("DROP TRIGGER reject_query_histories_oplog").Error; err != nil {
		t.Fatal(err)
	}
	if err := value.Set(`[{"QueryText":"1+1"}]`); err != nil {
		t.Fatal(err)
	}
	if got := value.Get(); got != `[{"QueryText":"1+1"}]` {
		t.Fatalf("memory = %q", got)
	}
	if err := db.Where("key = ?", "QueryHistories").First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	var oplogs int64
	if err := db.Model(&database.Oplog{}).Where("key = ?", "QueryHistories").Count(&oplogs).Error; err != nil {
		t.Fatal(err)
	}
	if oplogs != 1 {
		t.Fatalf("oplogs = %d, want 1", oplogs)
	}
}

func TestPluginSetWithSyncWritesSettingAndOplog(t *testing.T) {
	db := openSettingSyncDB(t)
	store := NewPluginSettingStore(db, "plugin")
	if err := store.SetWithSync("watch", "1", true); err != nil {
		t.Fatal(err)
	}

	var row database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", "plugin", "watch").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != "1" || row.IsLocal {
		t.Fatalf("row = %+v", row)
	}
	var oplogs int64
	if err := db.Model(&database.Oplog{}).Where("entity_id = ? AND key = ?", "plugin", "watch").Count(&oplogs).Error; err != nil {
		t.Fatal(err)
	}
	if oplogs != 1 {
		t.Fatalf("oplogs = %d, want 1", oplogs)
	}
}

func TestRetrySQLiteWriteStopsOnPermanentError(t *testing.T) {
	calls := 0
	err := retrySQLiteWrite(func() error {
		calls++
		return errors.New("oplog rejected")
	})
	if err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestRetrySQLiteWriteRetriesLockError(t *testing.T) {
	calls := 0
	err := retrySQLiteWrite(func() error {
		calls++
		if calls < 2 {
			return errors.New("database is locked")
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func openSettingSyncDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "wox.db")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.WoxSetting{}, &database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}
