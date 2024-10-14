package system

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"time"
	"wox/ai"
	"wox/plugin"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/keyboard"

	"github.com/disintegration/imaging"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiCommandIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)

type commandSetting struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Model   string `json:"model"`
	Prompt  string `json:"prompt"`
	Vision  bool   `json:"vision"` // does the command interact with vision
}

func (c *commandSetting) AIModel() (model ai.Model) {
	err := json.Unmarshal([]byte(c.Model), &model)
	if err != nil {
		return ai.Model{}
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
		Description:   "Make your daily tasks easier with AI commands",
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
					Title:   "Commands",
					Tooltip: "The commands to run.\r\nE.g. `translate`, user will type `ai translate` to run translate based on the prompt",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "Name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "The name of the ai command. E.g. `Translator`",
						},
						{
							Key:     "command",
							Label:   "Command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "The command to run. E.g. `translate`, user will type `ai translate` to run this command",
						},
						{
							Key:     "model",
							Label:   "Model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   100,
							Tooltip: "The ai model to use.",
						},
						{
							Key:          "prompt",
							Label:        "Prompt",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Tooltip:      "The prompt to send to the ai. %s will be replaced with the user input",
						},
						{
							Key:     "vision",
							Label:   "Vision",
							Type:    definition.PluginSettingValueTableColumnTypeCheckbox,
							Width:   60,
							Tooltip: "Does the command interact with vision?",
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
		if query.Selection.Type == util.SelectionTypeFile {
			if !command.Vision {
				continue
			}
		}
		if query.Selection.Type == util.SelectionTypeText {
			if command.Vision {
				continue
			}
		}

		var startAnsweringTime int64
		onPreparing := func(current plugin.RefreshableResult) plugin.RefreshableResult {
			current.Preview.PreviewData = "Answering..."
			current.SubTitle = "Answering..."
			startAnsweringTime = util.GetSystemTimestamp()
			return current
		}

		isFirstAnswer := true
		onAnswering := func(current plugin.RefreshableResult, deltaAnswer string, isFinished bool) plugin.RefreshableResult {
			if isFirstAnswer {
				current.Preview.PreviewData = ""
				isFirstAnswer = false
			}

			current.SubTitle = "Answering..."
			current.Preview.PreviewData += deltaAnswer
			current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom

			if isFinished {
				current.RefreshInterval = 0 // stop refreshing
				current.SubTitle = fmt.Sprintf("Answered, cost %d ms", util.GetSystemTimestamp()-startAnsweringTime)
			}
			return current
		}
		onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
			current.Preview.PreviewData += fmt.Sprintf("\n\nError: %s", err.Error())
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		var conversations []ai.Conversation
		if query.Selection.Type == util.SelectionTypeFile {
			var images []image.Image
			for _, imagePath := range query.Selection.FilePaths {
				img, imgErr := imaging.Open(imagePath)
				if imgErr != nil {
					continue
				}
				images = append(images, img)
			}
			conversations = append(conversations, ai.Conversation{
				Role:   ai.ConversationRoleUser,
				Text:   command.Prompt,
				Images: images,
			})
		}
		if query.Selection.Type == util.SelectionTypeText {
			conversations = append(conversations, ai.Conversation{
				Role: ai.ConversationRoleUser,
				Text: fmt.Sprintf(command.Prompt, query.Selection.Text),
			})
		}

		startGenerate := false
		results = append(results, plugin.QueryResult{
			Title:           command.Name,
			SubTitle:        fmt.Sprintf("%s - %s", command.AIModel().Provider, command.AIModel().Name),
			Icon:            aiCommandIcon,
			Preview:         plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: "Enter to start chat"},
			RefreshInterval: 100,
			OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, command.AIModel(), conversations, func() bool {
				return startGenerate
			}, onPreparing, onAnswering, onAnswerErr),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "Run",
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
				Title: "No ai commands found",
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
					Name:                   "Run",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						c.api.ChangeQuery(ctx, share.PlainQuery{
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
				Title: "Type to start chat",
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
				Title: "No ai commands found",
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
				Title: "No ai command found",
				Icon:  aiCommandIcon,
			},
		}
	}

	if aiCommandSetting.Prompt == "" {
		return []plugin.QueryResult{
			{
				Title: "Prompt is empty for this ai command",
				Icon:  aiCommandIcon,
			},
		}
	}

	var prompts = strings.Split(aiCommandSetting.Prompt, "{wox:new_ai_conversation}")
	var conversations []ai.Conversation
	for index, message := range prompts {
		msg := fmt.Sprintf(message, query.Search)
		if index%2 == 0 {
			conversations = append(conversations, ai.Conversation{
				Role: ai.ConversationRoleUser,
				Text: msg,
			})
		} else {
			conversations = append(conversations, ai.Conversation{
				Role: ai.ConversationRoleSystem,
				Text: msg,
			})
		}
	}

	onAnswering := func(current plugin.RefreshableResult, deltaAnswer string, isFinished bool) plugin.RefreshableResult {
		current.Preview.PreviewData += deltaAnswer
		current.Preview.ScrollPosition = plugin.WoxPreviewScrollPositionBottom
		current.ContextData = current.Preview.PreviewData
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

	return []plugin.QueryResult{{
		Title:           fmt.Sprintf("Chat with %s", aiCommandSetting.Name),
		SubTitle:        fmt.Sprintf("%s - %s", aiCommandSetting.AIModel().Provider, aiCommandSetting.AIModel().Name),
		Preview:         plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: ""},
		Icon:            aiCommandIcon,
		RefreshInterval: 100,
		OnRefresh: createLLMOnRefreshHandler(ctx, c.api.AIChatStream, aiCommandSetting.AIModel(), conversations, func() bool {
			return true
		}, nil, onAnswering, onAnswerErr),
		Actions: []plugin.QueryResultAction{
			{
				Name: "Copy",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(actionContext.ContextData)
				},
			},
			{
				Name: "Copy and Paste to active app",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(actionContext.ContextData)
					util.Go(context.Background(), "clipboard to copy", func() {
						time.Sleep(time.Millisecond * 100)
						err := keyboard.SimulatePaste()
						if err != nil {
							c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("simulate paste clipboard failed, err=%s", err.Error()))
						} else {
							c.api.Log(ctx, plugin.LogLevelInfo, "simulate paste clipboard success")
						}
					})
				},
			},
		},
	}}
}
