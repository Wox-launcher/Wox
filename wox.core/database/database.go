package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wox/analytics"
	"wox/util"
	"wox/util/sqlitememory"

	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// IntegrityReport holds results of startup integrity checks
type IntegrityReport struct {
	Ran              bool
	QuickCheckOK     bool
	QuickCheckIssues []string
	FKViolationCount int
	AffectedTables   []string
}

var integrityReport IntegrityReport

// GetIntegrityReport returns the last integrity check report
func GetIntegrityReport() IntegrityReport {
	return integrityReport
}

var db *gorm.DB

type WoxSetting struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

type PluginSetting struct {
	PluginID string `gorm:"primaryKey"`
	Key      string `gorm:"primaryKey"`
	Value    string
	IsLocal  bool `gorm:"not null;default:false"`
}

type Oplog struct {
	ID                       uint `gorm:"primaryKey;autoIncrement"`
	EntityType               string
	EntityID                 string
	Operation                string
	Key                      string
	Value                    string
	Timestamp                int64
	SyncAfter                int64  `gorm:"index;default:0"`
	SyncedToCloud            bool   `gorm:"default:false"`
	CloudSyncDiscarded       bool   `gorm:"default:false"`
	CloudSyncPushFailedCount int    `gorm:"default:0"`
	CloudSyncLastPushError   string `gorm:"default:''"`
}

type CloudSyncState struct {
	ID           uint `gorm:"primaryKey"`
	Cursor       string
	LastPullTs   int64
	LastPushTs   int64
	BackoffUntil int64
	RetryCount   int
	LastError    string
	Bootstrapped bool
}

// CloudSyncRecordVersion is the newest client timestamp known for one sync identity.
// Remote applies remember it here because they do not create a local oplog.
type CloudSyncRecordVersion struct {
	EntityType string `gorm:"primaryKey"`
	EntityID   string `gorm:"primaryKey"`
	Key        string `gorm:"primaryKey"`
	ClientTs   int64
}

// DeviceIdentity stores the local cloud sync device identifier outside synced settings.
type DeviceIdentity struct {
	ID       uint `gorm:"primaryKey"`
	DeviceID string
}

// TelemetryState stores anonymous telemetry delivery state locally.
type TelemetryState struct {
	ID              uint `gorm:"primaryKey"`
	InstallID       string
	LastSentAt      int64
	LastSentVersion string
}

// CloudSyncHistory stores local device sync attempts for user-visible diagnostics.
type CloudSyncHistory struct {
	ID               uint `gorm:"primaryKey;autoIncrement"`
	Operation        string
	Reason           string
	Status           string
	StartedAt        int64 `gorm:"index"`
	FinishedAt       int64
	DurationMs       int64
	ItemCount        int
	EntityCountsJSON string `gorm:"type:text"`
	RecordKeysJSON   string `gorm:"type:text"`
	Error            string
}

type AccountState struct {
	ID              uint `gorm:"primaryKey"`
	LoggedIn        bool
	Email           string
	SyncEligible    bool
	SyncPlan        string
	SyncDeviceLimit int
	DeviceCount     int
	SyncEnabled     bool
	SessionExpired  bool
	UpdatedAt       int64
}

type MRURecord struct {
	Hash        string `gorm:"primaryKey"` // MD5 hash of pluginId+title+subTitle
	PluginID    string `gorm:"not null"`
	Title       string `gorm:"not null"`
	SubTitle    string
	Icon        string // JSON serialized WoxImage
	ContextData string // Plugin context data for restoration
	LastUsed    int64  `gorm:"not null"`
	UseCount    int    `gorm:"default:1"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AttentionItem stores persistent plugin-sourced user attention items.
type AttentionItem struct {
	IdentityKey        string `gorm:"primaryKey"`
	PluginID           string `gorm:"index;not null"`
	Key                string `gorm:"index;not null"`
	Title              string `gorm:"not null"`
	Description        string
	Icon               string
	Action             string
	ContentFingerprint string `gorm:"not null"`
	IsRead             bool   `gorm:"index;not null;default:false"`
	CreatedTimestamp   int64  `gorm:"index;not null"`
	UpdatedTimestamp   int64  `gorm:"index;not null"`
	ReadTimestamp      int64  `gorm:"index"`
}

// MigrationRecord tracks one-time application migrations (data/setting compatibility upgrades).
// IDs are managed by the migration package and are ordered lexicographically.
type MigrationRecord struct {
	ID        string `gorm:"primaryKey"`
	AppliedAt int64  `gorm:"not null"`
	Status    string `gorm:"not null"` // applied | skipped
}

func Init(ctx context.Context) error {
	util.GetLogger().Info(ctx, "initializing database")
	location := util.GetLocation()
	handle, err := openWithRecovery(ctx, filepath.Join(location.GetUserDataDirectory(), FileName), location.GetBackupDirectory())
	if err != nil {
		return err
	}
	db = handle
	integrityReport = IntegrityReport{Ran: true, QuickCheckOK: true}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlitememory.Register(sqlDB, "wox")
	return nil
}

func GetDB() *gorm.DB { return db }

// openWoxDatabase checks the file before modifying its schema and closes all handles on failure.
func openWoxDatabase(ctx context.Context, path string) (*gorm.DB, error) {
	needsCheck := true
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("database path is not a regular file")
		}
		if info.Size() < 100 {
			return nil, errDatabaseIntegrity
		}
		// Check existing data before opening in DELETE mode, which can checkpoint a WAL.
		if err := CheckDatabaseFile(ctx, path); err != nil {
			var sqliteErr sqlite3.Error
			if !errors.As(err, &sqliteErr) ||
				(sqliteErr.ExtendedCode != sqlite3.ErrReadonlyRollback && sqliteErr.ExtendedCode != sqlite3.ErrReadonlyRecovery) {
				return nil, err
			}
			// A hot journal is an interrupted transaction, not evidence of corruption.
		} else {
			needsCheck = false
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	conn, err := openDatabaseFile(path, "rwc&_journal_mode=DELETE&_synchronous=FULL&_foreign_keys=true&_cache_size=-2000")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = conn.Close()
		}
	}()
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(time.Hour)
	if needsCheck {
		if err := CheckDatabase(ctx, conn); err != nil {
			return nil, err
		}
	}
	for _, pragma := range []string{"PRAGMA temp_store=memory", "PRAGMA mmap_size=268435456"} {
		if _, err := conn.ExecContext(ctx, pragma); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("configure SQLite %s: %v", pragma, err))
		}
	}
	handle, err := gorm.Open(sqlite.New(sqlite.Config{Conn: conn}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	if err := handle.WithContext(ctx).AutoMigrate(databaseModels()...); err != nil {
		return nil, fmt.Errorf("failed to migrate database schema: %w", err)
	}
	keep = true
	return handle, nil
}

func databaseModels() []any {
	return []any{
		&analytics.Event{}, &WoxSetting{}, &PluginSetting{}, &Oplog{}, &CloudSyncState{},
		&CloudSyncRecordVersion{}, &DeviceIdentity{}, &TelemetryState{}, &CloudSyncHistory{},
		&AccountState{}, &MRURecord{}, &AttentionItem{}, &MigrationRecord{},
	}
}
