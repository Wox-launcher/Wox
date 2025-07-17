package database

import (
	"context"
	"fmt"
	"path/filepath"
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

	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	err = db.AutoMigrate(
		&WoxSetting{},
		&PluginSetting{},
		&Oplog{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database schema: %w", err)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("database initialized at %s", dbPath))
	return nil
}

func GetDB() *gorm.DB {
	return db
}
