package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
	"wox/util"
)

func init() { Register(&filesearchColumnStoreMigration{}) }

type filesearchColumnStoreMigration struct{}

func (*filesearchColumnStoreMigration) ID() string { return "20261013_filesearch_column_store" }

func (*filesearchColumnStoreMigration) Description() string {
	return "Retire the derived SQLite filename index before rebuilding the common column store."
}

// Up removes only the old derived filename cache. Root configuration remains in
// plugin settings, content lives in its own database, and the replacement uses
// directory.db. Running before plugin startup makes deletion safe and repeatable
// even if the application exits before its migration record commits.
func (*filesearchColumnStoreMigration) Up(ctx context.Context, _ *gorm.DB) error {
	return retireFilenameCache(ctx, util.GetLocation().GetFileSearchDirectory())
}

// retireFilenameCache deliberately names each legacy file so retries cannot
// erase a replacement generation or independently configured content index.
func retireFilenameCache(ctx context.Context, directory string) error {
	for _, name := range []string{"filesearch.db", "filesearch.db-wal", "filesearch.db-shm"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(directory, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("retire %s: %w", name, err)
		}
	}
	return nil
}
