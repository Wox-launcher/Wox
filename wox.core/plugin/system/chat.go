package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/selection"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiChatIcon = plugin.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats           []common.AIChatData
	agents          []common.AIAgent
	resultChatIdMap *util.HashMap[string /*chat id*/, string /*result id*/] // map of result id and chat id, used to update the chat title
	mcpServers      []common.AIChatMCPServerConfig
	mcpToolsMap     []common.MCPTool
	api             plugin.API
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "AI Chat",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "Chat with AI",
		Icon:            aiChatIcon.String(),
		TriggerKeywords: []string{"chat"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "enable_fallback_search",
					DefaultValue: "true",
					Label:        "i18n:plugin_ai_chat_enable_fallback_search",
					Tooltip:      "i18n:plugin_ai_chat_enable_fallback_search_tooltip",
				},
			},
			{
				Type:  definition.PluginSettingDefinitionTypeNewLine,
				Value: &definition.PluginSettingValueNewLine{},
			},
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "enable_auto_focus_to_chat_input",
					DefaultValue: "true",
					Label:        "i18n:plugin_ai_chat_enable_auto_focus_to_chat_input",
					Tooltip:      "i18n:plugin_ai_chat_enable_auto_focus_to_chat_input_tooltip",
				},
			},
			{
				Type:  definition.PluginSettingDefinitionTypeNewLine,
				Value: &definition.PluginSettingValueNewLine{},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:     "default_model",
					Label:   "i18n:plugin_ai_chat_default_model",
					Tooltip: "i18n:plugin_ai_chat_default_model_tooltip",
					Style: definition.PluginSettingValueStyle{
						PaddingBottom: 8,
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "agents",
					Title:   "i18n:plugin_ai_chat_agents",
					Tooltip: "i18n:plugin_ai_chat_agents_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "icon",
							Label:   "i18n:plugin_ai_chat_agent_icon",
							Type:    definition.PluginSettingValueTableColumnTypeWoxImage,
							Width:   45,
							Tooltip: "i18n:plugin_ai_chat_agent_icon_tooltip",
						},
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_chat_agent_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "prompt",
							Label:        "i18n:plugin_ai_chat_agent_prompt",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Tooltip:      "i18n:plugin_ai_chat_agent_prompt_tooltip",
						},
						{
							Key:     "model",
							Label:   "i18n:plugin_ai_chat_agent_model",
							Type:    definition.PluginSettingValueTableColumnTypeSelectAIModel,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_model_tooltip",
						},
						{
							Key:     "tools",
							Label:   "i18n:plugin_ai_chat_agent_tools",
							Type:    definition.PluginSettingValueTableColumnTypeAISelectMCPServerTools,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_tools_tooltip",
						},
					},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     "mcp_servers",
					Title:   "i18n:plugin_ai_chat_mcp_servers",
					Tooltip: "i18n:plugin_ai_chat_mcp_servers_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:     "name",
							Label:   "i18n:plugin_ai_chat_mcp_server_name",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_name_tooltip",
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:          "tools",
							Label:        "i18n:plugin_ai_chat_mcp_server_tools",
							Tooltip:      "i18n:plugin_ai_chat_mcp_server_tools_tooltip",
							Type:         definition.PluginSettingValueTableColumnTypeAIMCPServerTools,
							Width:        50,
							HideInUpdate: true,
						},
						{
							Key:   "disabled",
							Label: "i18n:plugin_ai_chat_mcp_server_disabled",
							Type:  definition.PluginSettingValueTableColumnTypeCheckbox,
							Width: 80,
						},
						{
							Key:     "type",
							Label:   "i18n:plugin_ai_chat_mcp_server_type",
							Type:    definition.PluginSettingValueTableColumnTypeSelect,
							Width:   60,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_type_tooltip",
							SelectOptions: []definition.PluginSettingValueSelectOption{
								{
									Label: "STUDIO",
									Value: string(common.AIChatMCPServerTypeSTDIO),
								},
								{
									Label: "SSE",
									Value: string(common.AIChatMCPServerTypeSSE),
								},
							},
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
						{
							Key:     "command",
							Label:   "i18n:plugin_ai_chat_mcp_server_command",
							Type:    definition.PluginSettingValueTableColumnTypeText,
							Width:   80,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_command_tooltip",
						},
						{
							Key:     "environmentVariables",
							Label:   "i18n:plugin_ai_chat_mcp_server_environment_variables",
							Type:    definition.PluginSettingValueTableColumnTypeTextList,
							Width:   160,
							Tooltip: "i18n:plugin_ai_chat_mcp_server_environment_variables_tooltip",
						},
						{
							Key:          "url",
							Label:        "i18n:plugin_ai_chat_mcp_server_url",
							Type:         definition.PluginSettingValueTableColumnTypeText,
							TextMaxLines: 10,
							Width:        80,
							Tooltip:      "i18n:plugin_ai_chat_mcp_server_url_tooltip",
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
				Name: plugin.MetadataFeatureAI,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]string{
					"WidthRatio": "0.25",
				},
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.resultChatIdMap = util.NewHashMap[string, string]()
	r.api = initParams.API
	r.mcpServers = []common.AIChatMCPServerConfig{}

	chats, err := r.loadChats(ctx)
	if err != nil {
		r.chats = []common.AIChatData{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load chats: %s", err.Error()))
	} else {
		r.chats = chats
	}

	agents, err := r.loadAgents(ctx)
	if err != nil {
		r.agents = []common.AIAgent{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load agents: %s", err.Error()))
	} else {
		r.agents = agents
	}

	r.api.OnSettingChanged(ctx, func(key string, value string) {
		if key == "agents" {
			agents, err := r.loadAgents(ctx)
			if err != nil {
				r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load agents: %s", err.Error()))
				return
			}

			r.agents = agents

			plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "agents")
		}

		if key == "mcp_servers" {
			r.reloadMCPServers(ctx)
		}
	})

	// Delay MCP servers reload to avoid websocket server initialization race condition
	util.Go(ctx, "reload MCP servers", func() {
		time.Sleep(time.Millisecond * 1000) // Wait for websocket server to be ready
		r.reloadMCPServers(util.NewTraceContext())
	})
}

func (r *AIChatPlugin) IsAutoFocusToChatInputWhenOpenWithQueryHotkey(ctx context.Context) bool {
	enableAutoFocusToChatInput := r.api.GetSetting(ctx, "enable_auto_focus_to_chat_input")
	return enableAutoFocusToChatInput == "true"
}

func (r *AIChatPlugin) QueryFallback(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	fallbackSearchSetting := r.api.GetSetting(ctx, "enable_fallback_search")
	isEnableFallbackSearch := fallbackSearchSetting == "true"
	if !isEnableFallbackSearch {
		return []plugin.QueryResult{}
	}

	fallbackSearchTitle := r.api.GetTranslation(ctx, "plugin_ai_chat_fallback_search_chat_for")
	fallbackSearchTitle = strings.ReplaceAll(fallbackSearchTitle, "%s", query.RawQuery)

	return []plugin.QueryResult{
		{
			Title: fallbackSearchTitle,
			Icon:  aiChatIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_ai_chat_start_chat",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						r.Chat(ctx, common.AIChatData{
							Id:    uuid.NewString(),
							Title: query.RawQuery,
							Model: r.GetDefaultModel(ctx),
							Conversations: []common.Conversation{
								{
									Id:        uuid.NewString(),
									Role:      common.ConversationRoleUser,
									Text:      query.RawQuery,
									Timestamp: util.GetSystemTimestamp(),
								},
							},
							//TODO: let user customize the default tools, just like model
							Tools: lo.Map(r.GetAllTools(ctx), func(tool common.MCPTool, _ int) string {
								return tool.Name
							}),
							CreatedAt: util.GetSystemTimestamp(),
							UpdatedAt: util.GetSystemTimestamp(),
						}, 0)

						r.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType:      plugin.QueryTypeInput,
							QueryText:      "chat " + query.RawQuery,
							QuerySelection: selection.Selection{},
						})

						util.Go(ctx, "focus to chat input", func() {
							time.Sleep(time.Millisecond * 300)
							plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
						})
					},
				},
			},
		},
	}
}

func (r *AIChatPlugin) GetDefaultModel(ctx context.Context) common.Model {
	model := r.api.GetSetting(context.Background(), "default_model")
	if model != "" {
		var m common.Model
		err := json.Unmarshal([]byte(model), &m)
		if err == nil {
			return m
		} else {
			r.api.Log(context.Background(), plugin.LogLevelError, fmt.Sprintf("AI: Failed to unmarshal default model: %s", err.Error()))
		}
	}

	// get last chat model
	if len(r.chats) > 0 {
		lastChat := r.chats[0]
		return common.Model{
			Name:     lastChat.Model.Name,
			Provider: lastChat.Model.Provider,
		}
	}

	return common.Model{}
}

func (r *AIChatPlugin) reloadMCPServers(ctx context.Context) {
	r.api.Log(ctx, plugin.LogLevelInfo, "AI: Reloading MCP servers")

	mcpServers, err := r.loadMCPServers(ctx)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load mcp servers: %s", err.Error()))
	} else {
		r.mcpServers = mcpServers
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Loaded %d mcp servers", len(r.mcpServers)))
	}

	var mcpTools []common.MCPTool
	for _, mcpServer := range r.mcpServers {
		if mcpServer.Disabled {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: MCP server %s is disabled", mcpServer.Name))
			continue
		}

		tools, err := ai.MCPListTools(ctx, mcpServer)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to list tool: %s", err.Error()))
		}

		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Found %d tools for MCP server %s", len(tools), mcpServer.Name))
		for _, tool := range tools {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: %s tool %s", mcpServer.Name, tool.Name))
			mcpTools = append(mcpTools, tool)
		}
	}

	r.mcpToolsMap = mcpTools

	plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "tools")
}

func (r *AIChatPlugin) loadMCPServers(ctx context.Context) ([]common.AIChatMCPServerConfig, error) {
	mcpServersJson := r.api.GetSetting(ctx, "mcp_servers")
	if mcpServersJson == "" {
		return []common.AIChatMCPServerConfig{}, nil
	}

	var mcpServers []common.AIChatMCPServerConfig
	err := json.Unmarshal([]byte(mcpServersJson), &mcpServers)
	if err != nil {
		return []common.AIChatMCPServerConfig{}, err
	}

	return mcpServers, nil
}

func (r *AIChatPlugin) loadChats(ctx context.Context) ([]common.AIChatData, error) {
	chats := []common.AIChatData{}
	chatsJson := r.api.GetSetting(ctx, aiChatsSettingKey)
	if chatsJson == "" {
		return []common.AIChatData{}, nil
	}

	err := json.Unmarshal([]byte(chatsJson), &chats)
	if err != nil {
		return []common.AIChatData{}, err
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].UpdatedAt > chats[j].UpdatedAt
	})

	return chats, nil
}

func (r *AIChatPlugin) saveChats(ctx context.Context) {
	chatsJson, err := json.Marshal(r.chats)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chats: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, aiChatsSettingKey, string(chatsJson), false)
}

func (r *AIChatPlugin) loadAgents(ctx context.Context) ([]common.AIAgent, error) {
	agents := []common.AIAgent{}
	agentsJson := r.api.GetSetting(ctx, "agents")
	if agentsJson == "" {
		return []common.AIAgent{}, nil
	}

	gjson.Parse(agentsJson).ForEach(func(_, agent gjson.Result) bool {
		gModel := gjson.Parse(agent.Get("model").String())
		modelName := gModel.Get("Name").String()
		modelProvider := gModel.Get("Provider").String()

		// Parse icon if available
		var icon common.WoxImage
		iconJson := agent.Get("icon").String()
		if iconJson != "" {
			gIcon := gjson.Parse(iconJson)
			icon = common.WoxImage{
				ImageType: gIcon.Get("ImageType").String(),
				ImageData: gIcon.Get("ImageData").String(),
			}
		} else {
			// Default icon if not set
			icon = common.WoxImage{
				ImageType: common.WoxImageTypeEmoji,
				ImageData: "🤖",
			}
		}

		agents = append(agents, common.AIAgent{
			Name:   agent.Get("name").String(),
			Prompt: agent.Get("prompt").String(),
			Model:  common.Model{Name: modelName, Provider: common.ProviderName(modelProvider)},
			Tools: lo.Map(agent.Get("tools").Array(), func(tool gjson.Result, _ int) string {
				return tool.String()
			}),
			Icon: icon,
		})
		return true
	})

	return agents, nil
}

func (r *AIChatPlugin) GetAllTools(ctx context.Context) []common.MCPTool {
	return r.mcpToolsMap
}

func (r *AIChatPlugin) GetAllAgents(ctx context.Context) []common.AIAgent {
	return r.agents
}

func (r *AIChatPlugin) Chat(ctx context.Context, aiChatData common.AIChatData, chatLoopCount int) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Starting chat with ID: %s, loop: %d, title: %s, model: %s, conversations: %d", aiChatData.Id, chatLoopCount, aiChatData.Title, aiChatData.Model.Name, len(aiChatData.Conversations)))

	if len(aiChatData.Tools) > 0 {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Selected tools: %v", aiChatData.Tools))
	}

	if aiChatData.AgentName != "" && chatLoopCount == 0 {
		for _, agent := range r.agents {
			if agent.Name == aiChatData.AgentName {
				r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Using agent: %s", agent.Name))

				if agent.Prompt != "" {
					systemPrompt := common.Conversation{
						Id:        uuid.NewString(),
						Role:      common.ConversationRoleSystem,
						Text:      agent.Prompt,
						Timestamp: util.GetSystemTimestamp(),
					}

					aiChatData.Conversations = append([]common.Conversation{systemPrompt}, aiChatData.Conversations...)
				}

				if agent.Model.Name != "" {
					aiChatData.Model = agent.Model
				}

				if len(agent.Tools) > 0 {
					aiChatData.Tools = agent.Tools
				}

				break
			}
		}
	}

	r.appendOrUpdateChatData(aiChatData)
	r.saveChats(ctx)

	var tools []common.MCPTool
	if len(aiChatData.Tools) > 0 {
		tools = lo.Filter(r.mcpToolsMap, func(tool common.MCPTool, _ int) bool {
			return lo.Contains(aiChatData.Tools, tool.Name)
		})
	}

	var responseId = uuid.NewString()
	chatErr := r.api.AIChatStream(ctx, aiChatData.Model, aiChatData.Conversations, common.ChatOptions{
		Tools: tools,
	}, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream receiving data, status: %s, data: %s", streamResult.Status, streamResult.Data))

		// update conversations and sync to UI
		if streamResult.Data != "" {
			r.appendOrUpdateConversation(&aiChatData, common.Conversation{
				Id:        responseId,
				Role:      common.ConversationRoleAssistant,
				Text:      streamResult.Data,
				Timestamp: util.GetSystemTimestamp(),
			})
		}
		if len(streamResult.ToolCalls) > 0 {
			for _, toolCall := range streamResult.ToolCalls {
				r.appendOrUpdateConversation(&aiChatData, common.Conversation{
					Id:           toolCall.Id,
					Role:         common.ConversationRoleTool,
					Text:         toolCall.Delta,
					ToolCallInfo: toolCall,
					Timestamp:    toolCall.StartTimestamp,
				})
			}
		}
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)

		if streamResult.Status == common.ChatStreamStatusFinished {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream finished: %s", streamResult.Data))
			r.appendOrUpdateChatData(aiChatData)
			r.saveChats(ctx)

			// only summarize the chat title if there is no tool call
			// if there is any toolcall, we need to wait for the tool call to finish
			if len(streamResult.ToolCalls) == 0 {
				r.summaryTitleIfNecessary(ctx, aiChatData)
			}

			if streamResult.IsAllToolCallsSucceeded() {
				// recursively call the chat to continue
				r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: recursively calling the chat to continue, loop: %d", chatLoopCount+1))
				r.Chat(ctx, aiChatData, chatLoopCount+1)
			}
		}
	})

	if chatErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to chat: %s", chatErr.Error()))
		r.api.Notify(ctx, "Failed to chat, please try again")
	}
}

func (r *AIChatPlugin) summaryTitleIfNecessary(ctx context.Context, aiChatData common.AIChatData) {
	summarizeIndex := []int{2, 3, 4, 10}
	for _, index := range summarizeIndex {
		nonToolConversationCount := lo.CountBy(aiChatData.Conversations, func(conversation common.Conversation) bool {
			return conversation.Role != common.ConversationRoleTool
		})
		if nonToolConversationCount == index {
			r.summarizeChat(ctx, aiChatData)
			break
		}
	}

}

func (r *AIChatPlugin) appendOrUpdateConversation(aiChatData *common.AIChatData, conversation common.Conversation) {
	for i := range aiChatData.Conversations {
		if aiChatData.Conversations[i].Id == conversation.Id {
			aiChatData.Conversations[i] = conversation
			return
		}
	}

	aiChatData.Conversations = append(aiChatData.Conversations, conversation)
}

func (r *AIChatPlugin) appendOrUpdateChatData(aiChatData common.AIChatData) {
	for i := range r.chats {
		if r.chats[i].Id == aiChatData.Id {
			r.chats[i] = aiChatData
			return
		}
	}

	r.chats = append(r.chats, aiChatData)
	sort.Slice(r.chats, func(i, j int) bool {
		return r.chats[i].UpdatedAt > r.chats[j].UpdatedAt
	})
}

func (r *AIChatPlugin) getNewChatPreviewData(ctx context.Context) plugin.QueryResult {
	var chatData common.AIChatData
	chatData.Id = uuid.NewString()
	chatData.Title = ""
	chatData.CreatedAt = util.GetSystemTimestamp()
	chatData.UpdatedAt = util.GetSystemTimestamp()
	chatData.Conversations = []common.Conversation{}
	chatData.Model = r.GetDefaultModel(ctx)

	previewData, err := json.Marshal(chatData)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chat preview data: %s", err.Error()))
		return plugin.QueryResult{}
	}

	resultId := uuid.NewString()
	r.resultChatIdMap.Store(chatData.Id, resultId)

	return plugin.QueryResult{
		Id:          resultId,
		Title:       "New Chat",
		SubTitle:    "Create a new chat",
		Icon:        aiChatIcon,
		ContextData: chatData.Id,
		Preview: plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeChat,
			PreviewData:    string(previewData),
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
		},
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "Start Chat",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
				},
			},
		},
		Group:      "New Chat",
		GroupScore: 1000,
	}
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	r.resultChatIdMap.Clear()

	if query.Search == "" {
		// add the new chat result for user to create a new chat if there is no search
		results = append(results, r.getNewChatPreviewData(ctx))
	}

	for i, chat := range r.chats {
		previewData, err := json.Marshal(chat)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chat preview data: %s", err.Error()))
			continue
		}

		// filter chat by query
		if query.Search != "" && !strings.Contains(chat.Title, query.Search) {
			continue
		}

		resultId := uuid.NewString()
		r.resultChatIdMap.Store(chat.Id, resultId)

		continueChatText := "Continue Chat"
		if len(chat.Conversations) == 0 {
			continueChatText = "Start Chat"
		}

		// use agent icon
		resultIcon := aiChatIcon
		if chat.AgentName != "" {
			for _, agent := range r.agents {
				if agent.Name == chat.AgentName {
					resultIcon = agent.Icon
					break
				}
			}
		}

		group, groupScore := r.getResultGroup(ctx, chat)
		results = append(results, plugin.QueryResult{
			Id:          resultId,
			Title:       chat.Title,
			Icon:        resultIcon,
			ContextData: chat.Id,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeChat,
				PreviewData:    string(previewData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:                   continueChatText,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// focus to chat input
						plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
					},
				},
				{
					Name:                   "Delete Chat",
					Icon:                   plugin.TrashIcon,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// delete chat
						r.chats = append(r.chats[:i], r.chats[i+1:]...)
						r.saveChats(ctx)

						// refresh the query results
						r.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType:      plugin.QueryTypeInput,
							QueryText:      query.RawQuery,
							QuerySelection: selection.Selection{},
						})
					},
				},
				{
					Name:                   "Summarize Chat",
					Icon:                   common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="currentColor" d="M5 5.5C5 6.33 5.67 7 6.5 7h4v10.5c0 .83.67 1.5 1.5 1.5s1.5-.67 1.5-1.5V7h4c.83 0 1.5-.67 1.5-1.5S18.33 4 17.5 4h-11C5.67 4 5 4.67 5 5.5"/></svg>`),
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						chatId := actionContext.ContextData
						for _, chat := range r.chats {
							if chat.Id == chatId {
								r.summarizeChat(ctx, chat)
								break
							}
						}
					},
				},
			},
			Group:      group,
			GroupScore: groupScore,
		})
	}

	return results
}

func (r *AIChatPlugin) summarizeChat(ctx context.Context, chat common.AIChatData) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Summarizing chat: %s", chat.Id))

	var conversations []common.Conversation
	// skip tool conversations
	conversations = lo.Filter(chat.Conversations, func(conversation common.Conversation, _ int) bool {
		return conversation.Role != common.ConversationRoleTool
	})
	conversations = append(conversations, common.Conversation{
		Id:   uuid.NewString(),
		Role: common.ConversationRoleUser,
		Text: `Please summarize our conversation above and provide a clear and concise title. Requirements:
		1. The title should be no more than 10 characters.
		2. The language of the title should be the same as the language of the conversation.
		3. The title should be a single sentence.
		4. The response should be only the title, no other text.
`,
		Images:    []common.WoxImage{},
		Timestamp: util.GetSystemTimestamp(),
	})

	r.api.AIChatStream(ctx, chat.Model, conversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat summarize stream data: %s", streamResult.Data))
		if streamResult.Status == common.ChatStreamStatusFinished {
			title := streamResult.Data

			// remove the thinking tags
			_, title = processAIThinking(title)
			title = strings.ReplaceAll(title, "\n", "")

			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Summarized chat title: %s", title))

			// update the chat title
			for i := range r.chats {
				if r.chats[i].Id == chat.Id {
					r.chats[i].Title = title
					break
				}
			}
			r.saveChats(ctx)

			if resultId, ok := r.resultChatIdMap.Load(chat.Id); ok {
				plugin.GetPluginManager().GetUI().UpdateResult(ctx, plugin.UpdatableResult{
					Id:    resultId,
					Title: &title,
				})
			}
		}
	})
}

func (c *AIChatPlugin) getResultGroup(ctx context.Context, chat common.AIChatData) (string, int64) {
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24 {
		return "Today", 90
	}
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24*2 {
		return "Yesterday", 80
	}

	return "History", 10
}
