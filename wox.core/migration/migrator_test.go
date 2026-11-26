package migration

import (
	"context"
	"path/filepath"
	"testing"
	"wox/database"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackupBeforeMigrationsCreatesMissingRoot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, database.FileName)), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	defer conn.Close()
	require.NoError(t, db.AutoMigrate(&database.WoxSetting{}))
	require.NoError(t, db.Create(&database.WoxSetting{Key: "test-setting", Value: "retained"}).Error)

	root := filepath.Join(dir, "profile", "backup")
	require.NoDirExists(t, root)
	require.NoError(t, backupBeforeMigrations(ctx, db, root))

	snapshots, err := filepath.Glob(filepath.Join(root, "pre_migration_*", database.FileName))
	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.NoError(t, database.CheckDatabaseFile(ctx, snapshots[0]))
}
