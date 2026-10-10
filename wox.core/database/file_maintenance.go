package database

import "sync"

var fileMaintenanceMu sync.Mutex

// WithFileMaintenance serializes raw database file copies and maintenance writes.
// Filesystem copies do not participate in SQLite locks and must not overlap compaction.
func WithFileMaintenance(fn func() error) error {
	fileMaintenanceMu.Lock()
	defer fileMaintenanceMu.Unlock()
	return fn()
}
