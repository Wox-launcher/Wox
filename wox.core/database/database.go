package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"wox/util"

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
}

type Oplog struct {
	ID            uint `gorm:"primaryKey;autoIncrement"`
	EntityType    string
	EntityID      string
	Operation     string
	Key           string
	Value         string
	Timestamp     int64
	SyncedToCloud bool `gorm:"default:false"`
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

// MigrationRecord tracks one-time application migrations (data/setting compatibility upgrades).
// IDs are managed by the migration package and are ordered lexicographically.
type MigrationRecord struct {
	ID        string `gorm:"primaryKey"`
	AppliedAt int64  `gorm:"not null"`
	Status    string `gorm:"not null"` // applied | skipped
}

func Init(ctx context.Context) error {
	util.GetLogger().Info(ctx, "initializing database")

	dbPath := filepath.Join(util.GetLocation().GetUserDataDirectory(), "wox.db")

	// Configure SQLite with proper concurrency settings
	dsn := dbPath + "?" +
		"_journal_mode=DELETE&" + // Use DELETE journal mode for cloud-friendly single-file sync
		"_synchronous=FULL&" + // Safer for DELETE mode
		"_cache_size=1000&" + // Set cache size
		"_foreign_keys=true&" + // Enable foreign key constraints
		"_busy_timeout=5000" // Set busy timeout to 5 seconds

	var err error
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	// Configure connection pool for better concurrency handling
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(10)           // Maximum number of open connections
	sqlDB.SetMaxIdleConns(5)            // Maximum number of idle connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Maximum lifetime of a connection

	// Execute additional PRAGMA statements for optimal concurrency
	pragmas := []string{
		"PRAGMA journal_mode=DELETE", // Ensure WAL mode is enabled
		"PRAGMA synchronous=FULL",    // Balance safety and performance
		"PRAGMA cache_size=1000",     // Set cache size
		"PRAGMA foreign_keys=ON",     // Enable foreign key constraints
		"PRAGMA temp_store=memory",   // Store temporary tables in memory
		"PRAGMA mmap_size=268435456", // Set memory-mapped I/O size (256MB)
	}

	for _, pragma := range pragmas {
		if _, err := sqlDB.Exec(pragma); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to execute pragma %s: %v", pragma, err))
		}
	}

	runIntegrityChecks(ctx, sqlDB)

	err = db.AutoMigrate(
		&WoxSetting{},
		&PluginSetting{},
		&Oplog{},
		&MRURecord{},
		&ToolbarMute{},
		&MigrationRecord{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database schema: %w", err)
	}

	return nil
}

func GetDB() *gorm.DB {
	return db
}

// runIntegrityChecks runs a lightweight PRAGMA quick_check only to detect corruption.
func runIntegrityChecks(ctx context.Context, sqlDB *sql.DB) {
	logger := util.GetLogger()

	rows, err := sqlDB.Query("PRAGMA quick_check")
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("sqlite quick_check failed: %v", err))
		return
	}
	defer rows.Close()

	report := IntegrityReport{Ran: true}

	issues := make([]string, 0)
	for rows.Next() {
		var msg string
		if scanErr := rows.Scan(&msg); scanErr == nil {
			if msg != "ok" {
				issues = append(issues, msg)
			}
		}
	}
	if len(issues) == 0 {
		report.QuickCheckOK = true
		logger.Info(ctx, "sqlite quick_check: ok")
	} else {
		report.QuickCheckOK = false
		report.QuickCheckIssues = issues
		maxShow := 5
		if len(issues) < maxShow {
			maxShow = len(issues)
		}
		logger.Error(ctx, fmt.Sprintf("sqlite quick_check found %d issue(s). sample: %s", len(issues), strings.Join(issues[:maxShow], "; ")))
	}

	integrityReport = report
}
