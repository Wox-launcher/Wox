package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/selection"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

var aiChatIcon = common.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

const aiChatEnterChatModeActionId = "__wox_internal_enter_chat_mode__"

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats       []common.AIChatData
	agents      []common.AIAgent
	mcpServers  []common.AIChatMCPServerConfig
	mcpToolsMap []common.MCPTool
	api         plugin.API
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "i18n:plugin_ai_chat_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_ai_chat_plugin_description",
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
							Key:     "skills",
							Label:   "i18n:plugin_ai_chat_agent_skills",
							Type:    definition.PluginSettingValueTableColumnTypeAISelectSkills,
							Width:   100,
							Tooltip: "i18n:plugin_ai_chat_agent_skills_tooltip",
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
				Params: map[string]any{
					"WidthRatio": 0.25,
				},
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.mcpServers = []common.AIChatMCPServerConfig{}

	// Register builtin tools that need the plugin manager (test_query, ask_user
	// UI hook) before MCP servers load so the registry is ready at startup.
	registerPluginBuiltinTools(ctx)

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

	if err := r.reloadSkills(ctx, false); err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load skills: %s", err.Error()))
	}

	r.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == "agents" {
			agents, err := r.loadAgents(callbackCtx)
			if err != nil {
				r.api.Log(callbackCtx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load agents: %s", err.Error()))
				return
			}

			r.agents = agents

			plugin.GetPluginManager().GetUI().ReloadChatResources(callbackCtx, "agents")
		}

	})

	util.Go(ctx, "reload MCP servers", func() {
		// Startup only warms the core MCP tool cache; Flutter loads chat resources lazily after it is ready.
		r.ReloadMCPServers(util.NewTraceContext(), false)
	})
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
							CreatedAt: util.GetSystemTimestamp(),
							UpdatedAt: util.GetSystemTimestamp(),
						}, 0)

						r.api.ChangeQuery(ctx, common.PlainQuery{
							QueryType:      plugin.QueryTypeInput,
							QueryText:      "chat " + query.RawQuery,
							QuerySelection: selection.Selection{},
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
			Name:          lastChat.Model.Name,
			Provider:      lastChat.Model.Provider,
			ProviderAlias: lastChat.Model.ProviderAlias,
		}
	}

	return common.Model{}
}

// ReloadMCPServers reloads global MCP server settings into the active tool registry.
func (r *AIChatPlugin) ReloadMCPServers(ctx context.Context, notifyUI bool) {
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

	tools := lo.Map(mcpTools, func(mcpTool common.MCPTool, _ int) common.Tool {
		return mcpTool.ToTool()
	})
	// Replace all MCP tools so removed or disabled servers stop exposing stale
	// tool definitions. Builtin tools remain registered independently.
	ai.GetToolRegistry().ReplaceSource(common.ToolSourceMCP, tools)
	r.mcpToolsMap = mcpTools

	if notifyUI {
		plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "tools")
	}
}

func (r *AIChatPlugin) loadMCPServers(ctx context.Context) ([]common.AIChatMCPServerConfig, error) {
	return setting.GetSettingManager().GetWoxSetting(ctx).AIMCPServers.Get(), nil
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
		modelProviderAlias := gModel.Get("ProviderAlias").String()

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
			Model: common.Model{
				Name:          modelName,
				Provider:      common.ProviderName(modelProvider),
				ProviderAlias: modelProviderAlias,
			},
			Skills: lo.Map(agent.Get("skills").Array(), func(skill gjson.Result, _ int) string {
				return skill.String()
			}),
			Icon: icon,
		})
		return true
	})

	return agents, nil
}

func (r *AIChatPlugin) GetAllTools(ctx context.Context) []common.MCPTool {
	// Keep returning MCPTool shape for UI compatibility: the UI only reads
	// Name/Description/Parameters. The registry is the source of truth now.
	tools := ai.GetToolRegistry().List()
	return lo.Map(tools, func(t common.Tool, _ int) common.MCPTool {
		return common.MCPTool{
			Name:         t.Name,
			Description:  t.Description,
			Parameters:   t.Parameters,
			ServerConfig: t.ServerConfig,
		}
	})
}

func (r *AIChatPlugin) GetAllAgents(ctx context.Context) []common.AIAgent {
	return r.agents
}

// ReloadSkills reloads discovered skills into the active skill registry.
func (r *AIChatPlugin) ReloadSkills(ctx context.Context) error {
	return r.reloadSkills(ctx, true)
}

func (r *AIChatPlugin) reloadSkills(ctx context.Context, notifyUI bool) error {
	skills, discoverErr := ai.DiscoverSkills(ctx)
	ai.GetSkillRegistry().ReplaceAll(skills)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Loaded %d skills", len(skills)))
	if discoverErr != nil {
		r.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI: skill discovery completed with warnings: %s", discoverErr.Error()))
	}
	if notifyUI {
		plugin.GetPluginManager().GetUI().ReloadChatResources(ctx, "skills")
	}
	return nil
}

func (r *AIChatPlugin) GetAllSkills(ctx context.Context) []common.Skill {
	return ai.GetSkillRegistry().List()
}

func (r *AIChatPlugin) Chat(ctx context.Context, aiChatData common.AIChatData, chatLoopCount int) {
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Starting chat with ID: %s, loop: %d, title: %s, model: %s, conversations: %d", aiChatData.Id, chatLoopCount, aiChatData.Title, aiChatData.Model.Name, len(aiChatData.Conversations)))

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

				// Selected skills are discovered bundles. Pass metadata and the
				// manifest path as user context so the model can inspect SKILL.md
				// when the task calls for it.
				for _, skillId := range agent.Skills {
					skill, ok := ai.GetSkillRegistry().Get(skillId)
					if !ok {
						r.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI: skill not found: %s", skillId))
						continue
					}
					if !skill.Enabled {
						continue
					}
					if skill.ManifestPath != "" {
						aiChatData.Conversations = append(aiChatData.Conversations, common.Conversation{
							Id:        uuid.NewString(),
							Role:      common.ConversationRoleUser,
							Text:      formatSkillReferencePrompt(skill),
							Timestamp: util.GetSystemTimestamp(),
						})
					}
				}

				break
			}
		}
	}

	r.appendOrUpdateChatData(aiChatData)
	r.saveChats(ctx)

	const chatStreamUIUpdateMinIntervalMs int64 = 120
	var chatDataMu sync.Mutex
	var lastChatResponseAt int64
	var responseId = uuid.NewString()

	snapshotChatData := func(force bool) (common.AIChatData, bool) {
		now := util.GetSystemTimestamp()
		if !force && lastChatResponseAt > 0 && now-lastChatResponseAt < chatStreamUIUpdateMinIntervalMs {
			return common.AIChatData{}, false
		}

		lastChatResponseAt = now
		return cloneAIChatDataForUI(aiChatData), true
	}

	// Plain chat intentionally does not expose the global tool registry yet;
	// short prompts should not fan out into every MCP/builtin tool without an
	// explicit tool policy.
	chatErr := r.api.AIChatStream(ctx, aiChatData.Model, aiChatData.Conversations, common.ChatOptions{
		LoopPolicy: common.LoopPolicy{MaxIterations: 25, RetryOnFailure: true, MaxRetries: 3},
		ContextPolicy: common.ContextPolicy{
			MaxConversations: 50,
			SummarizeToCount: 20,
			Enabled:          true,
		},
		OnSummarize: r.maybeSummarizeConversations,
	}, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream receiving data, status: %s, data: %s", streamResult.Status, streamResult.Data))

		var snapshot common.AIChatData
		var shouldSendSnapshot bool
		var finishedSnapshot common.AIChatData
		var isFinished bool

		chatDataMu.Lock()
		// Update conversations and sync to UI.
		if streamResult.Data != "" || streamResult.Reasoning != "" {
			r.appendOrUpdateConversation(&aiChatData, common.Conversation{
				Id:        responseId,
				Role:      common.ConversationRoleAssistant,
				Text:      streamResult.Data,
				Reasoning: streamResult.Reasoning,
				Timestamp: util.GetSystemTimestamp(),
			})
		}
		if len(streamResult.ToolCalls) > 0 {
			for i, toolCall := range streamResult.ToolCalls {
				reasoning := ""
				if i == 0 {
					reasoning = streamResult.Reasoning
				}
				r.appendOrUpdateConversation(&aiChatData, common.Conversation{
					Id:           toolCall.Id,
					Role:         common.ConversationRoleTool,
					Text:         toolCall.Delta,
					Reasoning:    reasoning,
					ToolCallInfo: toolCall,
					Timestamp:    toolCall.StartTimestamp,
				})
			}
		}

		forceSend := streamResult.Status != common.ChatStreamStatusStreaming
		snapshot, shouldSendSnapshot = snapshotChatData(forceSend)

		if streamResult.Status == common.ChatStreamStatusFinished {
			isFinished = true
			finishedSnapshot = snapshot
		}
		chatDataMu.Unlock()

		if shouldSendSnapshot {
			plugin.GetPluginManager().GetUI().SendChatResponse(ctx, snapshot)
		}

		if isFinished {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream finished: %s", streamResult.Data))
			r.appendOrUpdateChatData(finishedSnapshot)
			r.saveChats(ctx)

			// Only summarize the chat title if there is no tool call. If any
			// tool calls are present, the agent loop has more context to add.
			if len(streamResult.ToolCalls) == 0 {
				r.summaryTitleIfNecessary(ctx, finishedSnapshot)
			}
			// The loop now continues inside AIChatStream; Chat() is only invoked once.
		}
	})

	if chatErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to chat: %s", chatErr.Error()))
		r.appendOrUpdateConversation(&aiChatData, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleAssistant,
			Text:      fmt.Sprintf(r.api.GetTranslation(ctx, "ui_ai_chat_error"), chatErr.Error()),
			Timestamp: util.GetSystemTimestamp(),
		})
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)
		r.appendOrUpdateChatData(aiChatData)
		r.saveChats(ctx)
		r.api.Notify(ctx, r.api.GetTranslation(ctx, "ui_ai_chat_failed_to_chat"))
	}
}

// cloneAIChatDataForUI copies mutable slices before websocket serialization so
// concurrent stream callbacks cannot mutate the payload while it is sent.
func cloneAIChatDataForUI(aiChatData common.AIChatData) common.AIChatData {
	snapshot := aiChatData
	snapshot.Conversations = append([]common.Conversation(nil), aiChatData.Conversations...)
	return snapshot
}

func formatSkillReferencePrompt(skill common.Skill) string {
	var builder strings.Builder
	builder.WriteString("The following local skill is available when relevant. Read its SKILL.md before using it.\n")
	builder.WriteString("Name: ")
	builder.WriteString(skill.Name)
	builder.WriteString("\nDescription: ")
	builder.WriteString(skill.Description)
	builder.WriteString("\nManifest path: ")
	builder.WriteString(skill.ManifestPath)
	builder.WriteString("\nBundle path: ")
	builder.WriteString(skill.Path)
	return builder.String()
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
			sort.Slice(r.chats, func(i, j int) bool {
				return r.chats[i].UpdatedAt > r.chats[j].UpdatedAt
			})
			return
		}
	}

	r.chats = append(r.chats, aiChatData)
	sort.Slice(r.chats, func(i, j int) bool {
		return r.chats[i].UpdatedAt > r.chats[j].UpdatedAt
	})
}

// DeleteChat removes a persisted chat by id and reports whether it existed.
func (r *AIChatPlugin) DeleteChat(ctx context.Context, chatId string) bool {
	for i := range r.chats {
		if r.chats[i].Id == chatId {
			r.chats = append(r.chats[:i], r.chats[i+1:]...)
			r.saveChats(ctx)
			return true
		}
	}

	return false
}

// SummarizeChat starts an asynchronous title refresh for a persisted chat.
func (r *AIChatPlugin) SummarizeChat(ctx context.Context, chatId string) bool {
	for i := range r.chats {
		if r.chats[i].Id == chatId {
			chat := r.chats[i]
			util.Go(ctx, "summarize chat", func() {
				r.summarizeChat(ctx, chat)
			})
			return true
		}
	}

	return false
}

func (r *AIChatPlugin) newChatData(ctx context.Context) common.AIChatData {
	var chatData common.AIChatData
	chatData.Id = uuid.NewString()
	chatData.Title = ""
	chatData.CreatedAt = util.GetSystemTimestamp()
	chatData.UpdatedAt = util.GetSystemTimestamp()
	chatData.Conversations = []common.Conversation{}
	chatData.Model = r.GetDefaultModel(ctx)
	return chatData
}

func (r *AIChatPlugin) getChatPreviewData(ctx context.Context) plugin.QueryResult {
	previewData, err := json.Marshal(common.AIChatPreviewData{
		ActiveChat: r.newChatData(ctx),
		Chats:      r.chats,
	})
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chat preview data: %s", err.Error()))
		return plugin.QueryResult{}
	}

	resultId := uuid.NewString()
	return plugin.QueryResult{
		Id:       resultId,
		Title:    "i18n:ui_ai_chat_new_chat",
		SubTitle: "i18n:ui_ai_chat_create_new_chat",
		Icon:     aiChatIcon,
		Actions: []plugin.QueryResultAction{
			{
				Id:                     aiChatEnterChatModeActionId,
				Name:                   "i18n:ui_ai_chat_start_chat",
				Icon:                   aiChatIcon,
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					// Flutter handles this internal action locally because entering chat mode is UI-only state.
				},
			},
		},
		Preview: plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeChat,
			PreviewData:    string(previewData),
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
		},
		Group:      "i18n:ui_ai_chat_new_chat",
		GroupScore: 1000,
	}
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	response := plugin.NewQueryResponse([]plugin.QueryResult{r.getChatPreviewData(ctx)})
	response.Layout = plugin.QueryLayout{ChatMode: true}
	return response
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
			// Use Data directly since Reasoning is now separated
			title := streamResult.Data
			title = strings.ReplaceAll(title, "\n", "")

			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Summarized chat title: %s", title))

			// update the chat title
			updatedChat := chat
			updatedChat.Title = title
			for i := range r.chats {
				if r.chats[i].Id == chat.Id {
					r.chats[i].Title = title
					updatedChat = r.chats[i]
					break
				}
			}
			r.saveChats(ctx)
			plugin.GetPluginManager().GetUI().SendChatResponse(ctx, updatedChat)
		}
	})
}

// maybeSummarizeConversations compacts a conversation list when it exceeds the
// policy threshold to avoid token overflow in long agent loops. The first
// (system) message and a tail of recent messages are preserved; the middle is
// summarized into a single system message. On any summarization failure the
// original list is returned unchanged so the chat continues rather than aborts.
func (r *AIChatPlugin) maybeSummarizeConversations(ctx context.Context, conversations []common.Conversation, policy common.ContextPolicy) []common.Conversation {
	if !policy.Enabled || policy.MaxConversations <= 0 || len(conversations) <= policy.MaxConversations {
		return conversations
	}

	keepCount := policy.SummarizeToCount
	if keepCount <= 0 || keepCount >= len(conversations) {
		return conversations
	}

	headCount := 1 // preserve the leading system prompt
	if headCount >= len(conversations) {
		return conversations
	}
	tailStart := len(conversations) - keepCount
	if tailStart <= headCount {
		return conversations
	}

	toSummarize := conversations[headCount:tailStart]
	summaryText, summarizeErr := r.summarizeConversationsSection(ctx, conversations[0].Id, toSummarize)
	if summarizeErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: failed to summarize conversations: %s", summarizeErr.Error()))
		return conversations
	}

	summaryConv := common.Conversation{
		Id:        uuid.NewString(),
		Role:      common.ConversationRoleSystem,
		Text:      "Summary of previous conversation:\n" + summaryText,
		Timestamp: util.GetSystemTimestamp(),
	}

	result := make([]common.Conversation, 0, headCount+1+keepCount)
	result = append(result, conversations[:headCount]...)
	result = append(result, summaryConv)
	result = append(result, conversations[tailStart:]...)
	return result
}

// summarizeConversationsSection asks the model to summarize a slice of the
// conversation. It blocks the caller because it runs in the loop iteration
// before the next model request.
func (r *AIChatPlugin) summarizeConversationsSection(ctx context.Context, _ string, section []common.Conversation) (string, error) {
	promptConversations := lo.Filter(section, func(c common.Conversation, _ int) bool {
		return c.Role != common.ConversationRoleTool
	})
	promptConversations = append(promptConversations, common.Conversation{
		Id:   uuid.NewString(),
		Role: common.ConversationRoleUser,
		Text: "Summarize the conversation above into a concise set of facts and decisions an assistant would need to continue. Keep tool results and key context. Do not add anything new.",
	})

	var sb strings.Builder
	var done = make(chan struct{})
	streamErr := r.api.AIChatStream(ctx, r.GetDefaultModel(ctx), promptConversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
		if streamResult.Status == common.ChatStreamStatusFinished {
			sb.WriteString(streamResult.Data)
			close(done)
		}
		if streamResult.Status == common.ChatStreamStatusError {
			sb.WriteString(streamResult.Data)
			close(done)
		}
	})
	if streamErr != nil {
		return "", streamErr
	}

	select {
	case <-done:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return sb.String(), nil
}

func (c *AIChatPlugin) getResultGroup(ctx context.Context, chat common.AIChatData) (string, int64) {
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24 {
		return "i18n:ui_ai_chat_history_today", 90
	}
	if util.GetSystemTimestamp()-chat.UpdatedAt < 1000*60*60*24*2 {
		return "i18n:ui_ai_chat_history_yesterday", 80
	}

	return "i18n:ui_ai_chat_history_history", 10
}
