package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"wox/analytics"
	"wox/util"
	"wox/util/shell"

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
		&analytics.Event{},
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

type RecoveryResult struct {
	RecoveredPath string
	BackupPath    string
	Swapped       bool
}

func RecoverDatabase(ctx context.Context) (RecoveryResult, error) {
	logger := util.GetLogger()
	result := RecoveryResult{}

	if _, err := exec.LookPath("sqlite3"); err != nil {
		return result, fmt.Errorf("sqlite3 not found in PATH: %w", err)
	}

	dbPath := filepath.Join(util.GetLocation().GetUserDataDirectory(), "wox.db")
	if _, err := os.Stat(dbPath); err != nil {
		return result, fmt.Errorf("failed to stat database: %w", err)
	}

	ts := util.GetSystemTimestamp()
	backupDir := filepath.Join(util.GetLocation().GetBackupDirectory(), fmt.Sprintf("db_repair_%d", ts))
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return result, fmt.Errorf("failed to create repair directory: %w", err)
	}

	workingDbPath := filepath.Join(backupDir, "wox.db")
	if err := copyFile(dbPath, workingDbPath); err != nil {
		return result, fmt.Errorf("failed to copy database: %w", err)
	}

	for _, suffix := range []string{"-wal", "-shm"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err == nil {
			if copyErr := copyFile(src, workingDbPath+suffix); copyErr != nil {
				logger.Warn(ctx, fmt.Sprintf("failed to copy %s: %v", src, copyErr))
			}
		}
	}

	recoverSQLPath := filepath.Join(backupDir, "recover.sql")
	sqlFile, err := os.Create(recoverSQLPath)
	if err != nil {
		return result, fmt.Errorf("failed to create recovery SQL: %w", err)
	}

	recoverCmd := shell.BuildCommand("sqlite3", nil, workingDbPath, ".recover")
	recoverCmd.Stdout = sqlFile
	recoverErrOutput := &bytes.Buffer{}
	recoverCmd.Stderr = recoverErrOutput
	if err := recoverCmd.Run(); err != nil {
		_ = sqlFile.Close()
		logger.Error(ctx, fmt.Sprintf("sqlite3 .recover stderr: %s", strings.TrimSpace(recoverErrOutput.String())))
		return result, fmt.Errorf("sqlite3 .recover failed: %w", err)
	}
	if err := sqlFile.Close(); err != nil {
		return result, fmt.Errorf("failed to close recovery SQL: %w", err)
	}

	recoveredDbPath := filepath.Join(backupDir, "wox.recovered.db")

	recoverInput, err := os.Open(recoverSQLPath)
	if err != nil {
		return result, fmt.Errorf("failed to open recovery SQL: %w", err)
	}
	defer recoverInput.Close()

	importCmd := shell.BuildCommand("sqlite3", nil, recoveredDbPath)
	importCmd.Stdin = recoverInput
	importErrOutput := &bytes.Buffer{}
	importCmd.Stderr = importErrOutput
	if err := importCmd.Run(); err != nil {
		logger.Error(ctx, fmt.Sprintf("sqlite3 import stderr: %s", strings.TrimSpace(importErrOutput.String())))
		return result, fmt.Errorf("sqlite3 import failed: %w", err)
	}

	checkCmd := shell.BuildCommand("sqlite3", nil, recoveredDbPath, "PRAGMA integrity_check;")
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return result, fmt.Errorf("sqlite3 integrity_check failed: %w: %s", err, strings.TrimSpace(string(checkOutput)))
	}
	if strings.TrimSpace(string(checkOutput)) != "ok" {
		return result, fmt.Errorf("sqlite3 integrity_check not ok: %s", strings.TrimSpace(string(checkOutput)))
	}
	result.RecoveredPath = recoveredDbPath

	backupOriginalPath := fmt.Sprintf("%s.before_repair_%d", dbPath, ts)
	if err := os.Rename(dbPath, backupOriginalPath); err != nil {
		result.BackupPath = backupOriginalPath
		return result, fmt.Errorf("failed to rename original database: %w", err)
	}
	result.BackupPath = backupOriginalPath

	for _, suffix := range []string{"-wal", "-shm"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err == nil {
			if renameErr := os.Rename(src, backupOriginalPath+suffix); renameErr != nil {
				logger.Warn(ctx, fmt.Sprintf("failed to rename %s: %v", src, renameErr))
			}
		}
	}

	if err := copyFile(recoveredDbPath, dbPath); err != nil {
		return result, fmt.Errorf("failed to replace database: %w", err)
	}

	result.Swapped = true
	result.RecoveredPath = dbPath
	return result, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		return err
	}
	if err := dstFile.Close(); err != nil {
		return err
	}

	if info, err := os.Stat(src); err == nil {
		_ = os.Chmod(dst, info.Mode())
	}

	return nil
}
