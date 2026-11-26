package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "wox.db"

var errDatabaseIntegrity = errors.New("invalid SQLite database")

// openDatabaseFile opens a named SQLite database without letting a missing file become a new one.
func openDatabaseFile(path, mode string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=" + mode + "&_busy_timeout=5000"}
	return sql.Open("sqlite3", uri.String())
}

// CheckDatabaseFile verifies that an existing copy is readable and free of structural errors.
func CheckDatabaseFile(ctx context.Context, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() < 100 {
		return fmt.Errorf("invalid SQLite database file")
	}
	source, err := openDatabaseFile(path, "ro")
	if err != nil {
		return err
	}
	defer source.Close()
	return CheckDatabase(ctx, source)
}

// CheckDatabase checks every reported row and treats failed or empty checks as unhealthy.
func CheckDatabase(ctx context.Context, source *sql.DB) error {
	return checkDatabase(ctx, source, "PRAGMA quick_check")
}

func checkDatabase(ctx context.Context, source *sql.DB, pragma string) error {
	rows, err := source.QueryContext(ctx, pragma)
	if err != nil {
		return err
	}
	defer rows.Close()
	checked := false
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return err
		}
		if message != "ok" {
			return fmt.Errorf("%w: %s: %s", errDatabaseIntegrity, pragma, message)
		}
		checked = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !checked {
		return fmt.Errorf("%w: %s returned no rows", errDatabaseIntegrity, pragma)
	}
	return nil
}

// SnapshotDatabase writes and verifies a transaction-consistent copy before publishing it.
func SnapshotDatabase(ctx context.Context, source *sql.DB, destination string) error {
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return errors.New("database snapshot destination already exists")
	}
	dir, err := os.MkdirTemp(filepath.Dir(destination), ".sqlite-snapshot-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	staged := filepath.Join(dir, FileName)
	if _, err := source.ExecContext(ctx, "VACUUM INTO ?", staged); err != nil {
		return err
	}
	if err := CheckDatabaseFile(ctx, staged); err != nil {
		return err
	}
	f, err := os.OpenFile(staged, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return err
	}
	return os.Rename(staged, destination)
}
