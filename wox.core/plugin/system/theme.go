package system

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/shell"

	"github.com/samber/lo"
)

var themeIcon = icons.Get(icons.PluginTheme)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ThemePlugin{})
}

type ThemePlugin struct {
	api plugin.API
}

func (c *ThemePlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "58a59382-8b3a-48c2-89ac-0a9a0e12e03f",
		Name:          "i18n:plugin_theme_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_theme_plugin_description",
		Icon:          themeIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"theme",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "restore",
				Description: "i18n:plugin_theme_restore_command_description",
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
				Params: map[string]any{
					"HashBy": "scoreKey",
				},
			},
		},
	}
}

func (c *ThemePlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.api.OnMRURestore(ctx, c.handleMRURestore)
}

func (c *ThemePlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Command == "restore" {
		return plugin.NewQueryResponse(c.queryRestore(ctx, query))
	}

	uiManager := plugin.GetPluginManager().GetUI()
	installedThemes := uiManager.GetAllThemes(ctx)
	storeThemes := ui.GetStoreManager().GetThemes()
	iconCatalog := append(append([]common.Theme{}, installedThemes...), storeThemes...)
	changeThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_change_theme")
	uninstallThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_uninstall_theme")
	currentGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_current")
	systemGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_system")
	availableGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_available")
	storeGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_store")
	installThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_install_theme")
	systemTagText := i18n.GetI18nManager().TranslateWox(ctx, "ui_setting_theme_system_tag")
	openThemeFolderText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_open_containing_folder")

	results := lo.FilterMap(installedThemes, func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		match, _ := plugin.IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			themePath := filepath.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
			result := plugin.QueryResult{
				Title:    theme.ThemeName,
				SubTitle: theme.Description,
				Icon:     themeResultIcon(theme, iconCatalog),
				ScoreKey: theme.ThemeId,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   changeThemeText,
						Icon:                   icons.Get(icons.ActionRun),
						PreventHideAfterAction: true,
						ContextData:            themeMRUContext(theme.ThemeId),
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							uiManager.ChangeTheme(ctx, theme)
						},
					},
				},
			}
			if theme.IsSystem {
				result.Tails = append(result.Tails, plugin.QueryResultTail{
					Type: plugin.QueryResultTailTypeText,
					Text: systemTagText,
				})
			} else {
				result.Actions = append(result.Actions, plugin.QueryResultAction{
					Name: openThemeFolderText,
					Icon: icons.Get(icons.ActionOpenContainingFolder),
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if err := shell.OpenFileInFolder(themePath); err != nil {
							c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open theme folder %s: %s", themePath, err.Error()))
						}
					},
				})
				result.Actions = append(result.Actions, plugin.QueryResultAction{
					Name:                   uninstallThemeText,
					Icon:                   icons.Get(icons.ActionDelete),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						uiManager.UninstallTheme(ctx, theme)
						c.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s ", query.TriggerKeyword),
						})
					},
				})
			}
			currentThemeId := setting.GetSettingManager().GetWoxSetting(ctx).ThemeId.Get()
			result.Group, result.GroupScore = installedThemeListGroup(currentThemeId == theme.ThemeId, theme.IsSystem, currentGroup, systemGroup, availableGroup)

			return result, true
		} else {
			return plugin.QueryResult{}, false
		}
	})

	installedThemeIds := lo.Map(installedThemes, func(t common.Theme, _ int) string { return t.ThemeId })

	storeResults := lo.FilterMap(storeThemes, func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		// Skip if already installed
		if lo.Contains(installedThemeIds, theme.ThemeId) {
			return plugin.QueryResult{}, false
		}

		match, _ := plugin.IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			result := plugin.QueryResult{
				Title:      theme.ThemeName,
				SubTitle:   theme.Description,
				Icon:       themeResultIcon(theme, iconCatalog),
				Group:      storeGroup,
				GroupScore: 0,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   installThemeText,
						Icon:                   icons.Get(icons.ActionInstall),
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							uiManager.InstallTheme(ctx, theme)
						},
					},
				},
			}
			return result, true
		}
		return plugin.QueryResult{}, false
	})

	return plugin.NewQueryResponse(append(results, storeResults...))
}

func themeMRUContext(themeID string) common.ContextData {
	return common.ContextData{"themeId": themeID}
}

// handleMRURestore rebuilds an installed theme result; uninstalled themes are skipped.
func (c *ThemePlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	themeID := strings.TrimSpace(mruData.ContextData["themeId"])
	if themeID == "" {
		return nil, fmt.Errorf("empty theme id in context data")
	}

	uiManager := plugin.GetPluginManager().GetUI()
	installedThemes := uiManager.GetAllThemes(ctx)
	var found *common.Theme
	for i := range installedThemes {
		if installedThemes[i].ThemeId == themeID {
			found = &installedThemes[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("theme is no longer installed: %s", themeID)
	}

	changeThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_change_theme")
	result := plugin.QueryResult{
		Title:    found.ThemeName,
		SubTitle: found.Description,
		Icon:     themeResultIcon(*found, installedThemes),
		ScoreKey: found.ThemeId,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   changeThemeText,
				Icon:                   icons.Get(icons.ActionRun),
				PreventHideAfterAction: true,
				ContextData:            themeMRUContext(found.ThemeId),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					uiManager.ChangeTheme(ctx, *found)
				},
			},
		},
	}
	return &result, nil
}

func (c *ThemePlugin) queryRestore(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return []plugin.QueryResult{
		{
			Title: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_restore_title"),
			Icon:  themeIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_restore_action"),
					Icon:                   icons.Get(icons.ActionUpdate),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetPluginManager().GetUI().RestoreTheme(ctx)
					},
				},
			},
		},
	}
}

// themeResultIcon reuses the Settings catalog swatch, including AUTO's split variants.
func themeResultIcon(theme common.Theme, catalog []common.Theme) common.WoxImage {
	if !theme.IsAutoAppearance {
		return common.NewWoxImageTheme(theme)
	}
	return common.NewWoxImageThemeAuto(
		themeSwatchVariant(catalog, theme.LightThemeId, true),
		themeSwatchVariant(catalog, theme.DarkThemeId, false),
	)
}

func themeSwatchVariant(catalog []common.Theme, id string, light bool) common.Theme {
	for _, theme := range catalog {
		if theme.ThemeId == id {
			return theme
		}
	}
	if light {
		return common.Theme{
			AppBackgroundColor:              "#F5F5F5",
			QueryBoxBackgroundColor:         "#E8E8E8",
			ResultItemActiveBackgroundColor: "#D8D8D8",
		}
	}
	return common.Theme{
		AppBackgroundColor:              "#2B2B2B",
		QueryBoxBackgroundColor:         "#3D3D3D",
		ResultItemActiveBackgroundColor: "#4A4A4A",
	}
}

// installedThemeListGroup places the active theme first, then remaining system
// themes, then user-installed themes. Higher GroupScore sorts a group higher.
func installedThemeListGroup(isCurrent, isSystem bool, currentGroup, systemGroup, availableGroup string) (string, int64) {
	if isCurrent {
		return currentGroup, 100
	}
	if isSystem {
		return systemGroup, 75
	}
	return availableGroup, 50
}
