package migration

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"wox/database"

	"gorm.io/gorm"
)

func init() {
	Register(&filesearchSkipHiddenFilesSettingMigration{})
}

type filesearchSkipHiddenFilesSettingMigration struct{}

type filesearchIgnorePatternSetting struct {
	Pattern string `json:"Pattern"`
}

func (m *filesearchSkipHiddenFilesSettingMigration) ID() string {
	return "20260516_filesearch_skip_hidden_files_setting"
}

func (m *filesearchSkipHiddenFilesSettingMigration) Description() string {
	return "Move File Search's broad hidden-file ignore behavior from the editable ignore-pattern list into the dedicated skip-hidden-files setting."
}

func (m *filesearchSkipHiddenFilesSettingMigration) Up(ctx context.Context, tx *gorm.DB) error {
	var existing database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", fileSearchPluginID, "ignorePatterns").First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	var patterns []filesearchIgnorePatternSetting
	if unmarshalErr := json.Unmarshal([]byte(existing.Value), &patterns); unmarshalErr != nil {
		return nil
	}

	filtered := make([]filesearchIgnorePatternSetting, 0, len(patterns))
	changed := false
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern.Pattern) == ".*" {
			// Migration change: `.*` was the old hidden-file switch, but leaving it in
			// the editable table would make the new checkbox appear broken when users
			// turn hidden-file skipping off. Specific dot-folder patterns such as .git
			// stay user-editable and can still be removed manually.
			changed = true
			continue
		}
		filtered = append(filtered, pattern)
	}
	if !changed {
		return nil
	}

	payload, marshalErr := json.Marshal(filtered)
	if marshalErr != nil {
		return marshalErr
	}
	existing.Value = string(payload)
	return tx.Save(&existing).Error
}
