package migration

import (
	"context"
	"wox/setting"

	"gorm.io/gorm"
)

func init() {
	Register(&resetThemeMigration{})
}

type resetThemeMigration struct{}

func (m *resetThemeMigration) ID() string { return "20251219_reset_theme" }

func (m *resetThemeMigration) Description() string {
	return "Reset ThemeId to DefaultThemeId, since we introduce a bug in version v2.0.0-beta6 which causes some users to have invalid theme settings."
}

func (m *resetThemeMigration) Up(ctx context.Context, tx *gorm.DB) error {
	return setting.NewWoxSettingStore(tx).Set("ThemeId", setting.DefaultThemeId)
}
