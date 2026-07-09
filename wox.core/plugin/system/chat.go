package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"wox/ai"
	_ "wox/ai/builtintool"
	aitool "wox/ai/builtintool/wox"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/util"
	"wox/util/selection"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

var aiChatIcon = common.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

const aiChatEnterChatModeActionId = "__wox_internal_enter_chat_mode__"

const (
	aiChatCompactionTriggerEstimatedTokens = 24000
	aiChatCompactionRecentTargetTokens     = 12000
	aiChatCompactionMinRecentUserTurns     = 3
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats       []common.AIChatData
	mcpServers  []common.AIChatMCPServerConfig
	mcpToolsMap []common.MCPTool
	api         plugin.API

	// activeChatCancels maps chatId to the active streaming cancellation entry.
	activeChatCancels sync.Map
}

type activeAIChatCancel struct {
	cancel context.CancelFunc
}

type aiChatRuntimeContext struct {
	Conversations []common.Conversation
	DebugTrace    *common.AIChatDebugTrace
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

// configurePluginBuiltinToolHooks wires builtin tools that need plugin manager access.
func (r *AIChatPlugin) configurePluginBuiltinToolHooks() {
	// Wire the ask_user UI hook so the tool package can push questions to the UI
	// without importing wox/plugin.
	aitool.SendAIQuestionHook = func(ctx context.Context, questionId string, question string, options []common.AIQuestionOption) {
		plugin.GetPluginManager().GetUI().SendAIQuestion(ctx, questionId, question, options)
	}

	aitool.TestQueryHook = func(ctx context.Context, queryStr string) (string, error) {
		query := plugin.Query{
			Id:        uuid.NewString(),
			SessionId: util.GetContextSessionId(ctx),
			Type:      plugin.QueryTypeInput,
			RawQuery:  queryStr,
			Search:    queryStr,
		}
		ok := plugin.GetPluginManager().QuerySilent(ctx, query)
		if ok {
			return "query executed successfully (single result, default action ran)", nil
		}
		return "query did not produce a single executable result", nil
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API
	r.mcpServers = []common.AIChatMCPServerConfig{}

	// Configure hooks that let builtin tools call back into the plugin manager.
	r.configurePluginBuiltinToolHooks()

	chats, err := r.loadChats(ctx)
	if err != nil {
		r.chats = []common.AIChatData{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load chats: %s", err.Error()))
	} else {
		r.chats = chats
	}

	if err := r.reloadSkills(ctx, false); err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to load skills: %s", err.Error()))
	}

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
						// Generate the chat id up front so it can be shared with
						// the subsequent chat-mode query via ContextData. Without
						// this, getChatPreviewData would create a brand-new empty
						// chat as ActiveChat and the UI would show a blank new
						// conversation while the real streaming chat (this one)
						// only appears in the history sidebar.
						chatId := uuid.NewString()
						r.Chat(ctx, common.AIChatData{
							Id:    chatId,
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
							ContextData:    common.ContextData{"ai_chat_active_id": chatId},
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
	persistedChats := make([]common.AIChatData, len(r.chats))
	for i, chat := range r.chats {
		persistedChats[i] = cloneAIChatDataForState(chat)
	}
	chatsJson, err := json.Marshal(persistedChats)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to marshal chats: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, aiChatsSettingKey, string(chatsJson), false)
}

func (r *AIChatPlugin) GetAllTools(ctx context.Context) []common.MCPTool {
	// Keep returning MCPTool shape for UI compatibility: the UI only reads
	// Name/Description/Parameters. The registry is the source of truth now.
	tools := r.availableToolsForRuntime(ctx)
	tools = lo.Filter(tools, func(t common.Tool, _ int) bool {
		return !ai.IsRuntimeOnlyTool(t.Name)
	})
	return lo.Map(tools, func(t common.Tool, _ int) common.MCPTool {
		return common.MCPTool{
			Name:         t.Name,
			Description:  t.Description,
			Parameters:   t.Parameters,
			ServerConfig: t.ServerConfig,
		}
	})
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

	r.appendOrUpdateChatData(aiChatData)
	r.saveChats(ctx)

	runtimeContext := r.buildRuntimeRequestContext(ctx, &aiChatData)
	r.appendOrUpdateChatData(aiChatData)
	r.saveChats(ctx)

	// AIChatStream schedules the loop asynchronously, so keep the cancel entry
	// registered until a terminal stream callback cleans up this exact entry.
	chatCtx, cancelChat := context.WithCancel(ctx)
	activeCancel := &activeAIChatCancel{cancel: cancelChat}
	r.activeChatCancels.Store(aiChatData.Id, activeCancel)
	cleanupActiveCancel := func() {
		cancelChat()
		r.activeChatCancels.CompareAndDelete(aiChatData.Id, activeCancel)
	}

	const chatStreamUIUpdateMinIntervalMs int64 = 120
	var chatDataMu sync.Mutex
	var lastChatResponseAt int64
	var responseId = uuid.NewString()
	var prevStatus common.ChatStreamDataStatus

	snapshotChatData := func(force bool) (common.AIChatData, bool) {
		now := util.GetSystemTimestamp()
		if !force && lastChatResponseAt > 0 && now-lastChatResponseAt < chatStreamUIUpdateMinIntervalMs {
			return common.AIChatData{}, false
		}

		lastChatResponseAt = now
		snapshot := cloneAIChatDataForUI(aiChatData)
		if runtimeContext.DebugTrace != nil {
			snapshot.DebugTrace = cloneAIChatDebugTrace(runtimeContext.DebugTrace)
		}
		return snapshot, true
	}

	// Plain chat exposes runtime discovery tools plus high-frequency web tools.
	// Other catalog tools become callable after the model requests them with load_tools.
	chatErr := r.api.AIChatStream(chatCtx, aiChatData.Model, runtimeContext.Conversations, common.ChatOptions{
		Tools:          r.initialToolsForRuntime(ctx),
		LoopPolicy:     common.LoopPolicy{MaxIterations: 25, RetryOnFailure: true, MaxRetries: 3},
		DebugTrace:     runtimeContext.DebugTrace,
		DebugTraceName: "chat",
	}, func(streamResult common.ChatStreamData) {
		r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream receiving data, status: %s, data: %s", streamResult.Status, streamResult.Data))

		var snapshot common.AIChatData
		var shouldSendSnapshot bool
		var finishedSnapshot common.AIChatData
		var isFinished bool

		chatDataMu.Lock()

		// Detect the start of a new model call iteration (streaming after a
		// non-streaming status like running_tool_call). Generate a fresh
		// responseId so each iteration gets its own assistant message instead
		// of overwriting the previous iteration's reasoning and text.
		if streamResult.Status == common.ChatStreamStatusStreaming && prevStatus != "" && prevStatus != common.ChatStreamStatusStreaming {
			responseId = uuid.NewString()
		}
		prevStatus = streamResult.Status

		// Update conversations and sync to UI.
		if streamResult.Data != "" || streamResult.Reasoning != "" {
			r.appendOrUpdateConversationAtEnd(&aiChatData, common.Conversation{
				Id:        responseId,
				Role:      common.ConversationRoleAssistant,
				Text:      streamResult.Data,
				Reasoning: streamResult.Reasoning,
				Timestamp: util.GetSystemTimestamp(),
			})
		}
		if len(streamResult.ToolCalls) > 0 {
			for _, toolCall := range streamResult.ToolCalls {
				if isInternalChatToolCall(toolCall) {
					continue
				}
				r.appendOrUpdateConversation(&aiChatData, common.Conversation{
					Id:           toolCall.Id,
					Role:         common.ConversationRoleTool,
					Text:         toolCall.Delta,
					Reasoning:    "",
					ToolCallInfo: toolCall,
					Timestamp:    toolCall.StartTimestamp,
				})
			}
		}
		updateMainChatDebugTrace(runtimeContext.DebugTrace, aiChatData)

		forceSend := streamResult.Status != common.ChatStreamStatusStreaming
		snapshot, shouldSendSnapshot = snapshotChatData(forceSend)

		// Set the transient IsStreaming flag so the UI knows when to toggle the stop button.
		snapshot.IsStreaming = streamResult.IsNotFinished()

		if streamResult.Status == common.ChatStreamStatusFinished {
			isFinished = true
			finishedSnapshot = snapshot
		}
		// When the loop ends with an error (e.g. max iterations exceeded),
		// still persist the chat data so the user can see the conversation
		// when they reopen the chat later.
		isTerminalError := streamResult.Status == common.ChatStreamStatusError
		chatDataMu.Unlock()

		if shouldSendSnapshot {
			plugin.GetPluginManager().GetUI().SendChatResponse(ctx, snapshot)
		}

		if isFinished {
			cleanupActiveCancel()
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream finished: %s", streamResult.Data))
			r.appendOrUpdateChatData(finishedSnapshot)
			r.saveChats(ctx)

			// Only summarize the chat title if there is no tool call. If any
			// tool calls are present, the loop has more context to add.
			if len(streamResult.ToolCalls) == 0 {
				r.summaryTitleIfNecessary(ctx, finishedSnapshot)
			}
			// The loop now continues inside AIChatStream; Chat() is only invoked once.
		} else if isTerminalError {
			cleanupActiveCancel()
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: chat stream ended with error, saving chat data: %s", streamResult.Data))
			r.appendOrUpdateChatData(snapshot)
			r.saveChats(ctx)
		}
	})

	if chatErr != nil {
		cleanupActiveCancel()
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: Failed to chat: %s", chatErr.Error()))
		r.appendOrUpdateConversation(&aiChatData, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleAssistant,
			Text:      fmt.Sprintf(r.api.GetTranslation(ctx, "ui_ai_chat_error"), chatErr.Error()),
			Timestamp: util.GetSystemTimestamp(),
		})
		aiChatData.IsStreaming = false
		plugin.GetPluginManager().GetUI().SendChatResponse(ctx, aiChatData)
		r.appendOrUpdateChatData(aiChatData)
		r.saveChats(ctx)
		r.api.Notify(ctx, r.api.GetTranslation(ctx, "ui_ai_chat_failed_to_chat"))
	}
}

// isInternalChatToolCall hides runtime-only tool calls from persisted chat history.
func isInternalChatToolCall(toolCall common.ToolCallInfo) bool {
	return ai.IsRuntimeOnlyTool(toolCall.Name)
}

// buildRuntimeRequestContext prepares the provider-facing message list without
// mutating persisted conversations. Long chats use the latest compaction entry
// as a runtime checkpoint, while the saved chat keeps the full history.
func (r *AIChatPlugin) buildRuntimeRequestContext(ctx context.Context, aiChatData *common.AIChatData) aiChatRuntimeContext {
	r.maybeAppendCompactionEntry(ctx, aiChatData)

	runtimeConversations := r.composeRuntimeConversations(ctx, *aiChatData, latestCompactionEntry(aiChatData.CompactionEntries), true)
	trace := (*common.AIChatDebugTrace)(nil)
	if util.IsDev() {
		trace = &common.AIChatDebugTrace{
			EstimatedPersistedTokens: estimateConversationTokens(aiChatData.Conversations),
			EstimatedRuntimeTokens:   estimateConversationTokens(runtimeConversations),
		}
	}

	return aiChatRuntimeContext{Conversations: runtimeConversations, DebugTrace: trace}
}

func (r *AIChatPlugin) composeRuntimeConversations(ctx context.Context, aiChatData common.AIChatData, compactionEntry *common.AIChatCompactionEntry, expandCurrentSkillRefs bool) []common.Conversation {
	recentConversations := r.recentConversationsForRuntime(ctx, aiChatData.Conversations, compactionEntry)
	runtimeConversations := make([]common.Conversation, 0, len(recentConversations)+4)
	runtimeConversations = append(runtimeConversations, common.Conversation{
		Id:        uuid.NewString(),
		Role:      common.ConversationRoleSystem,
		Text:      formatRuntimeTimePrompt(util.GetSystemTime()),
		Timestamp: util.GetSystemTimestamp(),
	})
	if availableSkillsPrompt := ai.FormatAvailableSkillsPrompt(ai.GetSkillRegistry().ListEnabled()); availableSkillsPrompt != "" {
		runtimeConversations = append(runtimeConversations, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleSystem,
			Text:      availableSkillsPrompt,
			Timestamp: util.GetSystemTimestamp(),
		})
	}
	if availableToolsPrompt := ai.FormatAvailableToolsPrompt(r.availableToolsForRuntime(ctx)); availableToolsPrompt != "" {
		runtimeConversations = append(runtimeConversations, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleSystem,
			Text:      availableToolsPrompt,
			Timestamp: util.GetSystemTimestamp(),
		})
	}
	if compactionEntry != nil && strings.TrimSpace(compactionEntry.Summary) != "" {
		runtimeConversations = append(runtimeConversations, common.Conversation{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleSystem,
			Text:      "Summary of previous conversation:\n" + strings.TrimSpace(compactionEntry.Summary),
			Timestamp: util.GetSystemTimestamp(),
		})
	}

	currentSkillMessageId := ""
	if expandCurrentSkillRefs {
		currentSkillMessageId = lastUserConversationId(recentConversations)
	}
	for _, conversation := range recentConversations {
		if conversation.Id == currentSkillMessageId {
			runtimeConversations = append(runtimeConversations, r.withMessageSkillReferences(ctx, conversation))
		} else {
			runtimeConversations = append(runtimeConversations, cloneAIConversation(conversation))
		}
	}
	return runtimeConversations
}

// formatRuntimeTimePrompt gives models a stable anchor for relative dates.
func formatRuntimeTimePrompt(now time.Time) string {
	return fmt.Sprintf(
		"Current local date and time: %s.\nCurrent local date: %s.\nResolve relative dates such as today, tomorrow, yesterday, this week, and recent using this local date. For current or time-sensitive requests, use this date instead of model training data when forming searches or answers.",
		now.Format("2006-01-02 15:04:05 MST (-07:00)"),
		now.Format("2006-01-02"),
	)
}

// initialToolsForRuntime returns all builtin tools on the first model step so
// common actions like reading files, running commands, or editing code do not
// require an extra load_tools round trip. MCP tools remain in the catalog and
// are loaded on demand via load_tools.
func (r *AIChatPlugin) initialToolsForRuntime(ctx context.Context) []common.Tool {
	return ai.GetToolRegistry().ListBySource(common.ToolSourceBuiltin)
}

// availableToolsForRuntime exposes all registered tools to the model because web access is enabled by default.
func (r *AIChatPlugin) availableToolsForRuntime(ctx context.Context) []common.Tool {
	return ai.GetToolRegistry().List()
}

func (r *AIChatPlugin) recentConversationsForRuntime(ctx context.Context, conversations []common.Conversation, compactionEntry *common.AIChatCompactionEntry) []common.Conversation {
	if compactionEntry == nil || strings.TrimSpace(compactionEntry.FirstKeptConversationId) == "" {
		return cloneAIConversations(conversations)
	}

	for i, conversation := range conversations {
		if conversation.Id == compactionEntry.FirstKeptConversationId {
			return cloneAIConversations(conversations[i:])
		}
	}

	r.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI: compaction entry first kept conversation not found: %s", compactionEntry.FirstKeptConversationId))
	return cloneAIConversations(conversations)
}

func (r *AIChatPlugin) maybeAppendCompactionEntry(ctx context.Context, aiChatData *common.AIChatData) {
	latestEntry := latestCompactionEntry(aiChatData.CompactionEntries)
	beforeRuntime := r.composeRuntimeConversations(ctx, *aiChatData, latestEntry, true)
	estimatedTokensBefore := estimateConversationTokens(beforeRuntime)
	if estimatedTokensBefore <= aiChatCompactionTriggerEstimatedTokens {
		return
	}

	keepStart := findCompactionKeepStartIndex(aiChatData.Conversations)
	if keepStart <= 0 || keepStart >= len(aiChatData.Conversations) {
		return
	}

	firstNewCompactedIndex := 0
	firstCompactedConversationId := aiChatData.Conversations[0].Id
	previousSummary := ""
	if latestEntry != nil {
		previousSummary = latestEntry.Summary
		if latestEntry.FirstCompactedConversationId != "" {
			firstCompactedConversationId = latestEntry.FirstCompactedConversationId
		}
		if previousLastIndex := conversationIndexById(aiChatData.Conversations, latestEntry.LastCompactedConversationId); previousLastIndex >= 0 {
			firstNewCompactedIndex = previousLastIndex + 1
		}
	}
	if keepStart <= firstNewCompactedIndex {
		return
	}

	section := cloneAIConversations(aiChatData.Conversations[firstNewCompactedIndex:keepStart])
	summaryText, summarizeErr := r.summarizeConversationsSection(ctx, aiChatData.Model, previousSummary, section)
	if summarizeErr != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: failed to compact conversations: %s", summarizeErr.Error()))
		return
	}
	if strings.TrimSpace(summaryText) == "" {
		return
	}

	entry := common.AIChatCompactionEntry{
		Id:                           uuid.NewString(),
		Summary:                      strings.TrimSpace(summaryText),
		FirstCompactedConversationId: firstCompactedConversationId,
		LastCompactedConversationId:  aiChatData.Conversations[keepStart-1].Id,
		FirstKeptConversationId:      aiChatData.Conversations[keepStart].Id,
		EstimatedTokensBefore:        estimatedTokensBefore,
		ConversationCount:            len(aiChatData.Conversations),
		Model:                        aiChatData.Model,
		CreatedAt:                    util.GetSystemTimestamp(),
	}
	entry.EstimatedTokensAfter = estimateConversationTokens(r.composeRuntimeConversations(ctx, *aiChatData, &entry, true))
	aiChatData.CompactionEntries = append(aiChatData.CompactionEntries, entry)
	r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: created compaction entry, chat: %s, beforeTokens: %d, afterTokens: %d, firstKept: %s", aiChatData.Id, entry.EstimatedTokensBefore, entry.EstimatedTokensAfter, entry.FirstKeptConversationId))
}

// withMessageSkillReferences expands explicit message-level skill refs for the current provider call only.
func (r *AIChatPlugin) withMessageSkillReferences(ctx context.Context, conversation common.Conversation) common.Conversation {
	if conversation.Role != common.ConversationRoleUser || len(conversation.SkillRefs) == 0 {
		return cloneAIConversation(conversation)
	}

	var builder strings.Builder
	seenSkills := map[string]bool{}
	for _, ref := range conversation.SkillRefs {
		skill, ok := ai.ResolveSkillRef(ref)
		if !ok {
			r.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI: skill reference not found: id=%s name=%s path=%s", ref.Id, ref.Name, ref.Path))
			continue
		}
		if !skill.Enabled || strings.TrimSpace(skill.ManifestPath) == "" || seenSkills[skill.Id] {
			continue
		}
		seenSkills[skill.Id] = true

		skillPrompt, err := ai.FormatSkillInvocation(skill)
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("AI: failed to load referenced skill %s: %s", skill.Id, err.Error()))
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(skillPrompt)
	}
	if builder.Len() == 0 {
		return cloneAIConversation(conversation)
	}

	cloned := cloneAIConversation(conversation)
	// Strip {skill:xxx} tags from the text sent to the model — they are an
	// internal UI representation and the skill content is already injected
	// above. The tags remain in the persisted conversation for display.
	cleanedText := ai.StripSkillTags(conversation.Text)
	cloned.Text = builder.String() + "\n\nUser request:\n" + cleanedText
	return cloned
}

// updateMainChatDebugTrace keeps the dev inspector token summary synchronized while streaming.
func updateMainChatDebugTrace(trace *common.AIChatDebugTrace, aiChatData common.AIChatData) {
	if trace == nil {
		return
	}

	trace.SetEstimatedPersistedTokens(estimateConversationTokens(aiChatData.Conversations))
}

type aiChatConversationTurn struct {
	start int
	end   int
}

func findCompactionKeepStartIndex(conversations []common.Conversation) int {
	turns := splitConversationTurns(conversations)
	if len(turns) <= aiChatCompactionMinRecentUserTurns {
		return 0
	}

	keptTokens := 0
	keptUserTurns := 0
	keepStart := turns[len(turns)-1].start
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		turnTokens := estimateConversationTokens(conversations[turn.start:turn.end])
		if keptUserTurns >= aiChatCompactionMinRecentUserTurns && keptTokens+turnTokens > aiChatCompactionRecentTargetTokens {
			break
		}
		keptTokens += turnTokens
		keptUserTurns++
		keepStart = turn.start
	}
	return keepStart
}

func splitConversationTurns(conversations []common.Conversation) []aiChatConversationTurn {
	userStarts := []int{}
	for i, conversation := range conversations {
		if conversation.Role == common.ConversationRoleUser {
			userStarts = append(userStarts, i)
		}
	}
	if len(userStarts) == 0 {
		return []aiChatConversationTurn{}
	}

	turns := make([]aiChatConversationTurn, 0, len(userStarts))
	for i, userStart := range userStarts {
		start := userStart
		if i == 0 && userStart > 0 {
			start = 0
		}
		end := len(conversations)
		if i+1 < len(userStarts) {
			end = userStarts[i+1]
		}
		turns = append(turns, aiChatConversationTurn{start: start, end: end})
	}
	return turns
}

func conversationIndexById(conversations []common.Conversation, id string) int {
	if id == "" {
		return -1
	}
	for i, conversation := range conversations {
		if conversation.Id == id {
			return i
		}
	}
	return -1
}

func lastUserConversationId(conversations []common.Conversation) string {
	for i := len(conversations) - 1; i >= 0; i-- {
		if conversations[i].Role == common.ConversationRoleUser {
			return conversations[i].Id
		}
	}
	return ""
}

func latestCompactionEntry(entries []common.AIChatCompactionEntry) *common.AIChatCompactionEntry {
	if len(entries) == 0 {
		return nil
	}
	entry := entries[len(entries)-1]
	return &entry
}

func estimateConversationTokens(conversations []common.Conversation) int {
	total := 0
	for _, conversation := range conversations {
		total += 8
		total += estimateTextTokens(string(conversation.Role))
		total += estimateTextTokens(conversation.Text)
		total += estimateTextTokens(conversation.Reasoning)
		total += len(conversation.Images) * 1024
		for _, ref := range conversation.SkillRefs {
			total += estimateTextTokens(ref.Id) + estimateTextTokens(ref.Name) + estimateTextTokens(ref.Path) + estimateTextTokens(ref.Source)
		}
		if conversation.ToolCallInfo.Id != "" {
			total += 16
			total += estimateTextTokens(conversation.ToolCallInfo.Id)
			total += estimateTextTokens(conversation.ToolCallInfo.Name)
			total += estimateTextTokens(fmt.Sprintf("%v", conversation.ToolCallInfo.Arguments))
			total += estimateTextTokens(conversation.ToolCallInfo.Delta)
			total += estimateTextTokens(conversation.ToolCallInfo.Response)
		}
	}
	return total
}

func estimateTextTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runeCount := len([]rune(text))
	return (runeCount + 3) / 4
}

// cloneAIChatDataForUI copies mutable slices before websocket serialization so
// concurrent stream callbacks cannot mutate the payload while it is sent.
func cloneAIChatDataForUI(aiChatData common.AIChatData) common.AIChatData {
	snapshot := aiChatData
	snapshot.Conversations = cloneAIConversations(aiChatData.Conversations)
	snapshot.CompactionEntries = cloneAIChatCompactionEntries(aiChatData.CompactionEntries)
	snapshot.DebugTrace = cloneAIChatDebugTrace(aiChatData.DebugTrace)
	return snapshot
}

func cloneAIChatDataForState(aiChatData common.AIChatData) common.AIChatData {
	snapshot := cloneAIChatDataForUI(aiChatData)
	snapshot.DebugTrace = nil
	snapshot.IsStreaming = false
	snapshot.IsSummary = false
	return snapshot
}

// cloneAIChatDataForPreviewList keeps the chat sidebar lightweight while
// preserving enough metadata for grouping and selection.
func (r *AIChatPlugin) cloneAIChatDataForPreviewList(aiChatData common.AIChatData) common.AIChatData {
	snapshot := aiChatData
	snapshot.Conversations = nil
	snapshot.CompactionEntries = nil
	snapshot.DebugTrace = nil
	snapshot.IsStreaming = r.isChatStreaming(aiChatData.Id)
	snapshot.IsSummary = true
	return snapshot
}

// isChatStreaming reports the transient runtime state omitted from persisted chat data.
func (r *AIChatPlugin) isChatStreaming(chatId string) bool {
	if chatId == "" {
		return false
	}
	_, ok := r.activeChatCancels.Load(chatId)
	return ok
}

// cloneAIConversations deep-copies conversation slices used by streaming UI snapshots.
func cloneAIConversations(conversations []common.Conversation) []common.Conversation {
	cloned := make([]common.Conversation, len(conversations))
	for i, conversation := range conversations {
		cloned[i] = cloneAIConversation(conversation)
	}
	return cloned
}

func cloneAIChatCompactionEntries(entries []common.AIChatCompactionEntry) []common.AIChatCompactionEntry {
	return append([]common.AIChatCompactionEntry(nil), entries...)
}

func cloneAIChatCompactionEntryPtr(entry *common.AIChatCompactionEntry) *common.AIChatCompactionEntry {
	if entry == nil {
		return nil
	}
	cloned := *entry
	return &cloned
}

func cloneAIChatDebugTrace(trace *common.AIChatDebugTrace) *common.AIChatDebugTrace {
	if trace == nil {
		return nil
	}
	cloned := trace.Snapshot()
	return &cloned
}

// cloneAIConversation copies nested mutable fields while keeping scalar metadata intact.
func cloneAIConversation(conversation common.Conversation) common.Conversation {
	cloned := conversation
	cloned.Images = append([]common.WoxImage(nil), conversation.Images...)
	cloned.SkillRefs = append([]common.AISkillRef(nil), conversation.SkillRefs...)
	if conversation.ToolCallInfo.Arguments != nil {
		cloned.ToolCallInfo.Arguments = map[string]any{}
		for key, value := range conversation.ToolCallInfo.Arguments {
			cloned.ToolCallInfo.Arguments[key] = value
		}
	}
	return cloned
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

// appendOrUpdateConversationAtEnd appends a new conversation at the end of
// the list, or updates it in place if it already exists. This preserves
// chronological order: once an assistant message is positioned before its
// tool calls, subsequent updates (e.g. tool result callbacks) will not
// move it after the tool calls.
func (r *AIChatPlugin) appendOrUpdateConversationAtEnd(aiChatData *common.AIChatData, conversation common.Conversation) {
	for i := range aiChatData.Conversations {
		if aiChatData.Conversations[i].Id == conversation.Id {
			aiChatData.Conversations[i] = conversation
			return
		}
	}

	aiChatData.Conversations = append(aiChatData.Conversations, conversation)
}

func (r *AIChatPlugin) appendOrUpdateChatData(aiChatData common.AIChatData) {
	aiChatData = cloneAIChatDataForState(aiChatData)
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

// StopChat cancels the active streaming context for the given chat id.
// Returns true if a streaming session was found and cancelled.
func (r *AIChatPlugin) StopChat(ctx context.Context, chatId string) bool {
	if cancelEntry, ok := r.activeChatCancels.LoadAndDelete(chatId); ok {
		if activeCancel, ok := cancelEntry.(*activeAIChatCancel); ok {
			r.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("AI: Stopping chat with ID: %s", chatId))
			activeCancel.cancel()
			return true
		}
	}
	return false
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

// GetChat returns the full chat payload for a summary item selected in the UI.
func (r *AIChatPlugin) GetChat(ctx context.Context, chatId string) (common.AIChatData, bool) {
	for i := range r.chats {
		if r.chats[i].Id == chatId {
			snapshot := cloneAIChatDataForUI(r.chats[i])
			snapshot.IsStreaming = r.isChatStreaming(chatId)
			snapshot.IsSummary = false
			return snapshot, true
		}
	}

	return common.AIChatData{}, false
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

func (r *AIChatPlugin) getChatPreviewData(ctx context.Context, activeChatId string) plugin.QueryResult {
	activeChat := r.newChatData(ctx)
	if activeChatId != "" {
		activeChat.Id = activeChatId
		activeChat.IsStreaming = r.isChatStreaming(activeChatId)
	}
	chatSummaries := make([]common.AIChatData, 0, len(r.chats))
	for _, chat := range r.chats {
		chatSummaries = append(chatSummaries, r.cloneAIChatDataForPreviewList(chat))
	}

	previewData, err := json.Marshal(common.AIChatPreviewData{
		ActiveChat:   activeChat,
		ActiveChatId: activeChatId,
		Chats:        chatSummaries,
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
	activeChatId := ""
	if query.ContextData != nil {
		activeChatId = query.ContextData["ai_chat_active_id"]
	}
	response := plugin.NewQueryResponse([]plugin.QueryResult{r.getChatPreviewData(ctx, activeChatId)})
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

	debugTrace := cloneAIChatDebugTrace(chat.DebugTrace)
	chatOptions := common.EmptyChatOptions
	if util.IsDev() && debugTrace != nil {
		chatOptions.DebugTrace = debugTrace
		chatOptions.DebugTraceName = "title_summary"
	}

	r.api.AIChatStream(ctx, chat.Model, conversations, chatOptions, func(streamResult common.ChatStreamData) {
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
			updatedSnapshot := cloneAIChatDataForUI(updatedChat)
			// Title updates come from persisted chat state, so restore the transient debug trace for the UI snapshot.
			updatedSnapshot.DebugTrace = cloneAIChatDebugTrace(debugTrace)
			plugin.GetPluginManager().GetUI().SendChatResponse(ctx, updatedSnapshot)
		}
	})
}

// summarizeConversationsSection asks the model to update the cumulative
// compaction summary for a contiguous range of complete user turns.
func (r *AIChatPlugin) summarizeConversationsSection(ctx context.Context, model common.Model, previousSummary string, section []common.Conversation) (string, error) {
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Create an updated cumulative summary for the assistant to continue this chat.\n")
	promptBuilder.WriteString("Requirements:\n")
	promptBuilder.WriteString("1. Preserve durable facts, user preferences, decisions, tool outcomes, and unresolved tasks.\n")
	promptBuilder.WriteString("2. Do not invent details.\n")
	promptBuilder.WriteString("3. Return only the updated summary.\n")
	if strings.TrimSpace(previousSummary) != "" {
		promptBuilder.WriteString("\nExisting cumulative summary:\n")
		promptBuilder.WriteString(strings.TrimSpace(previousSummary))
		promptBuilder.WriteString("\n")
	}
	promptBuilder.WriteString("\nNew conversation section to fold into the summary:\n")
	promptBuilder.WriteString(formatConversationsForCompactionSummary(section))

	promptConversations := []common.Conversation{
		{
			Id:        uuid.NewString(),
			Role:      common.ConversationRoleUser,
			Text:      promptBuilder.String(),
			Timestamp: util.GetSystemTimestamp(),
		},
	}

	var sb strings.Builder
	var done = make(chan struct{})
	if model.Name == "" {
		model = r.GetDefaultModel(ctx)
	}
	streamErr := r.api.AIChatStream(ctx, model, promptConversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
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

func formatConversationsForCompactionSummary(conversations []common.Conversation) string {
	var builder strings.Builder
	for _, conversation := range conversations {
		builder.WriteString("\n---\n")
		builder.WriteString("role: ")
		builder.WriteString(string(conversation.Role))
		builder.WriteString("\n")
		if strings.TrimSpace(conversation.Text) != "" {
			builder.WriteString("text:\n")
			builder.WriteString(conversation.Text)
			builder.WriteString("\n")
		}
		if strings.TrimSpace(conversation.Reasoning) != "" {
			builder.WriteString("reasoning:\n")
			builder.WriteString(conversation.Reasoning)
			builder.WriteString("\n")
		}
		if conversation.ToolCallInfo.Id != "" {
			builder.WriteString("tool_call:\n")
			builder.WriteString("name: ")
			builder.WriteString(conversation.ToolCallInfo.Name)
			builder.WriteString("\narguments: ")
			builder.WriteString(fmt.Sprintf("%v", conversation.ToolCallInfo.Arguments))
			builder.WriteString("\nresponse:\n")
			builder.WriteString(conversation.ToolCallInfo.Response)
			builder.WriteString("\n")
		}
	}
	return builder.String()
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
