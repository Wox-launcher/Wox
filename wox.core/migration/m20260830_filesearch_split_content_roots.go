package migration

import (
	"context"
	"errors"
	"strings"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

func init() {
	Register(&filesearchSplitContentRootsMigration{})
}

type filesearchSplitContentRootsMigration struct{}

func (m *filesearchSplitContentRootsMigration) ID() string {
	return "20260830_filesearch_split_content_roots"
}

func (m *filesearchSplitContentRootsMigration) Description() string {
	return "Copy File Search filename roots into a dedicated content-search directory setting so the two indexes can be managed separately."
}

func (m *filesearchSplitContentRootsMigration) Up(ctx context.Context, tx *gorm.DB) error {
	platform := util.GetCurrentPlatform()
	targetKey := setting.PlatformSettingKey("contentRoots", platform)
	if existing, err := loadFilesearchPluginSetting(tx, fileSearchPluginID, targetKey); err != nil {
		return err
	} else if existing != nil {
		return nil
	}

	for _, sourceKey := range []string{
		setting.PlatformSettingKey("roots", platform),
		"roots",
	} {
		source, err := loadFilesearchPluginSetting(tx, fileSearchPluginID, sourceKey)
		if err != nil {
			return err
		}
		if source == nil || strings.TrimSpace(source.Value) == "" {
			continue
		}
		return tx.Save(&database.PluginSetting{
			PluginID: fileSearchPluginID,
			Key:      targetKey,
			Value:    source.Value,
		}).Error
	}
	return nil
}

func loadFilesearchPluginSetting(tx *gorm.DB, pluginID, key string) (*database.PluginSetting, error) {
	var row database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", pluginID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
