package system

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
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
		Commands: []plugin.MetadataCommand{
			{
				Command:     "translate",
				Description: "Translate text",
			},
		},
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
	var results []plugin.QueryResult
	if c.client == nil {
		return []plugin.QueryResult{
			{
				Title:    "API Key is empty",
				SubTitle: "Please input API Key in settings",
				Icon:     chatgptIcon,
			},
		}
	}

	if query.Command == "translate" {
		resp, err := c.client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: openai.GPT3Dot5Turbo,
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleUser,
						Content: `你是一个翻译引擎，请将给到的文本翻译成中文。请列出3种（如果有）最常用翻译结果：单词或短语，并列出对应的适用语境（用中文阐述）、音标或转写、词性、双语示例。请按照markdown的语法返回,并按照下面格式用中文阐述：
  <序号><单词或短语> · [<词性缩写>] <适用语境（用中文阐述）> 例句：<例句>(例句翻译)`,
					},
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: `好的，我明白了，请给我这个单词。`,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: fmt.Sprintf(`单词是：%s`, query.Search),
					},
				},
			},
		)
		if err != nil {
			return []plugin.QueryResult{
				{
					Title:    "chatgpt error",
					SubTitle: err.Error(),
					Icon:     chatgptIcon,
				},
			}
		}

		results = append(results, plugin.QueryResult{
			Title:    resp.Choices[0].Message.Content,
			SubTitle: "Press Enter to copy",
			Icon:     chatgptIcon,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeMarkdown,
				PreviewData: resp.Choices[0].Message.Content,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name: "Copy",
					Action: func(actionContext plugin.ActionContext) {
					},
				},
			},
		})
	}

	return results
}
