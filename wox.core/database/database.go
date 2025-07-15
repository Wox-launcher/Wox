package database

import (
	"fmt"
	"path/filepath"
	"sync"
	"wox/common"
	"wox/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	db   *gorm.DB
	once sync.Once
)

const dbFileName = "wox.db"

// Models

type Setting struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

type Hotkey struct {
	ID                uint   `gorm:"primaryKey;autoIncrement"`
	Hotkey            string `gorm:"unique"`
	Query             string
	IsSilentExecution bool
}

type QueryShortcut struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	Shortcut string `gorm:"unique"`
	Query    string
}

type AIProvider struct {
	ID     uint `gorm:"primaryKey;autoIncrement"`
	Name   common.ProviderName
	ApiKey string
	Host   string
}

type QueryHistory struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	Query     string
	Timestamp int64
}

type FavoriteResult struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	PluginID string `gorm:"uniqueIndex:idx_fav"`
	Title    string `gorm:"uniqueIndex:idx_fav"`
	Subtitle string `gorm:"uniqueIndex:idx_fav"`
}

type PluginSetting struct {
	ID       uint   `gorm:"primaryKey;autoIncrement"`
	PluginID string `gorm:"uniqueIndex:idx_plugin_setting"`
	Key      string `gorm:"uniqueIndex:idx_plugin_setting"`
	Value    string
}

type ActionedResult struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	PluginID  string
	Title     string
	Subtitle  string
	Timestamp int64
	Query     string
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

// Init initializes the database connection and migrates the schema.
func Init() error {
	var err error
	once.Do(func() {
		dbPath := filepath.Join(util.GetLocation().GetUserDataDirectory(), dbFileName)

		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})

		if err != nil {
			err = fmt.Errorf("failed to connect to database: %w", err)
			return
		}

		// AutoMigrate will create tables, columns, and indexes, but not delete them.
		err = migrateSchema()
		if err != nil {
			err = fmt.Errorf("failed to migrate database schema: %w", err)
			return
		}
	})
	return err
}

// GetDB returns the GORM database instance.
func GetDB() *gorm.DB {
	return db
}

// migrateSchema runs GORM's AutoMigrate function.
func migrateSchema() error {
	return db.AutoMigrate(
		&Setting{},
		&Hotkey{},
		&QueryShortcut{},
		&AIProvider{},
		&QueryHistory{},
		&FavoriteResult{},
		&PluginSetting{},
		&ActionedResult{},
		&Oplog{},
	)
}
