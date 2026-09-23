package migration

import (
	"context"
	"fmt"
	"os"
	"wox/util"

	"gorm.io/gorm"
)

func init() { Register(&removePluginLogDirectoriesMigration{}) }

type removePluginLogDirectoriesMigration struct{}

func (m *removePluginLogDirectoriesMigration) ID() string {
	return "20260922_remove_plugin_log_directories"
}

func (m *removePluginLogDirectoriesMigration) Description() string {
	return "Remove legacy per-plugin log directories now that plugins write into the shared Wox log."
}

// Up has no database work; the directory removal runs after commit so a locked log file
// on Windows cannot fail the runner and block the migrations that follow this one.
func (m *removePluginLogDirectoriesMigration) Up(context.Context, *gorm.DB) error {
	return nil
}

// AfterCommit deletes ~/.wox/log/plugins. Every line there was already duplicated in wox.log
// with the plugin name as component, so nothing is lost; the directory is simply no longer
// produced. A failure is only logged by the runner and leaves harmless files behind.
func (m *removePluginLogDirectoriesMigration) AfterCommit(ctx context.Context) error {
	directory := util.GetLocation().GetLogPluginDirectory()
	if !util.IsDirExists(directory) {
		return nil
	}
	if err := os.RemoveAll(directory); err != nil {
		return fmt.Errorf("remove legacy plugin log directory: %w", err)
	}
	util.GetLogger().Info(ctx, "removed legacy per-plugin log directory")
	return nil
}
