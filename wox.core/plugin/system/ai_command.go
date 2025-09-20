package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/selection"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiCommandIcon = plugin.PluginAICommandIcon

type commandSetting struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Vision  bool   `json:"vision"` // does the command interact with vision
}

func (c *commandSetting) AIModel() (model common.Model) {
	err := json.Unmarshal([]byte(c.Model), &model)
	if err != nil {
		return common.Model{}
	}

	return model
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &Plugin{})
}

type Plugin struct {
	api plugin.API
}

func (c *Plugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c9910664-1c28-47ae-bad6-e7332a02d471",
		Name:          "AI Commands",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_ai_command_description",
		Icon:          aiCommandIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"ai",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "commands",
					Title:   "i18n:plugin_ai_command_commands",
					Tooltip: "i18n:plugin_ai_command_commands_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_command_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_command_name_tooltip",
						},
						{
							Key:     "command",
							Label:   "i18n:plugin_ai_command_command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_ai_command_command_tooltip",
						},
						{
							Key:     "model",
							Label:   "i18n:plugin_ai_command_model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   100,
							Tooltip: "i18n:plugin_ai_command_model_tooltip",
						},
						{
							Key:          "prompt",
							Label:        "i18n:plugin_ai_command_prompt",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Tooltip:      "i18n:plugin_ai_command_prompt_tooltip",
						},
						{
							Key:     "vision",
							Label:   "i18n:plugin_ai_command_vision",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
							Tooltip: "i18n:plugin_ai_command_vision_tooltip",
						},
					},
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureQuerySelection,
			},
			{
				Name: plugin.MetadataFeatureAI,
			},
		},
	}
}

func (c *Plugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	c.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == "commands" {
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("ai command setting changed: %s", value))
			var commands []plugin.MetadataCommand
			gjson.Parse(value).ForEach(func(_, command gjson.Result) bool {
				commands = append(commands, plugin.MetadataCommand{
					Command:     command.Get("command").String(),
					Description: command.Get("name").String(),
				})

				return true
			})
			c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("registering query commands: %v", commands))
			c.api.RegisterQueryCommands(ctx, commands)
		}
	})
}

func (c *Plugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Type == plugin.QueryTypeSelection {
		return c.querySelection(ctx, query)
	}

	if query.Command == "" {
		return c.listAllCommands(ctx, query)
	}

	return c.queryCommand(ctx, query)
}

func (c *Plugin) querySelection(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{}
	}

	var results []plugin.QueryResult
	for _, command := range commands {
		if query.Selection.Type == selection.SelectionTypeFile {
			if !command.Vision {
				continue
			}
		}
		if query.Selection.Type == selection.SelectionTypeText {
			if command.Vision {
				continue
			}
		}

		var startAnsweringTime int64
		onPreparing := func(current plugin.RefreshableResult) plugin.RefreshableResult {
			current.Preview.PreviewData = i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_answering")
			current.SubTitle = "i18n:plugin_ai_command_answering"
			startAnsweringTime = util.GetSystemTimestamp()
			return current
		}

		isFirstAnswer := true
		onAnswering := func(current plugin.RefreshableResult, deltaAnswer string, isFinished bool) plugin.RefreshableResult {
			if isFirstAnswer {
				current.Preview.PreviewData = ""
				current.ContextData = ""
				isFirstAnswer = false
			}

			current.SubTitle = "i18n:plugin_ai_command_answering"
			current.ContextData = deltaAnswer

			// Process thinking tags to convert them to markdown quote format
			thinking, content := processAIThinking(current.ContextData)
			current.Preview.PreviewData = convertAIThinkingToMarkdown(thinking, content)
			current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom

			if isFinished {
				current.RefreshInterval = 0
				current.SubTitle = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_answered_cost"), util.GetSystemTimestamp()-startAnsweringTime)
				current.Actions = []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_ai_command_copy",
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							clipboard.WriteText(content)
						},
					},
				}

				// paste to active window
				pasteToActiveWindowAction, pasteToActiveWindowErr := GetPasteToActiveWindowAction(ctx, c.api, func() {
					clipboard.WriteText(content)
				})
				if pasteToActiveWindowErr == nil {
					current.Actions = append(current.Actions, pasteToActiveWindowAction)
				}
			}
			return current
		}
		onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
			current.Preview.PreviewData += fmt.Sprintf("\n\nError: %s", err.Error())
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		var conversations []common.Conversation
		if query.Selection.Type == selection.SelectionTypeFile {
			var images []common.WoxImage
			for _, imagePath := range query.Selection.FilePaths {
				images = append(images, common.WoxImage{
					ImageType: common.WoxImageTypeAbsolutePath,
					ImageData: imagePath,
				})
			}
			conversations = append(conversations, common.Conversation{
				Role:   common.ConversationRoleUser,
				Text:   command.Prompt,
				Images: images,
			})
		}
		if query.Selection.Type == selection.SelectionTypeText {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleUser,
				Text: fmt.Sprintf(command.Prompt, query.Selection.Text),
			})
		}

		startGenerate := false
		results = append(results, plugin.QueryResult{
			Title:           command.Name,
			SubTitle:        fmt.Sprintf("%s - %s", command.AIModel().Provider, command.AIModel().Name),
			Icon:            aiCommandIcon,
			Preview:         plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: "i18n:plugin_ai_command_enter_to_start"},
			RefreshInterval: 100,
			OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, command.AIModel(), conversations, common.EmptyChatOptions, func() bool {
				return startGenerate
			}, onPreparing, onAnswering, onAnswerErr),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_ai_command_run",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						startGenerate = true
					},
				},
			},
		})
	}
	return results
}

func (c *Plugin) listAllCommands(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{
			{
				Title:    "Failed to get ai commands",
				SubTitle: commandsErr.Error(),
				Icon:     aiCommandIcon,
			},
		}
	}

	if len(commands) == 0 {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_no_commands",
				Icon:  aiCommandIcon,
			},
		}
	}

	var results []plugin.QueryResult
	for _, command := range commands {
		results = append(results, plugin.QueryResult{
			Title:    command.Command,
			SubTitle: command.Name,
			Icon:     aiCommandIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_ai_command_run",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, command.Command),
						})
					},
				},
			},
		})
	}
	return results
}

func (c *Plugin) getAllCommands(ctx context.Context) (commands []commandSetting, err error) {
	commandSettings := c.api.GetSetting(ctx, "commands")
	if commandSettings == "" {
		return nil, nil
	}

	err = json.Unmarshal([]byte(commandSettings), &commands)
	return
}

func (c *Plugin) queryCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Search == "" {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_type_to_start",
				Icon:  aiCommandIcon,
			},
		}
	}

	commands, commandsErr := c.getAllCommands(ctx)
	if commandsErr != nil {
		return []plugin.QueryResult{
			{
				Title:    "Failed to get ai commands",
				SubTitle: commandsErr.Error(),
				Icon:     aiCommandIcon,
			},
		}
	}
	if len(commands) == 0 {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_no_commands",
				Icon:  aiCommandIcon,
			},
		}
	}

	aiCommandSetting, commandExist := lo.Find(commands, func(tool commandSetting) bool {
		return tool.Command == query.Command
	})
	if !commandExist {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_not_found",
				Icon:  aiCommandIcon,
			},
		}
	}

	if aiCommandSetting.Prompt == "" {
		return []plugin.QueryResult{
			{
				Title: "i18n:plugin_ai_command_empty_prompt",
				Icon:  aiCommandIcon,
			},
		}
	}

	var prompts = strings.Split(aiCommandSetting.Prompt, "{wox:new_ai_conversation}")
	var conversations []common.Conversation
	for index, message := range prompts {
		msg := fmt.Sprintf(message, query.Search)
		if index%2 == 0 {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleUser,
				Text: msg,
			})
		} else {
			conversations = append(conversations, common.Conversation{
				Role: common.ConversationRoleAssistant,
				Text: msg,
			})
		}
	}

	onAnswering := func(current plugin.RefreshableResult, deltaAnswer string, isFinished bool) plugin.RefreshableResult {
		current.ContextData = deltaAnswer
		// Process thinking tags to convert them to markdown quote format
		thinking, content := processAIThinking(current.ContextData)
		current.Preview.PreviewData = convertAIThinkingToMarkdown(thinking, content)
		current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom
		if isFinished {
			current.RefreshInterval = 0 // stop refreshing
		}

		return current
	}
	onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
		current.Preview.PreviewData += fmt.Sprintf("\n\nError: %s", err.Error())
		current.RefreshInterval = 0 // stop refreshing
		return current
	}

	result := plugin.QueryResult{
		Title:           fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_chat_with"), aiCommandSetting.Name),
		SubTitle:        fmt.Sprintf("%s - %s", aiCommandSetting.AIModel().Provider, aiCommandSetting.AIModel().Name),
		Preview:         plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: ""},
		Icon:            aiCommandIcon,
		RefreshInterval: 100,
		OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, aiCommandSetting.AIModel(), conversations, common.EmptyChatOptions, func() bool {
			return true
		}, nil, onAnswering, onAnswerErr),
		Actions: []plugin.QueryResultAction{
			{
				Name: "i18n:plugin_ai_command_copy",
				Icon: plugin.CopyIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					_, content := processAIThinking(actionContext.ContextData)
					clipboard.WriteText(content)
				},
			},
		},
	}

	// paste to active window
	pasteToActiveWindowAction, pasteToActiveWindowErr := GetPasteToActiveWindowAction(ctx, c.api, func() {
		_, content := processAIThinking(result.ContextData)
		clipboard.WriteText(content)
	})
	if pasteToActiveWindowErr == nil {
		result.Actions = append(result.Actions, pasteToActiveWindowAction)
	}

	return []plugin.QueryResult{result}
}
