package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
)

const enterChatModeActionID = "__wox_internal_enter_chat_mode__"

const (
	chatCommandPanel             = "commands"
	chatCatalogRowHeight         = float32(38)
	chatCatalogGroupHeaderHeight = float32(28)
)

var chatSkillTagPattern = regexp.MustCompile(`\{skill:([^}]+)\}`)

type chatPreviewData struct {
	ActiveChat   chatData   `json:"ActiveChat"`
	ActiveChatID string     `json:"ActiveChatId"`
	Chats        []chatData `json:"Chats"`
}

type chatData struct {
	ID                string             `json:"Id"`
	Title             string             `json:"Title"`
	Conversations     []chatConversation `json:"Conversations"`
	CompactionEntries []json.RawMessage  `json:"CompactionEntries"`
	Model             aiModel            `json:"Model"`
	DebugTrace        json.RawMessage    `json:"DebugTrace,omitempty"`
	CreatedAt         int64              `json:"CreatedAt"`
	UpdatedAt         int64              `json:"UpdatedAt"`
	IsStreaming       bool               `json:"IsStreaming"`
	IsSummary         bool               `json:"IsSummary"`
}

type chatConversation struct {
	ID           string           `json:"Id"`
	Role         string           `json:"Role"`
	Text         string           `json:"Text"`
	Reasoning    string           `json:"Reasoning"`
	Images       []woxImage       `json:"Images"`
	SkillRefs    []chatSkillRef   `json:"SkillRefs"`
	ToolCallInfo chatToolCallInfo `json:"ToolCallInfo"`
	Timestamp    int64            `json:"Timestamp"`
}

type chatSkillRef struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Path   string `json:"Path"`
	Source string `json:"Source"`
}

type chatSkill struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Description  string `json:"Description"`
	Path         string `json:"Path"`
	ManifestPath string `json:"ManifestPath"`
	Source       string `json:"Source"`
	SourceName   string `json:"SourceName"`
	Error        string `json:"Error"`
	Enabled      bool   `json:"Enabled"`
}

type chatToolCallInfo struct {
	ID             string         `json:"Id"`
	Name           string         `json:"Name"`
	Arguments      map[string]any `json:"Arguments"`
	Status         string         `json:"Status"`
	Delta          string         `json:"Delta"`
	Response       string         `json:"Response"`
	StartTimestamp int64          `json:"StartTimestamp"`
	EndTimestamp   int64          `json:"EndTimestamp"`
}

func chatDataFromContract(source common.AIChatData) (chatData, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return chatData{}, err
	}
	var chat chatData
	if err := json.Unmarshal(encoded, &chat); err != nil {
		return chatData{}, err
	}
	return chat, nil
}

func chatDataToContract(source chatData) (common.AIChatData, error) {
	encoded, err := json.Marshal(source)
	if err != nil {
		return common.AIChatData{}, err
	}
	var chat common.AIChatData
	if err := json.Unmarshal(encoded, &chat); err != nil {
		return common.AIChatData{}, err
	}
	return chat, nil
}

type aiQuestionOption struct {
	Value       string            `json:"Value"`
	Title       string            `json:"Title"`
	SubTitle    string            `json:"SubTitle"`
	Recommended bool              `json:"Recommended"`
	Extra       map[string]string `json:"Extra"`
}

type aiQuestion struct {
	QuestionID string             `json:"QuestionId"`
	Question   string             `json:"Question"`
	Options    []aiQuestionOption `json:"Options"`
}

type chatPreviewState struct {
	key              string
	queryID          string
	resultID         string
	chat             chatData
	chats            []chatData
	editor           *woxui.TextEditor
	active           bool
	scroll           float32
	autoFollow       bool
	loading          bool
	sending          bool
	error            string
	revision         uint64
	remoteVersion    uint64
	panel            string
	panelQuery       string
	panelSelected    int
	panelScroll      float32
	panelViewport    float32
	panelMaxScroll   float32
	question         *aiQuestion
	questionEditor   *woxui.TextEditor
	questionSelected int
	expandedRounds   map[string]bool
}

type chatPreviewSnapshot struct {
	key              string
	queryID          string
	resultID         string
	chat             chatData
	editing          woxui.TextEditingState
	active           bool
	scroll           float32
	loading          bool
	sending          bool
	error            string
	chats            []chatData
	models           []aiModel
	modelsLoading    bool
	modelsError      string
	skills           []chatSkill
	skillsLoading    bool
	skillsError      string
	panel            string
	panelQuery       string
	panelSelected    int
	panelScroll      float32
	panelViewport    float32
	question         *aiQuestion
	questionEditing  woxui.TextEditingState
	questionSelected int
	expandedRounds   map[string]bool
}

type chatCommandPaletteItem struct {
	group       string
	sourceIndex int
	title       string
	subtitle    string
	searchText  string
	current     bool
}

type chatSlashToken struct {
	start int
	end   int
	query string
}

// findChatSlashToken returns the whitespace-delimited slash token surrounding the caret.
func findChatSlashToken(editing woxui.TextEditingState) (chatSlashToken, bool) {
	runes := []rune(editing.Text)
	cursor := min(max(0, editing.Selection.Focus), len(runes))
	start := cursor
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	if start >= len(runes) || runes[start] != '/' {
		return chatSlashToken{}, false
	}
	end := cursor
	for end < len(runes) && !unicode.IsSpace(runes[end]) {
		end++
	}
	return chatSlashToken{start: start, end: end, query: strings.TrimSpace(string(runes[start+1 : end]))}, true
}

// chatCommandPaletteItems mirrors Flutter's case-insensitive model and skill filtering.
func chatCommandPaletteItems(models []aiModel, skills []chatSkill, current aiModel, query, filterGroup string) []chatCommandPaletteItem {
	query = strings.ToLower(strings.TrimSpace(query))
	items := make([]chatCommandPaletteItem, 0, len(models)+len(skills))
	if filterGroup == "" || filterGroup == chatCommandPanel || filterGroup == "models" {
		for index, model := range models {
			subtitle := model.Provider
			if model.ProviderAlias != "" {
				subtitle += " (" + model.ProviderAlias + ")"
			}
			item := chatCommandPaletteItem{group: "models", sourceIndex: index, title: model.Name, subtitle: subtitle, current: model == current}
			item.searchText = "model 模型 " + model.Name + " " + model.Provider + " " + model.ProviderAlias
			if query == "" || strings.Contains(strings.ToLower(item.searchText), query) || strings.Contains(strings.ToLower(item.subtitle), query) {
				items = append(items, item)
			}
		}
	}
	if filterGroup == "" || filterGroup == chatCommandPanel || filterGroup == "skills" {
		for index, skill := range skills {
			subtitle := skill.SourceName
			if subtitle == "" {
				subtitle = skill.Source
			}
			if skill.Description != "" {
				if subtitle != "" {
					subtitle += " · "
				}
				subtitle += skill.Description
			}
			item := chatCommandPaletteItem{group: "skills", sourceIndex: index, title: skill.Name, subtitle: subtitle}
			item.searchText = "skill 技能 " + skill.Name + " " + skill.Description + " " + skill.Source + " " + skill.SourceName
			if query == "" || strings.Contains(strings.ToLower(item.searchText), query) || strings.Contains(strings.ToLower(item.subtitle), query) {
				items = append(items, item)
			}
		}
	}
	return items
}

// chatCommandItemOffset includes each visible group header before the target row.
func chatCommandItemOffset(items []chatCommandPaletteItem, target int) float32 {
	offset := float32(0)
	group := ""
	for index, item := range items {
		if item.group != group {
			group = item.group
			offset += chatCatalogGroupHeaderHeight
		}
		if index == target {
			return offset
		}
		offset += chatCatalogRowHeight
	}
	return offset
}

func chatCommandContentHeight(items []chatCommandPaletteItem) float32 {
	if len(items) == 0 {
		return 0
	}
	return chatCommandItemOffset(items, len(items)-1) + chatCatalogRowHeight
}

// cloneChatData isolates nested message slices before transport and render state diverge.
func cloneChatData(source chatData) chatData {
	cloned := source
	cloned.Conversations = make([]chatConversation, len(source.Conversations))
	for index, conversation := range source.Conversations {
		cloned.Conversations[index] = conversation
		cloned.Conversations[index].Images = append([]woxImage(nil), conversation.Images...)
		cloned.Conversations[index].SkillRefs = append([]chatSkillRef(nil), conversation.SkillRefs...)
		if conversation.ToolCallInfo.Arguments != nil {
			cloned.Conversations[index].ToolCallInfo.Arguments = make(map[string]any, len(conversation.ToolCallInfo.Arguments))
			for key, value := range conversation.ToolCallInfo.Arguments {
				cloned.Conversations[index].ToolCallInfo.Arguments[key] = value
			}
		}
	}
	cloned.CompactionEntries = append([]json.RawMessage(nil), source.CompactionEntries...)
	cloned.DebugTrace = append(json.RawMessage(nil), source.DebugTrace...)
	return cloned
}

// chatSummary drops heavy message data while retaining one history-list entry.
func chatSummary(source chatData) chatData {
	summary := source
	summary.Conversations = nil
	summary.CompactionEntries = nil
	summary.DebugTrace = nil
	summary.IsSummary = true
	return summary
}

// sortChatSummaries keeps the most recently updated conversation first.
func sortChatSummaries(chats []chatData) {
	sort.SliceStable(chats, func(i, j int) bool {
		return chats[i].UpdatedAt > chats[j].UpdatedAt
	})
}

// upsertChatSummaryLocked mirrors active and background stream updates into history.
func upsertChatSummaryLocked(state *chatPreviewState, chat chatData) {
	if state == nil || chat.ID == "" || len(chat.Conversations) == 0 {
		return
	}
	summary := chatSummary(chat)
	for index := range state.chats {
		if state.chats[index].ID == chat.ID {
			state.chats[index] = summary
			sortChatSummaries(state.chats)
			return
		}
	}
	state.chats = append(state.chats, summary)
	sortChatSummaries(state.chats)
}

// cloneChatQuestion returns an immutable question snapshot for frame building.
func cloneChatQuestion(source *aiQuestion) *aiQuestion {
	if source == nil {
		return nil
	}
	cloned := *source
	cloned.Options = append([]aiQuestionOption(nil), source.Options...)
	for index := range cloned.Options {
		if source.Options[index].Extra != nil {
			cloned.Options[index].Extra = make(map[string]string, len(source.Options[index].Extra))
			for key, value := range source.Options[index].Extra {
				cloned.Options[index].Extra[key] = value
			}
		}
	}
	return &cloned
}

// snapshotChatPreviewLocked copies mutable chat and editor state during a UI-thread snapshot.
func snapshotChatPreviewLocked(state *chatPreviewState) *chatPreviewSnapshot {
	if state == nil {
		return nil
	}
	snapshot := &chatPreviewSnapshot{
		key:              state.key,
		queryID:          state.queryID,
		resultID:         state.resultID,
		chat:             cloneChatData(state.chat),
		active:           state.active,
		scroll:           state.scroll,
		loading:          state.loading,
		sending:          state.sending,
		error:            state.error,
		chats:            append([]chatData(nil), state.chats...),
		panel:            state.panel,
		panelQuery:       state.panelQuery,
		panelSelected:    state.panelSelected,
		panelScroll:      state.panelScroll,
		panelViewport:    state.panelViewport,
		question:         cloneChatQuestion(state.question),
		questionSelected: state.questionSelected,
		expandedRounds:   make(map[string]bool, len(state.expandedRounds)),
	}
	for roundID, expanded := range state.expandedRounds {
		snapshot.expandedRounds[roundID] = expanded
	}
	if state.editor != nil {
		snapshot.editing = state.editor.State()
	}
	if state.questionEditor != nil {
		snapshot.questionEditing = state.questionEditor.State()
	}
	return snapshot
}

// chatPreviewDataAndKey validates the payload and derives its stable controller identity.
func chatPreviewDataAndKey(result queryResult, preview queryPreview) (chatPreviewData, string, error) {
	var data chatPreviewData
	if err := json.Unmarshal([]byte(preview.PreviewData), &data); err != nil {
		return chatPreviewData{}, "", fmt.Errorf("decode chat preview: %w", err)
	}
	if data.ActiveChat.ID == "" {
		return chatPreviewData{}, "", fmt.Errorf("chat preview has no active chat id")
	}
	hash := sha256.Sum256([]byte(preview.PreviewData))
	return data, fmt.Sprintf("%s|%s|%x", result.QueryID, result.ID, hash), nil
}

// activateChatPreview bootstraps shared chat state without overwriting newer streamed snapshots.
func (a *App) activateChatPreview(result queryResult, preview queryPreview) error {
	data, key, err := chatPreviewDataAndKey(result, preview)
	if err != nil {
		return err
	}
	changed := a.chatPreview != nil && a.chatPreview.key != key
	keepFullscreen := a.chatFullscreen
	if changed {
		a.deactivateChatPreview()
		if keepFullscreen {
			a.chatFullscreen = true
		}
	}

	shouldLoad := false
	loadChatID := ""
	if a.chatPreview == nil || a.chatPreview.key != key {
		a.chatPreview = &chatPreviewState{
			key:            key,
			queryID:        result.QueryID,
			resultID:       result.ID,
			chat:           cloneChatData(data.ActiveChat),
			chats:          append([]chatData(nil), data.Chats...),
			editor:         woxui.NewTextEditor(""),
			active:         a.chatFullscreen,
			autoFollow:     true,
			scroll:         float32(math.MaxFloat32),
			expandedRounds: make(map[string]bool),
		}
		if data.ActiveChatID != "" {
			a.chatPreview.loading = true
			a.chatPreview.revision++
			shouldLoad = true
			loadChatID = data.ActiveChatID
		}
		sortChatSummaries(a.chatPreview.chats)
	}
	revision := a.chatPreview.revision

	if shouldLoad {
		util.Go(a.lifecycleCtx, "load chat preview", func() {
			a.loadChatPreview(key, loadChatID, revision)
		})
	}
	return nil
}

// chatPreviewSnapshotFor returns the state prepared by the lifecycle coordinator.
func (a *App) chatPreviewSnapshotFor(result queryResult, preview queryPreview) (*chatPreviewSnapshot, error) {
	_, key, err := chatPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	if a.chatPreview == nil || a.chatPreview.key != key {
		return nil, fmt.Errorf("chat preview is not ready")
	}
	snapshot := snapshotChatPreviewLocked(a.chatPreview)
	snapshot.models = a.aiSettings.Models()
	snapshot.modelsLoading = a.aiSettings.ModelsLoading()
	snapshot.modelsError = a.aiSettings.ModelsError()
	snapshot.skills = a.aiSettings.Skills()
	snapshot.skillsLoading = a.aiSettings.SkillsLoading()
	snapshot.skillsError = a.aiSettings.SkillsError()
	return snapshot, nil
}

// loadChatPreview resolves one lightweight history entry through the chat service.
func (a *App) loadChatPreview(key, chatID string, revision uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loaded, err := a.services.ChatByID(ctx, a.sessionID, chatID)
	chat := chatData{}
	if err == nil {
		chat, err = chatDataFromContract(loaded)
	}
	if dispatchErr := a.runOnUI("apply chat preview", func() {
		if state := a.chatPreview; state != nil && state.key == key && state.revision == revision {
			state.loading = false
			if state.remoteVersion == 0 {
				if err != nil {
					state.error = err.Error()
				} else {
					state.chat = cloneChatData(chat)
					state.error = ""
					if state.autoFollow {
						state.scroll = float32(math.MaxFloat32)
					}
				}
			}
		}
		_ = a.window.Invalidate()
	}); dispatchErr != nil {
		log.Printf("dispatch chat preview result: %v", dispatchErr)
	}
	if err != nil {
		log.Printf("load chat preview: %v", err)
	}
}

// applyChatResponse replaces only the matching active chat and preserves local input and scroll ownership.
func (a *App) applyChatResponse(chat chatData) {
	if chat.ID == "" {
		return
	}
	state := a.chatPreview
	if state == nil {
		return
	}
	upsertChatSummaryLocked(state, chat)
	if state.chat.ID == chat.ID {
		state.chat = cloneChatData(chat)
		state.remoteVersion++
		state.loading = false
		state.sending = false
		state.error = ""
		if state.autoFollow {
			state.scroll = float32(math.MaxFloat32)
		}
	}
	_ = a.window.Invalidate()
}

// toggleChatDisclosure expands or collapses a round, tool group, or tool detail.
func (a *App) toggleChatDisclosure(disclosureID string) {
	state := a.chatPreview
	if state == nil || disclosureID == "" {
		return
	}
	if state.expandedRounds == nil {
		state.expandedRounds = make(map[string]bool)
	}
	state.expandedRounds[disclosureID] = !state.expandedRounds[disclosureID]
	_ = a.window.Invalidate()
}

// toggleChatPanel opens the history or model catalog and loads shared resources on demand.
func (a *App) toggleChatPanel(panel string) {
	requestModels := false
	requestSkills := false
	editorActive := false
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.question != nil {
		return
	}
	if state.panel == panel {
		state.panel = ""
		state.panelQuery = ""
	} else {
		state.panel = panel
		state.panelQuery = ""
		state.panelScroll = 0
		state.panelViewport = 0
		state.panelMaxScroll = 0
		state.panelSelected = 0
		if panel == "history" {
			for index, chat := range state.chats {
				if chat.ID == state.chat.ID {
					state.panelSelected = index
					break
				}
			}
		}
		if panel == "models" || panel == chatCommandPanel {
			for index, model := range a.aiSettings.Models() {
				if model == state.chat.Model {
					state.panelSelected = index
					break
				}
			}
			requestModels = !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
			if requestModels {
				a.aiSettings.SetModelsLoading(true)
			}
		}
		if panel == "skills" || panel == chatCommandPanel {
			requestSkills = !a.aiSettings.SkillsLoaded() && !a.aiSettings.SkillsLoading()
			if requestSkills {
				a.aiSettings.SetSkillsLoading(true)
			}
		}
	}
	state.active = true
	editorActive = state.panel == ""
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "load AI skills for chat", a.loadAISkills)
	}
	a.updateChatTextInput(editorActive)
	_ = a.window.Invalidate()
}

// reloadChatResource invalidates only catalogs affected by a core resource notification.
func (a *App) reloadChatResourceName(resource string) {
	if resource != "models" && resource != "skills" && resource != "all" {
		return
	}
	requestModels := false
	requestSkills := false
	if resource == "models" || resource == "all" {
		a.aiSettings.ResetModels()
		if state := a.chatPreview; state != nil && (state.panel == "models" || state.panel == chatCommandPanel) && !a.aiSettings.ModelsLoading() {
			a.aiSettings.SetModelsLoading(true)
			requestModels = true
		}
	}
	if resource == "skills" || resource == "all" {
		a.aiSettings.ResetSkills()
		if state := a.chatPreview; state != nil && (state.panel == "skills" || state.panel == chatCommandPanel) && !a.aiSettings.SkillsLoading() {
			a.aiSettings.SetSkillsLoading(true)
			requestSkills = true
		}
	}
	if requestModels {
		util.Go(a.lifecycleCtx, "reload AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "reload AI skills for chat", a.loadAISkills)
	}
}

// loadAISkills shares the enabled skill catalog with chat composition.
func (a *App) loadAISkills() {
	a.aiSettings.LoadAISkills(context.Background(), a.services, a.sessionID, func(skills []chatSkill) {
		if skills == nil {
			log.Printf("load AI skills: see controller error")
			_ = a.window.Invalidate()
			return
		}
		if a.chatPreview != nil && (a.chatPreview.panel == "skills" || a.chatPreview.panel == chatCommandPanel) {
			a.chatPreview.panelSelected = 0
			a.chatPreview.panelScroll = 0
			a.chatPreview.panelViewport = 0
		}
		_ = a.window.Invalidate()
	})
}

// startNewChat resets the active draft while retaining the user's current model choice.
func (a *App) startNewChat() {
	questionID := ""
	requestDefault := false
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.chat.IsStreaming || state.sending {
		state.error = "Stop the active response before starting another chat."
		_ = a.window.Invalidate()
		return
	}
	if state.question != nil {
		questionID = state.question.QuestionID
	}
	now := time.Now().UnixMilli()
	model := state.chat.Model
	state.chat = chatData{ID: newID(), Model: model, CreatedAt: now, UpdatedAt: now}
	state.editor.SetText("", false)
	state.loading = false
	state.sending = false
	state.error = ""
	state.remoteVersion = 0
	state.panel = ""
	state.question = nil
	state.questionEditor = nil
	clear(state.expandedRounds)
	state.autoFollow = true
	state.scroll = float32(math.MaxFloat32)
	state.revision++
	key := state.key
	revision := state.revision
	requestDefault = model.Name == ""
	if questionID != "" {
		util.Go(a.lifecycleCtx, "cancel previous AI question", func() {
			a.answerAIQuestion(questionID, "User cancelled")
		})
	}
	if requestDefault {
		util.Go(a.lifecycleCtx, "load default chat model", func() {
			a.loadDefaultChatModel(key, revision)
		})
	}
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// loadDefaultChatModel fills a new draft without overwriting a model selected in the meantime.
func (a *App) loadDefaultChatModel(key string, revision uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loaded, err := a.services.DefaultChatModel(ctx, a.sessionID)
	model := aiModel{Name: loaded.Name, Provider: string(loaded.Provider), ProviderAlias: loaded.ProviderAlias}
	if dispatchErr := a.runOnUI("apply default chat model", func() {
		if state := a.chatPreview; state != nil && state.key == key && state.revision == revision && state.chat.Model.Name == "" {
			if err != nil {
				state.error = err.Error()
			} else {
				state.chat.Model = model
			}
		}
		_ = a.window.Invalidate()
	}); dispatchErr != nil {
		log.Printf("dispatch default chat model: %v", dispatchErr)
	}
	if err != nil {
		log.Printf("load default chat model: %v", err)
	}
}

// selectChatHistory loads a summary only when it is not already the active full conversation.
func (a *App) selectChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	questionID := ""
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.chat.ID != chatID && (state.chat.IsStreaming || state.sending) {
		state.error = "Stop the active response before switching conversations."
		_ = a.window.Invalidate()
		return
	}
	if state.chat.ID == chatID && !state.chat.IsSummary && len(state.chat.Conversations) > 0 {
		state.panel = ""
		state.active = true
		a.updateChatTextInput(true)
		_ = a.window.Invalidate()
		return
	}
	var selected *chatData
	for index := range state.chats {
		if state.chats[index].ID == chatID {
			copy := state.chats[index]
			selected = &copy
			break
		}
	}
	if selected == nil {
		return
	}
	if state.question != nil {
		questionID = state.question.QuestionID
	}
	state.chat = *selected
	clear(state.expandedRounds)
	state.editor.SetText("", false)
	state.loading = true
	state.sending = false
	state.error = ""
	state.remoteVersion = 0
	state.panel = ""
	state.question = nil
	state.questionEditor = nil
	state.autoFollow = true
	state.scroll = float32(math.MaxFloat32)
	state.revision++
	key := state.key
	revision := state.revision
	if questionID != "" {
		util.Go(a.lifecycleCtx, "cancel AI question before chat switch", func() {
			a.answerAIQuestion(questionID, "User cancelled")
		})
	}
	util.Go(a.lifecycleCtx, "load selected chat history", func() {
		a.loadChatPreview(key, chatID, revision)
	})
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// deleteChatHistory removes persisted history through core and starts a draft if it was active.
func (a *App) deleteChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	if state := a.chatPreview; state != nil && state.chat.ID == chatID && (state.chat.IsStreaming || state.sending) {
		state.error = "Stop the active response before deleting this chat."
		_ = a.window.Invalidate()
		return
	}
	util.Go(a.lifecycleCtx, "delete chat history", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.services.DeleteChat(ctx, a.sessionID, chatID)
		activeDeleted := false
		if dispatchErr := a.runOnUI("apply deleted chat history", func() {
			if state := a.chatPreview; state != nil {
				if err != nil {
					state.error = err.Error()
				} else {
					state.chats = slices.DeleteFunc(state.chats, func(chat chatData) bool { return chat.ID == chatID })
					activeDeleted = state.chat.ID == chatID
					state.panelSelected = min(state.panelSelected, max(0, len(state.chats)-1))
				}
			}
			if activeDeleted {
				a.startNewChat()
			}
			_ = a.window.Invalidate()
		}); dispatchErr != nil {
			log.Printf("dispatch deleted chat history: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("delete chat history: %v", err)
		}
	})
}

// selectChatModel applies one catalog entry to the draft and closes the catalog.
func (a *App) selectChatModel(index int) {
	state := a.chatPreview
	if state == nil {
		return
	}
	model, ok := a.aiSettings.ModelAt(index)
	if !ok {
		return
	}
	if state.chat.IsStreaming || state.sending {
		state.error = "Stop the active response before changing models."
		state.panel = ""
		state.active = true
		a.updateChatTextInput(true)
		_ = a.window.Invalidate()
		return
	}
	if state.panel == chatCommandPanel {
		replaceChatSlashToken(state.editor, "")
	}
	state.chat.Model = model
	state.panelSelected = index
	state.panel = ""
	state.panelQuery = ""
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// insertChatSkill adds the stable inline tag that core expands through SkillRefs.
func (a *App) insertChatSkill(index int) {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	skill, ok := a.aiSettings.SkillAt(index)
	if !ok {
		return
	}
	tag := "{skill:" + skill.Name + "}"
	if state.panel == chatCommandPanel {
		replaceChatSlashToken(state.editor, tag)
	} else {
		state.editor.InsertText(tag + " ")
	}
	state.panelSelected = index
	state.panel = ""
	state.panelQuery = ""
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// replaceChatSlashToken replaces the active token while preserving surrounding message text.
func replaceChatSlashToken(editor *woxui.TextEditor, replacement string) {
	if editor == nil {
		return
	}
	state := editor.State()
	token, ok := findChatSlashToken(state)
	if !ok {
		if replacement != "" {
			editor.InsertText(replacement)
		}
		return
	}
	runes := []rune(state.Text)
	before := append([]rune(nil), runes[:token.start]...)
	after := runes[token.end:]
	inserted := []rune(replacement)
	if replacement == "" && len(before) > 0 && len(after) > 0 && !unicode.IsSpace(before[len(before)-1]) && !unicode.IsSpace(after[0]) {
		inserted = []rune{' '}
	}
	next := make([]rune, 0, len(before)+len(inserted)+len(after))
	next = append(next, before...)
	next = append(next, inserted...)
	next = append(next, after...)
	editor.SetText(string(next), false)
	editor.SetCaret(len(before) + len(inserted))
}

// chatSkillRefsFromText resolves unique inline tags against the current enabled catalog.
func chatSkillRefsFromText(text string, skills []chatSkill) []chatSkillRef {
	matches := chatSkillTagPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]chatSkillRef, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := strings.TrimSpace(match[1])
		for _, skill := range skills {
			if skill.Name != name || seen[skill.ID] {
				continue
			}
			path := skill.ManifestPath
			if path == "" {
				path = skill.Path
			}
			refs = append(refs, chatSkillRef{ID: skill.ID, Name: skill.Name, Path: path, Source: skill.Source})
			seen[skill.ID] = true
			break
		}
	}
	return refs
}

func unresolvedChatSkillTag(text string, skills []chatSkill) string {
	for _, match := range chatSkillTagPattern.FindAllStringSubmatch(text, -1) {
		name := strings.TrimSpace(match[1])
		found := false
		for _, skill := range skills {
			if skill.Name == name {
				found = true
				break
			}
		}
		if !found {
			return name
		}
	}
	return ""
}

// closeChatPanel returns keyboard and IME ownership to the chat composer.
func (a *App) closeChatPanel() {
	if state := a.chatPreview; state != nil {
		state.panel = ""
		state.panelQuery = ""
		state.active = true
	}
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// moveChatPanelSelection follows Flutter's clamped command-palette navigation.
func (a *App) moveChatPanelSelection(delta int) {
	state := a.chatPreview
	if state == nil || state.panel == "" {
		return
	}
	count := len(state.chats)
	var commands []chatCommandPaletteItem
	if state.panel != "history" {
		commands = chatCommandPaletteItems(a.aiSettings.Models(), a.aiSettings.Skills(), state.chat.Model, state.panelQuery, state.panel)
		count = len(commands)
	}
	if count > 0 {
		state.panelSelected = min(max(0, state.panelSelected+delta), count-1)
		if state.panel == "history" {
			ensureChatPanelSelectionVisibleLocked(state, count)
		} else {
			ensureChatCommandSelectionVisibleLocked(state, commands)
		}
	}
	_ = a.window.Invalidate()
}

// activateChatPanelSelection applies the selected history or model row.
func (a *App) activateChatPanelSelection() {
	state := a.chatPreview
	if state == nil {
		return
	}
	panel := state.panel
	selected := state.panelSelected
	chatID := ""
	if panel == "history" && selected >= 0 && selected < len(state.chats) {
		chatID = state.chats[selected].ID
	}
	if panel == "history" {
		a.selectChatHistory(chatID)
	} else {
		items := chatCommandPaletteItems(a.aiSettings.Models(), a.aiSettings.Skills(), state.chat.Model, state.panelQuery, panel)
		if selected < 0 || selected >= len(items) {
			return
		}
		if items[selected].group == "models" {
			a.selectChatModel(items[selected].sourceIndex)
		} else {
			a.insertChatSkill(items[selected].sourceIndex)
		}
	}
}

// setChatPanelViewport records the current catalog extent for scrolling and keyboard reveal.
func (a *App) setChatPanelViewport(height float32) {
	if state := a.chatPreview; state != nil {
		initialize := state.panelViewport <= 0
		state.panelViewport = max(float32(1), height)
		count := len(state.chats)
		commands := []chatCommandPaletteItem(nil)
		if state.panel != "history" {
			commands = chatCommandPaletteItems(a.aiSettings.Models(), a.aiSettings.Skills(), state.chat.Model, state.panelQuery, state.panel)
			count = len(commands)
		}
		if initialize {
			if state.panel == "history" {
				ensureChatPanelSelectionVisibleLocked(state, count)
			} else {
				ensureChatCommandSelectionVisibleLocked(state, commands)
			}
		} else {
			contentHeight := chatHistoryContentHeight(state.chats, time.Now())
			if state.panel != "history" {
				contentHeight = chatCommandContentHeight(commands)
			}
			maxOffset := max(float32(0), contentHeight-state.panelViewport)
			state.panelScroll = min(max(float32(0), state.panelScroll), maxOffset)
		}
	}
}

// ensureChatPanelSelectionVisibleLocked reveals only keyboard-driven selection changes.
func ensureChatPanelSelectionVisibleLocked(state *chatPreviewState, count int) {
	if state == nil || count <= 0 {
		return
	}
	const rowHeight = chatCatalogRowHeight
	maxOffset := max(float32(0), float32(count)*rowHeight-state.panelViewport)
	rowTop := float32(state.panelSelected) * rowHeight
	rowBottom := rowTop + rowHeight
	if rowTop < state.panelScroll {
		state.panelScroll = rowTop
	} else if rowBottom > state.panelScroll+state.panelViewport {
		state.panelScroll = rowBottom - state.panelViewport
	}
	state.panelScroll = min(max(float32(0), state.panelScroll), maxOffset)
}

// ensureChatCommandSelectionVisibleLocked accounts for grouped headers when revealing a row.
func ensureChatCommandSelectionVisibleLocked(state *chatPreviewState, items []chatCommandPaletteItem) {
	if state == nil || len(items) == 0 {
		return
	}
	state.panelSelected = min(max(0, state.panelSelected), len(items)-1)
	rowTop := chatCommandItemOffset(items, state.panelSelected)
	rowBottom := rowTop + chatCatalogRowHeight
	if rowTop < state.panelScroll {
		state.panelScroll = rowTop
	} else if rowBottom > state.panelScroll+state.panelViewport {
		state.panelScroll = rowBottom - state.panelViewport
	}
	maxOffset := max(float32(0), chatCommandContentHeight(items)-state.panelViewport)
	state.panelScroll = min(max(float32(0), state.panelScroll), maxOffset)
}

// scrollChatPanel applies pointer-wheel movement without changing the selected row.
func (a *App) scrollChatPanel(delta float32) {
	state := a.chatPreview
	if state == nil || state.panel == "" {
		return
	}
	contentHeight := chatHistoryContentHeight(state.chats, time.Now())
	if state.panel != "history" {
		items := chatCommandPaletteItems(a.aiSettings.Models(), a.aiSettings.Skills(), state.chat.Model, state.panelQuery, state.panel)
		contentHeight = chatCommandContentHeight(items)
	}
	maxOffset := max(float32(0), contentHeight-state.panelViewport)
	state.panelScroll = min(max(float32(0), state.panelScroll+delta), maxOffset)
	_ = a.window.Invalidate()
}

// clampChatDebugScroll records the JSON inspector extent derived by the shared text layout.
func (a *App) clampChatDebugScroll(maxOffset float32) {
	if state := a.chatPreview; state != nil && state.panel == "debug" {
		state.panelMaxScroll = max(float32(0), maxOffset)
		state.panelScroll = min(max(float32(0), state.panelScroll), state.panelMaxScroll)
	}
}

// scrollChatDebugPanel applies pointer and keyboard movement to the portable trace inspector.
func (a *App) scrollChatDebugPanel(delta float32) {
	if state := a.chatPreview; state != nil && state.panel == "debug" {
		state.panelScroll = min(max(float32(0), state.panelScroll+delta), state.panelMaxScroll)
	}
	_ = a.window.Invalidate()
}

// applyTypedAIQuestion routes ask_user into the visible shared chat surface and cancels if no chat can answer it.
func (a *App) applyTypedAIQuestion(question aiQuestion) error {
	if question.QuestionID == "" {
		return fmt.Errorf("AI question has no QuestionId")
	}
	options := question.Options[:0]
	for _, option := range question.Options {
		if option.Value == "" {
			option.Value = option.Title
		}
		if option.Title == "" {
			option.Title = option.Value
		}
		if option.Title != "" {
			options = append(options, option)
		}
	}
	question.Options = options

	state := a.chatPreview
	selectedChatVisible := false
	if state != nil {
		selectedChatVisible = a.selected >= 0 && a.selected < len(a.results) && a.results[a.selected].Preview.PreviewType == "chat" && a.results[a.selected].ID == state.resultID
	}
	if state == nil || !a.visible || !selectedChatVisible {
		util.Go(a.lifecycleCtx, "cancel hidden AI question", func() {
			a.answerAIQuestion(question.QuestionID, "User cancelled")
		})
		return nil
	}
	if state.question != nil && state.question.QuestionID != question.QuestionID {
		previousID := state.question.QuestionID
		util.Go(a.lifecycleCtx, "cancel replaced AI question", func() {
			a.answerAIQuestion(previousID, "User cancelled")
		})
	}
	state.question = &question
	state.questionEditor = woxui.NewTextEditor("")
	state.panel = ""
	state.questionSelected = 0
	for index, option := range question.Options {
		if option.Recommended {
			state.questionSelected = index
			break
		}
	}
	state.active = true
	state.error = ""
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
	return nil
}

// answerAIQuestion resolves the core-side tool wait without blocking the UI thread.
func (a *App) answerAIQuestion(questionID, answer string) {
	if questionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.services.AnswerAIQuestion(ctx, a.sessionID, questionID, answer); err != nil {
		log.Printf("answer AI question: %v", err)
	}
}

// submitAIQuestionAnswer clears the overlay before resolving the pending core request.
func (a *App) submitAIQuestionAnswer(answer string) {
	state := a.chatPreview
	if state == nil || state.question == nil {
		return
	}
	questionID := state.question.QuestionID
	state.question = nil
	state.questionEditor = nil
	state.questionSelected = 0
	state.error = ""
	util.Go(a.lifecycleCtx, "answer AI question", func() {
		a.answerAIQuestion(questionID, answer)
	})
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// submitSelectedAIQuestionAnswer maps the selected choice or free text to the stable option value.
func (a *App) submitSelectedAIQuestionAnswer() {
	state := a.chatPreview
	if state == nil || state.question == nil {
		return
	}
	question := cloneChatQuestion(state.question)
	selected := state.questionSelected
	answer := ""
	if len(question.Options) == 0 && state.questionEditor != nil {
		answer = strings.TrimSpace(state.questionEditor.State().Text)
	} else if selected >= 0 && selected < len(question.Options) {
		if selected == len(question.Options)-1 && state.questionEditor != nil {
			answer = strings.TrimSpace(state.questionEditor.State().Text)
		}
		if answer == "" {
			answer = question.Options[selected].Value
		}
	}
	if answer == "" {
		answer = "User cancelled"
	}
	a.submitAIQuestionAnswer(answer)
}

// selectAIQuestionOption changes the pending choice without submitting it.
func (a *App) selectAIQuestionOption(index int) {
	if state := a.chatPreview; state != nil && state.question != nil && index >= 0 && index < len(state.question.Options) {
		state.questionSelected = index
	}
	_ = a.window.Invalidate()
}

// beginChatRequestLocked moves a prepared chat into its shared UI-owned streaming state.
func beginChatRequestLocked(state *chatPreviewState) (string, uint64, chatData) {
	state.chat.IsStreaming = true
	upsertChatSummaryLocked(state, state.chat)
	state.sending = true
	state.error = ""
	state.autoFollow = true
	state.scroll = float32(math.MaxFloat32)
	state.revision++
	return state.key, state.revision, cloneChatData(state.chat)
}

// postChatRequest sends one immutable chat snapshot and reconciles service failure with the current revision.
func (a *App) postChatRequest(key string, revision uint64, chat chatData) {
	util.Go(a.lifecycleCtx, "post chat request", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// DebugTrace is an incoming development snapshot and must not be echoed into the next model request.
		chat.DebugTrace = nil
		payload, err := chatDataToContract(chat)
		if err == nil {
			err = a.services.Chat(ctx, a.sessionID, payload)
		}
		if dispatchErr := a.runOnUI("apply chat request result", func() {
			if current := a.chatPreview; current != nil && current.key == key && current.revision == revision {
				current.sending = false
				if err != nil {
					current.chat.IsStreaming = false
					current.error = err.Error()
				}
			}
			_ = a.window.Invalidate()
		}); dispatchErr != nil {
			log.Printf("dispatch chat request result: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("send chat message: %v", err)
		}
	})
}

// sendChatMessage appends the local user turn before core begins pushing authoritative snapshots.
func (a *App) sendChatMessage() {
	state := a.chatPreview
	if state == nil || state.editor == nil || state.loading || state.sending || state.chat.IsStreaming || state.question != nil {
		return
	}
	text := strings.TrimSpace(state.editor.State().Text)
	if text == "" {
		state.error = "Enter a message first."
		_ = a.window.Invalidate()
		return
	}
	if strings.TrimSpace(state.chat.Model.Name) == "" {
		state.error = "Select an AI model in Wox settings first."
		_ = a.window.Invalidate()
		return
	}
	hasSkillTags := chatSkillTagPattern.MatchString(text)
	if hasSkillTags && !a.aiSettings.SkillsLoaded() {
		requestSkills := !a.aiSettings.SkillsLoading()
		if requestSkills {
			a.aiSettings.SetSkillsLoading(true)
		}
		state.error = "Loading skills; send again when the catalog is ready."
		if requestSkills {
			util.Go(a.lifecycleCtx, "load AI skills before chat send", a.loadAISkills)
		}
		_ = a.window.Invalidate()
		return
	}
	now := time.Now().UnixMilli()
	skills := a.aiSettings.Skills()
	skillRefs := chatSkillRefsFromText(text, skills)
	if unresolved := unresolvedChatSkillTag(text, skills); unresolved != "" {
		state.error = fmt.Sprintf("Unknown or disabled skill: %s", unresolved)
		_ = a.window.Invalidate()
		return
	}
	state.chat.Conversations = append(state.chat.Conversations, chatConversation{ID: newID(), Role: "user", Text: text, SkillRefs: skillRefs, Timestamp: now})
	state.chat.UpdatedAt = now
	state.editor.SetText("", false)
	key, revision, chat := beginChatRequestLocked(state)
	_ = a.window.Invalidate()
	a.postChatRequest(key, revision, chat)
}

// stopChatMessage cancels the active core stream while leaving its last snapshot visible.
func (a *App) stopChatMessage() {
	state := a.chatPreview
	if state == nil || state.chat.ID == "" || (!state.chat.IsStreaming && !state.sending) {
		return
	}
	chatID := state.chat.ID
	util.Go(a.lifecycleCtx, "stop chat message", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		stopped, err := a.services.StopChat(ctx, a.sessionID, chatID)
		if dispatchErr := a.runOnUI("apply stopped chat", func() {
			if state := a.chatPreview; state != nil && state.chat.ID == chatID {
				if err != nil {
					state.error = err.Error()
				} else {
					state.chat.IsStreaming = false
					state.sending = false
					if !stopped {
						state.error = "The chat stream was already stopped."
					}
				}
			}
			_ = a.window.Invalidate()
		}); dispatchErr != nil {
			log.Printf("dispatch stopped chat: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("stop chat message: %v", err)
		}
	})
}

// copyChatText reports clipboard failures inside the chat surface while keeping native details below Window.
func (a *App) copyChatText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if err := a.window.WriteClipboardText(text); err != nil {
		if state := a.chatPreview; state != nil {
			state.error = fmt.Sprintf("Copy failed: %v", err)
		}
		_ = a.window.Invalidate()
	}
}

// editChatConversation restores a user turn into the composer and discards the stale branch after it.
func (a *App) editChatConversation(messageID string) {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	if state.chat.IsStreaming || state.sending || state.question != nil {
		state.error = "Stop the active response before editing a message."
		_ = a.window.Invalidate()
		return
	}
	messageIndex := slices.IndexFunc(state.chat.Conversations, func(message chatConversation) bool {
		return message.ID == messageID && message.Role == "user"
	})
	if messageIndex < 0 {
		state.error = "The user message is no longer available."
		_ = a.window.Invalidate()
		return
	}
	text := state.chat.Conversations[messageIndex].Text
	state.chat.Conversations = slices.Clone(state.chat.Conversations[:messageIndex])
	state.chat.CompactionEntries = nil
	state.chat.DebugTrace = nil
	state.chat.UpdatedAt = time.Now().UnixMilli()
	state.editor.SetText(text, false)
	state.panel = ""
	state.active = true
	state.autoFollow = true
	state.scroll = float32(math.MaxFloat32)
	state.error = ""
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// regenerateChatConversation resends the user turn preceding an assistant response through the normal chat channel.
func (a *App) regenerateChatConversation(messageID string) {
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.chat.IsStreaming || state.sending || state.question != nil {
		state.error = "Stop the active response before retrying it."
		_ = a.window.Invalidate()
		return
	}
	assistantIndex := slices.IndexFunc(state.chat.Conversations, func(message chatConversation) bool {
		return message.ID == messageID && message.Role == "assistant"
	})
	userIndex := -1
	for index := assistantIndex - 1; index >= 0; index-- {
		if state.chat.Conversations[index].Role == "user" {
			userIndex = index
			break
		}
	}
	if assistantIndex < 0 || userIndex < 0 {
		state.error = "No user message is available for this response."
		_ = a.window.Invalidate()
		return
	}
	state.chat.Conversations = slices.Clone(state.chat.Conversations[:userIndex+1])
	state.chat.CompactionEntries = nil
	state.chat.DebugTrace = nil
	state.chat.UpdatedAt = time.Now().UnixMilli()
	key, revision, chat := beginChatRequestLocked(state)
	_ = a.window.Invalidate()
	a.postChatRequest(key, revision, chat)
}

// onChatPreviewKey keeps chat editing and ask_user behavior identical on every platform.
func (a *App) onChatPreviewKey(event woxui.KeyEvent) bool {
	// Key releases must not repeat palette navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	state := a.chatPreview
	active := state != nil && state.active
	hasQuestion := active && state.question != nil
	panel := ""
	panelSelected := 0
	panelChatID := ""
	if active {
		panel = state.panel
		panelSelected = state.panelSelected
		if panel == "history" && panelSelected >= 0 && panelSelected < len(state.chats) {
			panelChatID = state.chats[panelSelected].ID
		}
	}
	questionOptions := 0
	questionSelected := 0
	if hasQuestion {
		questionOptions = len(state.question.Options)
		questionSelected = state.questionSelected
	}
	if active && event.Key == woxui.Key("b") && event.Modifiers.HasPrimary() {
		// Ctrl/Cmd+B toggles the conversation sidebar, matching Flutter's preview fullscreen shortcut.
		a.toggleChatPanel("history")
		return true
	}
	if panel == "debug" {
		switch event.Key {
		case woxui.KeyEscape:
			a.closeChatPanel()
		case woxui.KeyArrowUp:
			a.scrollChatDebugPanel(-44)
		case woxui.KeyArrowDown, woxui.KeyTab:
			delta := float32(44)
			if event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -delta
			}
			a.scrollChatDebugPanel(delta)
		}
		return true
	}
	if panel != "" {
		switch event.Key {
		case woxui.KeyEscape:
			a.closeChatPanel()
			return true
		case woxui.KeyArrowUp:
			a.moveChatPanelSelection(-1)
			return true
		case woxui.KeyArrowDown, woxui.KeyTab:
			delta := 1
			if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -1
			}
			a.moveChatPanelSelection(delta)
			return true
		case woxui.KeyEnter:
			a.activateChatPanelSelection()
			return true
		case woxui.KeyDelete:
			if panel == "history" {
				a.deleteChatHistory(panelChatID)
				return true
			}
		}
		return false
	}
	if !active {
		return false
	}
	if hasQuestion {
		if event.Key == woxui.KeyEscape {
			a.submitAIQuestionAnswer("User cancelled")
			return true
		}
		if questionOptions > 0 {
			freeTextSelected := questionSelected == questionOptions-1
			switch event.Key {
			case woxui.KeyArrowUp:
				a.moveAIQuestionSelection(-1)
			case woxui.KeyArrowDown, woxui.KeyTab:
				delta := 1
				if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
					delta = -1
				}
				a.moveAIQuestionSelection(delta)
			case woxui.KeyEnter:
				if freeTextSelected && event.Modifiers&woxui.KeyModifierShift != 0 {
					return false
				} else {
					a.submitSelectedAIQuestionAnswer()
				}
			default:
				if freeTextSelected {
					return false
				}
			}
			return true
		}
		if event.Key == woxui.KeyEnter && event.Modifiers&woxui.KeyModifierShift == 0 {
			a.submitSelectedAIQuestionAnswer()
			return true
		}
		return false
	}
	if event.Key == woxui.KeyEscape {
		if a.isPrimary {
			a.exitChatMode()
		} else {
			a.closePreviewWindow()
		}
		return true
	}
	if event.Key == woxui.KeyEnter && event.Modifiers&woxui.KeyModifierShift == 0 {
		a.sendChatMessage()
		return true
	}
	if event.Key == woxui.KeyPageUp || event.Key == woxui.KeyPageDown {
		delta := float32(-240)
		if event.Key == woxui.KeyPageDown {
			delta = 240
		}
		a.scrollChatPreview(delta, float32(math.MaxFloat32))
		return true
	}
	return false
}

// onChatPreviewTextInput routes committed and composing text to the currently visible chat editor.
func (a *App) onChatPreviewTextInput(_ woxui.TextInputEvent) bool {
	state := a.chatPreview
	if state == nil || !state.active {
		return false
	}
	return true
}

func (a *App) editChatKey(event woxui.KeyEvent) {
	if state := a.chatPreview; state != nil && state.active && state.editor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 5}}
		_, changed := handleFormEditorKey(state.editor, definition, event)
		if changed {
			state.error = ""
		}
	}
	_ = a.window.Invalidate()
}

func (a *App) editAIQuestionKey(event woxui.KeyEvent) {
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 4}}
		_, changed := handleFormEditorKey(state.questionEditor, definition, event)
		if changed {
			state.error = ""
		}
	}
	_ = a.window.Invalidate()
}

func (a *App) moveAIQuestionSelection(delta int) {
	if state := a.chatPreview; state != nil && state.question != nil && len(state.question.Options) > 0 {
		state.questionSelected = (state.questionSelected + delta + len(state.question.Options)) % len(state.question.Options)
	}
	_ = a.window.Invalidate()
}

func (a *App) focusChatInput() {
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) focusAIQuestionInput() {
	if state := a.chatPreview; state != nil && state.question != nil {
		state.active = true
	}
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) setChatText(value string) {
	requestModels := false
	requestSkills := false
	if state := a.chatPreview; state != nil && state.editor != nil && state.question == nil {
		state.editor.SetText(value, false)
		state.error = ""
		token, hasToken := findChatSlashToken(state.editor.State())
		if hasToken {
			queryChanged := state.panel != chatCommandPanel || state.panelQuery != token.query
			state.panel = chatCommandPanel
			state.panelQuery = token.query
			state.active = true
			if queryChanged {
				state.panelSelected = 0
				state.panelScroll = 0
				state.panelViewport = 0
			}
			requestModels = !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
			requestSkills = !a.aiSettings.SkillsLoaded() && !a.aiSettings.SkillsLoading()
			if requestModels {
				a.aiSettings.SetModelsLoading(true)
			}
			if requestSkills {
				a.aiSettings.SetSkillsLoading(true)
			}
		} else if state.panel == chatCommandPanel {
			state.panel = ""
			state.panelQuery = ""
		}
	}
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "load AI skills for chat", a.loadAISkills)
	}
	_ = a.window.Invalidate()
}

func (a *App) setAIQuestionText(value string) {
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		state.questionEditor.SetText(value, false)
		state.error = ""
	}
	_ = a.window.Invalidate()
}

// enterChatMode hides launcher chrome while retaining the same native window and shared preview state.
func (a *App) enterChatMode() {
	a.chatFullscreen = true
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.updateChatTextInput(true)
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

// exitChatMode restores query ownership without destroying the in-progress conversation.
func (a *App) exitChatMode() {
	a.chatFullscreen = false
	if state := a.chatPreview; state != nil {
		state.active = false
	}
	a.restoreQueryTextInput()
	// Returning from chat selects the whole query so a new search starts with a clean slate.
	a.editor.SelectAll()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

// deactivateChatPreview releases input ownership and cancels an ask_user request that is no longer visible.
func (a *App) deactivateChatPreview() {
	state := a.chatPreview
	wasActive := state != nil && state.active
	wasFullscreen := a.chatFullscreen
	a.chatFullscreen = false
	questionID := ""
	if state != nil {
		state.active = false
		if state.question != nil {
			questionID = state.question.QuestionID
			state.question = nil
			state.questionEditor = nil
		}
	}
	if questionID != "" {
		util.Go(a.lifecycleCtx, "cancel deactivated AI question", func() {
			a.answerAIQuestion(questionID, "User cancelled")
		})
	}
	if wasActive || wasFullscreen {
		a.restoreQueryTextInput()
		_ = a.window.Invalidate()
	}
}

// resetChatPreview discards state at launcher lifecycle boundaries and unblocks any pending ask_user call.
func (a *App) resetChatPreview() {
	questionID := ""
	if a.chatPreview != nil && a.chatPreview.question != nil {
		questionID = a.chatPreview.question.QuestionID
	}
	a.chatPreview = nil
	a.chatFullscreen = false
	if questionID != "" {
		util.Go(a.lifecycleCtx, "cancel reset AI question", func() {
			a.answerAIQuestion(questionID, "User cancelled")
		})
	}
}

func (a *App) updateChatTextInput(enabled bool) {
	state := woxui.TextInputState{}
	if enabled {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 32, Y: 420, Width: 1, Height: 22}}
	}
	_ = a.window.SetTextInputState(state)
}

// clampChatPreviewScroll records whether future stream updates should keep following the bottom.
func (a *App) clampChatPreviewScroll(maxOffset float32) {
	if state := a.chatPreview; state != nil {
		state.scroll = min(max(float32(0), state.scroll), maxOffset)
		state.autoFollow = maxOffset-state.scroll <= 36
	}
}

// scrollChatPreview preserves manual scrollback until the user returns near the latest message.
func (a *App) scrollChatPreview(delta, maxOffset float32) {
	if delta == 0 {
		return
	}
	if state := a.chatPreview; state != nil {
		state.scroll = min(max(float32(0), state.scroll+delta), maxOffset)
		state.autoFollow = maxOffset-state.scroll <= 36
	}
	_ = a.window.Invalidate()
}
