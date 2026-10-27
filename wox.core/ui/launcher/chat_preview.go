package launcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"wox/common"
	chatview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	"wox/util"
)

const enterChatModeActionID = "__wox_internal_enter_chat_mode__"

const (
	chatCommandPanel             = "commands"
	chatCatalogRowHeight         = float32(38)
	chatCatalogGroupHeaderHeight = float32(28)
)

// chatOverlayPanel reports floating catalogs that must not replace the history drawer.
func chatOverlayPanel(panel string) bool {
	return panel == "models" || panel == "skills" || panel == chatCommandPanel || panel == chatMentionPanel || panel == chatProjectPanel
}

// chatHistoryVisible reports whether the conversation sidebar should stay in layout.
func chatHistoryVisible(panel string, sidebarOpen bool) bool {
	return sidebarOpen || panel == "history"
}

// rememberChatHistorySidebarLocked stores drawer scroll so a slash overlay can reopen it later.
func rememberChatHistorySidebarLocked(state *chatPreviewState) {
	if state == nil {
		return
	}
	state.sidebarOpen = true
	state.sidebarSelected = state.panelSelected
	state.sidebarScroll = state.panelScroll
	state.sidebarViewport = state.panelViewport
}

// restoreChatHistoryPanelLocked closes an overlay without hiding an already-open sidebar.
func restoreChatHistoryPanelLocked(state *chatPreviewState) {
	if state == nil {
		return
	}
	state.panelQuery = ""
	if !state.sidebarOpen {
		state.panel = ""
		return
	}
	state.panel = "history"
	state.panelSelected = state.sidebarSelected
	state.panelScroll = state.sidebarScroll
	state.panelViewport = state.sidebarViewport
}

var chatSkillTagPattern = regexp.MustCompile(`\{skill:([^}]+)\}`)

type chatPreviewData struct {
	ActiveChat         chatData                  `json:"ActiveChat"`
	ActiveChatID       string                    `json:"ActiveChatId"`
	Chats              []chatData                `json:"Chats"`
	SidebarOpen        bool                      `json:"SidebarOpen"`
	InitialAttachments []common.AIChatAttachment `json:"InitialAttachments,omitempty"`
}

type chatData struct {
	ProjectID         string             `json:"ProjectId,omitempty"`
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
	ID           string                    `json:"Id"`
	Role         string                    `json:"Role"`
	Text         string                    `json:"Text"`
	Reasoning    string                    `json:"Reasoning"`
	Attachments  []common.AIChatAttachment `json:"Attachments,omitempty"`
	Images       []woxImage                `json:"Images"`
	SkillRefs    []chatSkillRef            `json:"SkillRefs"`
	Mentions     []chatMentionRef          `json:"Mentions"`
	ToolCallInfo chatToolCallInfo          `json:"ToolCallInfo"`
	Timestamp    int64                     `json:"Timestamp"`
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

type chatPluginMention struct {
	ID     string
	Name   string
	NameEn string
	Icon   woxImage
}

type chatToolCallInfo struct {
	ID             string         `json:"Id"`
	Name           string         `json:"Name"`
	Source         string         `json:"Source"`
	Server         string         `json:"Server"`
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
	projectSkills          []chatSkill
	projectSkillsProjectID string
	projectSkillsRevision  uint64
	projectSkillsLoading   bool
	projects               chatProjectUI
	key                    string
	queryID                string
	resultID               string
	chat                   chatData
	// nextModel survives snapshots from the current stream until the next request starts.
	nextModel     *aiModel
	chats         []chatData
	editor        *woxui.TextEditor
	active        bool
	scroll        chatview.ChatScrollState
	loading       bool
	sending       bool
	error         string
	revision      uint64
	remoteVersion uint64
	panel         string
	panelQuery    string
	panelFilter   string
	panelSelected int
	panelScroll   float32
	panelViewport float32
	// Loading rows affect scrolling but never enter the selectable command list.
	panelLoadingHeight float32
	panelLeadingHeight float32
	panelDensityScale  float32
	// sidebarOpen keeps the conversation drawer visible while a slash/model overlay is open.
	sidebarOpen      bool
	sidebarSelected  int
	sidebarScroll    float32
	sidebarViewport  float32
	question         *aiQuestion
	questionEditor   *woxui.TextEditor
	questionSelected int
	expandedRounds   map[string]bool
	attachments      []common.AIChatAttachment
	importing        bool
}

type chatPreviewSnapshot struct {
	projects              chatProjectUI
	projectNoneLabel      string
	key                   string
	queryID               string
	resultID              string
	chat                  chatData
	editing               woxui.TextEditingState
	active                bool
	scroll                chatview.ChatScrollState
	loading               bool
	sending               bool
	error                 string
	chats                 []chatData
	models                []aiModel
	modelsLoading         bool
	modelsError           string
	skills                []chatSkill
	skillsLoading         bool
	skillsError           string
	mentions              []chatMention
	pluginMentions        []chatPluginMention
	pluginMentionsLoading bool
	pluginMentionsError   string
	usePinYin             bool
	panel                 string
	panelQuery            string
	panelFilter           string
	panelSelected         int
	panelScroll           float32
	panelViewport         float32
	sidebarOpen           bool
	sidebarSelected       int
	sidebarScroll         float32
	sidebarViewport       float32
	question              *aiQuestion
	questionEditing       woxui.TextEditingState
	questionSelected      int
	expandedRounds        map[string]bool
	attachments           []common.AIChatAttachment
	importing             bool
}

type chatCommandPaletteItem struct {
	group       string
	sourceIndex int
	title       string
	subtitle    string
	searchText  string
	score       int64
	current     bool
}

type chatSlashToken struct {
	start int
	end   int
	query string
}

type chatSkillTagRange struct {
	start int
	end   int
	name  string
}

// chatSkillTagRanges returns complete {skill:name} placeholders in rune offsets.
func chatSkillTagRanges(text string) []chatSkillTagRange {
	matches := chatSkillTagPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	ranges := make([]chatSkillTagRange, 0, len(matches))
	for _, match := range matches {
		if len(match) < 4 || match[0] < 0 || match[1] < 0 || match[2] < 0 || match[3] < 0 {
			continue
		}
		ranges = append(ranges, chatSkillTagRange{
			start: utf8.RuneCountInString(text[:match[0]]),
			end:   utf8.RuneCountInString(text[:match[1]]),
			name:  text[match[2]:match[3]],
		})
	}
	return ranges
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

func findChatAtToken(editing woxui.TextEditingState) (chatSlashToken, bool) {
	runes := []rune(editing.Text)
	cursor := min(max(0, editing.Selection.Focus), len(runes))
	start := cursor
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	if start >= len(runes) || runes[start] != '@' {
		return chatSlashToken{}, false
	}
	end := cursor
	for end < len(runes) && !unicode.IsSpace(runes[end]) {
		end++
	}
	return chatSlashToken{start: start, end: end, query: strings.TrimSpace(string(runes[start+1 : end]))}, true
}

// chatCatalogMatchScore uses the best field match, as in the mention picker.
func chatCatalogMatchScore(query string, usePinYin bool, fields ...string) (int64, bool) {
	if query == "" {
		return 0, true
	}
	var best int64
	matched := false
	for _, field := range fields {
		if ok, score := util.IsStringMatchScore(field, query, usePinYin); ok && (!matched || score > best) {
			best, matched = score, true
		}
	}
	return best, matched
}

// sortChatCatalogResults ranks matches without changing their source indices.
func sortChatCatalogResults(items []chatCommandPaletteItem) {
	slices.SortStableFunc(items, func(a, b chatCommandPaletteItem) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return 0
	})
}

// chatCommandPaletteItems filters the complete catalog and keeps provider instances together.
func chatCommandPaletteItems(models []aiModel, skills []chatSkill, current aiModel, query, filterGroup string, pinyin ...bool) []chatCommandPaletteItem {
	query = strings.TrimSpace(query)
	usePinYin := len(pinyin) > 0 && pinyin[0]
	items := make([]chatCommandPaletteItem, 0, len(models)+len(skills))
	if filterGroup == "" || filterGroup == chatCommandPanel || filterGroup == "models" {
		for index, model := range models {
			subtitle := model.Provider
			if model.ProviderAlias != "" {
				subtitle += " (" + model.ProviderAlias + ")"
			}
			item := chatCommandPaletteItem{group: "models", sourceIndex: index, title: model.Name, subtitle: subtitle, current: model == current}
			item.searchText = "model 模型 " + model.Name + " " + model.Provider + " " + model.ProviderAlias
			if score, ok := chatCatalogMatchScore(query, usePinYin, model.Name, model.Provider, model.ProviderAlias, item.subtitle, "model", "模型"); ok {
				item.score = score
				items = append(items, item)
			}
		}
		// Keep configured provider instances together without changing their source indices.
		providers := make(map[string]int)
		for _, item := range items {
			if _, exists := providers[item.subtitle]; !exists {
				providers[item.subtitle] = len(providers)
			}
		}
		slices.SortStableFunc(items, func(a, b chatCommandPaletteItem) int {
			return providers[a.subtitle] - providers[b.subtitle]
		})
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
			if score, ok := chatCatalogMatchScore(query, usePinYin, skill.Name, skill.Description, skill.Source, skill.SourceName, item.subtitle, "skill", "技能"); ok {
				item.score = score
				items = append(items, item)
			}
		}
	}
	if query != "" {
		sortChatCatalogResults(items)
	}
	return items
}

// chatSlashPaletteItems puts projects between models and skills so loading rows keep their order.
func chatSlashPaletteItems(models []aiModel, skills []chatSkill, current aiModel, projects chatProjectUI, projectID, noneLabel, query, filter string, pinyin ...bool) []chatCommandPaletteItem {
	usePinYin := len(pinyin) > 0 && pinyin[0]
	items := chatCommandPaletteItems(models, skills, current, query, filter, usePinYin)
	if !chatCommandPaletteShowsGroup(filter, chatProjectPanel) || !projects.Loaded && len(projects.Projects) == 0 {
		return items
	}
	projectItems := chatProjectPaletteItems(projects.Projects, projectID, query, noneLabel, usePinYin)
	if query = strings.TrimSpace(query); query != "" {
		if score, ok := chatCatalogMatchScore(query, usePinYin, noneLabel); ok {
			projectItems[0].score = score
		} else {
			projectItems = projectItems[1:]
		}
	}
	index := slices.IndexFunc(items, func(item chatCommandPaletteItem) bool { return item.group == "skills" })
	if index < 0 {
		index = len(items)
	}
	items = slices.Insert(items, index, projectItems...)
	if query != "" {
		sortChatCatalogResults(items)
	}
	return items
}

func (a *chatController) commandPaletteItems() []chatCommandPaletteItem {
	state := a.chatPreview
	filter := chatCommandFilter(state.panel, state.panelFilter)
	if state.panel != chatCommandPanel {
		return chatCommandPaletteItems(a.aiSettings().Models(), a.chatSkills(), state.chat.Model, state.panelQuery, filter, a.usePinYin())
	}
	return chatSlashPaletteItems(a.aiSettings().Models(), a.chatSkills(), state.chat.Model, state.projects, state.chat.ProjectID, a.language.translate("i18n:plugin_ai_chat_project_none"), state.panelQuery, filter, a.usePinYin())
}

// chatCommandItemOffset includes each visible group header before the target row.
func chatCommandItemOffset(items []chatCommandPaletteItem, target int, density ...float32) float32 {
	scale := float32(1)
	if len(density) > 0 {
		scale = density[0]
	}
	offset := float32(0)
	section, group := "", ""
	groupSet := false
	for index, item := range items {
		if item.group != section {
			if section != "" {
				offset += chatview.ChatScaledSize(scale, chatview.ChatPickerSectionGap)
			}
			section, group, groupSet = item.group, "", false
			offset += chatview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight)
		}
		if item.group == "models" && (!groupSet || item.subtitle != group) {
			if groupSet {
				offset += chatview.ChatScaledSize(scale, chatview.ChatPickerSectionGap)
			}
			group, groupSet = item.subtitle, true
			offset += chatview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight)
		}
		if index == target {
			return offset
		}
		offset += chatview.ChatScaledSize(scale, chatCommandItemHeight(item))
	}
	return offset
}

func chatCommandContentHeight(items []chatCommandPaletteItem, density ...float32) float32 {
	if len(items) == 0 {
		return 0
	}
	scale := float32(1)
	if len(density) > 0 {
		scale = density[0]
	}
	return chatCommandItemOffset(items, len(items)-1, scale) + chatview.ChatScaledSize(scale, chatCommandItemHeight(items[len(items)-1]))
}

func chatCommandItemGroup(item chatCommandPaletteItem) string {
	if item.group == "models" {
		return item.group + ":" + item.subtitle
	}
	return item.group
}

func chatCommandItemHeight(item chatCommandPaletteItem) float32 {
	if item.group == "skills" {
		return 50
	}
	return chatCatalogRowHeight
}

func chatCommandFilter(panel, filter string) string {
	if panel == chatCommandPanel && filter != "" {
		return filter
	}
	return panel
}

// chatCommandPaletteShowsLoading keeps a group visible while its catalog is still in flight.
func chatCommandPaletteShowsLoading(items []chatCommandPaletteItem, group string, loading bool) bool {
	if !loading {
		return false
	}
	for _, item := range items {
		if item.group == group {
			return false
		}
	}
	return true
}

func chatCommandPaletteShowsGroup(filterGroup, group string) bool {
	return filterGroup == "" || filterGroup == chatCommandPanel || filterGroup == group
}

// chatCommandPaletteLoadingHeight reserves the Models/Skills header and row shown while a catalog loads.
func chatCommandPaletteLoadingHeight(items []chatCommandPaletteItem, filterGroup string, modelsLoading, skillsLoading bool, density ...float32) float32 {
	scale := float32(1)
	if len(density) > 0 {
		scale = density[0]
	}
	height := float32(0)
	if chatCommandPaletteShowsGroup(filterGroup, "models") && chatCommandPaletteShowsLoading(items, "models", modelsLoading) {
		height += chatview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight) + chatview.ChatScaledSize(scale, chatCatalogRowHeight)
		if len(items) > 0 {
			height += chatview.ChatScaledSize(scale, chatview.ChatPickerSectionGap)
		}
	}
	if chatCommandPaletteShowsGroup(filterGroup, "skills") && chatCommandPaletteShowsLoading(items, "skills", skillsLoading) {
		if len(items) > 0 || height > 0 {
			height += chatview.ChatScaledSize(scale, chatview.ChatPickerSectionGap)
		}
		height += chatview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight) + chatview.ChatScaledSize(scale, chatCatalogRowHeight)
	}
	return height
}

// cloneChatData isolates nested message slices before transport and render state diverge.
func cloneChatData(source chatData) chatData {
	cloned := source
	cloned.Conversations = make([]chatConversation, len(source.Conversations))
	for index, conversation := range source.Conversations {
		cloned.Conversations[index] = conversation
		cloned.Conversations[index].Attachments = slices.Clone(conversation.Attachments)
		cloned.Conversations[index].Images = append([]woxImage(nil), conversation.Images...)
		cloned.Conversations[index].SkillRefs = append([]chatSkillRef(nil), conversation.SkillRefs...)
		cloned.Conversations[index].Mentions = append([]chatMentionRef(nil), conversation.Mentions...)
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
		projects: cloneChatProjectUI(state.projects),
		// Payload revisions must not recreate scroll/drag state on every streamed update.
		key:              state.resultID + "|" + state.chat.ID,
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
		panelFilter:      state.panelFilter,
		panelSelected:    state.panelSelected,
		panelScroll:      state.panelScroll,
		panelViewport:    state.panelViewport,
		sidebarOpen:      state.sidebarOpen,
		sidebarSelected:  state.sidebarSelected,
		sidebarScroll:    state.sidebarScroll,
		sidebarViewport:  state.sidebarViewport,
		question:         cloneChatQuestion(state.question),
		questionSelected: state.questionSelected,
		expandedRounds:   make(map[string]bool, len(state.expandedRounds)),
		attachments:      slices.Clone(state.attachments),
		importing:        state.importing,
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

// chatPreviewDataAndKey validates the payload and derives its content revision for update detection.
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
func (a *chatController) activateChatPreview(result queryResult, preview queryPreview) error {
	// A new launcher query must not replace the shared conversation, but the
	// originating result can still publish preview updates while fullscreen.
	if a.chatPreview != nil && (a.chatWindowOpen() || a.chatFullscreen && (a.chatPreview.queryID != result.QueryID || a.chatPreview.resultID != result.ID)) {
		return nil
	}
	data, key, err := chatPreviewDataAndKey(result, preview)
	if err != nil {
		return err
	}
	if !a.chatHistoryInitialized {
		a.chatHistoryOpen = data.SidebarOpen
		a.chatHistoryInitialized = true
	}
	if state := a.chatPreview; state != nil && state.queryID == result.QueryID && state.resultID == result.ID && state.chat.ID == data.ActiveChat.ID && data.ActiveChatID == "" {
		if state.key != key {
			// Complete previews update conversation data, not draft, scroll, or ask_user ownership.
			// Once the chat service has published a snapshot, it remains authoritative.
			state.key = key
			if state.remoteVersion == 0 {
				state.chat = cloneChatData(data.ActiveChat)
				state.chats = append([]chatData(nil), data.Chats...)
				sortChatSummaries(state.chats)
				if state.nextModel != nil {
					state.chat.Model = *state.nextModel
				}
				state.loading = false
			}
		}
		return nil
	}
	changed := a.chatPreview != nil && a.chatPreview.key != key
	keepFullscreen := a.chatFullscreen
	if changed {
		a.chatHistoryOpen = chatHistoryVisible(a.chatPreview.panel, a.chatPreview.sidebarOpen)
		a.deactivateChatPreview()
		if keepFullscreen {
			a.chatFullscreen = true
		}
	}

	shouldLoad := false
	loadChatID := ""
	if a.chatPreview == nil || a.chatPreview.key != key {
		if a.chatProjectChoiceSaving && data.ActiveChatID == "" {
			data.ActiveChat.ProjectID = a.chatProjectChoice
		}
		a.chatPreview = &chatPreviewState{
			key:         key,
			queryID:     result.QueryID,
			resultID:    result.ID,
			chat:        cloneChatData(data.ActiveChat),
			chats:       append([]chatData(nil), data.Chats...),
			editor:      woxui.NewTextEditor(""),
			active:      a.chatFullscreen,
			sidebarOpen: a.chatHistoryOpen,

			expandedRounds: make(map[string]bool),
			attachments:    slices.Clone(data.InitialAttachments),
		}
		if a.chatHistoryOpen {
			a.chatPreview.panel = "history"
		}
		if data.ActiveChatID != "" {
			a.chatPreview.loading = true
			a.chatPreview.revision++
			shouldLoad = true
			loadChatID = data.ActiveChatID
		}
		sortChatSummaries(a.chatPreview.chats)
		a.prefetchChatCatalogs()
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
func (a *chatController) chatPreviewSnapshotFor(result queryResult, preview queryPreview) (*chatPreviewSnapshot, error) {
	_, key, err := chatPreviewDataAndKey(result, preview)
	if err != nil {
		return nil, err
	}
	if a.chatPreview == nil || (!a.chatWindowOpen() && !a.chatFullscreen && a.chatPreview.key != key) {
		return nil, fmt.Errorf("chat preview is not ready")
	}
	snapshot := snapshotChatPreviewLocked(a.chatPreview)
	a.attachChatPreviewCatalogs(snapshot)
	return snapshot, nil
}

// prefetchChatCatalogs starts a one-shot provider, model and skill fetch when chat becomes visible
// and the shared catalogs are still empty. The picker then reuses the cache instead of
// flashing a loading row every time it opens.
func (a *chatController) prefetchChatCatalogs() {
	a.loadChatProjects()
	if a == nil || a.aiSettings == nil || a.aiSettings() == nil || a.services == nil {
		return
	}
	ai := a.aiSettings()
	if !ai.providersLoaded && !ai.providersLoading {
		util.Go(a.lifecycleCtx, "load AI provider icons for chat", func() {
			ai.ReloadProviders(a.lifecycleCtx, a.services, a.sessionID, func([]aiProviderInfo) {
				a.invalidateChatSurfaces()
			})
		})
	}
	requestModels := !a.aiSettings().ModelsLoaded() && !a.aiSettings().ModelsLoading()
	requestSkills := !a.aiSettings().SkillsLoaded() && !a.aiSettings().SkillsLoading()
	requestPlugins := !a.aiSettings().PluginMentionsLoaded() && !a.aiSettings().PluginMentionsLoading()
	if requestModels {
		a.aiSettings().SetModelsLoading(true)
	}
	if requestSkills {
		a.aiSettings().SetSkillsLoading(true)
	}
	if requestPlugins {
		a.aiSettings().SetPluginMentionsLoading(true)
	}
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "load AI skills for chat", a.loadAISkills)
	}
	if requestPlugins {
		util.Go(a.lifecycleCtx, "load chat plugin mentions", a.loadChatPluginMentions)
	}
}

// attachChatPreviewCatalogs copies the shared model and skill catalogs onto a render snapshot.
func (a *chatController) attachChatPreviewCatalogs(snapshot *chatPreviewSnapshot) {
	if snapshot == nil {
		return
	}
	snapshot.projectNoneLabel = a.language.translate("i18n:plugin_ai_chat_project_none")
	if a.aiSettings() == nil {
		return
	}
	snapshot.models = a.aiSettings().Models()
	snapshot.modelsLoading = a.aiSettings().ModelsLoading()
	snapshot.modelsError = a.aiSettings().ModelsError()
	snapshot.skills = a.chatSkills()
	snapshot.skillsLoading = a.aiSettings().SkillsLoading() || (a.chatPreview != nil && a.chatPreview.projectSkillsLoading)
	snapshot.skillsError = a.aiSettings().SkillsError()
	snapshot.pluginMentions = a.aiSettings().PluginMentions()
	snapshot.pluginMentionsLoading = a.aiSettings().PluginMentionsLoading()
	snapshot.pluginMentionsError = a.aiSettings().PluginMentionsError()
	snapshot.mentions = append(chatFileMentions(a.language.translate), chatMentionsFromPlugins(snapshot.pluginMentions)...)
	snapshot.usePinYin = a.chatMentionUsePinYin()
}

// loadChatPreview resolves one lightweight history entry through the chat service.
func (a *chatController) loadChatPreview(key, chatID string, revision uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loaded, err := a.services.ChatByID(ctx, a.sessionID, chatID)
	chat := chatData{}
	if err == nil {
		chat = fromCoreChatData(loaded)
	}
	if dispatchErr := a.runOnUI("apply chat preview", func() {
		if state := a.chatPreview; state != nil && state.key == key && state.revision == revision {
			state.loading = false
			if state.remoteVersion == 0 {
				if err != nil {
					state.error = err.Error()
				} else {
					state.chat = cloneChatData(chat)
					a.refreshChatProjectSkills()
					if state.nextModel != nil {
						state.chat.Model = *state.nextModel
					}
					state.error = ""
				}
			}
		}
		a.invalidateChatSurfaces()
	}); dispatchErr != nil {
		log.Printf("dispatch chat preview result: %v", dispatchErr)
	}
	if err != nil {
		log.Printf("load chat preview: %v", err)
	}
}

// applyChatResponse replaces only the matching active chat and preserves local input and scroll ownership.
func (a *chatController) applyChatResponse(chat chatData) {
	if chat.ID == "" {
		return
	}
	state := a.chatPreview
	if state == nil {
		return
	}
	upsertChatSummaryLocked(state, chat)
	if state.chat.ID == chat.ID {
		previousProject := state.chat.ProjectID
		state.chat = cloneChatData(chat)
		if state.chat.ProjectID != previousProject {
			a.refreshChatProjectSkills()
		}
		if state.nextModel != nil {
			state.chat.Model = *state.nextModel
		}
		state.remoteVersion++
		state.loading = false
		state.sending = false
		state.error = ""
	}
	a.invalidateChatSurfaces()
}

// toggleChatDisclosure expands or collapses a round, tool group, or tool detail.
func (a *chatController) toggleChatDisclosure(disclosureID string) {
	state := a.chatPreview
	if state == nil || disclosureID == "" {
		return
	}
	if state.expandedRounds == nil {
		state.expandedRounds = make(map[string]bool)
	}
	state.expandedRounds[disclosureID] = !state.expandedRounds[disclosureID]
	a.invalidateChatSurfaces()
}

// toggleChatPanel opens the history or model catalog and loads shared resources on demand.
func (a *chatController) toggleChatPanel(panel string) {
	requestModels := false
	requestSkills := false
	requestPlugins := false
	editorActive := false
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.question != nil {
		return
	}
	if panel == "history" && chatOverlayPanel(state.panel) {
		state.sidebarOpen = !state.sidebarOpen
		if state.sidebarOpen {
			for index, chat := range state.chats {
				if chat.ID == state.chat.ID {
					state.sidebarSelected = index
					break
				}
			}
		}
	} else if state.panel == panel {
		if chatOverlayPanel(panel) {
			restoreChatHistoryPanelLocked(state)
		} else {
			state.panel = ""
			state.panelQuery = ""
			state.sidebarOpen = false
		}
	} else {
		if state.panel == "history" {
			rememberChatHistorySidebarLocked(state)
		}
		state.panel = panel
		state.panelFilter = ""
		state.panelQuery = ""
		state.panelScroll = 0
		state.panelViewport = 0
		state.panelSelected = 0
		if panel == "history" {
			state.sidebarOpen = true
			for index, chat := range state.chats {
				if chat.ID == state.chat.ID {
					state.panelSelected = index
					state.sidebarSelected = index
					break
				}
			}
		}
		if panel == chatProjectPanel {
			state.panelLeadingHeight, state.panelLoadingHeight = 0, 0
			for index, item := range a.projectPaletteItems() {
				if item.current {
					state.panelSelected = index
					break
				}
			}
		}
		if panel == "models" || panel == chatCommandPanel {
			for index, item := range chatCommandPaletteItems(a.aiSettings().Models(), nil, state.chat.Model, "", "models") {
				if item.current {
					state.panelSelected = index
					break
				}
			}
			requestModels = !a.aiSettings().ModelsLoaded() && !a.aiSettings().ModelsLoading()
			if requestModels {
				a.aiSettings().SetModelsLoading(true)
			}
		}
		if panel == "skills" || panel == chatCommandPanel {
			requestSkills = !a.aiSettings().SkillsLoaded() && !a.aiSettings().SkillsLoading()
			if requestSkills {
				a.aiSettings().SetSkillsLoading(true)
			}
		}
		if panel == chatMentionPanel {
			requestPlugins = !a.aiSettings().PluginMentionsLoaded() && !a.aiSettings().PluginMentionsLoading()
			if requestPlugins {
				a.aiSettings().SetPluginMentionsLoading(true)
			}
		}
	}
	if panel == "history" {
		a.rememberChatSidebar(state.sidebarOpen)
	}
	state.active = true
	editorActive = state.panel == "" || state.panel == "history" || state.panel == chatCommandPanel || state.panel == chatMentionPanel
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "load AI skills for chat", a.loadAISkills)
	}
	if requestPlugins {
		util.Go(a.lifecycleCtx, "load chat plugin mentions", a.loadChatPluginMentions)
	}
	a.updateChatTextInput(editorActive)
	a.invalidateChatSurfaces()
}

// reloadChatResource invalidates only catalogs affected by a core resource notification.
func (a *chatController) reloadChatResourceName(resource string) {
	if resource == "projects" || resource == "all" {
		a.loadChatProjects()
	}
	if resource == "tools" || resource == "all" {
		a.refreshAIMCPServerToolsLocked()
	}
	if resource == "mentions" || resource == "tools" || resource == "all" {
		if a.aiSettings() != nil {
			a.aiSettings().ResetPluginMentions()
			if state := a.chatPreview; state != nil && state.panel == chatMentionPanel && !a.aiSettings().PluginMentionsLoading() {
				a.aiSettings().SetPluginMentionsLoading(true)
				util.Go(a.lifecycleCtx, "reload chat plugin mentions", a.loadChatPluginMentions)
			}
		}
		a.invalidateChatSurfaces()
	}
	if resource != "models" && resource != "skills" && resource != "all" {
		return
	}
	requestModels := false
	requestSkills := false
	if resource == "models" || resource == "all" {
		a.aiSettings().ResetModels()
		if state := a.chatPreview; state != nil && (state.panel == "models" || state.panel == chatCommandPanel) && !a.aiSettings().ModelsLoading() {
			a.aiSettings().SetModelsLoading(true)
			requestModels = true
		}
	}
	if resource == "skills" || resource == "all" {
		a.aiSettings().ResetSkills()
		if state := a.chatPreview; state != nil && (state.panel == "skills" || state.panel == chatCommandPanel) && !a.aiSettings().SkillsLoading() {
			a.aiSettings().SetSkillsLoading(true)
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
func (a *chatController) loadAISkills() {
	a.aiSettings().LoadAISkills(context.Background(), a.services, a.sessionID, func(skills []chatSkill) {
		if skills == nil {
			log.Printf("load AI skills: see controller error")
			a.invalidateChatSurfaces()
			return
		}
		if a.chatPreview != nil && (a.chatPreview.panel == "skills" || a.chatPreview.panel == chatCommandPanel) {
			a.chatPreview.panelSelected = 0
			a.chatPreview.panelScroll = 0
			a.chatPreview.panelViewport = 0
		}
		a.invalidateChatSurfaces()
	})
}

// loadChatPluginMentions refreshes the mention overlay after the shared catalog is loaded.
func (a *chatController) loadChatPluginMentions() {
	a.aiSettings().LoadChatPluginMentions(context.Background(), a.services, a.sessionID, func(mentions []chatPluginMention) {
		if mentions == nil {
			util.GetLogger().Error(context.Background(), "load chat plugin mentions: "+a.aiSettings().PluginMentionsError())
			a.invalidateChatSurfaces()
			return
		}
		if a.chatPreview != nil && a.chatPreview.panel == chatMentionPanel {
			a.chatPreview.panelSelected = 0
			a.chatPreview.panelScroll = 0
			a.chatPreview.panelViewport = 0
		}
		a.invalidateChatSurfaces()
	})
}

// startNewChat resets the active draft while retaining the user's current model choice.
// An in-flight response keeps running in the background until the user stops that chat.
func (a *chatController) startNewChat() {
	questionID := ""
	requestDefault := false
	state := a.chatPreview
	if state == nil {
		return
	}
	a.cancelChatAttachmentImports()
	if state.question != nil {
		questionID = state.question.QuestionID
	}
	upsertChatSummaryLocked(state, state.chat)
	now := time.Now().UnixMilli()
	model := state.chat.Model
	state.chat = chatData{ProjectID: state.chat.ProjectID, ID: newID(), Model: model, CreatedAt: now, UpdatedAt: now}
	state.nextModel = nil
	state.editor.SetText("", false)
	state.attachments = nil
	state.loading = false
	state.sending = false
	state.error = ""
	state.remoteVersion = 0
	if chatOverlayPanel(state.panel) {
		restoreChatHistoryPanelLocked(state)
	} else if state.panel != "history" {
		state.panel = ""
	}
	state.question = nil
	state.questionEditor = nil
	clear(state.expandedRounds)
	state.scroll.FollowLatest()
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
	a.invalidateChatSurfaces()
}

// loadDefaultChatModel fills a new draft without overwriting a model selected in the meantime.
func (a *chatController) loadDefaultChatModel(key string, revision uint64) {
	if a.services == nil {
		return
	}
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
		a.invalidateChatSurfaces()
	}); dispatchErr != nil {
		log.Printf("dispatch default chat model: %v", dispatchErr)
	}
	if err != nil {
		log.Printf("load default chat model: %v", err)
	}
}

// keepChatHistoryPanelLocked leaves the conversation sidebar open on row activation.
func keepChatHistoryPanelLocked(state *chatPreviewState, chatID string) {
	if state == nil {
		return
	}
	state.panel = "history"
	state.sidebarOpen = true
	for index, chat := range state.chats {
		if chat.ID == chatID {
			state.panelSelected = index
			state.sidebarSelected = index
			return
		}
	}
}

// selectChatHistory loads a summary only when it is not already the active full conversation.
// The previous chat keeps streaming in the background; only its Stop button cancels it.
func (a *chatController) selectChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	questionID := ""
	state := a.chatPreview
	if state == nil {
		return
	}
	if state.chat.ID == chatID && !state.chat.IsSummary && len(state.chat.Conversations) > 0 {
		keepChatHistoryPanelLocked(state, chatID)
		state.active = true
		a.updateChatTextInput(true)
		a.invalidateChatSurfaces()
		return
	}
	a.cancelChatAttachmentImports()
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
	if state.chat.ID != chatID {
		upsertChatSummaryLocked(state, state.chat)
	}
	state.chat = *selected
	a.refreshChatProjectSkills()
	state.nextModel = nil
	clear(state.expandedRounds)
	state.editor.SetText("", false)
	state.attachments = nil
	state.loading = true
	state.sending = false
	state.error = ""
	state.remoteVersion = 0
	keepChatHistoryPanelLocked(state, chatID)
	state.question = nil
	state.questionEditor = nil
	state.scroll.FollowLatest()
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
	a.invalidateChatSurfaces()
}

// deleteChatHistory removes persisted history through core and starts a draft if it was active.
func (a *chatController) deleteChatHistory(chatID string) {
	if chatID == "" {
		return
	}
	if state := a.chatPreview; state != nil && state.chat.ID == chatID && (state.chat.IsStreaming || state.sending) {
		state.error = "Stop the active response before deleting this chat."
		a.invalidateChatSurfaces()
		return
	}
	util.Go(a.lifecycleCtx, "delete chat history", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := a.services.DeleteChat(ctx, a.sessionID, chatID)
		if dispatchErr := a.runOnUI("apply deleted chat history", func() {
			if err != nil {
				if state := a.chatPreview; state != nil {
					state.error = err.Error()
				}
				a.invalidateChatSurfaces()
				return
			}
			a.applyDeletedChat(chatID)
		}); dispatchErr != nil {
			log.Printf("dispatch deleted chat history: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("delete chat history: %v", err)
		}
	})
}

// applyDeletedChat removes a chat from the history list and leaves the active view on a new
// chat when the deleted one was open. Must run on the UI thread.
func (a *chatController) applyDeletedChat(chatID string) {
	activeDeleted := false
	if state := a.chatPreview; state != nil {
		state.chats = slices.DeleteFunc(state.chats, func(chat chatData) bool { return chat.ID == chatID })
		activeDeleted = state.chat.ID == chatID
		state.panelSelected = min(state.panelSelected, max(0, len(state.chats)-1))
	}
	if activeDeleted {
		a.startNewChat()
	}
	a.invalidateChatSurfaces()
}

// selectChatModel applies one catalog entry to the next send and closes the catalog.
func (a *chatController) selectChatModel(index int) {
	state := a.chatPreview
	if state == nil {
		return
	}
	model, ok := a.aiSettings().ModelAt(index)
	if !ok {
		return
	}
	if state.panel == chatCommandPanel {
		replaceChatSlashToken(state.editor, "")
	}
	state.chat.Model = model
	state.nextModel = &model
	state.panelSelected = index
	restoreChatHistoryPanelLocked(state)
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
	// Persist outside the draft so the next chat reopen keeps this selection.
	persisted := common.Model{Name: model.Name, Provider: common.ProviderName(model.Provider), ProviderAlias: model.ProviderAlias}
	util.Go(a.lifecycleCtx, "persist selected chat model", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.services.SetDefaultChatModel(ctx, a.sessionID, persisted); err != nil {
			log.Printf("persist selected chat model: %v", err)
		}
	})
}

// insertChatSkill adds the stable inline tag that core expands through SkillRefs.
func (a *chatController) insertChatSkill(index int) {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	skills := a.chatSkills()
	if index < 0 || index >= len(skills) {
		return
	}
	skill := skills[index]
	tag := "{skill:" + skill.Name + "}"
	if state.panel == chatCommandPanel {
		replaceChatSlashToken(state.editor, tag+" ")
	} else {
		state.editor.InsertText(tag + " ")
	}
	state.panelSelected = index
	restoreChatHistoryPanelLocked(state)
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

func (a *chatController) insertChatMention(item chatCommandPaletteItem) {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	mentions := a.chatMentionCatalog()
	if item.sourceIndex < 0 || item.sourceIndex >= len(mentions) {
		return
	}
	mention := mentions[item.sourceIndex]
	if directory, ok := chatMentionOpensPicker(mention); ok {
		a.prepareChatComposerFilePick(true)
		a.pickChatComposerAttachment(directory)
		return
	}
	payload := strings.TrimSpace(mention.ID)
	if payload == "" {
		payload = strings.TrimSpace(mention.Name)
	}
	if mention.Kind == "" || payload == "" {
		return
	}
	tag := chatMentionTag(mention.Kind, payload)
	if state.panel == chatMentionPanel {
		replaceChatAtToken(state.editor, tag+" ")
	} else {
		state.editor.InsertText(tag + " ")
	}
	state.panelSelected = item.sourceIndex
	restoreChatHistoryPanelLocked(state)
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

// replaceChatSlashToken replaces the active token while preserving surrounding message text.
func replaceChatSlashToken(editor *woxui.TextEditor, replacement string) {
	replaceChatPrefixToken(editor, findChatSlashToken, replacement)
}

func replaceChatAtToken(editor *woxui.TextEditor, replacement string) {
	replaceChatPrefixToken(editor, findChatAtToken, replacement)
}

func replaceChatPrefixToken(editor *woxui.TextEditor, find func(woxui.TextEditingState) (chatSlashToken, bool), replacement string) {
	if editor == nil {
		return
	}
	state := editor.State()
	token, ok := find(state)
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
func (a *chatController) closeChatPanel() {
	if state := a.chatPreview; state != nil {
		if state.panel != "history" {
			restoreChatHistoryPanelLocked(state)
		}
		state.active = true
	}
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

// moveChatPanelSelection follows Flutter's clamped command-palette navigation.
func (a *chatController) moveChatPanelSelection(delta int) {
	state := a.chatPreview
	if state == nil || state.panel == "" {
		return
	}
	count := len(chatProjectHistoryRows(state.projects, state.chats))
	var commands []chatCommandPaletteItem
	if state.panel == chatProjectPanel {
		commands = a.projectPaletteItems()
		count = len(commands)
	} else if state.panel == chatMentionPanel {
		commands = a.filteredChatMentionItems(a.chatMentionCatalog(), state.panelQuery)
		count = len(commands)
	} else if state.panel != "history" {
		commands = a.commandPaletteItems()
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
	a.invalidateChatSurfaces()
}

// chatCatalogCategories defines the shared tab order for rendering and keyboard cycling.
func chatCatalogCategories(panel string) []struct{ id, key string } {
	if panel == chatMentionPanel {
		return []struct{ id, key string }{{"", "ui_all"}, {chatMentionKindFiles, "ui_ai_chat_mention_files"}, {chatMentionKindPlugin, "ui_ai_chat_mention_plugins"}}
	}
	return []struct{ id, key string }{{"", "ui_all"}, {"models", "ui_ai_chat_models"}, {chatProjectPanel, "plugin_ai_chat_project_section"}, {"skills", "ui_ai_skills"}}
}

// setChatCatalogFilter preserves the input token and resets navigation in the new category.
func (a *chatController) setChatCatalogFilter(filter string) {
	state := a.chatPreview
	if state == nil || (state.panel != chatCommandPanel && state.panel != chatMentionPanel) {
		return
	}
	state.panelFilter = filter
	state.panelSelected, state.panelScroll, state.panelViewport = 0, 0, 0
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

// setChatCatalogQuery filters the button-opened picker without editing the message draft.
func (a *chatController) setChatCatalogQuery(query string) {
	state := a.chatPreview
	if state == nil || state.panel != "models" && state.panel != "skills" && state.panel != chatProjectPanel {
		return
	}
	state.panelQuery = query
	state.panelSelected, state.panelScroll, state.panelViewport = 0, 0, 0
	a.invalidateChatSurfaces()
}

// activateChatPanelSelection applies the selected history or model row.
func (a *chatController) activateChatPanelSelection() {
	state := a.chatPreview
	if state == nil {
		return
	}
	panel := state.panel
	selected := state.panelSelected
	if panel == "history" {
		rows := chatProjectHistoryRows(state.projects, state.chats)
		if selected >= 0 && selected < len(rows) {
			a.activateProjectHistoryRow(rows[selected])
		}
	} else if panel == chatProjectPanel {
		items := a.projectPaletteItems()
		if selected >= 0 && selected < len(items) {
			id := ""
			if items[selected].sourceIndex >= 0 {
				id = state.projects.Projects[items[selected].sourceIndex].Id
			}
			a.chooseChatProject(id)
		}
	} else if panel == chatMentionPanel {
		items := a.filteredChatMentionItems(a.chatMentionCatalog(), state.panelQuery)
		if selected < 0 || selected >= len(items) {
			return
		}
		a.insertChatMention(items[selected])
	} else {
		items := a.commandPaletteItems()
		if selected < 0 || selected >= len(items) {
			return
		}
		if items[selected].group == chatProjectPanel {
			id := ""
			if items[selected].sourceIndex >= 0 {
				id = state.projects.Projects[items[selected].sourceIndex].Id
			}
			a.chooseChatProject(id)
		} else if items[selected].group == "models" {
			a.selectChatModel(items[selected].sourceIndex)
		} else {
			a.insertChatSkill(items[selected].sourceIndex)
		}
	}
}

// setChatSidebarViewport records the history drawer extent while a slash overlay owns panelViewport.
func (a *chatController) setChatSidebarViewport(height float32) {
	if state := a.chatPreview; state != nil {
		state.sidebarViewport = max(float32(1), height)
		maxOffset := max(float32(0), projectHistoryHeight(state)-state.sidebarViewport)
		state.sidebarScroll = min(max(float32(0), state.sidebarScroll), maxOffset)
	}
}

// scrollChatHistoryDrawer moves the conversation list even when a slash overlay is open.
func (a *chatController) scrollChatHistoryDrawer(delta float32) {
	state := a.chatPreview
	if state == nil || !chatHistoryVisible(state.panel, state.sidebarOpen) {
		return
	}
	if state.panel == "history" {
		a.scrollChatPanel(delta)
		return
	}
	maxOffset := max(float32(0), projectHistoryHeight(state)-state.sidebarViewport)
	state.sidebarScroll = min(max(float32(0), state.sidebarScroll+delta), maxOffset)
	a.invalidateChatSurfaces()
}

// setChatPanelViewport records the current catalog extent for scrolling and keyboard reveal.
func (a *chatController) setChatPanelViewport(height float32) {
	if state := a.chatPreview; state != nil {
		initialize := state.panelViewport <= 0
		state.panelViewport = max(float32(1), height)
		count := len(chatProjectHistoryRows(state.projects, state.chats))
		commands := []chatCommandPaletteItem(nil)
		if state.panel == chatProjectPanel {
			commands = a.projectPaletteItems()
			count = len(commands)
		} else if state.panel == chatMentionPanel {
			commands = a.filteredChatMentionItems(a.chatMentionCatalog(), state.panelQuery)
			count = len(commands)
		} else if state.panel != "history" {
			commands = a.commandPaletteItems()
			count = len(commands)
		}
		if initialize {
			if state.panel == "history" {
				ensureChatPanelSelectionVisibleLocked(state, count)
			} else {
				ensureChatCommandSelectionVisibleLocked(state, commands)
			}
		} else {
			contentHeight := projectHistoryHeight(state)
			if state.panel != "history" {
				contentHeight = chatCommandContentHeight(commands, state.panelDensityScale)
				contentHeight += state.panelLoadingHeight
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
	rowHeight := chatview.ChatScaledSize(state.panelDensityScale, chatCatalogRowHeight)
	headerHeight := chatview.ChatScaledSize(state.panelDensityScale, 46)
	gap := chatview.ChatScaledSize(state.panelDensityScale, chatview.ChatHistorySectionGap)
	maxOffset := max(float32(0), headerHeight+float32(count)*rowHeight+gap-state.panelViewport)
	rowTop := headerHeight + float32(state.panelSelected)*rowHeight
	for index, row := range chatProjectHistoryRows(state.projects, state.chats) {
		if row.Group && row.ProjectID == chatRecentsSection && index <= state.panelSelected {
			rowTop += gap
			break
		}
	}
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
	rowTop := chatCommandItemOffset(items, state.panelSelected, state.panelDensityScale) + state.panelLeadingHeight
	rowBottom := rowTop + chatview.ChatScaledSize(state.panelDensityScale, chatCommandItemHeight(items[state.panelSelected]))
	revealTop := rowTop
	item := items[state.panelSelected]
	if item.group == "models" || item.group == "skills" || item.group == chatProjectPanel || state.panel == chatMentionPanel {
		if state.panelSelected == 0 || chatCommandItemGroup(items[state.panelSelected-1]) != chatCommandItemGroup(item) {
			// Reveal the first row's context too; row-only scrolling hides non-selectable headers.
			revealTop -= chatview.ChatScaledSize(state.panelDensityScale, chatCatalogGroupHeaderHeight)
			if item.group == "models" && (state.panelSelected == 0 || items[state.panelSelected-1].group != "models") {
				revealTop -= chatview.ChatScaledSize(state.panelDensityScale, chatCatalogGroupHeaderHeight)
			}
			// Keep the selected row visible when the viewport cannot fit its headers as well.
			revealTop = max(revealTop, rowBottom-state.panelViewport)
		}
	}
	if revealTop < state.panelScroll {
		state.panelScroll = revealTop
	} else if rowBottom > state.panelScroll+state.panelViewport {
		state.panelScroll = rowBottom - state.panelViewport
	}
	maxOffset := max(float32(0), chatCommandContentHeight(items, state.panelDensityScale)+state.panelLoadingHeight-state.panelViewport)
	state.panelScroll = min(max(float32(0), state.panelScroll), maxOffset)
}

// scrollChatPanel applies pointer-wheel movement without changing the selected row.
func (a *chatController) scrollChatPanel(delta float32) {
	state := a.chatPreview
	if state == nil || state.panel == "" {
		return
	}
	contentHeight := projectHistoryHeight(state)
	if state.panel == chatProjectPanel {
		contentHeight = chatCommandContentHeight(a.projectPaletteItems(), state.panelDensityScale)
	} else if state.panel == chatMentionPanel {
		items := a.filteredChatMentionItems(a.chatMentionCatalog(), state.panelQuery)
		contentHeight = chatCommandContentHeight(items, state.panelDensityScale)
		if a.aiSettings() != nil && state.panelFilter != chatMentionKindFiles {
			contentHeight += chatMentionStatusHeight(items, a.aiSettings().PluginMentionsLoading(), a.aiSettings().PluginMentionsError(), state.panelDensityScale)
		}
	} else if state.panel != "history" {
		items := a.commandPaletteItems()
		contentHeight = chatCommandContentHeight(items, state.panelDensityScale)
		contentHeight += state.panelLoadingHeight
	}
	maxOffset := max(float32(0), contentHeight-state.panelViewport)
	state.panelScroll = min(max(float32(0), state.panelScroll+delta), maxOffset)
	a.invalidateChatSurfaces()
}

// applyTypedAIQuestion routes ask_user into the visible shared chat surface and cancels if no chat can answer it.
func (a *chatController) applyTypedAIQuestion(question aiQuestion) error {
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
		selectedChatVisible = a.selectedChatVisible(state.resultID)
	}
	if state == nil || !a.chatSurfaceVisible() || !(a.chatWindowOpen() || selectedChatVisible) {
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
	a.invalidateChatSurfaces()
	return nil
}

// answerAIQuestion resolves the core-side tool wait without blocking the UI thread.
func (a *chatController) answerAIQuestion(questionID, answer string) {
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
func (a *chatController) submitAIQuestionAnswer(answer string) {
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
	a.invalidateChatSurfaces()
}

// submitSelectedAIQuestionAnswer maps the selected choice or free text to the stable option value.
func (a *chatController) submitSelectedAIQuestionAnswer() {
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
func (a *chatController) selectAIQuestionOption(index int) {
	if state := a.chatPreview; state != nil && state.question != nil && index >= 0 && index < len(state.question.Options) {
		state.questionSelected = index
	}
	a.invalidateChatSurfaces()
}

// beginChatRequestLocked moves a prepared chat into its shared UI-owned streaming state.
func beginChatRequestLocked(state *chatPreviewState) (string, uint64, chatData) {
	state.nextModel = nil
	state.chat.IsStreaming = true
	upsertChatSummaryLocked(state, state.chat)
	state.sending = true
	state.error = ""
	state.scroll.FollowLatest()
	state.revision++
	return state.key, state.revision, cloneChatData(state.chat)
}

// postChatRequest sends one immutable chat snapshot and reconciles service failure with the current revision.
func (a *chatController) postChatRequest(key string, revision uint64, chat chatData) {
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
			if current := a.chatPreview; current != nil && current.key == key {
				if current.revision == revision {
					current.sending = false
					if err != nil {
						current.chat.IsStreaming = false
						upsertChatSummaryLocked(current, current.chat)
						current.error = err.Error()
					}
				} else if err != nil {
					for index := range current.chats {
						if current.chats[index].ID == chat.ID {
							current.chats[index].IsStreaming = false
							break
						}
					}
				}
			}
			a.invalidateChatSurfaces()
		}); dispatchErr != nil {
			log.Printf("dispatch chat request result: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("send chat message: %v", err)
		}
	})
}

// sendChatMessage appends the local user turn before core begins pushing authoritative snapshots.
func (a *chatController) sendChatMessage() {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	chatview.SubmitChatMessage(state.editor.State().Text, a.submitChatMessage, func() {
		state.editor.SetText("", false)
		a.invalidateChatSurfaces()
	})
}

// submitChatMessage validates and commits a turn; the shared submission flow clears the draft.
func (a *chatController) submitChatMessage(text string) bool {
	state := a.chatPreview
	if state == nil || state.editor == nil || state.loading || state.sending || state.importing || state.chat.IsStreaming || state.question != nil {
		return false
	}
	if !a.validateChatProjectDraft() {
		return false
	}
	if text == "" {
		state.error = "Enter a message first."
		a.invalidateChatSurfaces()
		return false
	}
	if strings.TrimSpace(state.chat.Model.Name) == "" {
		state.error = "Select an AI model in Wox settings first."
		a.invalidateChatSurfaces()
		return false
	}
	hasSkillTags := chatSkillTagPattern.MatchString(text)
	if hasSkillTags && !a.aiSettings().SkillsLoaded() {
		requestSkills := !a.aiSettings().SkillsLoading()
		if requestSkills {
			a.aiSettings().SetSkillsLoading(true)
		}
		state.error = "Loading skills; send again when the catalog is ready."
		if requestSkills {
			util.Go(a.lifecycleCtx, "load AI skills before chat send", a.loadAISkills)
		}
		a.invalidateChatSurfaces()
		return false
	}
	if hasSkillTags && state.projectSkillsLoading {
		state.error = "Loading skills; send again when the catalog is ready."
		a.invalidateChatSurfaces()
		return false
	}
	if chatMentionTagsPresent(text) && !a.aiSettings().PluginMentionsLoaded() {
		requestPlugins := !a.aiSettings().PluginMentionsLoading()
		if requestPlugins {
			a.aiSettings().SetPluginMentionsLoading(true)
		}
		state.error = a.language.translate("i18n:ui_ai_chat_mentions_loading_retry")
		if requestPlugins {
			util.Go(a.lifecycleCtx, "load chat mentions before send", a.loadChatPluginMentions)
		}
		a.invalidateChatSurfaces()
		return false
	}
	now := time.Now().UnixMilli()
	skills := a.chatSkills()
	skillRefs := chatSkillRefsFromText(text, skills)
	if unresolved := unresolvedChatSkillTag(text, skills); unresolved != "" {
		state.error = fmt.Sprintf("Unknown or disabled skill: %s", unresolved)
		a.invalidateChatSurfaces()
		return false
	}
	mentions := a.chatMentionCatalog()
	mentionRefs := chatMentionRefsFromText(text, mentions)
	if unresolved := unresolvedChatMentionTag(text, mentions); unresolved != "" {
		state.error = fmt.Sprintf(a.language.translate("i18n:ui_ai_chat_unknown_mention"), unresolved)
		a.invalidateChatSurfaces()
		return false
	}
	state.chat.Conversations = append(state.chat.Conversations, chatConversation{ID: newID(), Role: "user", Text: text, Attachments: slices.Clone(state.attachments), SkillRefs: skillRefs, Mentions: mentionRefs, Timestamp: now})
	state.chat.UpdatedAt = now
	state.attachments = nil
	key, revision, chat := beginChatRequestLocked(state)
	a.invalidateChatSurfaces()
	a.postChatRequest(key, revision, chat)
	return true
}

// stopChatMessage cancels the active core stream while leaving its last snapshot visible.
func (a *chatController) stopChatMessage() {
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
			if state := a.chatPreview; state != nil {
				if state.chat.ID == chatID {
					if err != nil {
						state.error = err.Error()
					} else {
						state.chat.IsStreaming = false
						state.sending = false
						upsertChatSummaryLocked(state, state.chat)
						if !stopped {
							state.error = "The chat stream was already stopped."
						}
					}
				} else if err == nil {
					for index := range state.chats {
						if state.chats[index].ID == chatID {
							state.chats[index].IsStreaming = false
							break
						}
					}
				}
			}
			a.invalidateChatSurfaces()
		}); dispatchErr != nil {
			log.Printf("dispatch stopped chat: %v", dispatchErr)
		}
		if err != nil {
			log.Printf("stop chat message: %v", err)
		}
	})
}

// copyChatText reports clipboard failures inside the chat surface while keeping native details below Window.
func (a *chatController) copyChatText(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if err := a.window().WriteClipboardText(text); err != nil {
		if state := a.chatPreview; state != nil {
			state.error = fmt.Sprintf("Copy failed: %v", err)
		}
		a.invalidateChatSurfaces()
		return false
	}
	return true
}

// editChatConversation restores a user turn into the composer and discards the stale branch after it.
func (a *chatController) editChatConversation(messageID string) {
	state := a.chatPreview
	if state == nil || state.editor == nil {
		return
	}
	if state.chat.IsStreaming || state.sending || state.importing || state.question != nil {
		state.error = "Stop the active response before editing a message."
		a.invalidateChatSurfaces()
		return
	}
	a.cancelChatAttachmentImports()
	// Editing replaces the current draft. Bump revision so an in-flight import cannot append to it.
	state.revision++
	messageIndex := slices.IndexFunc(state.chat.Conversations, func(message chatConversation) bool {
		return message.ID == messageID && message.Role == "user"
	})
	if messageIndex < 0 {
		state.error = "The user message is no longer available."
		a.invalidateChatSurfaces()
		return
	}
	text := state.chat.Conversations[messageIndex].Text
	state.attachments = slices.Clone(state.chat.Conversations[messageIndex].Attachments)
	state.chat.Conversations = slices.Clone(state.chat.Conversations[:messageIndex])
	state.chat.CompactionEntries = nil
	state.chat.DebugTrace = nil
	state.chat.UpdatedAt = time.Now().UnixMilli()
	state.editor.SetText(text, false)
	state.panel = ""
	state.active = true
	state.scroll.FollowLatest()
	state.error = ""
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

// regenerateChatConversation resends the user turn preceding an assistant response through the normal chat channel.
func (a *chatController) regenerateChatConversation(messageID string) {
	state := a.chatPreview
	if state == nil {
		return
	}
	// Retry also advances the request revision used by pending attachment imports.
	if state.importing {
		return
	}
	if state.chat.IsStreaming || state.sending || state.question != nil {
		state.error = "Stop the active response before retrying it."
		a.invalidateChatSurfaces()
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
		a.invalidateChatSurfaces()
		return
	}
	state.chat.Conversations = slices.Clone(state.chat.Conversations[:userIndex+1])
	state.chat.CompactionEntries = nil
	state.chat.DebugTrace = nil
	state.chat.UpdatedAt = time.Now().UnixMilli()
	key, revision, chat := beginChatRequestLocked(state)
	a.invalidateChatSurfaces()
	a.postChatRequest(key, revision, chat)
}

// launcherChatInputActive excludes ordinary searches when a separate chat stays active.
func (a *chatController) launcherChatInputActive() bool {
	if !a.chatWindowOpen() || a.chatFullscreen {
		return true
	}
	return a.chatPreviewSelected()
}

// onChatPreviewKey routes only the launcher's visible chat surface.
func (a *chatController) onChatPreviewKey(event woxui.KeyEvent) bool {
	if !a.launcherChatInputActive() {
		return false
	}
	return a.handleChatKey(event, false)
}

func (a *chatController) onDedicatedChatKey(event woxui.KeyEvent) bool {
	return a.handleChatKey(event, true)
}

// handleChatKey shares conversation actions while keeping window dismissal local.
func (a *chatController) handleChatKey(event woxui.KeyEvent, dedicated bool) bool {
	// Key releases must not repeat palette navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	state := a.chatPreview
	active := state != nil && state.active
	if state != nil && (state.projects.Editor != nil || state.projects.DeleteID != "") {
		if event.Key == woxui.KeyEscape {
			a.dismissChatProject()
			return true
		}
		// Let the modal's focused fields/buttons own Enter, Tab and text input.
		return false
	}
	hasQuestion := active && state.question != nil
	panel := ""
	if active {
		panel = state.panel
	}
	questionOptions := 0
	questionSelected := 0
	if hasQuestion {
		questionOptions = len(state.question.Options)
		questionSelected = state.questionSelected
	}
	if active && !dedicated && hotkeyMatches(chatOpenWindowHotkey(), event) {
		if !event.Repeat && a.openDedicatedChatWindow != nil {
			a.openDedicatedChatWindow()
		}
		return true
	}
	if active && event.Key == woxui.Key("b") && event.Modifiers.HasPrimary() {
		// Ctrl/Cmd+B toggles the conversation sidebar, matching Flutter's preview fullscreen shortcut.
		a.toggleChatPanel("history")
		return true
	}
	// Both pickers own Tab so category cycling keeps focus in the composer.
	if (panel == chatCommandPanel || panel == chatMentionPanel) && event.Key == woxui.KeyTab {
		categories := chatCatalogCategories(panel)
		index := slices.IndexFunc(categories, func(category struct{ id, key string }) bool { return category.id == state.panelFilter })
		step := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			step = -1
		}
		a.setChatCatalogFilter(categories[(index+step+len(categories))%len(categories)].id)
		return true
	}
	// A persistent history drawer does not own composer keys. Its rows and
	// delete buttons handle activation through their own retained focus nodes.
	if panel != "" && panel != "history" {
		return chatview.ChatCatalogKey(event, a.moveChatPanelSelection, a.activateChatPanelSelection, a.closeChatPanel)
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
		if dedicated {
			// The dedicated window is the root surface. Escape may dismiss a
			// floating overlay, but it must not climb into the persistent
			// history sidebar or close the composer window.
			return true
		}
		if a.isPrimary {
			a.exitChatMode()
		} else {
			a.closePreviewWindow()
		}
		return true
	}
	return chatview.ChatComposerKey(event, state.chat.IsStreaming || state.sending, state.importing, a.sendChatMessage, func(delta float32) {
		state.scroll.ScrollPage(delta)
		a.invalidateChatSurfaces()
	})
}

// onChatPreviewTextInput routes committed and composing text to the currently visible chat editor.
func (a *chatController) onChatPreviewTextInput(_ woxui.TextInputEvent) bool {
	if !a.launcherChatInputActive() {
		return false
	}
	state := a.chatPreview
	if state == nil || !state.active {
		return false
	}
	return true
}

func (a *chatController) editChatKey(event woxui.KeyEvent) {
	if state := a.chatPreview; state != nil && state.active && state.editor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 5}}
		_, changed := handleFormEditorKey(state.editor, definition, event)
		if changed {
			state.error = ""
		}
	}
	a.invalidateChatSurfaces()
}

func (a *chatController) editAIQuestionKey(event woxui.KeyEvent) {
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		definition := formDefinition{Type: "textbox", Value: formDefinitionValue{MaxLines: 4}}
		_, changed := handleFormEditorKey(state.questionEditor, definition, event)
		if changed {
			state.error = ""
		}
	}
	a.invalidateChatSurfaces()
}

func (a *chatController) moveAIQuestionSelection(delta int) {
	if state := a.chatPreview; state != nil && state.question != nil && len(state.question.Options) > 0 {
		state.questionSelected = (state.questionSelected + delta + len(state.question.Options)) % len(state.question.Options)
	}
	a.invalidateChatSurfaces()
}

func (a *chatController) focusChatInput() {
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

func (a *chatController) focusAIQuestionInput() {
	if state := a.chatPreview; state != nil && state.question != nil {
		state.active = true
	}
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

func (a *chatController) setChatText(value string) {
	requestModels := false
	requestSkills := false
	requestPlugins := false
	if state := a.chatPreview; state != nil && state.editor != nil && state.question == nil {
		state.editor.SetText(value, false)
		state.error = ""
		slashToken, hasSlash := findChatSlashToken(state.editor.State())
		atToken, hasAt := findChatAtToken(state.editor.State())
		if hasSlash {
			if state.panel != chatCommandPanel {
				a.refreshChatProjectSkills()
			}
			queryChanged := state.panel != chatCommandPanel || state.panelQuery != slashToken.query
			if state.panel != chatCommandPanel {
				state.panelFilter = ""
			}
			if state.panel == "history" {
				rememberChatHistorySidebarLocked(state)
			}
			state.panel = chatCommandPanel
			state.panelQuery = slashToken.query
			state.active = true
			if queryChanged {
				state.panelSelected = 0
				state.panelScroll = 0
				state.panelViewport = 0
			}
			if a.aiSettings() != nil {
				requestModels = !a.aiSettings().ModelsLoaded() && !a.aiSettings().ModelsLoading()
				requestSkills = !a.aiSettings().SkillsLoaded() && !a.aiSettings().SkillsLoading()
				if requestModels {
					a.aiSettings().SetModelsLoading(true)
				}
				if requestSkills {
					a.aiSettings().SetSkillsLoading(true)
				}
			}
		} else if hasAt {
			queryChanged := state.panel != chatMentionPanel || state.panelQuery != atToken.query
			if state.panel != chatMentionPanel {
				state.panelFilter = ""
			}
			if state.panel == "history" {
				rememberChatHistorySidebarLocked(state)
			}
			state.panel = chatMentionPanel
			state.panelQuery = atToken.query
			state.active = true
			if queryChanged {
				state.panelSelected = 0
				state.panelScroll = 0
				state.panelViewport = 0
			}
			if a.aiSettings() != nil {
				requestPlugins = !a.aiSettings().PluginMentionsLoaded() && !a.aiSettings().PluginMentionsLoading()
				if requestPlugins {
					a.aiSettings().SetPluginMentionsLoading(true)
				}
			}
		} else if state.panel == chatCommandPanel || state.panel == chatMentionPanel {
			restoreChatHistoryPanelLocked(state)
		}
	}
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for chat", a.loadAIModels)
	}
	if requestSkills {
		util.Go(a.lifecycleCtx, "load AI skills for chat", a.loadAISkills)
	}
	if requestPlugins {
		util.Go(a.lifecycleCtx, "load chat plugin mentions", a.loadChatPluginMentions)
	}
	a.invalidateChatSurfaces()
}

func (a *chatController) setAIQuestionText(value string) {
	if state := a.chatPreview; state != nil && state.question != nil && state.questionEditor != nil {
		state.questionEditor.SetText(value, false)
		state.error = ""
	}
	a.invalidateChatSurfaces()
}

// enterChatMode hides launcher chrome while retaining the same native window and shared preview state.
func (a *chatController) enterChatMode() {
	a.chatFullscreen = true
	if state := a.chatPreview; state != nil {
		state.active = true
	}
	a.prefetchChatCatalogs()
	a.updateChatTextInput(true)
	_ = a.applyWindowBounds()
	a.invalidateChatSurfaces()
}

// exitChatMode restores query ownership without destroying the in-progress conversation.
func (a *chatController) exitChatMode() {
	a.chatFullscreen = false
	if state := a.chatPreview; state != nil {
		state.active = a.chatWindowOpen()
	}
	a.restoreQueryTextInput()
	// Returning from chat selects the whole query so a new search starts with a clean slate.
	a.selectEntireQuery()
	_ = a.applyWindowBounds()
	a.invalidateChatSurfaces()
}

// deactivateChatPreview releases input ownership and cancels an ask_user request that is no longer visible.
func (a *chatController) deactivateChatPreview() {
	if a.chatWindowOpen() {
		a.chatFullscreen = false
		return
	}
	a.chatMarkdown = chatMarkdownCache{}
	a.cancelChatAttachmentImports()
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
		a.invalidateChatSurfaces()
	}
}

// resetChatPreview discards state at launcher lifecycle boundaries and unblocks any pending ask_user call.
func (a *chatController) resetChatPreview() {
	if a.chatWindowOpen() {
		a.chatFullscreen = false
		return
	}
	questionID := ""
	if a.chatPreview != nil {
		// Preview state is discarded on hide, but the user's sidebar choice is not.
		a.chatHistoryOpen = chatHistoryVisible(a.chatPreview.panel, a.chatPreview.sidebarOpen)
		if a.chatPreview.question != nil {
			questionID = a.chatPreview.question.QuestionID
		}
	}
	a.cancelChatAttachmentImports()
	a.chatPreview = nil
	a.chatMarkdown = chatMarkdownCache{}
	a.chatFullscreen = false
	if questionID != "" {
		util.Go(a.lifecycleCtx, "cancel reset AI question", func() {
			a.answerAIQuestion(questionID, "User cancelled")
		})
	}
}

func (a *chatController) updateChatTextInput(enabled bool) {
	state := woxui.TextInputState{}
	if enabled {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 32, Y: 420, Width: 1, Height: 22}}
	}
	target := a.chatTextInputWindow()
	if target != nil {
		_ = target.SetTextInputState(state)
	}
}

// clampChatPreviewScroll updates the shared component's keyboard-scroll bounds.
func (a *chatController) clampChatPreviewScroll(maxOffset float32) {
	if state := a.chatPreview; state != nil {
		state.scroll.SetExtent(maxOffset)
	}
}

// scrollChatPreview preserves manual scrollback until the user returns near the latest message.
func (a *chatController) scrollChatPreview(delta, maxOffset float32) {
	if delta == 0 {
		return
	}
	if state := a.chatPreview; state != nil {
		state.scroll.Scroll(delta, maxOffset)
	}
	a.invalidateChatSurfaces()
}

// dismissChatAttachment removes one pending reference without sending it.
func (a *chatController) dismissChatAttachment(id string) {
	if state := a.chatPreview; state != nil {
		state.attachments = slices.DeleteFunc(state.attachments, func(attachment common.AIChatAttachment) bool { return attachment.ID == id })
	}
	a.invalidateChatSurfaces()
}
