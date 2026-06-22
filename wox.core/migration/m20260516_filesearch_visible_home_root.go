package migration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

const fileSearchPluginID = "979d6363-025a-4f51-88d3-0b04e9dc56bf"

func init() {
	Register(&filesearchVisibleHomeRootMigration{})
}

type filesearchVisibleHomeRootMigration struct{}

type filesearchRootSetting struct {
	Path string `json:"Path"`
}

func (m *filesearchVisibleHomeRootMigration) ID() string {
	return "20260516_filesearch_visible_home_root"
}

func (m *filesearchVisibleHomeRootMigration) Description() string {
	return "Backfill File Search roots with a visible expanded home root so the engine no longer depends on hidden default roots."
}

func (m *filesearchVisibleHomeRootMigration) Up(ctx context.Context, tx *gorm.DB) error {
	// Test runs set explicit roots and must not inherit the developer machine's
	// home directory. Production users get the visible home root either from this
	// migration or from the plugin metadata default on a fresh install.
	if util.IsTestMode() {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return nil
	}
	homeDir = filepath.Clean(homeDir)

	var existing database.PluginSetting
	err = tx.Where("plugin_id = ? AND key = ?", fileSearchPluginID, "roots").First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	roots := make([]filesearchRootSetting, 0)
	if err == nil && strings.TrimSpace(existing.Value) != "" {
		if unmarshalErr := json.Unmarshal([]byte(existing.Value), &roots); unmarshalErr != nil {
			// Migration hardening: a malformed roots setting should not block app
			// startup forever. Treat it as empty and restore the visible home root,
			// matching the plugin's new default behavior.
			roots = roots[:0]
		}
	}

	normalized := make([]filesearchRootSetting, 0, len(roots)+1)
	seen := map[string]struct{}{}
	hasHome := false
	for _, root := range roots {
		expanded := expandFilesearchMigrationRootPath(root.Path, homeDir)
		if expanded == "" {
			continue
		}
		if _, exists := seen[expanded]; exists {
			continue
		}
		seen[expanded] = struct{}{}
		if expanded == homeDir {
			hasHome = true
		}
		normalized = append(normalized, filesearchRootSetting{Path: expanded})
	}
	if !hasHome {
		normalized = append([]filesearchRootSetting{{Path: homeDir}}, normalized...)
	}

	payload, marshalErr := json.Marshal(normalized)
	if marshalErr != nil {
		return marshalErr
	}
	return tx.Save(&database.PluginSetting{
		PluginID: fileSearchPluginID,
		Key:      "roots",
		Value:    string(payload),
	}).Error
}

func expandFilesearchMigrationRootPath(rawPath string, homeDir string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return ""
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		// Migration change: older settings may contain a literal ~. Store the
		// expanded absolute path so the Settings table shows exactly what the engine
		// indexes and future duplicate checks do not depend on shell-style syntax.
		return filepath.Clean(filepath.Join(homeDir, strings.TrimPrefix(path, "~/")))
	}
	return filepath.Clean(path)
}
