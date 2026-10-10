package cloudsync

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// oplogVacuumMinFreeBytes is the freelist size that justifies rewriting wox.db.
// DELETE journal mode keeps freed pages in the file until VACUUM.
var oplogVacuumMinFreeBytes int64 = 32 << 20

// StartOplogMaintenance schedules cleanup at 02:00 local time, keeping compaction off startup.
func StartOplogMaintenance(ctx context.Context) {
	util.Go(ctx, "oplog maintenance", func() {
		for {
			timer := time.NewTimer(durationUntilOplogMaintenance(time.Now()))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				MaintainOplogs(ctx)
			}
		}
	})
}

// durationUntilOplogMaintenance follows local calendar days rather than fixed 24-hour intervals.
func durationUntilOplogMaintenance(now time.Time) time.Duration {
	day := now.Day()
	if now.Hour() >= 2 {
		day++
	}
	next := time.Date(now.Year(), now.Month(), day, 2, 0, 0, 0, now.Location())
	// A spring DST jump can normalize the missing 02:00 backwards to 01:00.
	// Run at 03:00 on that day instead of skipping the day's maintenance.
	if next.Hour() < 2 {
		next = time.Date(now.Year(), now.Month(), day, 3, 0, 0, 0, now.Location())
	}
	return next.Sub(now)
}

// MaintainOplogs deletes oplog snapshots that sync no longer needs and rewrites the
// database file when the freed space is large enough to matter.
func MaintainOplogs(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	db := database.GetDB()
	if db == nil {
		return
	}

	started := time.Now()
	var deleted, cleared int64
	var vacuumed bool
	err := database.WithFileMaintenance(func() error {
		if err := WithLocalSyncMutation(func() error {
			var pruneErr error
			deleted, cleared, pruneErr = pruneRetiredOplogs(ctx, db)
			return pruneErr
		}); err != nil {
			return err
		}
		// VACUUM has its own SQLite transaction; waiting for readers must not hold
		// the application-wide setting mutation lock.
		var vacuumErr error
		vacuumed, vacuumErr = vacuumOplogStorage(ctx, db)
		return vacuumErr
	})
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("oplog maintenance failed: %v", err))
		return
	}

	message := fmt.Sprintf("oplog maintenance: deleted=%d cleared=%d vacuumed=%t costMs=%d", deleted, cleared, vacuumed, time.Since(started).Milliseconds())
	if deleted > 0 || cleared > 0 || vacuumed {
		util.GetLogger().Info(ctx, message)
		return
	}
	util.GetLogger().Debug(ctx, message)
}

// pruneRetiredOplogs drops historical snapshots while preserving sync correctness.
// Each identity keeps its newest row, so an older pending write cannot become
// uploadable, and its newest non-discarded row, whose timestamp is the local
// last-write-wins baseline. Superseded pending rows never upload and can be
// removed too. Discarded tombstones do not need their payload.
func pruneRetiredOplogs(ctx context.Context, db *gorm.DB) (int64, int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	var retiredIDs []uint
	if err := db.WithContext(ctx).Raw(`
		SELECT id FROM (
			SELECT id,
			       cloud_sync_discarded,
			       ROW_NUMBER() OVER (
			           PARTITION BY entity_type, entity_id, key
			           ORDER BY timestamp DESC, id DESC
			       ) AS latest_rn,
			       ROW_NUMBER() OVER (
			           PARTITION BY entity_type, entity_id, key
			           ORDER BY cloud_sync_discarded ASC, timestamp DESC, id DESC
			       ) AS live_rn
			FROM oplogs
		) AS ranked
		WHERE latest_rn != 1
		  AND NOT (cloud_sync_discarded = 0 AND live_rn = 1)
	`).Scan(&retiredIDs).Error; err != nil {
		return 0, 0, fmt.Errorf("list retired oplogs: %w", err)
	}

	var deleted int64
	batchSize := oplogIDBatchSize
	if batchSize < 1 {
		batchSize = 1
	}
	for start := 0; start < len(retiredIDs); start += batchSize {
		if err := ctx.Err(); err != nil {
			return deleted, 0, err
		}
		end := start + batchSize
		if end > len(retiredIDs) {
			end = len(retiredIDs)
		}
		result := db.WithContext(ctx).Where("id IN ?", retiredIDs[start:end]).Delete(&database.Oplog{})
		if result.Error != nil {
			return deleted, 0, fmt.Errorf("delete retired oplogs: %w", result.Error)
		}
		deleted += result.RowsAffected
	}

	cleared := db.WithContext(ctx).Exec(`UPDATE oplogs SET value = '' WHERE cloud_sync_discarded = 1 AND value <> ''`)
	if cleared.Error != nil {
		return deleted, 0, fmt.Errorf("clear discarded oplog values: %w", cleared.Error)
	}
	return deleted, cleared.RowsAffected, nil
}

// vacuumOplogStorage rewrites wox.db once the freelist is large enough that the
// file size itself is the problem. Auto-vacuum is off, so DELETE leaves those pages allocated.
func vacuumOplogStorage(ctx context.Context, db *gorm.DB) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return false, err
	}

	freeBytes, err := sqliteFreeBytes(ctx, sqlDB)
	if err != nil {
		return false, err
	}
	if freeBytes < oplogVacuumMinFreeBytes {
		return false, nil
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("compacting wox.db: freeBytes=%d", freeBytes))

	// Keep the pool's normal busy timeout so maintenance can retry the next night
	// instead of waiting minutes or changing later queries' lock policy.
	if _, err := sqlDB.ExecContext(ctx, "VACUUM"); err != nil {
		return false, fmt.Errorf("vacuum database: %w", err)
	}
	return true, nil
}

// sqliteFreeBytes measures whole free pages that VACUUM can return to the filesystem.
func sqliteFreeBytes(ctx context.Context, sqlDB *sql.DB) (int64, error) {
	var pageSize, freelist int64
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA page_size").Scan(&pageSize); err != nil {
		return 0, fmt.Errorf("read page size: %w", err)
	}
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freelist); err != nil {
		return 0, fmt.Errorf("read freelist: %w", err)
	}
	if pageSize <= 0 || freelist <= 0 {
		return 0, nil
	}
	return pageSize * freelist, nil
}
