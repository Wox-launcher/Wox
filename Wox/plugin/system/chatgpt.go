package system

import (
	"context"
	"errors"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"io"
	"strings"
	"wox/plugin"
	"wox/setting"
)

var chatgptIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ChatgptPlugin{})
}

type ChatgptPlugin struct {
	api    plugin.API
	client *openai.Client
}

func (c *ChatgptPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c9910664-1c28-47ae-bad6-e7332a02d471",
		Name:          "Chatgpt",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Chatgpt for Wox",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"gpt",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Settings: setting.CustomizedPluginSettings{
			{
				Type: setting.PluginSettingTypeTextBox,
				Value: setting.PluginSettingValueTextBox{
					Key:   "api_key",
					Label: "API Key",
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureDebounce,
				Params: map[string]string{
					"intervalMs": "500",
				},
			},
		},
	}
}

func (c *ChatgptPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API
	apiKey := c.api.GetSetting(ctx, "api_key")
	if apiKey != "" {
		c.client = openai.NewClient(apiKey)
	}
}

func (c *ChatgptPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if c.client == nil {
		return []plugin.QueryResult{
			{
				Title:    "API Key is empty",
				SubTitle: "Please input API Key in settings",
				Icon:     chatgptIcon,
			},
		}
	}

	if query.Command != "" {
		if query.Search == "" {
			return []plugin.QueryResult{
				{
					Title: "Please input something",
					Icon:  chatgptIcon,
				},
			}
		}

		//TODO: need a simple way to read user settings
		var prompts []string
		commandContent := c.api.GetSetting(ctx, fmt.Sprintf("command_key_%s", query.Command))
		if commandContent != "" {
			prompts = strings.Split(commandContent, "|")
		}

		if len(prompts) > 0 {
			var chatMessages []openai.ChatCompletionMessage
			for index, message := range prompts {
				msg := fmt.Sprintf(message, query.Search)
				if index%2 == 0 {
					chatMessages = append(chatMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: msg,
					})
				} else {
					chatMessages = append(chatMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleSystem,
						Content: msg,
					})
				}
			}
			return []plugin.QueryResult{c.generateGptAnswer(ctx, chatMessages, nil)}
		}
	}

	return []plugin.QueryResult{}
}

func (c *ChatgptPlugin) generateGptAnswer(ctx context.Context, messages []openai.ChatCompletionMessage, action func(actionContext plugin.ActionContext)) plugin.QueryResult {
	var stream *openai.ChatCompletionStream
	var creatingStream bool
	return plugin.QueryResult{
		Title: "Answering...",
		Icon:  chatgptIcon,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: "",
		},
		RefreshInterval: 100,
		OnRefresh: func(current plugin.RefreshableResult) plugin.RefreshableResult {
			if stream == nil {
				if creatingStream {
					c.api.Log(ctx, "Already creating stream, waiting create finish")
					return current
				}

				c.api.Log(ctx, "Creating stream")
				creatingStream = true
				createdStream, createErr := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
					Stream:   true,
					Model:    openai.GPT3Dot5Turbo,
					Messages: messages,
				})
				creatingStream = false
				c.api.Log(ctx, "Created stream")
				if createErr != nil {
					current.Title = "Answer error"
					current.Preview.PreviewData = createErr.Error()
					current.RefreshInterval = 0 // stop refreshing
					return current
				}
				stream = createdStream
			}

			c.api.Log(ctx, "Reading stream")
			response, streamErr := stream.Recv()
			if errors.Is(streamErr, io.EOF) {
				stream.Close()
				current.Title = "Answer finished"
				current.RefreshInterval = 0 // stop refreshing
				return current
			}

			if streamErr != nil {
				stream.Close()
				current.Title = "Answer error"
				current.Preview.PreviewData = streamErr.Error()
				current.RefreshInterval = 0 // stop refreshing
				return current
			}

			current.Preview.PreviewData += response.Choices[0].Delta.Content
			return current
		},
		Actions: []plugin.QueryResultAction{
			{
				Name: "Copy",
				Action: func(actionContext plugin.ActionContext) {
					if action != nil {
						action(actionContext)
					}
				},
			},
		},
	}
}
