package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/xeonx/timeago"
	"io"
	"sort"
	"strings"
	"wox/plugin"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
)

var chatgptIcon = plugin.NewWoxImageBase64(`data:image/png;base64,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`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &ChatgptPlugin{})
}

type ChatgptPlugin struct {
	api            plugin.API
	client         *openai.Client
	nonActiveChats []*Chat

	activeChat       *Chat
	activeChatResult *plugin.QueryResult
	activeChatAnswer string
}

type Chat struct {
	Id               string
	Title            string
	Conversations    []Conversation
	CreatedTimestamp int64
}

func (c *Chat) Format() string {
	var result string
	for _, conversation := range c.Conversations {
		nick := "You"
		if conversation.Role == openai.ChatMessageRoleSystem {
			nick = "ChatGPT"
		}
		result += fmt.Sprintf("%s: %s\n\n", nick, conversation.Text)
	}

	return result
}

type Conversation struct {
	Role      string
	Text      string
	Timestamp int64
}

func (c *ChatgptPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c9910664-1c28-47ae-bad6-e7332a02d471",
		Name:          "Chatgpt",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Chatgpt for Wox",
		Icon:          chatgptIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"gpt", "chat",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:   "api_key",
					Label: "API Key",
				},
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
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

	c.loadChats(ctx)
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

	if query.Command == "" {
		// chat conversation
		return c.queryConversation(ctx, query)
	} else {
		// chat with command
		return c.queryCommand(ctx, query)
	}
}

func (c *ChatgptPlugin) queryConversation(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	if c.activeChat != nil {
		if c.activeChatResult == nil {
			c.activeChatResult = &plugin.QueryResult{
				Title:    c.activeChat.Title,
				SubTitle: "Current active chat",
				Icon:     chatgptIcon,
			}
		}

		chatHistory := c.activeChat.Format()
		if chatHistory == "" && query.Search == "" {
			chatHistory = "Please ask anything to continue..."
		}
		if query.Search != "" {
			chatHistory += fmt.Sprintf("You: %s\n", query.Search)
		}
		c.activeChatResult.Preview = plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: chatHistory,
		}
		c.api.Log(ctx, fmt.Sprintf("active chat refresh interval: %d", c.activeChatResult.RefreshInterval))
		c.activeChatResult.Actions = []plugin.QueryResultAction{
			{
				Name:                   "Send chat",
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(actionContext plugin.ActionContext) {
					if query.Search == "" {
						return
					}

					c.activeChat.Conversations = append(c.activeChat.Conversations, Conversation{
						Role:      openai.ChatMessageRoleUser,
						Text:      query.Search,
						Timestamp: util.GetSystemTimestamp(),
					})
					c.saveActiveChat(ctx)

					var chatMessages []openai.ChatCompletionMessage
					for _, conversation := range c.activeChat.Conversations {
						chatMessages = append(chatMessages, openai.ChatCompletionMessage{
							Role:    conversation.Role,
							Content: conversation.Text,
						})
					}

					onAnswering := func(current plugin.RefreshableResult, deltaAnswer string) plugin.RefreshableResult {
						if c.activeChatAnswer == "" {
							//first response
							deltaAnswer = fmt.Sprintf("ChatGPT: %s", deltaAnswer)
						}

						current.Preview.PreviewData += deltaAnswer
						c.activeChatAnswer += deltaAnswer
						return current
					}
					onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
						current.Preview.PreviewData += fmt.Sprintf("Error: %s", err.Error())
						current.RefreshInterval = 0 // stop refreshing
						c.activeChatResult.RefreshInterval = 0
						c.activeChatAnswer = ""
						return current
					}
					onAnswerFinished := func(current plugin.RefreshableResult) plugin.RefreshableResult {
						c.api.Log(ctx, "active chat finished")
						current.RefreshInterval = 0 // stop refreshing
						c.activeChatResult.RefreshInterval = 0
						c.activeChat.Conversations = append(c.activeChat.Conversations, Conversation{
							Role:      openai.ChatMessageRoleSystem,
							Text:      c.activeChatAnswer,
							Timestamp: util.GetSystemTimestamp(),
						})
						c.activeChatAnswer = ""
						c.saveActiveChat(ctx)
						return current
					}
					c.activeChatResult.RefreshInterval = 100
					c.activeChatAnswer = ""
					c.activeChatResult.OnRefresh = c.generateGptResultRefresh(ctx, chatMessages, onAnswering, onAnswerErr, onAnswerFinished)

					c.api.ChangeQuery(ctx, share.ChangedQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: query.TriggerKeyword + " ",
					})

					c.saveChats(ctx)
				},
			},
			{
				Name:                   "Delete chat",
				PreventHideAfterAction: true,
				Action: func(actionContext plugin.ActionContext) {
					c.deleteChat(ctx, c.activeChat.Id, true)
					c.api.ChangeQuery(ctx, share.ChangedQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: query.TriggerKeyword + " ",
					})
				},
			},
		}

		results = append(results, *c.activeChatResult)
	}

	newChatPreviewData := "Please input conversation title to continue..."
	if query.Search != "" {
		newChatPreviewData = fmt.Sprintf("Please input conversation title to continue\n\nTitle: %s", query.Search)
	}
	results = append(results, plugin.QueryResult{
		Title: "Start a new chat",
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeText,
			PreviewData: newChatPreviewData,
		},
		Icon: chatgptIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "Start",
				PreventHideAfterAction: true,
				Action: func(actionContext plugin.ActionContext) {
					if query.Search == "" {
						return
					}

					newChat := &Chat{
						Id:               uuid.NewString(),
						Title:            query.Search,
						CreatedTimestamp: util.GetSystemTimestamp(),
					}
					c.nonActiveChats = append(c.nonActiveChats, newChat)
					c.changeActiveChat(ctx, newChat.Id)
					c.api.ChangeQuery(ctx, share.ChangedQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: query.TriggerKeyword + " ",
					})
				},
			},
		},
	})

	for _, chat := range c.nonActiveChats {
		chatHistory := chat.Format()
		if chatHistory == "" {
			chatHistory = "No conversation"
		}
		results = append(results, plugin.QueryResult{
			Title:    chat.Title,
			SubTitle: timeago.English.Format(util.ParseTimeStamp(chat.CreatedTimestamp)),
			Icon:     chatgptIcon,
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeMarkdown,
				PreviewData: chatHistory,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "Activate",
					PreventHideAfterAction: true,
					Action: func(actionContext plugin.ActionContext) {
						c.changeActiveChat(ctx, chat.Id)
						c.api.ChangeQuery(ctx, share.ChangedQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: query.TriggerKeyword + " ",
						})
					},
				},
				{
					Name:                   "Delete chat",
					PreventHideAfterAction: true,
					Action: func(actionContext plugin.ActionContext) {
						c.deleteChat(ctx, chat.Id, true)
						c.api.ChangeQuery(ctx, share.ChangedQuery{
							QueryType: plugin.QueryTypeInput,
							QueryText: query.TriggerKeyword + " ",
						})
					},
				},
			},
		})
	}

	// sort by score desc
	for i := range results {
		results[i].Score = int64(len(results) - i)
	}

	return results
}

func (c *ChatgptPlugin) queryCommand(ctx context.Context, query plugin.Query) []plugin.QueryResult {
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

		onAnswering := func(current plugin.RefreshableResult, deltaAnswer string) plugin.RefreshableResult {
			current.Preview.PreviewData += deltaAnswer
			return current
		}
		onAnswerErr := func(current plugin.RefreshableResult, err error) plugin.RefreshableResult {
			current.Preview.PreviewData += fmt.Sprintf("Error: %s", err.Error())
			current.RefreshInterval = 0 // stop refreshing
			return current
		}
		onAnswerFinished := func(current plugin.RefreshableResult) plugin.RefreshableResult {
			current.Preview.PreviewData += "\n\nChat finished"
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		return []plugin.QueryResult{{
			Title:           fmt.Sprintf("Chat with %s", query.Command),
			RefreshInterval: 100,
			OnRefresh:       c.generateGptResultRefresh(ctx, chatMessages, onAnswering, onAnswerErr, onAnswerFinished),
		}}
	}

	return []plugin.QueryResult{}
}

// generate a result which will send chat messages to openai and show the result automatically
func (c *ChatgptPlugin) generateGptResultRefresh(ctx context.Context, messages []openai.ChatCompletionMessage,
	onAnswering func(plugin.RefreshableResult, string) plugin.RefreshableResult,
	onAnswerErr func(plugin.RefreshableResult, error) plugin.RefreshableResult,
	onAnswerFinished func(plugin.RefreshableResult) plugin.RefreshableResult) func(current plugin.RefreshableResult) plugin.RefreshableResult {

	var stream *openai.ChatCompletionStream
	var creatingStream bool
	return func(current plugin.RefreshableResult) plugin.RefreshableResult {
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
				if onAnswerErr != nil {
					current = onAnswerErr(current, createErr)
				}
				current.RefreshInterval = 0 // stop refreshing
				return current
			}
			stream = createdStream
		}

		c.api.Log(ctx, "Reading stream")
		response, streamErr := stream.Recv()
		if errors.Is(streamErr, io.EOF) {
			stream.Close()
			if onAnswerFinished != nil {
				current = onAnswerFinished(current)
			}
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		if streamErr != nil {
			stream.Close()
			if onAnswerErr != nil {
				current = onAnswerErr(current, streamErr)
			}
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		if onAnswering != nil {
			current = onAnswering(current, response.Choices[0].Delta.Content)
		}

		return current
	}
}

func (c *ChatgptPlugin) loadChats(ctx context.Context) {
	nonActiveChatStr := c.api.GetSetting(ctx, "non_active_chats")
	if nonActiveChatStr == "" {
		c.api.Log(ctx, "No non-active chats to load")
		c.nonActiveChats = []*Chat{}
	} else {
		unmarshalErr := json.Unmarshal([]byte(nonActiveChatStr), &c.nonActiveChats)
		if unmarshalErr != nil {
			c.api.Log(ctx, fmt.Sprintf("Failed to load nonActiveChats: %s", unmarshalErr.Error()))
		}

		//sort nonactive chats by created timestamp desc
		sort.Slice(c.nonActiveChats, func(i, j int) bool {
			return c.nonActiveChats[i].CreatedTimestamp > c.nonActiveChats[j].CreatedTimestamp
		})
	}

	activeChatStr := c.api.GetSetting(ctx, "active_chat")
	if activeChatStr == "" {
		c.api.Log(ctx, "No active chat to load")
		c.activeChat = nil
	} else {
		var activeChat *Chat
		unmarshalErr := json.Unmarshal([]byte(activeChatStr), &activeChat)
		if unmarshalErr != nil {
			c.api.Log(ctx, fmt.Sprintf("Failed to load activeChat: %s", unmarshalErr.Error()))
		}
		c.activeChat = activeChat

		if c.activeChat != nil {
			//sort active chat conversations by timestamp asc
			sort.Slice(c.activeChat.Conversations, func(i, j int) bool {
				return c.activeChat.Conversations[i].Timestamp < c.activeChat.Conversations[j].Timestamp
			})
		}
	}

	c.api.Log(ctx, fmt.Sprintf("Loaded %d nonactive chats, has active chat: %t", len(c.nonActiveChats), c.activeChat != nil))
}

func (c *ChatgptPlugin) saveChats(ctx context.Context) {
	c.saveActiveChat(ctx)
	c.saveNonActiveChats(ctx)
}

func (c *ChatgptPlugin) saveActiveChat(ctx context.Context) {
	if c.activeChat != nil {
		activeChatStr, marshalErr := json.Marshal(c.activeChat)
		if marshalErr != nil {
			c.api.Log(ctx, fmt.Sprintf("Failed to marshal activeChats: %s", marshalErr.Error()))
		}
		c.api.SaveSetting(ctx, "active_chat", string(activeChatStr), false)
	} else {
		c.api.SaveSetting(ctx, "active_chat", "", false)
	}
}

func (c *ChatgptPlugin) saveNonActiveChats(ctx context.Context) {
	if len(c.nonActiveChats) > 0 {
		nonActiveChatStr, marshalErr := json.Marshal(c.nonActiveChats)
		if marshalErr != nil {
			c.api.Log(ctx, fmt.Sprintf("Failed to marshal nonActiveChats: %s", marshalErr.Error()))
		}
		c.api.SaveSetting(ctx, "non_active_chats", string(nonActiveChatStr), false)
	} else {
		c.api.SaveSetting(ctx, "non_active_chats", "", false)
	}
}

func (c *ChatgptPlugin) changeActiveChat(ctx context.Context, newActiveChatId string) {
	if c.activeChat != nil {
		c.nonActiveChats = append(c.nonActiveChats, c.activeChat)
	}

	var newNonActiveChats []*Chat
	for _, chat := range c.nonActiveChats {
		if chat.Id == newActiveChatId {
			c.activeChat = chat
		} else {
			newNonActiveChats = append(newNonActiveChats, chat)
		}
	}
	c.nonActiveChats = newNonActiveChats

	//sort nonactive chats by created timestamp desc
	sort.Slice(c.nonActiveChats, func(i, j int) bool {
		return c.nonActiveChats[i].CreatedTimestamp > c.nonActiveChats[j].CreatedTimestamp
	})

	c.saveChats(ctx)
}

func (c *ChatgptPlugin) deleteChat(ctx context.Context, chatId string, isActive bool) {
	if isActive {
		if len(c.nonActiveChats) > 0 {
			c.activeChat = c.nonActiveChats[0]
			c.nonActiveChats = c.nonActiveChats[1:]
			c.activeChatResult = nil
			c.activeChatAnswer = ""
		} else {
			c.activeChat = nil
			c.activeChatResult = nil
			c.activeChatAnswer = ""
		}
	} else {
		var newNonActiveChats []*Chat
		for _, chat := range c.nonActiveChats {
			if chat.Id != chatId {
				newNonActiveChats = append(newNonActiveChats, chat)
			}
		}
		c.nonActiveChats = newNonActiveChats
	}

	c.saveChats(ctx)
}
