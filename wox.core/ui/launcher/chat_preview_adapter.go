package launcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildChatPreview prepares chat view props while retaining lifecycle and actions in the controller.
func (a *App) buildChatPreview(result queryResult, preview queryPreview, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	snapshot, err := a.chatPreviewSnapshotFor(result, preview)
	if err != nil {
		return previewview.PreviewError(fmt.Sprintf("Invalid chat preview: %v", err), width, height, palette.componentTheme())
	}

	const headerHeight = float32(52)
	const inputHeight = float32(98)
	innerWidth := max(float32(0), width-20)
	innerHeight := max(float32(0), height-14)
	questionHeight := chatQuestionPanelHeight(snapshot, innerHeight)
	debugHeight := float32(0)
	if snapshot.panel == "debug" {
		debugHeight = chatCatalogPanelHeight(snapshot, innerHeight-questionHeight)
	}
	messagesHeight := max(float32(80), innerHeight-headerHeight-inputHeight-questionHeight-debugHeight)

	var debug *previewview.ChatDebugProps
	if debugHeight > 0 {
		props := a.chatDebugProps(snapshot, palette, innerWidth, debugHeight)
		debug = &props
	}
	var question *previewview.ChatQuestionProps
	if questionHeight > 0 {
		props := a.chatQuestionProps(snapshot, palette, innerWidth, questionHeight)
		question = &props
	}
	var catalog *previewview.ChatCatalogProps
	if snapshot.panel == "history" {
		catalogWidth := min(float32(260), width)
		props := a.chatCatalogProps(snapshot, palette, catalogWidth, height)
		catalog = &props
	} else if snapshot.panel == "models" || snapshot.panel == "skills" || snapshot.panel == chatCommandPanel {
		catalogHeight := chatCatalogPanelHeight(snapshot, innerHeight-questionHeight)
		props := a.chatCatalogProps(snapshot, palette, innerWidth, catalogHeight)
		catalog = &props
	}
	panel := snapshot.panel
	return previewview.ChatPreview(previewview.ChatPreviewProps{
		Width: width, Height: height, Key: snapshot.key, Panel: panel,
		Header:   a.chatHeaderProps(snapshot, palette, innerWidth, headerHeight),
		Messages: a.chatMessagesProps(snapshot, palette, innerWidth, messagesHeight, imageScale),
		Debug:    debug, Question: question,
		Input:   a.chatInputProps(snapshot, palette, innerWidth, inputHeight),
		Catalog: catalog, OnDismiss: func() { a.toggleChatPanel(panel) },
	})
}

// chatHeaderProps resolves the current title and available controller actions.
func (a *App) chatHeaderProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatHeaderProps {
	title := strings.TrimSpace(snapshot.chat.Title)
	if title == "" {
		title = a.translate("i18n:ui_ai_chat_new_chat")
	}
	if strings.TrimSpace(title) == "" || title == "i18n:ui_ai_chat_new_chat" {
		title = "New chat"
	}
	hasDebug := len(bytes.TrimSpace(snapshot.chat.DebugTrace)) > 0 && !bytes.Equal(bytes.TrimSpace(snapshot.chat.DebugTrace), []byte("null"))
	exitLabel := a.translate("i18n:ui_close")
	if strings.TrimSpace(exitLabel) == "" || exitLabel == "i18n:ui_close" {
		exitLabel = "Close"
	}
	exitLabel += " (Esc)"
	historyLabel := a.translate("i18n:ui_action_toggle_sidebar")
	// The sidebar toggle advertises the same Ctrl/Cmd+B shortcut Flutter binds to preview fullscreen.
	historyTooltip := historyLabel + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("b")), "+") + ")"
	return previewview.ChatHeaderProps{
		Width: width, Height: height, Key: snapshot.key, Title: title, HistoryOpen: snapshot.panel == "history",
		ShowDebug: hasDebug, DebugOpen: snapshot.panel == "debug", ShowExit: launcherChromeHidden(a.show, a.chatFullscreen), ExitLabel: exitLabel,
		HistoryLabel: historyLabel, HistoryTooltip: historyTooltip, Theme: palette.componentTheme(),
		OnHistory: func() { a.toggleChatPanel("history") }, OnHistoryHover: a.setPreviewTooltip, OnDebug: func() { a.toggleChatPanel("debug") },
		OnExit: a.closePreviewWindow, OnDrag: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start chat preview window drag: %v", err)
			}
		}, OnExitHover: a.setPreviewTooltip,
	}
}

// chatCatalogPanelHeight gives catalogs room without permanently shrinking the message pane.
func chatCatalogPanelHeight(snapshot *chatPreviewSnapshot, available float32) float32 {
	if snapshot == nil || snapshot.panel == "" || snapshot.question != nil {
		return 0
	}
	if snapshot.panel == "models" || snapshot.panel == "skills" || snapshot.panel == chatCommandPanel {
		items := chatCommandPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.panelQuery, snapshot.panel)
		contentHeight := float32(len(items)) * chatCatalogRowHeight
		if snapshot.panel == chatCommandPanel {
			contentHeight = chatCommandContentHeight(items)
		} else if len(items) > 0 {
			contentHeight += chatCatalogGroupHeaderHeight
		}
		contentHeight = max(float32(40), contentHeight)
		return min(contentHeight+14, min(float32(310), max(float32(96), available-104)))
	}
	return min(float32(270), max(float32(150), available*0.44))
}

// chatCatalogProps prepares history, model, and skill rows without constructing widgets.
func (a *App) chatCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	if snapshot.panel == "history" {
		return a.chatHistoryCatalogProps(snapshot, palette, width, height)
	}
	grouped := snapshot.panel == chatCommandPanel
	viewportHeight := max(float32(40), height-44)
	if grouped {
		viewportHeight = max(float32(40), height-14)
	}
	count := len(snapshot.chats)
	label := a.translate("i18n:ui_ai_chat_new_chat")
	commands := []chatCommandPaletteItem(nil)
	if snapshot.panel != "history" {
		commands = chatCommandPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.panelQuery, snapshot.panel)
		count = len(commands)
	}
	if snapshot.panel == "models" {
		label = a.translate("i18n:ui_ai_chat_select_model_title")
	} else if snapshot.panel == "skills" {
		label = a.translate("i18n:ui_ai_skills")
	} else if grouped {
		label = ""
	}
	contentHeight := float32(count) * chatCatalogRowHeight
	if grouped {
		contentHeight = chatCommandContentHeight(commands)
	}
	contentHeight = max(viewportHeight, contentHeight)
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	if count > 0 && snapshot.panelViewport <= 0 {
		selected := min(max(0, snapshot.panelSelected), count-1)
		rowTop := float32(selected) * chatCatalogRowHeight
		if grouped {
			rowTop = chatCommandItemOffset(commands, selected)
		}
		rowBottom := rowTop + chatCatalogRowHeight
		if rowTop < offset {
			offset = rowTop
		} else if rowBottom > offset+viewportHeight {
			offset = rowBottom - viewportHeight
		}
	}
	a.setChatPanelViewport(viewportHeight)

	items := make([]previewview.ChatCatalogItemProps, 0, count)
	for index, command := range commands {
		groupLabel := ""
		if grouped {
			if command.group == "models" {
				groupLabel = a.translate("i18n:ui_ai_chat_select_model_title")
			} else {
				groupLabel = a.translate("i18n:ui_ai_skills")
			}
		}
		items = append(items, previewview.ChatCatalogItemProps{
			SelectID: fmt.Sprintf("chat-%s-row-%s-%d", command.group, snapshot.key, index), GroupLabel: groupLabel,
			Kind: command.group, Title: command.title, Subtitle: command.subtitle, Selected: index == snapshot.panelSelected, Current: command.current,
			OnSelect: func() {
				if command.group == "models" {
					a.selectChatModel(command.sourceIndex)
				} else {
					a.insertChatSkill(command.sourceIndex)
				}
			},
		})
	}
	emptyMessage := "No saved conversations"
	if snapshot.panel == "models" {
		emptyMessage = "No AI models configured"
		if snapshot.modelsLoading {
			emptyMessage = "Loading models…"
		} else if snapshot.modelsError != "" {
			emptyMessage = snapshot.modelsError
		}
	} else if snapshot.panel == "skills" {
		emptyMessage = "No enabled skills"
		if snapshot.skillsLoading {
			emptyMessage = "Loading skills…"
		} else if snapshot.skillsError != "" {
			emptyMessage = snapshot.skillsError
		}
	} else if grouped {
		emptyMessage = a.translate("i18n:ui_no_data")
		if snapshot.modelsLoading || snapshot.skillsLoading {
			emptyMessage = "Loading…"
		} else if snapshot.modelsError != "" && snapshot.skillsError != "" {
			emptyMessage = snapshot.modelsError + "; " + snapshot.skillsError
		}
	}
	return previewview.ChatCatalogProps{
		Width: width, Height: height, Key: snapshot.key, Label: label, Items: items, EmptyMessage: emptyMessage,
		Scroll: offset, ContentHeight: contentHeight, ShowNew: snapshot.panel == "history", NewLabel: a.translate("i18n:ui_ai_chat_new_chat"), Theme: palette.componentTheme(),
		OnScroll: a.scrollChatPanel, OnNew: a.startNewChat,
	}
}

// chatHistoryCatalogProps groups visible conversations using Flutter's local-day boundaries.
func (a *App) chatHistoryCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	viewportHeight := max(float32(40), height-24)
	items := make([]previewview.ChatCatalogItemProps, 0, len(snapshot.chats))
	groupLabels := map[string]string{
		"today":     a.translate("i18n:ui_ai_chat_history_today"),
		"yesterday": a.translate("i18n:ui_ai_chat_history_yesterday"),
		"history":   a.translate("i18n:ui_ai_chat_history_history"),
	}
	contentHeight := float32(46)
	now := time.Now()
	type indexedChat struct {
		index int
		chat  chatData
	}
	grouped := map[string][]indexedChat{"today": {}, "yesterday": {}, "history": {}}
	for index, chat := range snapshot.chats {
		if !chat.IsSummary && len(chat.Conversations) == 0 {
			continue
		}
		group := chatHistoryGroup(chat.UpdatedAt, now)
		grouped[group] = append(grouped[group], indexedChat{index: index, chat: chat})
	}
	for _, group := range []string{"today", "yesterday", "history"} {
		if len(grouped[group]) == 0 {
			continue
		}
		contentHeight += 32
		for groupIndex, entry := range grouped[group] {
			chatID := entry.chat.ID
			title := strings.TrimSpace(entry.chat.Title)
			if title == "" {
				title = a.translate("i18n:ui_ai_chat_new_chat")
			}
			groupLabel := ""
			if groupIndex == 0 {
				groupLabel = groupLabels[group]
			}
			contentHeight += 46
			items = append(items, previewview.ChatCatalogItemProps{
				SelectID: fmt.Sprintf("chat-history-row-%s-%d", snapshot.key, entry.index), DeleteID: fmt.Sprintf("chat-history-delete-%s-%d", snapshot.key, entry.index),
				Kind: "history", Title: title, GroupLabel: groupLabel, DeleteLabel: a.translate("i18n:ui_ai_chat_delete_chat"), Selected: chatID == snapshot.chat.ID,
				OnSelect: func() { a.selectChatHistory(chatID) }, OnDelete: func() { a.deleteChatHistory(chatID) },
			})
		}
	}
	contentHeight = max(viewportHeight, contentHeight)
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	a.setChatPanelViewport(viewportHeight)
	return previewview.ChatCatalogProps{
		Width: width, Height: height, Key: snapshot.key, Items: items, EmptyMessage: a.translate("i18n:ui_no_data"),
		Scroll: offset, ContentHeight: contentHeight, ShowNew: true, NewLabel: a.translate("i18n:ui_ai_chat_new_chat"), Theme: palette.componentTheme(),
		OnScroll: a.scrollChatPanel, OnNew: a.startNewChat,
	}
}

func chatHistoryGroup(updatedAt int64, now time.Time) string {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	updated := time.UnixMilli(updatedAt).In(now.Location())
	if !updated.Before(today) {
		return "today"
	}
	if !updated.Before(today.AddDate(0, 0, -1)) {
		return "yesterday"
	}
	return "history"
}

// chatDebugProps prepares the copyable trace while the controller owns cached text measurement and scrolling.
func (a *App) chatDebugProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatDebugProps {
	innerWidth := max(float32(0), width-20)
	viewportHeight := max(float32(40), height-42)
	summary, value := formatChatDebugTrace(snapshot.chat.DebugTrace)
	textWidth := max(float32(20), innerWidth-16)
	hash := sha256.Sum256([]byte(value))
	layout := a.previewTextLayout(fmt.Sprintf("chat-debug\x00%s\x00%x", snapshot.key, hash[:8]), value, woxui.TextStyle{Size: 10}, textWidth, 16)
	contentHeight := max(viewportHeight, layout.Size.Height+16)
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	a.clampChatDebugScroll(maxOffset)
	return previewview.ChatDebugProps{
		Width: width, Height: height, Key: snapshot.key, Summary: summary, Value: value, Layout: layout,
		Scroll: offset, ContentHeight: contentHeight, Theme: palette.componentTheme(), OnScroll: a.scrollChatDebugPanel, OnCopy: func() { a.copyChatText(value) },
	}
}

// formatChatDebugTrace keeps the raw protocol payload intact while surfacing a compact token and event summary.
func formatChatDebugTrace(raw json.RawMessage) (string, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "Debug trace", "No debug trace is available."
	}
	var metadata struct {
		Events                   []json.RawMessage `json:"Events"`
		EstimatedPersistedTokens int               `json:"EstimatedPersistedTokens"`
		EstimatedRuntimeTokens   int               `json:"EstimatedRuntimeTokens"`
	}
	_ = json.Unmarshal(trimmed, &metadata)
	summary := fmt.Sprintf("Trace · %d events · %d persisted / %d runtime tokens", len(metadata.Events), metadata.EstimatedPersistedTokens, metadata.EstimatedRuntimeTokens)
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, trimmed, "", "  "); err != nil {
		return summary, string(trimmed)
	}
	return summary, formatted.String()
}

// chatRenderItem separates controller conversations from the visible round disclosure state.
type chatRenderItem struct {
	kind          string
	conversation  chatConversation
	tools         []chatConversation
	showMeta      bool
	hideReasoning bool
	roundID       string
	roundStart    int64
	roundEnd      int64
	roundExpanded bool
}

// chatRenderItems groups assistant and tool messages into the same completed rounds as Flutter.
func chatRenderItems(conversations []chatConversation, streaming bool, expandedRounds map[string]bool) []chatRenderItem {
	items := make([]chatRenderItem, 0, len(conversations))
	round := make([]chatConversation, 0)
	appendVisible := func(messages []chatConversation, showLastMeta bool) {
		for index := 0; index < len(messages); {
			if messages[index].Role != "tool" {
				items = append(items, chatRenderItem{conversation: messages[index], showMeta: showLastMeta && index == len(messages)-1})
				index++
				continue
			}
			end := index + 1
			for end < len(messages) && messages[end].Role == "tool" {
				end++
			}
			tools := append([]chatConversation(nil), messages[index:end]...)
			activityID := "tool-activity:" + tools[0].ID
			items = append(items, chatRenderItem{kind: "tool-activity", roundID: activityID, tools: tools, roundExpanded: expandedRounds[activityID]})
			index = end
		}
	}
	closeRound := func(complete bool) {
		if len(round) == 0 {
			return
		}
		finalIndex := -1
		if round[len(round)-1].Role == "assistant" {
			finalIndex = len(round) - 1
		}
		intermediate := append([]chatConversation(nil), round...)
		if finalIndex >= 0 {
			intermediate = intermediate[:finalIndex]
			if reasoning := strings.TrimSpace(round[finalIndex].Reasoning); reasoning != "" {
				reasoningMessage := round[finalIndex]
				reasoningMessage.Text = ""
				reasoningMessage.Images = nil
				intermediate = append(intermediate, reasoningMessage)
			}
		}
		canCollapse := complete && finalIndex >= 0 && len(intermediate) > 0
		if canCollapse {
			firstID := round[0].ID
			lastID := round[finalIndex].ID
			roundID := "round:" + firstID + ":" + lastID
			start := int64(0)
			for _, message := range round {
				if message.Role == "assistant" {
					start = message.Timestamp
					break
				}
			}
			expanded := expandedRounds[roundID]
			items = append(items, chatRenderItem{kind: "round", roundID: roundID, roundStart: start, roundEnd: round[finalIndex].Timestamp, roundExpanded: expanded})
			if expanded {
				appendVisible(intermediate, false)
			}
			items = append(items, chatRenderItem{conversation: round[finalIndex], showMeta: true, hideReasoning: true})
			round = round[:0]
			return
		}
		appendVisible(round, finalIndex >= 0)
		round = round[:0]
	}

	for _, conversation := range conversations {
		if conversation.Role == "system" {
			continue
		}
		if conversation.Role == "user" {
			closeRound(true)
			items = append(items, chatRenderItem{conversation: conversation, showMeta: true})
			continue
		}
		round = append(round, conversation)
	}
	closeRound(!streaming)
	return items
}

// formatChatRoundDuration matches Flutter's compact seconds and minute labels.
func formatChatRoundDuration(start, end int64) string {
	duration := end - start
	if start <= 0 || end <= 0 || duration < 0 {
		duration = 0
	}
	seconds := (duration + 500) / 1000
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	seconds %= 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// chatMessagesProps prepares semantic messages and leaves their widget composition to the view.
func (a *App) chatMessagesProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height, imageScale float32) previewview.ChatMessagesProps {
	// ponytail: Add viewport virtualization after profiling a real long chat; the current full list preserves exact scroll height with less state.
	innerWidth := max(float32(0), width-4)
	innerHeight := max(float32(0), height-14)
	emptyMessage := a.translate("i18n:ui_ai_chat_empty_prompt")
	if strings.TrimSpace(emptyMessage) == "" || emptyMessage == "i18n:ui_ai_chat_empty_prompt" {
		emptyMessage = "What do you want to ask Wox today?"
	}
	if snapshot.loading {
		emptyMessage = "Loading conversation…"
	}
	emptyMetrics, _ := a.window.MeasureText(emptyMessage, woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold})
	props := previewview.ChatMessagesProps{
		Width: width, Height: height, Key: snapshot.key, EmptyMessage: emptyMessage,
		EmptyTextWidth: emptyMetrics.Size.Width, EmptyTextHeight: emptyMetrics.Size.Height,
		ContentHeight: innerHeight, Scroll: snapshot.scroll, Theme: palette.componentTheme(), OnScroll: a.scrollChatPreview,
	}
	if len(snapshot.chat.Conversations) == 0 {
		return props
	}
	renderItems := chatRenderItems(snapshot.chat.Conversations, snapshot.chat.IsStreaming || snapshot.sending, snapshot.expandedRounds)
	props.Messages = make([]previewview.ChatMessageProps, 0, len(renderItems))
	for index, item := range renderItems {
		if item.kind == "round" {
			label := a.translate("i18n:ui_ai_chat_round_worked_duration")
			if strings.TrimSpace(label) == "" || label == "i18n:ui_ai_chat_round_worked_duration" {
				label = "Worked for %s"
			}
			label = fmt.Sprintf(label, formatChatRoundDuration(item.roundStart, item.roundEnd))
			roundID := item.roundID
			props.Messages = append(props.Messages, previewview.ChatMessageProps{
				Key: roundID, Kind: "round", RoundLabel: label, RoundExpanded: item.roundExpanded, Theme: palette.componentTheme(), OnToggleRound: func() { a.toggleChatDisclosure(roundID) },
			})
			continue
		}
		if item.kind == "tool-activity" {
			props.Messages = append(props.Messages, a.chatToolActivityProps(item, snapshot.expandedRounds, palette, innerWidth))
			continue
		}
		props.Messages = append(props.Messages, a.chatMessageProps(snapshot.key, index, item.conversation, palette, innerWidth, item.showMeta, item.hideReasoning, imageScale))
	}
	props.ContentHeight = previewview.ChatMessagesContentHeight(props.Messages, innerHeight)
	maxOffset := max(float32(0), props.ContentHeight-innerHeight)
	a.clampChatPreviewScroll(maxOffset)
	return props
}

// chatToolActivityProps prepares Flutter's grouped tool summary and nested detail rows.
func (a *App) chatToolActivityProps(item chatRenderItem, expanded map[string]bool, palette uiPalette, width float32) previewview.ChatMessageProps {
	status := chatToolActivityStatus(item.tools)
	statusColor := chatToolStatusColor(status, palette.componentTheme())
	actions := make([]string, 0, len(item.tools))
	seen := make(map[string]bool)
	leading := "tool"
	for _, conversation := range item.tools {
		action := a.chatToolActionLabel(conversation.ToolCallInfo.Name)
		if !seen[action] {
			seen[action] = true
			actions = append(actions, action)
		}
		switch conversation.ToolCallInfo.Name {
		case "web_search":
			leading = "search"
		case "web_fetch":
			if leading != "search" {
				leading = "document"
			}
		case "load_tools":
			if leading == "tool" {
				leading = "extension"
			}
		}
	}
	count := a.translate("i18n:ui_ai_chat_tool_activity_count_one")
	if len(item.tools) != 1 {
		count = fmt.Sprintf(a.translate("i18n:ui_ai_chat_tool_activity_count_many"), strconv.Itoa(len(item.tools)))
	}
	separator := a.translate("i18n:ui_ai_chat_tool_activity_action_separator")
	summary := strings.Join([]string{a.chatToolActivityStatusLabel(status), strings.Join(actions, separator), count}, " · ")
	summaryWidth := float32(0)
	if metrics, err := a.window.MeasureText(summary, woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}); err == nil {
		summaryWidth = metrics.Size.Width
	}
	activityID := item.roundID
	props := previewview.ChatMessageProps{
		Key: activityID, Kind: "tool-activity", RoundExpanded: item.roundExpanded,
		ToolSummary: summary, ToolSummaryWidth: summaryWidth, ToolLeading: leading, ToolStatus: status, ToolStatusColor: statusColor,
		Theme: palette.componentTheme(), OnToggleRound: func() { a.toggleChatDisclosure(activityID) },
	}
	props.Tools = make([]previewview.ChatToolCallProps, 0, len(item.tools))
	for index, conversation := range item.tools {
		props.Tools = append(props.Tools, a.chatToolCallProps(activityID, index, conversation, expanded, palette, width-24))
	}
	return props
}

// chatToolCallProps measures one tool badge and its expandable details.
func (a *App) chatToolCallProps(activityID string, index int, conversation chatConversation, expanded map[string]bool, palette uiPalette, width float32) previewview.ChatToolCallProps {
	tool := conversation.ToolCallInfo
	status := tool.Status
	if status == "" {
		status = "pending"
	}
	start := tool.StartTimestamp
	if start <= 0 {
		start = conversation.Timestamp
	}
	end := tool.EndTimestamp
	if status == "streaming" || status == "pending" || status == "running" {
		end = time.Now().UnixMilli()
	} else if end <= 0 {
		end = start
	}
	duration := fmt.Sprintf("%dms", max(int64(0), end-start))
	durationWidth := float32(0)
	if metrics, err := a.window.MeasureText(duration, woxui.TextStyle{Size: 11}); err == nil {
		durationWidth = metrics.Size.Width
	}
	name := tool.Name
	if name == "" {
		name = a.translate("i18n:ui_ai_chat_tools")
	}
	nameWidth := float32(0)
	if metrics, err := a.window.MeasureText(name, woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}); err == nil {
		nameWidth = metrics.Size.Width
	}
	callID := conversation.ID
	if callID == "" {
		callID = fmt.Sprintf("%s:%d", activityID, index)
	}
	props := previewview.ChatToolCallProps{
		Key: callID, Name: name, NameWidth: nameWidth, Duration: duration, DurationWidth: durationWidth,
		Status: status, StatusColor: chatToolStatusColor(status, palette.componentTheme()), Expanded: expanded[callID],
		OnToggle: func() { a.toggleChatDisclosure(callID) },
	}
	params := tool.Delta
	if status != "streaming" {
		if encoded, err := json.Marshal(tool.Arguments); err == nil {
			params = string(encoded)
		}
	}
	details := [][2]string{
		{a.translate("i18n:ui_ai_chat_tool_detail_id"), tool.ID},
		{a.translate("i18n:ui_ai_chat_tool_detail_name"), tool.Name},
		{a.translate("i18n:ui_ai_chat_tool_detail_params"), params},
	}
	if tool.Response != "" {
		details = append(details, [2]string{a.translate("i18n:ui_ai_chat_tool_detail_response"), tool.Response})
	}
	detailTextWidth := max(float32(40), width-52)
	props.Details = make([]previewview.ChatToolDetailProps, 0, len(details))
	props.DetailsHeight = 16
	for detailIndex, detail := range details {
		layout := a.previewTextLayout(fmt.Sprintf("chat-tool-detail\x00%s\x00%d", callID, detailIndex), detail[1], woxui.TextStyle{Size: 11}, detailTextWidth, 16)
		props.Details = append(props.Details, previewview.ChatToolDetailProps{Label: detail[0], Value: detail[1], Layout: layout})
		props.DetailsHeight += layout.Size.Height + 40
	}
	return props
}

// chatToolActivityStatus applies Flutter's failed-first activity precedence.
func chatToolActivityStatus(tools []chatConversation) string {
	result := "succeeded"
	for _, conversation := range tools {
		switch conversation.ToolCallInfo.Status {
		case "failed":
			return "failed"
		case "running":
			result = "running"
		case "streaming":
			if result != "running" {
				result = "streaming"
			}
		case "pending", "":
			if result != "running" && result != "streaming" {
				result = "pending"
			}
		}
	}
	return result
}

// chatToolStatusColor maps protocol states to Flutter's status colors.
func chatToolStatusColor(status string, theme woxcomponent.Theme) woxui.Color {
	switch status {
	case "streaming", "running":
		return woxui.Color{R: 33, G: 150, B: 243, A: 255}
	case "succeeded":
		return woxui.Color{R: 76, G: 175, B: 80, A: 255}
	case "failed":
		return theme.ErrorText
	default:
		return theme.ResultSubtitle
	}
}

// chatToolActionLabel localizes the known tool activity verbs.
func (a *App) chatToolActionLabel(name string) string {
	key := ""
	switch name {
	case "web_search":
		key = "ui_ai_chat_tool_action_web_search"
	case "web_fetch":
		key = "ui_ai_chat_tool_action_web_fetch"
	case "read_skill":
		key = "ui_ai_chat_tool_action_read_skill"
	case "load_tools":
		key = "ui_ai_chat_tool_action_load_tools"
	}
	if key == "" {
		if name != "" {
			return name
		}
		return a.translate("i18n:ui_ai_chat_tools")
	}
	return a.translate("i18n:" + key)
}

func (a *App) chatToolActivityStatusLabel(status string) string {
	return a.translate("i18n:ui_ai_chat_tool_activity_status_" + status)
}

// chatMessageProps resolves text layouts, images, and controller actions for one conversation.
func (a *App) chatMessageProps(key string, index int, conversation chatConversation, palette uiPalette, width float32, showMeta, hideReasoning bool, imageScale float32) previewview.ChatMessageProps {
	cardWidth := width
	innerWidth := max(float32(40), cardWidth-4)
	if conversation.Role == "user" {
		cardWidth = width * 0.82
		innerWidth = max(float32(40), cardWidth-24)
	}
	props := previewview.ChatMessageProps{
		Key: fmt.Sprintf("%s-%d", key, index), Role: conversation.Role, ShowMeta: showMeta, Theme: palette.componentTheme(),
		CopyLabel: a.translate("i18n:ui_ai_chat_copy_message"), EditLabel: a.translate("i18n:ui_ai_chat_edit_message"), RetryLabel: a.translate("i18n:ui_ai_chat_regenerate_response"),
	}
	if conversation.Timestamp > 0 {
		props.Timestamp = time.UnixMilli(conversation.Timestamp).Local().Format("15:04")
		if metrics, err := a.window.MeasureText(props.Timestamp, woxui.TextStyle{Size: 11}); err == nil {
			props.TimestampWidth = metrics.Size.Width
		}
	}
	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		props.ToolText = formatChatToolCall(conversation)
		props.ToolLayout = a.previewTextLayout(fmt.Sprintf("chat-tool\x00%s\x00%d", key, index), props.ToolText, woxui.TextStyle{Size: 11}, innerWidth, 17)
	} else {
		if reasoning := strings.TrimSpace(conversation.Reasoning); reasoning != "" && !hideReasoning {
			props.Reasoning = reasoning
			props.ReasoningLayout = a.previewTextLayout(fmt.Sprintf("chat-reasoning\x00%s\x00%d", key, index), props.Reasoning, woxui.TextStyle{Size: 11}, innerWidth, 15.4)
		}
		props.Text = strings.TrimSpace(conversation.Text)
		if props.Text != "" {
			if conversation.Role == "assistant" {
				markdown := a.markdownProps(fmt.Sprintf("chat-markdown-%s-%d", key, index), props.Text, "", palette, innerWidth, imageScale)
				markdown.FontSize = 13
				props.Markdown = &markdown
				props.TextLayout.Size = woxwidget.MeasureStateless(a.window, woxcomponent.WoxMarkdown(markdown), innerWidth)
			} else {
				props.TextLayout = a.previewTextLayout(fmt.Sprintf("chat-text\x00%s\x00%d", key, index), props.Text, woxui.TextStyle{Size: 13}, innerWidth, 19)
			}
		}
	}
	if len(conversation.SkillRefs) > 0 {
		names := make([]string, 0, len(conversation.SkillRefs))
		for _, skill := range conversation.SkillRefs {
			if skill.Name != "" {
				names = append(names, "#"+skill.Name)
			}
		}
		props.Skills = strings.Join(names, "  ")
	}
	if len(conversation.Images) > 0 {
		props.Images = make([]*woxui.Image, 0, min(3, len(conversation.Images)))
		for _, source := range conversation.Images[:min(3, len(conversation.Images))] {
			props.Images = append(props.Images, a.imageFor(source))
		}
	}
	if conversation.Role == "user" {
		for _, line := range props.TextLayout.Lines {
			if metrics, err := a.window.MeasureText(line, woxui.TextStyle{Size: 13}); err == nil {
				props.ContentWidth = max(props.ContentWidth, metrics.Size.Width)
			}
		}
		if props.Skills != "" {
			if metrics, err := a.window.MeasureText(props.Skills, woxui.TextStyle{Size: 10}); err == nil {
				props.ContentWidth = max(props.ContentWidth, metrics.Size.Width)
			}
		}
		if len(props.Images) > 0 {
			props.ContentWidth = max(props.ContentWidth, float32(len(props.Images))*82+float32(len(props.Images)-1)*8)
		}
		props.ContentWidth = min(innerWidth, props.ContentWidth)
	}
	if copyText := chatConversationClipboardText(conversation); copyText != "" {
		props.OnCopy = func() { a.copyChatText(copyText) }
	}
	if conversation.ID != "" {
		conversationID := conversation.ID
		switch conversation.Role {
		case "user":
			props.OnEdit = func() { a.editChatConversation(conversationID) }
		case "assistant":
			props.OnRetry = func() { a.regenerateChatConversation(conversationID) }
		}
	}
	return props
}

// chatConversationClipboardText keeps platform clipboard behavior independent from chat rendering.
func chatConversationClipboardText(conversation chatConversation) string {
	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		return strings.TrimSpace(formatChatToolCall(conversation))
	}
	return strings.TrimSpace(conversation.Text)
}

// formatChatToolCall keeps tool name, state, arguments, and response visible in the first vertical slice.
func formatChatToolCall(conversation chatConversation) string {
	tool := conversation.ToolCallInfo
	name := tool.Name
	if name == "" {
		name = "Tool"
	}
	status := tool.Status
	if status == "" {
		status = "pending"
	}
	lines := []string{fmt.Sprintf("%s · %s", name, status)}
	if len(tool.Arguments) > 0 {
		if raw, err := json.Marshal(tool.Arguments); err == nil {
			lines = append(lines, string(raw))
		}
	}
	response := strings.TrimSpace(tool.Response)
	if response == "" {
		response = strings.TrimSpace(conversation.Text)
	}
	if response == "" {
		response = strings.TrimSpace(tool.Delta)
	}
	if response != "" {
		lines = append(lines, response)
	}
	return strings.Join(lines, "\n")
}

// chatInputProps prepares the controlled editor and toolbar actions.
func (a *App) chatInputProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatInputProps {
	hint := a.translate("i18n:ui_ai_chat_input_hint")
	if strings.TrimSpace(hint) == "" || hint == "i18n:ui_ai_chat_input_hint" {
		hint = "Type a message. Use / to switch models or insert skills"
	}
	model := strings.TrimSpace(snapshot.chat.Model.Name)
	if model == "" {
		model = a.translate("i18n:ui_ai_chat_select_model")
	}
	if strings.TrimSpace(model) == "" || model == "i18n:ui_ai_chat_select_model" {
		model = "Select model"
	}
	modelMetrics, _ := a.window.MeasureText(model, woxui.TextStyle{Size: 11})
	modelWidth := min(float32(267), modelMetrics.Size.Width+47)
	streaming := snapshot.chat.IsStreaming || snapshot.sending
	action := a.sendChatMessage
	actionLabel := a.translate("i18n:ui_ai_chat_send")
	if streaming {
		action = a.stopChatMessage
		actionLabel = a.translate("i18n:ui_ai_chat_stop")
	}
	status := ""
	statusColor := palette.resultSubtitle
	if snapshot.error != "" {
		status = snapshot.error
		statusColor = palette.componentTheme().ErrorText
	} else if snapshot.loading {
		status = "Loading…"
	} else if streaming {
		status = "Streaming…"
		statusColor = woxui.Color{R: 68, G: 196, B: 120, A: 255}
	}
	return previewview.ChatInputProps{
		Width: width, Height: height, Key: snapshot.key, Editing: snapshot.editing,
		Focused: snapshot.active && snapshot.question == nil, Hint: hint, Window: a.window,
		Model: model, ModelWidth: modelWidth, Status: status, StatusColor: statusColor, ActionLabel: actionLabel, Sending: streaming, Theme: palette.componentTheme(),
		OnFocus: a.focusChatInput, OnChanged: a.setChatText, OnKey: a.onChatPreviewKey,
		OnModels: func() { a.toggleChatPanel("models") }, OnSend: action,
	}
}

// chatQuestionPanelHeight bounds the tool question without starving the conversation viewport.
func chatQuestionPanelHeight(snapshot *chatPreviewSnapshot, available float32) float32 {
	if snapshot == nil || snapshot.question == nil {
		return 0
	}
	height := float32(152)
	if len(snapshot.question.Options) > 0 {
		height = 92 + float32(len(snapshot.question.Options))*46
		if snapshot.questionSelected == len(snapshot.question.Options)-1 {
			height += 56
		}
	}
	return min(max(float32(140), height), max(float32(140), available*0.48))
}

// chatQuestionProps prepares ask-user options and keeps selection and submission in the controller.
func (a *App) chatQuestionProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatQuestionProps {
	question := snapshot.question
	props := previewview.ChatQuestionProps{
		Width: width, Height: height, Question: question.Question, Theme: palette.componentTheme(),
		OnCancel: func() { a.submitAIQuestionAnswer("User cancelled") }, OnSubmit: a.submitSelectedAIQuestionAnswer,
	}
	props.Options = make([]previewview.ChatQuestionOptionProps, 0, len(question.Options))
	for index, option := range question.Options {
		label := option.Title
		if option.SubTitle != "" {
			label += " — " + option.SubTitle
		}
		if option.Recommended {
			label += "  · Recommended"
		}
		props.Options = append(props.Options, previewview.ChatQuestionOptionProps{
			ID: fmt.Sprintf("chat-question-%s-%d", question.QuestionID, index), Label: label,
			Selected: index == snapshot.questionSelected, OnSelect: func() { a.selectAIQuestionOption(index) },
		})
	}
	inputHeight := float32(0)
	if len(question.Options) == 0 {
		inputHeight = max(float32(42), height-92)
	} else if snapshot.questionSelected == len(question.Options)-1 {
		inputHeight = 48
	}
	if inputHeight > 0 {
		props.Input = &previewview.ChatQuestionInputProps{
			ID: "chat-question-input-" + question.QuestionID, Height: inputHeight, Editing: snapshot.questionEditing,
			Focused: snapshot.active, Window: a.window, OnFocus: a.focusAIQuestionInput, OnChanged: a.setAIQuestionText, OnKey: a.onChatPreviewKey,
		}
	}
	return props
}
