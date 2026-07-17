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

	"wox/ui/coreclient"
	woxui "wox/ui/runtime"
)

const enterChatModeActionID = "__wox_internal_enter_chat_mode__"

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
	panelSelected    int
	panelScroll      float32
	panelViewport    float32
	panelMaxScroll   float32
	question         *aiQuestion
	questionEditor   *woxui.TextEditor
	questionSelected int
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
	panelSelected    int
	panelScroll      float32
	panelViewport    float32
	question         *aiQuestion
	questionEditing  woxui.TextEditingState
	questionSelected int
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

// snapshotChatPreviewLocked copies mutable chat and editor state while App.mu is held.
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
		panelSelected:    state.panelSelected,
		panelScroll:      state.panelScroll,
		panelViewport:    state.panelViewport,
		question:         cloneChatQuestion(state.question),
		questionSelected: state.questionSelected,
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
	a.mu.RLock()
	changed := a.chatPreview != nil && a.chatPreview.key != key
	keepFullscreen := a.chatFullscreen
	a.mu.RUnlock()
	if changed {
		a.deactivateChatPreview()
		if keepFullscreen {
			a.mu.Lock()
			a.chatFullscreen = true
			a.mu.Unlock()
		}
	}

	shouldLoad := false
	loadChatID := ""
	a.mu.Lock()
	if a.chatPreview == nil || a.chatPreview.key != key {
		a.chatPreview = &chatPreviewState{
			key:        key,
			queryID:    result.QueryID,
			resultID:   result.ID,
			chat:       cloneChatData(data.ActiveChat),
			chats:      append([]chatData(nil), data.Chats...),
			editor:     woxui.NewTextEditor(""),
			active:     a.chatFullscreen,
			autoFollow: true,
			scroll:     float32(math.MaxFloat32),
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
	a.mu.Unlock()

	if shouldLoad {
		go a.loadChatPreview(key, loadChatID, revision)
	}
	return nil
}

// chatPreviewSnapshotFor returns the state prepared by the lifecycle coordinator.
func (a *App) chatPreviewSnapshotFor(result queryResult, preview queryPreview) (*chatPreviewSnapshot, error) {
	_, key, err := chatPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.chatPreview == nil || a.chatPreview.key != key {
		return nil, fmt.Errorf("chat preview is not ready")
	}
	snapshot := snapshotChatPreviewLocked(a.chatPreview)
	snapshot.models = append([]aiModel(nil), a.aiModels...)
	snapshot.modelsLoading = a.aiModelsLoading
	snapshot.modelsError = a.aiModelsError
	snapshot.skills = append([]chatSkill(nil), a.aiSkills...)
	snapshot.skillsLoading = a.aiSkillsLoading
	snapshot.skillsError = a.aiSkillsError
	return snapshot, nil
}

// loadChatPreview resolves lightweight history entries through the existing core endpoint.
func (a *App) loadChatPreview(key, chatID string, revision uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var chat chatData
	err := a.client.Post(ctx, "/ai/chat/get", map[string]any{"chatId": chatID}, &chat)
	a.mu.Lock()
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
	a.mu.Unlock()
	if err != nil {
		log.Printf("load chat preview: %v", err)
	}
	_ = a.window.Invalidate()
}

// applyChatResponse replaces only the matching active chat and preserves local input and scroll ownership.
func (a *App) applyChatResponse(chat chatData) {
	if chat.ID == "" {
		return
	}
	a.mu.Lock()
	state := a.chatPreview
	if state == nil {
		a.mu.Unlock()
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
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// toggleChatPanel opens the history or model catalog and loads shared resources on demand.
func (a *App) toggleChatPanel(panel string) {
	requestModels := false
	requestSkills := false
	editorActive := false
	a.mu.Lock()
	state := a.chatPreview
	if state == nil {
		a.mu.Unlock()
		return
	}
	if state.question != nil {
		a.mu.Unlock()
		return
	}
	if state.panel == panel {
		state.panel = ""
	} else {
		state.panel = panel
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
		if panel == "models" {
			for index, model := range a.aiModels {
				if model == state.chat.Model {
					state.panelSelected = index
					break
				}
			}
			requestModels = !a.aiModelsLoaded && !a.aiModelsLoading
			if requestModels {
				a.aiModelsLoading = true
			}
		}
		if panel == "skills" {
			requestSkills = !a.aiSkillsLoaded && !a.aiSkillsLoading
			if requestSkills {
				a.aiSkillsLoading = true
			}
		}
	}
	state.active = true
	editorActive = state.panel == ""
	a.mu.Unlock()
	if requestModels {
		go a.loadAIModels()
	}
	if requestSkills {
		go a.loadAISkills()
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
	a.mu.Lock()
	if resource == "models" || resource == "all" {
		a.aiModelsLoaded = false
		a.aiModelsError = ""
		if state := a.chatPreview; state != nil && state.panel == "models" && !a.aiModelsLoading {
			a.aiModelsLoading = true
			requestModels = true
		}
	}
	if resource == "skills" || resource == "all" {
		a.aiSkillsLoaded = false
		a.aiSkillsError = ""
		if state := a.chatPreview; state != nil && state.panel == "skills" && !a.aiSkillsLoading {
			a.aiSkillsLoading = true
			requestSkills = true
		}
	}
	a.mu.Unlock()
	if requestModels {
		go a.loadAIModels()
	}
	if requestSkills {
		go a.loadAISkills()
	}
}

// loadAISkills shares the enabled skill catalog with chat composition.
func (a *App) loadAISkills() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var skills []chatSkill
	err := a.client.Post(ctx, "/ai/skills", map[string]any{}, &skills)
	if err == nil {
		skills = slices.DeleteFunc(skills, func(skill chatSkill) bool { return !skill.Enabled || strings.TrimSpace(skill.Name) == "" })
		sort.SliceStable(skills, func(i, j int) bool {
			left := skills[i].SourceName + "\x00" + skills[i].Source + "\x00" + skills[i].Name
			right := skills[j].SourceName + "\x00" + skills[j].Source + "\x00" + skills[j].Name
			return left < right
		})
	}
	a.mu.Lock()
	a.aiSkillsLoading = false
	a.aiSkillsLoaded = true
	if err != nil {
		a.aiSkillsError = err.Error()
	} else {
		a.aiSkills = skills
		a.aiSkillsError = ""
		if a.chatPreview != nil && a.chatPreview.panel == "skills" {
			a.chatPreview.panelSelected = 0
			a.chatPreview.panelScroll = 0
			a.chatPreview.panelViewport = 0
		}
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load AI skills: %v", err)
	}
	_ = a.window.Invalidate()
}

// startNewChat resets the active draft while retaining the user's current model choice.
func (a *App) startNewChat() {
	questionID := ""
	requestDefault := false
	a.mu.Lock()
	state := a.chatPreview
	if state == nil {
		a.mu.Unlock()
		return
	}
	if state.chat.IsStreaming || state.sending {
		state.error = "Stop the active response before starting another chat."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if state.question != nil {
		questionID = state.question.QuestionID
	}
	now := time.Now().UnixMilli()
	model := state.chat.Model
	state.chat = chatData{ID: coreclient.NewID(), Model: model, CreatedAt: now, UpdatedAt: now}
	state.editor.SetText("", false)
	state.loading = false
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
	requestDefault = model.Name == ""
	a.mu.Unlock()
	if questionID != "" {
		go a.answerAIQuestion(questionID, "User cancelled")
	}
	if requestDefault {
		go a.loadDefaultChatModel(key, revision)
	}
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// loadDefaultChatModel fills a new draft without overwriting a model selected in the meantime.
func (a *App) loadDefaultChatModel(key string, revision uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var model aiModel
	err := a.client.Post(ctx, "/ai/model/default", map[string]any{}, &model)
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.key == key && state.revision == revision && state.chat.Model.Name == "" {
		if err != nil {
			state.error = err.Error()
		} else {
			state.chat.Model = model
		}
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load default chat model: %v", err)
	}
	_ = a.window.Invalidate()
}

// selectChatHistory loads a summary only when it is not already the active full conversation.
func (a *App) selectChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	questionID := ""
	a.mu.Lock()
	state := a.chatPreview
	if state == nil {
		a.mu.Unlock()
		return
	}
	if state.chat.ID != chatID && (state.chat.IsStreaming || state.sending) {
		state.error = "Stop the active response before switching conversations."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if state.chat.ID == chatID && !state.chat.IsSummary && len(state.chat.Conversations) > 0 {
		state.panel = ""
		state.active = true
		a.mu.Unlock()
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
		a.mu.Unlock()
		return
	}
	if state.question != nil {
		questionID = state.question.QuestionID
	}
	state.chat = *selected
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
	a.mu.Unlock()
	if questionID != "" {
		go a.answerAIQuestion(questionID, "User cancelled")
	}
	go a.loadChatPreview(key, chatID, revision)
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// deleteChatHistory removes persisted history through core and starts a draft if it was active.
func (a *App) deleteChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.chat.ID == chatID && (state.chat.IsStreaming || state.sending) {
		state.error = "Stop the active response before deleting this chat."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.client.Post(ctx, "/ai/chat/delete", map[string]any{"chatId": chatID}, nil)
		activeDeleted := false
		a.mu.Lock()
		if state := a.chatPreview; state != nil {
			if err != nil {
				state.error = err.Error()
			} else {
				state.chats = slices.DeleteFunc(state.chats, func(chat chatData) bool { return chat.ID == chatID })
				activeDeleted = state.chat.ID == chatID
				state.panelSelected = min(state.panelSelected, max(0, len(state.chats)-1))
			}
		}
		a.mu.Unlock()
		if err != nil {
			log.Printf("delete chat history: %v", err)
		} else if activeDeleted {
			a.startNewChat()
		}
		_ = a.window.Invalidate()
	}()
}

// selectChatModel applies one catalog entry to the draft and closes the catalog.
func (a *App) selectChatModel(index int) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || index < 0 || index >= len(a.aiModels) {
		a.mu.Unlock()
		return
	}
	if state.chat.IsStreaming || state.sending {
		state.error = "Stop the active response before changing models."
		state.panel = ""
		state.active = true
		a.mu.Unlock()
		a.updateChatTextInput(true)
		_ = a.window.Invalidate()
		return
	}
	state.chat.Model = a.aiModels[index]
	state.panelSelected = index
	state.panel = ""
	state.error = ""
	state.active = true
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// insertChatSkill adds the stable inline tag that core expands through SkillRefs.
func (a *App) insertChatSkill(index int) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.editor == nil || index < 0 || index >= len(a.aiSkills) {
		a.mu.Unlock()
		return
	}
	skill := a.aiSkills[index]
	state.editor.InsertText("{skill:" + skill.Name + "} ")
	state.panelSelected = index
	state.panel = ""
	state.error = ""
	state.active = true
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
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
	a.mu.Lock()
	if state := a.chatPreview; state != nil {
		state.panel = ""
		state.active = true
	}
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// moveChatPanelSelection wraps keyboard navigation inside the active catalog.
func (a *App) moveChatPanelSelection(delta int) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.panel == "" {
		a.mu.Unlock()
		return
	}
	count := len(state.chats)
	if state.panel == "models" {
		count = len(a.aiModels)
	} else if state.panel == "skills" {
		count = len(a.aiSkills)
	}
	if count > 0 {
		state.panelSelected = (state.panelSelected + delta + count) % count
		ensureChatPanelSelectionVisibleLocked(state, count)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// activateChatPanelSelection applies the selected history or model row.
func (a *App) activateChatPanelSelection() {
	a.mu.RLock()
	state := a.chatPreview
	if state == nil {
		a.mu.RUnlock()
		return
	}
	panel := state.panel
	selected := state.panelSelected
	chatID := ""
	if panel == "history" && selected >= 0 && selected < len(state.chats) {
		chatID = state.chats[selected].ID
	}
	a.mu.RUnlock()
	if panel == "history" {
		a.selectChatHistory(chatID)
	} else if panel == "models" {
		a.selectChatModel(selected)
	} else if panel == "skills" {
		a.insertChatSkill(selected)
	}
}

// setChatPanelViewport records the current catalog extent for scrolling and keyboard reveal.
func (a *App) setChatPanelViewport(height float32) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil {
		initialize := state.panelViewport <= 0
		state.panelViewport = max(float32(1), height)
		count := len(state.chats)
		if state.panel == "models" {
			count = len(a.aiModels)
		} else if state.panel == "skills" {
			count = len(a.aiSkills)
		}
		if initialize {
			ensureChatPanelSelectionVisibleLocked(state, count)
		} else {
			maxOffset := max(float32(0), float32(count)*44-state.panelViewport)
			state.panelScroll = min(max(float32(0), state.panelScroll), maxOffset)
		}
	}
	a.mu.Unlock()
}

// ensureChatPanelSelectionVisibleLocked reveals only keyboard-driven selection changes.
func ensureChatPanelSelectionVisibleLocked(state *chatPreviewState, count int) {
	if state == nil || count <= 0 {
		return
	}
	const rowHeight = float32(44)
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

// scrollChatPanel applies pointer-wheel movement without changing the selected row.
func (a *App) scrollChatPanel(delta float32) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.panel == "" {
		a.mu.Unlock()
		return
	}
	count := len(state.chats)
	if state.panel == "models" {
		count = len(a.aiModels)
	} else if state.panel == "skills" {
		count = len(a.aiSkills)
	}
	maxOffset := max(float32(0), float32(count)*44-state.panelViewport)
	state.panelScroll = min(max(float32(0), state.panelScroll+delta), maxOffset)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// clampChatDebugScroll records the JSON inspector extent derived by the shared text layout.
func (a *App) clampChatDebugScroll(maxOffset float32) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.panel == "debug" {
		state.panelMaxScroll = max(float32(0), maxOffset)
		state.panelScroll = min(max(float32(0), state.panelScroll), state.panelMaxScroll)
	}
	a.mu.Unlock()
}

// scrollChatDebugPanel applies pointer and keyboard movement to the portable trace inspector.
func (a *App) scrollChatDebugPanel(delta float32) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.panel == "debug" {
		state.panelScroll = min(max(float32(0), state.panelScroll+delta), state.panelMaxScroll)
	}
	a.mu.Unlock()
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

	a.mu.Lock()
	state := a.chatPreview
	selectedChatVisible := false
	if state != nil {
		selectedChatVisible = a.selected >= 0 && a.selected < len(a.results) && a.results[a.selected].Preview.PreviewType == "chat" && a.results[a.selected].ID == state.resultID
	}
	if state == nil || !a.visible || !selectedChatVisible {
		a.mu.Unlock()
		go a.answerAIQuestion(question.QuestionID, "User cancelled")
		return nil
	}
	if state.question != nil && state.question.QuestionID != question.QuestionID {
		previousID := state.question.QuestionID
		go a.answerAIQuestion(previousID, "User cancelled")
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
	a.mu.Unlock()
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
	if err := a.client.Post(ctx, "/ai/question/answer", map[string]any{"questionId": questionID, "answer": answer}, nil); err != nil {
		log.Printf("answer AI question: %v", err)
	}
}

// submitAIQuestionAnswer clears the overlay before resolving the pending core request.
func (a *App) submitAIQuestionAnswer(answer string) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.question == nil {
		a.mu.Unlock()
		return
	}
	questionID := state.question.QuestionID
	state.question = nil
	state.questionEditor = nil
	state.questionSelected = 0
	state.error = ""
	a.mu.Unlock()
	go a.answerAIQuestion(questionID, answer)
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// submitSelectedAIQuestionAnswer maps the selected choice or free text to the stable option value.
func (a *App) submitSelectedAIQuestionAnswer() {
	a.mu.RLock()
	state := a.chatPreview
	if state == nil || state.question == nil {
		a.mu.RUnlock()
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
	a.mu.RUnlock()
	if answer == "" {
		answer = "User cancelled"
	}
	a.submitAIQuestionAnswer(answer)
}

// selectAIQuestionOption changes the pending choice without submitting it.
func (a *App) selectAIQuestionOption(index int) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.question != nil && index >= 0 && index < len(state.question.Options) {
		state.questionSelected = index
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// beginChatRequestLocked moves a prepared chat into its shared streaming state while App.mu is held.
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

// postChatRequest sends one immutable chat snapshot and reconciles transport failure with the current revision.
func (a *App) postChatRequest(key string, revision uint64, chat chatData) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// DebugTrace is an incoming development snapshot and must not be echoed into the next model request.
		chat.DebugTrace = nil
		err := a.client.Post(ctx, "/ai/chat", map[string]any{"chatData": chat}, nil)
		a.mu.Lock()
		if current := a.chatPreview; current != nil && current.key == key && current.revision == revision {
			current.sending = false
			if err != nil {
				current.chat.IsStreaming = false
				current.error = err.Error()
			}
		}
		a.mu.Unlock()
		if err != nil {
			log.Printf("send chat message: %v", err)
		}
		_ = a.window.Invalidate()
	}()
}

// sendChatMessage appends the local user turn before core begins pushing authoritative snapshots.
func (a *App) sendChatMessage() {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.editor == nil || state.loading || state.sending || state.chat.IsStreaming || state.question != nil {
		a.mu.Unlock()
		return
	}
	text := strings.TrimSpace(state.editor.State().Text)
	if text == "" {
		state.error = "Enter a message first."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if strings.TrimSpace(state.chat.Model.Name) == "" {
		state.error = "Select an AI model in Wox settings first."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	hasSkillTags := chatSkillTagPattern.MatchString(text)
	if hasSkillTags && !a.aiSkillsLoaded {
		requestSkills := !a.aiSkillsLoading
		if requestSkills {
			a.aiSkillsLoading = true
		}
		state.error = "Loading skills; send again when the catalog is ready."
		a.mu.Unlock()
		if requestSkills {
			go a.loadAISkills()
		}
		_ = a.window.Invalidate()
		return
	}
	now := time.Now().UnixMilli()
	skillRefs := chatSkillRefsFromText(text, a.aiSkills)
	if unresolved := unresolvedChatSkillTag(text, a.aiSkills); unresolved != "" {
		state.error = fmt.Sprintf("Unknown or disabled skill: %s", unresolved)
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	state.chat.Conversations = append(state.chat.Conversations, chatConversation{ID: coreclient.NewID(), Role: "user", Text: text, SkillRefs: skillRefs, Timestamp: now})
	state.chat.UpdatedAt = now
	state.editor.SetText("", false)
	key, revision, chat := beginChatRequestLocked(state)
	a.mu.Unlock()
	_ = a.window.Invalidate()
	a.postChatRequest(key, revision, chat)
}

// stopChatMessage cancels the active core stream while leaving its last snapshot visible.
func (a *App) stopChatMessage() {
	a.mu.RLock()
	state := a.chatPreview
	if state == nil || state.chat.ID == "" || (!state.chat.IsStreaming && !state.sending) {
		a.mu.RUnlock()
		return
	}
	chatID := state.chat.ID
	a.mu.RUnlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var stopped bool
		err := a.client.Post(ctx, "/ai/chat/stop", map[string]any{"chatId": chatID}, &stopped)
		a.mu.Lock()
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
		a.mu.Unlock()
		if err != nil {
			log.Printf("stop chat message: %v", err)
		}
		_ = a.window.Invalidate()
	}()
}

// copyChatText reports clipboard failures inside the chat surface while keeping native details below Window.
func (a *App) copyChatText(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if err := a.window.WriteClipboardText(text); err != nil {
		a.mu.Lock()
		if state := a.chatPreview; state != nil {
			state.error = fmt.Sprintf("Copy failed: %v", err)
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
	}
}

// editChatConversation restores a user turn into the composer and discards the stale branch after it.
func (a *App) editChatConversation(messageID string) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil || state.editor == nil {
		a.mu.Unlock()
		return
	}
	if state.chat.IsStreaming || state.sending || state.question != nil {
		state.error = "Stop the active response before editing a message."
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	messageIndex := slices.IndexFunc(state.chat.Conversations, func(message chatConversation) bool {
		return message.ID == messageID && message.Role == "user"
	})
	if messageIndex < 0 {
		state.error = "The user message is no longer available."
		a.mu.Unlock()
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
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

// regenerateChatConversation resends the user turn preceding an assistant response through the normal chat channel.
func (a *App) regenerateChatConversation(messageID string) {
	a.mu.Lock()
	state := a.chatPreview
	if state == nil {
		a.mu.Unlock()
		return
	}
	if state.chat.IsStreaming || state.sending || state.question != nil {
		state.error = "Stop the active response before retrying it."
		a.mu.Unlock()
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
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	state.chat.Conversations = slices.Clone(state.chat.Conversations[:userIndex+1])
	state.chat.CompactionEntries = nil
	state.chat.DebugTrace = nil
	state.chat.UpdatedAt = time.Now().UnixMilli()
	key, revision, chat := beginChatRequestLocked(state)
	a.mu.Unlock()
	_ = a.window.Invalidate()
	a.postChatRequest(key, revision, chat)
}

// onChatPreviewKey keeps chat editing and ask_user behavior identical on every platform.
func (a *App) onChatPreviewKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
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
	a.mu.RUnlock()
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
		case woxui.KeyArrowUp:
			a.moveChatPanelSelection(-1)
		case woxui.KeyArrowDown, woxui.KeyTab:
			delta := 1
			if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -1
			}
			a.moveChatPanelSelection(delta)
		case woxui.KeyEnter:
			a.activateChatPanelSelection()
		case woxui.KeyDelete:
			if panel == "history" {
				a.deleteChatHistory(panelChatID)
			}
		}
		return true
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
		a.exitChatMode()
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
	a.mu.RLock()
	state := a.chatPreview
	if state == nil || !state.active {
		a.mu.RUnlock()
		return false
	}
	a.mu.RUnlock()
	return true
}

func (a *App) editChatKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.active && state.editor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 5}}
		_, changed := handleFormEditorKey(state.editor, definition, event)
		if changed {
			state.error = ""
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) editAIQuestionKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 4}}
		_, changed := handleFormEditorKey(state.questionEditor, definition, event)
		if changed {
			state.error = ""
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) moveAIQuestionSelection(delta int) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.question != nil && len(state.question.Options) > 0 {
		state.questionSelected = (state.questionSelected + delta + len(state.question.Options)) % len(state.question.Options)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) focusChatInput() {
	a.mu.Lock()
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) focusAIQuestionInput() {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.question != nil {
		state.active = true
	}
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) setChatText(value string) {
	openSkills := false
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.editor != nil && state.question == nil {
		state.editor.SetText(value, false)
		state.error = ""
		if value == "/" {
			state.editor.SetText("", false)
			openSkills = true
		}
	}
	a.mu.Unlock()
	if openSkills {
		a.toggleChatPanel("skills")
		return
	}
	_ = a.window.Invalidate()
}

func (a *App) setAIQuestionText(value string) {
	a.mu.Lock()
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		state.questionEditor.SetText(value, false)
		state.error = ""
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// enterChatMode hides launcher chrome while retaining the same native window and shared preview state.
func (a *App) enterChatMode() {
	a.mu.Lock()
	a.chatFullscreen = true
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.mu.Unlock()
	a.updateChatTextInput(true)
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

// exitChatMode restores query ownership without destroying the in-progress conversation.
func (a *App) exitChatMode() {
	a.mu.Lock()
	a.chatFullscreen = false
	if state := a.chatPreview; state != nil {
		state.active = false
	}
	a.mu.Unlock()
	a.restoreQueryTextInput()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

// deactivateChatPreview releases input ownership and cancels an ask_user request that is no longer visible.
func (a *App) deactivateChatPreview() {
	a.mu.Lock()
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
	a.mu.Unlock()
	if questionID != "" {
		go a.answerAIQuestion(questionID, "User cancelled")
	}
	if wasActive || wasFullscreen {
		a.restoreQueryTextInput()
		_ = a.window.Invalidate()
	}
}

// resetChatPreview discards state at launcher lifecycle boundaries and unblocks any pending ask_user call.
func (a *App) resetChatPreview() {
	a.mu.Lock()
	questionID := ""
	if a.chatPreview != nil && a.chatPreview.question != nil {
		questionID = a.chatPreview.question.QuestionID
	}
	a.chatPreview = nil
	a.chatFullscreen = false
	a.mu.Unlock()
	if questionID != "" {
		go a.answerAIQuestion(questionID, "User cancelled")
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
	a.mu.Lock()
	if state := a.chatPreview; state != nil {
		state.scroll = min(max(float32(0), state.scroll), maxOffset)
		state.autoFollow = maxOffset-state.scroll <= 36
	}
	a.mu.Unlock()
}

// scrollChatPreview preserves manual scrollback until the user returns near the latest message.
func (a *App) scrollChatPreview(delta, maxOffset float32) {
	if delta == 0 {
		return
	}
	a.mu.Lock()
	if state := a.chatPreview; state != nil {
		state.scroll = min(max(float32(0), state.scroll+delta), maxOffset)
		state.autoFollow = maxOffset-state.scroll <= 36
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}
