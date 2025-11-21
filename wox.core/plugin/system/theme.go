package system

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/setting/definition"
	"wox/ui"
	"wox/util"
	"wox/util/shell"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

var themeIcon = plugin.PluginThemeIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ThemePlugin{})
}

type ThemePlugin struct {
	api plugin.API
}

func (c *ThemePlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "58a59382-8b3a-48c2-89ac-0a9a0e12e03f",
		Name:          "Theme manager",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Theme manager",
		Icon:          themeIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"theme",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     "ai",
				Description: "i18n:plugin_theme_ai_command_description",
			},
			{
				Command:     "restore",
				Description: "i18n:plugin_theme_restore_command_description",
			},
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:     "model",
					Label:   "i18n:plugin_theme_setting_ai_model_label",
					Tooltip: `i18n:plugin_theme_setting_ai_model_tooltip`,
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureAI,
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (c *ThemePlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
}

func (c *ThemePlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Command == "ai" {
		return c.queryAI(ctx, query)
	}
	if query.Command == "restore" {
		return c.queryRestore(ctx, query)
	}

	uiManager := plugin.GetPluginManager().GetUI()
	installedThemes := uiManager.GetAllThemes(ctx)
	changeThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_change_theme")
	uninstallThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_uninstall_theme")
	currentGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_current")
	availableGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_available")
	storeGroup := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_group_store")
	storeSubTitleFormat := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_store_install_by")
	installThemeText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_install_theme")
	systemTagText := i18n.GetI18nManager().TranslateWox(ctx, "ui_setting_theme_system_tag")
	openThemeFolderText := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_open_containing_folder")

	results := lo.FilterMap(installedThemes, func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		match, _ := IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			themePath := filepath.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
			result := plugin.QueryResult{
				Title: theme.ThemeName,
				Icon:  common.NewWoxImageTheme(theme),
				Actions: []plugin.QueryResultAction{
					{
						Name:                   changeThemeText,
						PreventHideAfterAction: true,
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
					Icon: plugin.OpenContainingFolderIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						if err := shell.OpenFileInFolder(themePath); err != nil {
							c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to open theme folder %s: %s", themePath, err.Error()))
						}
					},
				})
				result.Actions = append(result.Actions, plugin.QueryResultAction{
					Name:                   uninstallThemeText,
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
			if currentThemeId == theme.ThemeId {
				result.Group = currentGroup
				result.GroupScore = 100
			} else {
				result.Group = availableGroup
				result.GroupScore = 50
			}

			return result, true
		} else {
			return plugin.QueryResult{}, false
		}
	})

	// Add store themes
	storeThemes := ui.GetStoreManager().GetThemes()
	installedThemeIds := lo.Map(installedThemes, func(t common.Theme, _ int) string { return t.ThemeId })

	storeResults := lo.FilterMap(storeThemes, func(theme common.Theme, _ int) (plugin.QueryResult, bool) {
		// Skip if already installed
		if lo.Contains(installedThemeIds, theme.ThemeId) {
			return plugin.QueryResult{}, false
		}

		match, _ := IsStringMatchScore(ctx, theme.ThemeName, query.Search)
		if match {
			result := plugin.QueryResult{
				Title: theme.ThemeName,
				SubTitle: fmt.Sprintf(
					storeSubTitleFormat,
					theme.ThemeName,
					theme.ThemeAuthor,
				),
				Icon:       common.NewWoxImageTheme(theme),
				Group:      storeGroup,
				GroupScore: 0,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   installThemeText,
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

	return append(results, storeResults...)
}

func (c *ThemePlugin) queryAI(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	modelStr := c.api.GetSetting(ctx, "model")
	if modelStr == "" {
		return []plugin.QueryResult{
			{
				Title: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_select_model"),
				Icon:  themeIcon,
				Actions: []plugin.QueryResultAction{
					{
						Name:                   i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_open_setting"),
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{
								Path:  "/plugin/setting",
								Param: c.GetMetadata().Name,
							})
						},
					},
				},
			},
		}
	}
	var aiModel common.Model
	unmarshalErr := json.Unmarshal([]byte(modelStr), &aiModel)
	if unmarshalErr != nil {
		c.api.Notify(ctx, unmarshalErr.Error())
		return []plugin.QueryResult{
			{
				Title:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_unmarshal_failed"),
				SubTitle: unmarshalErr.Error(),
				Icon:     themeIcon,
			},
		}
	}

	if query.Search == "" {
		return []plugin.QueryResult{
			{
				Title: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_input_hint"),
				Icon:  themeIcon,
			},
		}
	}

	embedThemes := resource.GetEmbedThemes(ctx)
	if len(embedThemes) == 0 {
		return []plugin.QueryResult{
			{
				Title: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_no_embed_theme"),
				Icon:  themeIcon,
			},
		}
	}

	exampleThemeJson := embedThemes[0]

	var conversations []common.Conversation
	conversations = append(conversations, common.Conversation{
		Role: common.ConversationRoleUser,
		Text: fmt.Sprintf(`
I am developing theme configuration functionality for the Wox application. Theme configurations are defined in JSON format and contain visual elements such as colors, fonts, spacing, and so on.

Refer to the example format:
%s

Please generate a new theme configuration based on the above JSON structure, theme requirements: %s

Generation rules:
1. **Output format**: must return a valid JSON object, starting with { and ending with }, without any explanatory text, comments, or block tags.
2. **Theme naming**: choose a meaningful and descriptive name for the theme that reflects the visual character or style of the theme
3. **Color contrast**: ensure good visual readability
   - There must be sufficient contrast between the background color and the foreground text color.
   - The colors of the selected and unselected states should be clearly differentiated.
   - Avoid using similar color values to ensure that users can clearly distinguish between different UI states.
4. **Completeness**: Include all required fields and attributes in the example.
5. **Consistency**: the color scheme should be coordinated and consistent with the overall design style.

Please directly output the JSON configuration, do not add any other content.
	`, exampleThemeJson, query.Search)})

	result := plugin.QueryResult{
		Id:       uuid.NewString(),
		Title:    i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_generate_title"),
		SubTitle: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_generate_subtitle"),
		Icon:     themeIcon,
		Preview:  plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: ""},
		Actions: []plugin.QueryResultAction{
			{
				Name:                   i18n.GetI18nManager().TranslateWox(ctx, "ui_setting_theme_apply"),
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					util.Go(ctx, "theme ai stream", func() {
						// Show preparing state
						if updatable := c.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil {
							subTitle := "i18n:plugin_theme_ai_contacting"
							previewData := i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_waiting")
							preview := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: previewData}
							updatable.SubTitle = &subTitle
							updatable.Preview = &preview
							if !c.api.UpdateResult(ctx, *updatable) {
								return
							}
						}

						// Start streaming
						err := c.api.AIChatStream(ctx, aiModel, conversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
							updatable := c.api.GetUpdatableResult(ctx, actionContext.ResultId)
							if updatable == nil {
								return
							}

							switch streamResult.Status {
							case common.ChatStreamStatusStreaming:
								subTitle := "i18n:plugin_theme_ai_generating"
								preview := plugin.WoxPreview{
									PreviewType:    plugin.WoxPreviewTypeMarkdown,
									PreviewData:    streamResult.Data,
									ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
								}
								updatable.SubTitle = &subTitle
								updatable.Preview = &preview
								c.api.UpdateResult(ctx, *updatable)

							case common.ChatStreamStatusFinished:
								subTitle := "i18n:plugin_theme_ai_generated"
								preview := plugin.WoxPreview{
									PreviewType:    plugin.WoxPreviewTypeMarkdown,
									PreviewData:    streamResult.Data,
									ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
								}
								updatable.SubTitle = &subTitle
								updatable.Preview = &preview
								c.api.UpdateResult(ctx, *updatable)

								// Extract and install theme
								themeJson := streamResult.Data
								util.Go(ctx, "theme generated", func() {
									group := util.FindRegexGroup(`(?ms){(?P<json>.*?)}`, themeJson)
									if len(group) == 0 {
										c.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_ai_extract_failed"))
										return
									}

									jsonTheme := fmt.Sprintf("{%s}", group["json"])
									var theme common.Theme
									unmarshalErr := json.Unmarshal([]byte(jsonTheme), &theme)
									if unmarshalErr != nil {
										c.api.Notify(ctx, unmarshalErr.Error())
										return
									}

									theme.ThemeId = uuid.NewString()
									theme.ThemeAuthor = "Wox launcher AI"
									theme.ThemeUrl = "https://www.github.com/wox-launcher/wox"
									theme.Version = "1.0.0"
									theme.IsSystem = false
									plugin.GetPluginManager().GetUI().InstallTheme(ctx, theme)
								})

							case common.ChatStreamStatusError:
								if updatable.Preview != nil {
									previewData := updatable.Preview.PreviewData + fmt.Sprintf("\n\nError: %s", streamResult.Data)
									preview := *updatable.Preview
									preview.PreviewData = previewData
									updatable.Preview = &preview
									c.api.UpdateResult(ctx, *updatable)
								}
							}
						})

						if err != nil {
							if updatable := c.api.GetUpdatableResult(ctx, actionContext.ResultId); updatable != nil && updatable.Preview != nil {
								previewData := updatable.Preview.PreviewData + fmt.Sprintf("\n\nError: %s", err.Error())
								preview := *updatable.Preview
								preview.PreviewData = previewData
								updatable.Preview = &preview
								c.api.UpdateResult(ctx, *updatable)
							}
						}
					})
				},
			},
		},
	}
	return []plugin.QueryResult{result}
}

func (c *ThemePlugin) queryRestore(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	return []plugin.QueryResult{
		{
			Title: i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_restore_title"),
			Icon:  themeIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_restore_action"),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						plugin.GetPluginManager().GetUI().RestoreTheme(ctx)
					},
				},
			},
		},
	}
}
