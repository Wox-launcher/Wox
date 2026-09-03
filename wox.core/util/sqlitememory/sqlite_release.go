package sqlitememory

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
	"wox/util"
)

var (
	registryMu sync.Mutex
	registry   = map[*sql.DB]string{}
)

// Register tracks an open SQLite handle so hide-path cleanup can ask every
// pooled connection to release unused page-cache pages.
func Register(db *sql.DB, name string) {
	if db == nil {
		return
	}
	registryMu.Lock()
	registry[db] = name
	registryMu.Unlock()
}

// Unregister drops a closed SQLite handle so hide-path cleanup cannot reuse it.
func Unregister(db *sql.DB) {
	if db == nil {
		return
	}
	registryMu.Lock()
	delete(registry, db)
	registryMu.Unlock()
}

// ReleaseIdleMemory asks every registered connection to give unused SQLite page
// cache back to the heap. cache_size otherwise stays resident until the process
// exits, including after the launcher hides.
func ReleaseIdleMemory(ctx context.Context) {
	registryMu.Lock()
	snapshot := make([]registeredDB, 0, len(registry))
	for db, name := range registry {
		snapshot = append(snapshot, registeredDB{db: db, name: name})
	}
	registryMu.Unlock()
	if len(snapshot) == 0 {
		return
	}

	before := UsedBytes()
	shrinkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, entry := range snapshot {
		if err := shrinkDatabase(shrinkCtx, entry.db); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to shrink sqlite memory (%s): %s", entry.name, err.Error()))
		}
	}
	after := UsedBytes()
	if before != after {
		util.GetLogger().Info(ctx, fmt.Sprintf("released idle sqlite memory: %s -> %s", formatReleaseBytes(before), formatReleaseBytes(after)))
	}
}

type registeredDB struct {
	db   *sql.DB
	name string
}

// shrinkDatabase walks every pooled connection so PRAGMA shrink_memory is not
// applied to only the first idle handle database/sql happens to hand out.
func shrinkDatabase(ctx context.Context, db *sql.DB) error {
	open := db.Stats().OpenConnections
	if open == 0 {
		return nil
	}

	conns := make([]*sql.Conn, 0, open)
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()

	for i := 0; i < open; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			break
		}
		if _, err := conn.ExecContext(ctx, "PRAGMA shrink_memory"); err != nil {
			_ = conn.Close()
			return err
		}
		conns = append(conns, conn)
	}
	return nil
}

func formatReleaseBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KB", "MB", "GB"} {
		value /= unit
		if value < unit || suffix == "GB" {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%d B", bytes)
}
