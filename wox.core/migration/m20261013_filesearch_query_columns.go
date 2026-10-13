package migration

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
	"wox/util"
	"wox/util/filesearch/column"
)

func init() { Register(&filesearchQueryColumnsMigration{}) }

type filesearchQueryColumnsMigration struct{}

func (*filesearchQueryColumnsMigration) ID() string { return "20261013_filesearch_query_columns" }
func (*filesearchQueryColumnsMigration) Description() string {
	return "Persist filename query columns so searches borrow file-backed arrays."
}

// Up replaces old derived snapshots before any engine can borrow their mappings.
// Each replacement is atomic and repeatable, including after an interrupted run.
func (*filesearchQueryColumnsMigration) Up(ctx context.Context, _ *gorm.DB) error {
	return upgradeFileSearchQueryColumns(ctx, util.GetLocation().GetFileSearchDirectory())
}

// upgradeFileSearchQueryColumns leaves manifests, journals and content databases intact.
func upgradeFileSearchQueryColumns(ctx context.Context, directory string) error {
	for _, folder := range []string{"columns", "native-columns"} {
		paths, err := filepath.Glob(filepath.Join(directory, folder, "*", "current.json"))
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var manifest struct{ Generation string }
			if err := json.Unmarshal(data, &manifest); err != nil {
				return err
			}
			token, err := hex.DecodeString(manifest.Generation)
			if err != nil || len(token) != 16 {
				return fmt.Errorf("invalid filename generation in %s", path)
			}
			if err := column.UpgradeQueryColumns(filepath.Join(filepath.Dir(path), manifest.Generation+".columns")); err != nil {
				return fmt.Errorf("upgrade filename query columns %s: %w", path, err)
			}
		}
	}
	return nil
}
