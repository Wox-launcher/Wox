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
	"wox/util/window"

	"github.com/disintegration/imaging"
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
		isAnwserFinished := false
		answerText := ""
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
				current.SubTitle = fmt.Sprintf("Answered, cost %d ms. Enter to copy", util.GetSystemTimestamp()-startAnsweringTime)
				isAnwserFinished = true
				answerText = current.Preview.PreviewData
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
						if isAnwserFinished {
							clipboard.WriteText(answerText)
							c.api.Notify(ctx, "Copied to clipboard")
						} else {
							startGenerate = true
						}
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

	result := plugin.QueryResult{
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
				Icon: plugin.CopyIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(actionContext.ContextData)
				},
			},
		},
	}

	// paste to active window
	windowName := window.GetActiveWindowName()
	windowIcon, windowIconErr := window.GetActiveWindowIcon()
	if windowIconErr == nil && windowName != "" {
		windowIconImage, err := plugin.NewWoxImage(windowIcon)
		if err == nil {
			result.Actions = append(result.Actions, plugin.QueryResultAction{
				Name: "Paste to " + windowName,
				Icon: windowIconImage,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(actionContext.ContextData)
					util.Go(ctx, "ai command paste", func() {
						time.Sleep(time.Millisecond * 150)
						err := keyboard.SimulatePaste()
						if err != nil {
							c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("simulate paste clipboard failed, err=%s", err.Error()))
						} else {
							c.api.Log(ctx, plugin.LogLevelInfo, "simulate paste clipboard success")
						}
					})
				},
			})
		}
	}

	return []plugin.QueryResult{result}
}
