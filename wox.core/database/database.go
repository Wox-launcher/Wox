package database

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
	"wox/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

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

func Init(ctx context.Context) error {
	dbPath := filepath.Join(util.GetLocation().GetUserDataDirectory(), "wox.db")

	// Configure SQLite with proper concurrency settings
	dsn := dbPath + "?" +
		"_journal_mode=WAL&" + // Enable WAL mode for better concurrency
		"_synchronous=NORMAL&" + // Balance between safety and performance
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
		"PRAGMA journal_mode=WAL",    // Ensure WAL mode is enabled
		"PRAGMA synchronous=NORMAL",  // Balance safety and performance
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

	err = db.AutoMigrate(
		&WoxSetting{},
		&PluginSetting{},
		&Oplog{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database schema: %w", err)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("database initialized at %s with WAL mode enabled", dbPath))
	return nil
}

func GetDB() *gorm.DB {
	return db
}
