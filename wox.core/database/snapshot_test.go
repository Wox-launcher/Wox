package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapshotDatabaseIncludesCommittedWALWrites(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	source, err := sql.Open("sqlite3", filepath.Join(root, FileName))
	require.NoError(t, err)
	defer source.Close()
	_, err = source.Exec("PRAGMA journal_mode = WAL")
	require.NoError(t, err)
	_, err = source.Exec("CREATE TABLE settings (value TEXT)")
	require.NoError(t, err)
	_, err = source.Exec("INSERT INTO settings VALUES ('kept')")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(root, FileName+"-wal"))

	destination := filepath.Join(root, "backup #1.db")
	require.NoError(t, SnapshotDatabase(ctx, source, destination))
	require.NoError(t, CheckDatabaseFile(ctx, destination))
	snapshot, err := openDatabaseFile(destination, "ro")
	require.NoError(t, err)
	defer snapshot.Close()
	var value string
	require.NoError(t, snapshot.QueryRow("SELECT value FROM settings").Scan(&value))
	require.Equal(t, "kept", value)
}
