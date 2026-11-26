package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

type Migration interface {
	ID() string
	Description() string
	Up(ctx context.Context, tx *gorm.DB) error
}

type PostCommitMigration interface {
	Migration
	AfterCommit(ctx context.Context) error
}

type ConditionalMigration interface {
	Migration
	IsNeeded(ctx context.Context, db *gorm.DB) (bool, error)
}

var registeredMigrations []Migration

func Register(m Migration) {
	if m == nil {
		panic("migration: Register(nil)")
	}
	id := strings.TrimSpace(m.ID())
	if id == "" {
		panic("migration: empty migration ID")
	}
	for _, existing := range registeredMigrations {
		if existing.ID() == id {
			panic("migration: duplicate migration ID: " + id)
		}
	}
	registeredMigrations = append(registeredMigrations, m)
}

func Run(ctx context.Context) error {
	db := database.GetDB()
	if db == nil {
		return fmt.Errorf("migration: database not initialized")
	}
	return runWithBackup(ctx, db, util.GetLocation().GetBackupDirectory())
}

func RunWithDB(ctx context.Context, db *gorm.DB) error {
	return runWithBackup(ctx, db, "")
}

// runWithBackup snapshots the live database once before the first pending migration.
func runWithBackup(ctx context.Context, db *gorm.DB, backupRoot string) error {
	logger := util.GetLogger()

	migrations := make([]Migration, 0, len(registeredMigrations))
	migrations = append(migrations, registeredMigrations...)
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].ID() < migrations[j].ID() })

	var applied []database.MigrationRecord
	if err := db.Find(&applied).Error; err != nil {
		return fmt.Errorf("migration: failed to load migration records: %w", err)
	}
	appliedSet := map[string]database.MigrationRecord{}
	for _, rec := range applied {
		appliedSet[rec.ID] = rec
	}

	for _, m := range migrations {
		id := m.ID()
		if _, ok := appliedSet[id]; ok {
			continue
		}
		if backupRoot != "" {
			if err := backupBeforeMigrations(ctx, db, backupRoot); err != nil {
				return fmt.Errorf("migration: create pre-migration snapshot: %w", err)
			}
			backupRoot = ""
		}

		if conditional, ok := m.(ConditionalMigration); ok {
			needed, err := conditional.IsNeeded(ctx, db)
			if err != nil {
				return fmt.Errorf("migration: %s IsNeeded failed: %w", id, err)
			}
			if !needed {
				if err := db.Create(&database.MigrationRecord{
					ID:        id,
					AppliedAt: time.Now().Unix(),
					Status:    "skipped",
				}).Error; err != nil {
					return fmt.Errorf("migration: %s failed to record skipped: %w", id, err)
				}
				logger.Info(ctx, fmt.Sprintf("migration skipped: %s", id))
				continue
			}
		}

		logger.Info(ctx, fmt.Sprintf("migration applying: %s", id))

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.Up(ctx, tx); err != nil {
				return err
			}
			return tx.Create(&database.MigrationRecord{
				ID:        id,
				AppliedAt: time.Now().Unix(),
				Status:    "applied",
			}).Error
		}); err != nil {
			return fmt.Errorf("migration: %s failed: %w", id, err)
		}

		if postCommit, ok := m.(PostCommitMigration); ok {
			if err := postCommit.AfterCommit(ctx); err != nil {
				logger.Warn(ctx, fmt.Sprintf("migration after-commit failed: %s: %v", id, err))
			}
		}

		logger.Info(ctx, fmt.Sprintf("migration applied: %s", id))
	}

	return nil
}

// backupBeforeMigrations retains the three latest verified snapshots outside daily backup rotation.
func backupBeforeMigrations(ctx context.Context, db *gorm.DB, root string) error {
	return database.WithFileMaintenance(func() error {
		source, err := db.DB()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(root, 0700); err != nil {
			return err
		}
		dir, err := os.MkdirTemp(root, "pre_migration_"+time.Now().UTC().Format("20060102T150405.000000000")+"-")
		if err != nil {
			return err
		}
		if err := database.SnapshotDatabase(ctx, source, filepath.Join(dir, database.FileName)); err != nil {
			_ = os.RemoveAll(dir)
			return err
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		var snapshots []string
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "pre_migration_") {
				path := filepath.Join(root, entry.Name(), database.FileName)
				if database.CheckDatabaseFile(ctx, path) == nil {
					snapshots = append(snapshots, entry.Name())
				}
			}
		}
		for _, name := range snapshots[:max(0, len(snapshots)-3)] {
			if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("remove old pre-migration snapshot: %v", err))
			}
		}
		return nil
	})
}
