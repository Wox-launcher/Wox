package migration

import (
	"context"

	"gorm.io/gorm"
)

func init() { Register(&clearLegacyShellHistoryMigration{}) }

type clearLegacyShellHistoryMigration struct{}

func (*clearLegacyShellHistoryMigration) ID() string { return "20261110_clear_legacy_shell_history" }

func (*clearLegacyShellHistoryMigration) Description() string {
	return "Remove legacy Shell history before switching to interactive Shell Preview."
}

// Up keeps history written by the interactive preview, which introduced the interactive: ID prefix.
func (*clearLegacyShellHistoryMigration) Up(ctx context.Context, tx *gorm.DB) error {
	db := tx.WithContext(ctx)
	if !db.Migrator().HasTable("shell_histories") {
		return nil
	}
	return db.Exec("DELETE FROM shell_histories WHERE id IS NULL OR id NOT GLOB 'interactive:*'").Error
}
