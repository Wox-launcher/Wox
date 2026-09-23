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
	storeManifests := ui.GetStoreManager().GetThemeManifests()
	iconCatalog := append([]common.Theme{}, installedThemes...)
	changeThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_change_theme")
	uninstallThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_uninstall_theme")
	currentGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_current")
	systemGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_system")
	availableGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_available")
	storeGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_store")
	installThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_install_theme")
	systemTagText := i18n.GetI18nManager().TranslateWox(ctx, "ui_setting_theme_system_tag")
	openThemeFolderText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_open_containing_folder")
	imageThemeHint := ""
	imageThemeIDs := map[string]bool{}
	for _, manifest := range storeManifests {
		if manifest.ImageTheme {
			imageThemeIDs[manifest.Id] = true
		}
	}
	if len(imageThemeIDs) > 0 {
		imageThemeHint = i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_image_memory_hint")
	}

	results := lo.FilterMap(installedThemes, func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		match := plugin.IsStringMatch(ctx, theme.GetName(ctx), query.Search) || plugin.IsStringMatch(ctx, theme.GetNameEnUs(), query.Search)
		if match {
			themeID := theme.ThemeId
			themePath := filepath.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
			packageManifest := filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId, "theme.json")
			if util.IsFileExists(packageManifest) {
				themePath = packageManifest
			}
			result := plugin.QueryResult{
				Title:    theme.GetName(ctx),
				SubTitle: themeStoreSubtitle(theme.GetDescription(ctx), imageThemeIDs[theme.ThemeId], imageThemeHint),
				Icon:     themeResultIcon(theme, iconCatalog),
				ScoreKey: theme.ThemeId,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   changeThemeText,
						Icon:                   icons.Get(icons.ActionRun),
						PreventHideAfterAction: true,
						ContextData:            themeMRUContext(theme.ThemeId),
						Action:                 changeThemeByID(uiManager, theme.ThemeId),
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
						if installed, ok := findInstalledTheme(uiManager.GetAllThemes(ctx), themeID); ok {
							uiManager.UninstallTheme(ctx, installed)
						}
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

	storeResults := lo.FilterMap(storeManifests, func(manifest common.StoreThemeManifest, _ int) (plugin.QueryResult, bool) {
		if lo.Contains(installedThemeIds, manifest.Id) {
			return plugin.QueryResult{}, false
		}

		match := plugin.IsStringMatch(ctx, manifest.GetName(ctx), query.Search) || plugin.IsStringMatch(ctx, manifest.GetNameEnUs(), query.Search)
		if !match {
			return plugin.QueryResult{}, false
		}
		icon := themeIcon
		if manifest.IconColors.HasColors() {
			icon = manifest.IconColors.SwatchImage()
		}
		return plugin.QueryResult{
			Title:      manifest.GetName(ctx),
			SubTitle:   themeStoreSubtitle(manifest.GetDescription(ctx), manifest.ImageTheme, imageThemeHint),
			Icon:       icon,
			Group:      storeGroup,
			GroupScore: 0,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   installThemeText,
					Icon:                   icons.Get(icons.ActionInstall),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if err := ui.GetStoreManager().InstallManifest(ctx, manifest); err != nil {
							c.api.Notify(ctx, err.Error())
						}
					},
				},
			},
		}, true
	})

	return plugin.NewQueryResponse(append(results, storeResults...))
}

// themeStoreSubtitle keeps the theme description and adds the image-theme memory note.
func themeStoreSubtitle(description string, imageTheme bool, hint string) string {
	if !imageTheme || hint == "" {
		return description
	}
	if description == "" {
		return hint
	}
	return description + " · " + hint
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
	found, ok := findInstalledTheme(installedThemes, themeID)
	if !ok {
		return nil, fmt.Errorf("theme is no longer installed: %s", themeID)
	}

	changeThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_change_theme")
	result := plugin.QueryResult{
		Title:    found.GetName(ctx),
		SubTitle: found.GetDescription(ctx),
		Icon:     themeResultIcon(found, installedThemes),
		ScoreKey: found.ThemeId,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   changeThemeText,
				Icon:                   icons.Get(icons.ActionRun),
				PreventHideAfterAction: true,
				ContextData:            themeMRUContext(found.ThemeId),
				Action:                 changeThemeByID(uiManager, themeID),
			},
		},
	}
	return &result, nil
}

// findInstalledTheme returns the theme value for id so callers do not hold pointers into the catalog slice.
func findInstalledTheme(themes []common.Theme, id string) (common.Theme, bool) {
	for _, theme := range themes {
		if theme.ThemeId == id {
			return theme, true
		}
	}
	return common.Theme{}, false
}

// changeThemeByID resolves the theme when the action runs. Result actions live in the query and
// MRU caches, so capturing the whole Theme value (or the entire installed catalog) kept every
// listed theme resident long after the query finished.
func changeThemeByID(uiManager common.UI, themeID string) func(context.Context, plugin.ActionContext) {
	return func(ctx context.Context, _ plugin.ActionContext) {
		theme, ok := findInstalledTheme(uiManager.GetAllThemes(ctx), themeID)
		if !ok {
			util.GetLogger().Warn(ctx, fmt.Sprintf("theme is no longer installed: %s", themeID))
			return
		}
		uiManager.ChangeTheme(ctx, theme)
	}
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
