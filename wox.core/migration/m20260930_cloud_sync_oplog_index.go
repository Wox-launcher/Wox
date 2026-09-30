package migration

import (
	"context"

	"gorm.io/gorm"
)

func init() { Register(&cloudSyncOplogIndexMigration{}) }

type cloudSyncOplogIndexMigration struct{}

func (m *cloudSyncOplogIndexMigration) ID() string { return "20260930_cloud_sync_oplog_index" }

func (m *cloudSyncOplogIndexMigration) Description() string {
	return "Index oplog identities and versions for cloud sync pending queries."
}

// Up lets latest-state ranking read a covering index instead of scanning large
// historical values and sorting every oplog on each sync status query.
func (m *cloudSyncOplogIndexMigration) Up(ctx context.Context, tx *gorm.DB) error {
	return tx.WithContext(ctx).Exec(`CREATE INDEX IF NOT EXISTS idx_oplogs_cloud_sync_latest
		ON oplogs (entity_type, entity_id, key, timestamp DESC, id DESC)`).Error
}
