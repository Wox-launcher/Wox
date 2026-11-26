package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"wox/util"
	"wox/util/shell"
)

// recoverCandidate works on a disposable copy and only publishes a verified recovery.
func recoverCandidate(ctx context.Context, quarantine, stage, destination string) (int, error) {
	dir, err := os.MkdirTemp(stage, ".salvage-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, FileName)
	if err := WithFileMaintenance(func() error {
		return copyDatabaseFiles(filepath.Join(quarantine, FileName), src)
	}); err != nil {
		return 0, err
	}
	if sqlite, err := exec.LookPath("sqlite3"); err == nil {
		cliPath := filepath.Join(dir, "cli.db")
		rows, err := recoverWithCLI(ctx, sqlite, src, cliPath)
		if err == nil {
			return rows, os.Rename(cliPath, destination)
		}
		util.GetLogger().Debug(ctx, fmt.Sprintf("SQLite CLI recovery did not produce a usable database: %v", err))
	}
	built := filepath.Join(dir, "salvaged.db")
	rows, err := salvageDatabase(ctx, src, built)
	if err != nil {
		return 0, err
	}
	if err := initializeRecoveryCandidate(ctx, built); err != nil {
		return 0, err
	}
	return rows, os.Rename(built, destination)
}

// recoverWithCLI can locate lost pages when an installed SQLite build enables recovery.
func recoverWithCLI(ctx context.Context, sqlite, src, dst string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	dump, err := os.Create(filepath.Join(filepath.Dir(dst), "recover.sql"))
	if err != nil {
		return 0, err
	}
	defer dump.Close()
	recover := shell.BuildCommandContext(ctx, sqlite, nil, "-batch", "-readonly", src, ".recover")
	recover.Stdout = dump
	if err := recover.Run(); err != nil {
		return 0, err
	}
	if _, err := dump.Seek(0, 0); err != nil {
		return 0, err
	}
	restore := shell.BuildCommandContext(ctx, sqlite, nil, "-batch", "-bail", dst)
	restore.Stdin = dump
	if err := restore.Run(); err != nil {
		return 0, err
	}
	if err := initializeRecoveryCandidate(ctx, dst); err != nil {
		return 0, err
	}
	conn, err := openDatabaseFile(dst, "ro")
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	if err := checkDatabase(ctx, conn, "PRAGMA integrity_check"); err != nil {
		return 0, err
	}
	var rows int
	if err := conn.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM wox_settings) + (SELECT count(*) FROM plugin_settings) + (SELECT count(*) FROM mru_records) + (SELECT count(*) FROM oplogs)").Scan(&rows); err != nil {
		return 0, err
	}
	if rows == 0 {
		return 0, fmt.Errorf("no user records recovered")
	}
	return rows, nil
}

// initializeRecoveryCandidate ensures a verified file can also migrate to the installed Wox schema.
func initializeRecoveryCandidate(ctx context.Context, path string) error {
	handle, err := openWoxDatabase(ctx, path)
	if err != nil {
		return err
	}
	conn, err := handle.DB()
	if err != nil {
		return err
	}
	checkErr := CheckDatabase(ctx, conn)
	if err := errors.Join(checkErr, conn.Close()); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	return errors.Join(file.Sync(), file.Close())
}

// salvageDatabase uses the embedded driver to rebuild only tables owned by Wox.
func salvageDatabase(ctx context.Context, src, dst string) (int, error) {
	input, err := openDatabaseFile(src, "rw")
	if err != nil {
		return 0, err
	}
	defer input.Close()
	handle, err := openWoxDatabase(ctx, dst)
	if err != nil {
		return 0, err
	}
	output, err := handle.DB()
	if err != nil {
		return 0, err
	}
	defer output.Close()
	count, userRows := 0, 0
	for _, model := range databaseModels() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		stmt := handle.Model(model).Statement
		if err := stmt.Parse(model); err != nil {
			return 0, err
		}
		copied, err := salvageTable(ctx, input, output, stmt.Schema.Table)
		if err != nil {
			return 0, err
		}
		count += copied
		switch stmt.Schema.Table {
		case "wox_settings", "plugin_settings", "mru_records", "oplogs":
			userRows += copied
		}
	}
	if userRows == 0 {
		return 0, fmt.Errorf("no user records recovered")
	}
	return count, nil
}

// salvageTable inserts readable rows into the current schema, with defaults for newer columns.
func salvageTable(ctx context.Context, src, dst *sql.DB, table string) (int, error) {
	sourceColumns, err := sqliteColumns(ctx, src, table)
	if err != nil {
		return 0, nil
	}
	targetColumns, err := sqliteColumns(ctx, dst, table)
	if err != nil {
		return 0, err
	}
	var columns, vars []string
	for _, name := range sourceColumns {
		if slices.Contains(targetColumns, name) {
			columns = append(columns, sqlIdentifier(name))
			vars = append(vars, "?")
		}
	}
	if len(columns) == 0 {
		return 0, nil
	}
	tx, err := dst.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	save, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO "+sqlIdentifier(table)+" ("+strings.Join(columns, ",")+") VALUES ("+strings.Join(vars, ",")+")")
	if err != nil {
		return 0, err
	}
	defer save.Close()
	query := "SELECT rowid," + strings.Join(columns, ",") + " FROM " + sqlIdentifier(table)
	var lo, hi int64
	bounds := src.QueryRowContext(ctx, "SELECT rowid FROM "+sqlIdentifier(table)+" ORDER BY rowid LIMIT 1").Scan(&lo) == nil &&
		src.QueryRowContext(ctx, "SELECT rowid FROM "+sqlIdentifier(table)+" ORDER BY rowid DESC LIMIT 1").Scan(&hi) == nil
	remainingSplits, count := 512, 0
	var scanRange func(int64, int64, bool) error
	scanRange = func(first, last int64, bounded bool) error {
		var rows *sql.Rows
		var err error
		if bounded {
			rows, err = src.QueryContext(ctx, query+" WHERE rowid BETWEEN ? AND ? ORDER BY rowid", first, last)
		} else {
			rows, err = src.QueryContext(ctx, query)
		}
		if err == nil {
			values := make([]any, len(columns))
			pointers := make([]any, len(values)+1)
			var rowID int64
			pointers[0] = &rowID
			for i := range values {
				pointers[i+1] = &values[i]
			}
			for rows.Next() {
				if err = rows.Scan(pointers...); err != nil {
					break
				}
				result, insertErr := save.ExecContext(ctx, values...)
				if insertErr != nil {
					_ = rows.Close()
					return insertErr
				}
				inserted, _ := result.RowsAffected()
				count += int(inserted)
				if bounded {
					if rowID == last {
						break
					}
					first = rowID + 1
				}
			}
			err = errors.Join(err, rows.Err(), rows.Close())
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil || !isDatabaseCorruption(err) || !bounded || first >= last || remainingSplits == 0 {
			return nil
		}
		// Bisect the unread portion to reach intact rows beyond a damaged leaf page.
		remainingSplits--
		mid := first + int64((uint64(last)-uint64(first))/2)
		if err := scanRange(first, mid, true); err != nil {
			return err
		}
		return scanRange(mid+1, last, true)
	}
	if err := scanRange(lo, hi, bounds); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

func sqliteColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM "+sqlIdentifier(table)+" LIMIT 0")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}

func sqlIdentifier(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

type RecoveryResult struct {
	RecoveredPath string
	BackupPath    string
	Swapped       bool
}

// RecoverDatabase exports an independently recovered copy for Doctor while Wox is running.
func RecoverDatabase(ctx context.Context) (RecoveryResult, error) {
	location := util.GetLocation()
	if err := os.MkdirAll(location.GetBackupDirectory(), 0700); err != nil {
		return RecoveryResult{}, err
	}
	dir, err := os.MkdirTemp(location.GetBackupDirectory(), "db_repair_")
	if err != nil {
		return RecoveryResult{}, err
	}
	path := filepath.Join(dir, "wox.recovered.db")
	_, err = recoverCandidate(ctx, location.GetUserDataDirectory(), dir, path)
	if err != nil {
		return RecoveryResult{}, err
	}
	return RecoveryResult{RecoveredPath: path}, nil
}
