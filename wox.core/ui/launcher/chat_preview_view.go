package launcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildChatPreview renders chat messages, input, streaming state, and ask_user through the shared display list.
func (a *App) buildChatPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	snapshot, err := a.ensureChatPreview(result, preview)
	if err != nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(18), Child: woxwidget.TextBlock{
			Value: fmt.Sprintf("Invalid chat preview: %v", err), Width: max(float32(0), width-36), Height: max(float32(0), height-36), Style: woxui.TextStyle{Size: 13}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
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
	children := []woxwidget.Widget{
		a.buildChatHeader(snapshot, palette, innerWidth, headerHeight),
		a.buildChatMessages(snapshot, palette, innerWidth, messagesHeight),
	}
	if debugHeight > 0 {
		children = append(children, a.buildChatDebugPanel(snapshot, palette, innerWidth, debugHeight))
	}
	if questionHeight > 0 {
		children = append(children, a.buildAIQuestionPanel(snapshot, palette, innerWidth, questionHeight))
	}
	children = append(children, a.buildChatInput(snapshot, palette, innerWidth, inputHeight))
	mainPane := woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
	layers := []woxwidget.StackChild{{Left: 10, Top: 6, Child: mainPane}}
	if snapshot.panel == "history" {
		drawerWidth := min(float32(260), max(float32(220), innerWidth*0.46))
		layers = append(layers, woxwidget.StackChild{Child: woxwidget.Gesture{ID: "chat-panel-dismiss-" + snapshot.key, OnTap: func() { a.toggleChatPanel("history") }, Child: woxwidget.Container{Width: width, Height: height}}})
		layers = append(layers, woxwidget.StackChild{Left: 10, Top: 6, Child: a.buildChatCatalogPanel(snapshot, palette, drawerWidth, innerHeight)})
	} else if snapshot.panel == "models" || snapshot.panel == "skills" {
		panelWidth := min(float32(440), innerWidth)
		panelHeight := chatCatalogPanelHeight(snapshot, innerHeight-questionHeight)
		panelTop := max(headerHeight, innerHeight-inputHeight-panelHeight-6)
		panel := snapshot.panel
		layers = append(layers, woxwidget.StackChild{Child: woxwidget.Gesture{ID: "chat-panel-dismiss-" + snapshot.key, OnTap: func() { a.toggleChatPanel(panel) }, Child: woxwidget.Container{Width: width, Height: height}}})
		layers = append(layers, woxwidget.StackChild{Left: 10 + (innerWidth-panelWidth)/2, Top: 6 + panelTop, Child: a.buildChatCatalogPanel(snapshot, palette, panelWidth, panelHeight)})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: layers}
}

// buildChatHeader mirrors Flutter's compact title bar and keeps catalogs out of the main reading flow.
func (a *App) buildChatHeader(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	title := strings.TrimSpace(snapshot.chat.Title)
	if title == "" {
		title = a.translate("i18n:ui_ai_chat_new_chat")
	}
	if strings.TrimSpace(title) == "" || title == "i18n:ui_ai_chat_new_chat" {
		title = "New chat"
	}
	menuBackground := woxui.Color{}
	menuLabel := "☰"
	if snapshot.panel == "history" {
		menuBackground = palette.actionBackground
		menuLabel = "×"
	}
	hasDebug := len(bytes.TrimSpace(snapshot.chat.DebugTrace)) > 0 && !bytes.Equal(bytes.TrimSpace(snapshot.chat.DebugTrace), []byte("null"))
	debugWidth := float32(0)
	if hasDebug {
		debugWidth = 48
	}
	titleWidth := max(float32(60), width-48-debugWidth)
	headerChildren := []woxwidget.StackChild{
		{Left: 2, Top: 5, Child: woxwidget.Gesture{ID: "chat-history-" + snapshot.key, OnTap: func() { a.toggleChatPanel("history") }, Child: woxwidget.Container{
			Width: 36, Height: 36, Radius: 7, Color: menuBackground, Padding: woxwidget.Insets{Left: 9, Top: 7}, Child: woxwidget.Text{Value: menuLabel, Style: woxui.TextStyle{Size: 18}, Color: palette.resultSubtitle},
		}}},
		{Left: 44, Top: 14, Child: woxwidget.Container{Width: titleWidth, Height: 22, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}}},
	}
	if hasDebug {
		headerChildren = append(headerChildren, woxwidget.StackChild{Left: width - 48, Top: 6, Child: a.buildChatHeaderButton("chat-debug-"+snapshot.key, "Trace", 46, snapshot.panel == "debug", palette, func() { a.toggleChatPanel("debug") })})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Stack{Width: width, Height: height, Children: headerChildren}}
}

// buildChatHeaderButton keeps compact header actions consistent across catalogs.
func (a *App) buildChatHeaderButton(id, label string, width float32, selected bool, palette uiPalette, action func()) woxwidget.Widget {
	background := palette.actionBackground
	if selected {
		background = palette.selectedBackground
	}
	return woxwidget.Gesture{ID: id, OnTap: action, Child: woxwidget.Container{Width: width, Height: 34, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 9, Top: 10, Right: 7}, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: palette.previewText,
	}}}
}

// chatCatalogPanelHeight gives catalogs room without permanently shrinking the message pane.
func chatCatalogPanelHeight(snapshot *chatPreviewSnapshot, available float32) float32 {
	if snapshot == nil || snapshot.panel == "" || snapshot.question != nil {
		return 0
	}
	return min(float32(270), max(float32(150), available*0.44))
}

// buildChatCatalogPanel renders history and model selection from core DTOs without native controls.
func (a *App) buildChatCatalogPanel(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	const rowHeight = float32(44)
	innerWidth := max(float32(0), width-20)
	viewportHeight := max(float32(40), height-42)
	count := len(snapshot.chats)
	label := "Conversations"
	if snapshot.panel == "models" {
		count = len(snapshot.models)
		label = "Models"
	} else if snapshot.panel == "skills" {
		count = len(snapshot.skills)
		label = "Skills"
	}
	contentHeight := max(viewportHeight, float32(count)*rowHeight)
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	if count > 0 && snapshot.panelViewport <= 0 {
		selected := min(max(0, snapshot.panelSelected), count-1)
		rowTop := float32(selected) * rowHeight
		rowBottom := rowTop + rowHeight
		if rowTop < offset {
			offset = rowTop
		} else if rowBottom > offset+viewportHeight {
			offset = rowBottom - viewportHeight
		}
	}
	a.setChatPanelViewport(viewportHeight)
	rows := make([]woxwidget.Widget, 0, count)
	if snapshot.panel == "history" {
		for index, chat := range snapshot.chats {
			index := index
			chat := chat
			rows = append(rows, a.buildChatHistoryRow(snapshot, index, chat, palette, innerWidth, rowHeight))
		}
	} else if snapshot.panel == "models" {
		for index, model := range snapshot.models {
			index := index
			model := model
			rows = append(rows, a.buildChatModelRow(snapshot, index, model, palette, innerWidth, rowHeight))
		}
	} else {
		for index, skill := range snapshot.skills {
			index := index
			skill := skill
			rows = append(rows, a.buildChatSkillRow(snapshot, index, skill, palette, innerWidth, rowHeight))
		}
	}
	if len(rows) == 0 {
		message := "No saved conversations"
		if snapshot.panel == "models" {
			message = "No AI models configured"
			if snapshot.modelsLoading {
				message = "Loading models…"
			} else if snapshot.modelsError != "" {
				message = snapshot.modelsError
			}
		} else if snapshot.panel == "skills" {
			message = "No enabled skills"
			if snapshot.skillsLoading {
				message = "Loading skills…"
			} else if snapshot.skillsError != "" {
				message = snapshot.skillsError
			}
		}
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: viewportHeight, Padding: woxwidget.Insets{Left: 10, Top: 18}, Child: woxwidget.TextBlock{Value: message, Width: max(float32(0), innerWidth-20), Height: 48, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: palette.resultSubtitle}})
	}
	header := woxwidget.Widget(woxwidget.Container{Width: innerWidth, Height: 24, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.actionHeader}})
	if snapshot.panel == "history" {
		header = woxwidget.Stack{Width: innerWidth, Height: 24, Children: []woxwidget.StackChild{
			{Top: 5, Child: woxwidget.Container{Width: max(float32(0), innerWidth-54), Height: 18, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.actionHeader}}},
			{Left: innerWidth - 48, Child: a.buildChatHeaderButton("chat-new-"+snapshot.key, "New", 48, false, palette, a.startNewChat)},
		}}
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 9, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
		header,
		woxwidget.Gesture{ID: "chat-catalog-scroll-" + snapshot.key, OnScroll: func(delta woxui.Point) { a.scrollChatPanel(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: viewportHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}},
	}}}
}

// buildChatDebugPanel renders the development trace as portable, copyable JSON without sending it back to core.
func (a *App) buildChatDebugPanel(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
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
	headerWidth := max(float32(0), innerWidth-54)
	header := woxwidget.Stack{Width: innerWidth, Height: 24, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: headerWidth, Height: 24, Child: woxwidget.Text{Value: summary, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: palette.actionHeader}}},
		{Left: innerWidth - 48, Child: woxwidget.Gesture{ID: "chat-debug-copy-" + snapshot.key, OnTap: func() { a.copyChatText(value) }, Child: woxwidget.Container{
			Width: 48, Height: 22, Radius: 6, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 10, Top: 5}, Child: woxwidget.Text{Value: "Copy", Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: palette.previewText},
		}}},
	}}
	body := woxwidget.Gesture{ID: "chat-debug-scroll-" + snapshot.key, OnScroll: func(delta woxui.Point) { a.scrollChatDebugPanel(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: innerWidth, Height: viewportHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.Container{Width: innerWidth, Height: contentHeight, Radius: 7, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 8, Top: 8, Right: 8, Bottom: 8}, Child: woxwidget.TextBlock{
			Value: value, Width: textWidth, Height: layout.Size.Height, Style: woxui.TextStyle{Size: 10}, LineHeight: 16, Color: palette.previewText, Layout: &layout,
		}},
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 9, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 7}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{header, body}}}
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

// buildChatHistoryRow separates selection and deletion into distinct hit targets.
func (a *App) buildChatHistoryRow(snapshot *chatPreviewSnapshot, index int, chat chatData, palette uiPalette, width, height float32) woxwidget.Widget {
	background := woxui.Color{}
	if index == snapshot.panelSelected {
		background = palette.selectedBackground
	}
	title := strings.TrimSpace(chat.Title)
	if title == "" {
		title = "Untitled chat"
	}
	subtitle := "Saved conversation"
	if chat.UpdatedAt > 0 {
		subtitle = time.UnixMilli(chat.UpdatedAt).Local().Format("2006-01-02 15:04")
	}
	if chat.ID == snapshot.chat.ID {
		subtitle += " · Active"
	}
	mainWidth := max(float32(80), width-44)
	main := woxwidget.Gesture{ID: fmt.Sprintf("chat-history-row-%s-%d", snapshot.key, index), OnTap: func() { a.selectChatHistory(chat.ID) }, Child: woxwidget.Container{
		Width: mainWidth, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), mainWidth-18), Height: 16, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
			woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 9}, Color: palette.resultSubtitle},
		}},
	}}
	remove := woxwidget.Gesture{ID: fmt.Sprintf("chat-history-delete-%s-%d", snapshot.key, index), OnTap: func() { a.deleteChatHistory(chat.ID) }, Child: woxwidget.Container{
		Width: 40, Height: height - 4, Radius: 7, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 15, Top: 12}, Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 14}, Color: palette.resultSubtitle},
	}}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: []woxwidget.Widget{main, remove}}}
}

// buildChatModelRow presents provider identity alongside the model name.
func (a *App) buildChatModelRow(snapshot *chatPreviewSnapshot, index int, model aiModel, palette uiPalette, width, height float32) woxwidget.Widget {
	background := palette.queryBackground
	if index == snapshot.panelSelected {
		background = palette.selectedBackground
	}
	provider := model.Provider
	if model.ProviderAlias != "" {
		provider += " (" + model.ProviderAlias + ")"
	}
	if model == snapshot.chat.Model {
		provider += " · Selected"
	}
	row := woxwidget.Gesture{ID: fmt.Sprintf("chat-model-row-%s-%d", snapshot.key, index), OnTap: func() { a.selectChatModel(index) }, Child: woxwidget.Container{
		Width: width, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), width-20), Height: 16, Child: woxwidget.Text{Value: model.Name, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
			woxwidget.Text{Value: provider, Style: woxui.TextStyle{Size: 9}, Color: palette.resultSubtitle},
		}},
	}}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: row}
}

// buildChatSkillRow exposes the source and description before inserting an inline skill reference.
func (a *App) buildChatSkillRow(snapshot *chatPreviewSnapshot, index int, skill chatSkill, palette uiPalette, width, height float32) woxwidget.Widget {
	background := palette.queryBackground
	if index == snapshot.panelSelected {
		background = palette.selectedBackground
	}
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
	row := woxwidget.Gesture{ID: fmt.Sprintf("chat-skill-row-%s-%d", snapshot.key, index), OnTap: func() { a.insertChatSkill(index) }, Child: woxwidget.Container{
		Width: width, Height: height - 4, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), width-20), Height: 16, Child: woxwidget.Text{Value: skill.Name, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
			woxwidget.Container{Width: max(float32(0), width-20), Height: 14, Child: woxwidget.Text{Value: subtitle, Style: woxui.TextStyle{Size: 9}, Color: palette.resultSubtitle}},
		}},
	}}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 4}, Child: row}
}

// buildChatMessages lays out semantic cards and derives the shared scroll extent.
func (a *App) buildChatMessages(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	// ponytail: Add viewport virtualization after profiling a real long chat; the current full list preserves exact scroll height with less state.
	innerWidth := max(float32(0), width-20)
	innerHeight := max(float32(0), height-16)
	if len(snapshot.chat.Conversations) == 0 {
		message := a.translate("i18n:ui_ai_chat_empty_prompt")
		if strings.TrimSpace(message) == "" || message == "i18n:ui_ai_chat_empty_prompt" {
			message = "What do you want to ask Wox today?"
		}
		if snapshot.loading {
			message = "Loading conversation…"
		}
		color := palette.resultTitle
		color.A = uint8(float32(color.A) * 0.59)
		style := woxui.TextStyle{Size: 28, Weight: woxui.FontWeightSemibold}
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Painter{
			Width: innerWidth, Height: innerHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				metrics, _ := a.window.MeasureText(message, style)
				textWidth := min(max(float32(0), bounds.Width-48), metrics.Size.Width)
				left := bounds.X + max(float32(24), (bounds.Width-textWidth)/2)
				top := bounds.Y + max(float32(0), (bounds.Height-metrics.Size.Height)/2)
				displayList.DrawText(message, woxui.Rect{X: left, Y: top, Width: textWidth, Height: metrics.Size.Height}, style, color)
			},
		}}
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.chat.Conversations))
	contentHeight := float32(0)
	actionsEnabled := !snapshot.chat.IsStreaming && !snapshot.sending && snapshot.question == nil
	for index, conversation := range snapshot.chat.Conversations {
		row, rowHeight := a.buildChatConversation(snapshot.key, index, conversation, palette, innerWidth, actionsEnabled)
		rows = append(rows, row)
		contentHeight += rowHeight
		if index > 0 {
			contentHeight += 10
		}
	}
	if len(rows) == 0 {
		message := "Ask Wox anything. Enter sends; Shift+Enter adds a line."
		if snapshot.loading {
			message = "Loading conversation…"
		}
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: innerHeight, Padding: woxwidget.Insets{Left: 14, Top: 28}, Child: woxwidget.TextBlock{
			Value: message, Width: max(float32(0), innerWidth-28), Height: 48, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: palette.resultSubtitle,
		}})
		contentHeight = innerHeight
	}
	contentHeight = max(innerHeight, contentHeight)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	offset := min(max(float32(0), snapshot.scroll), maxOffset)
	a.clampChatPreviewScroll(maxOffset)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Gesture{
		ID: "chat-message-scroll-" + snapshot.key,
		OnScroll: func(delta woxui.Point) {
			a.scrollChatPreview(-delta.Y, maxOffset)
		},
		Child: woxwidget.ScrollView{Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: rows}},
	}}
}

// buildChatConversation maps user, assistant, system, and tool roles to one portable card model.
func (a *App) buildChatConversation(key string, index int, conversation chatConversation, palette uiPalette, width float32, actionsEnabled bool) (woxwidget.Widget, float32) {
	cardWidth := width
	left := float32(0)
	background := palette.queryBackground
	textColor := palette.previewText
	role := strings.ToUpper(conversation.Role)
	if conversation.Role == "user" {
		cardWidth = max(float32(180), width*0.82)
		left = max(float32(0), width-cardWidth)
		background = palette.selectedBackground
		if background.A > 190 {
			background.A = 190
		}
		textColor = palette.selectedTitle
		role = "YOU"
	} else if conversation.Role == "assistant" {
		role = "WOX"
	} else if conversation.Role == "tool" {
		role = "TOOL"
		background = palette.actionBackground
	} else if conversation.Role == "system" {
		role = "SYSTEM"
		background = palette.actionBackground
	}
	if role == "" {
		role = "MESSAGE"
	}

	innerWidth := max(float32(40), cardWidth-24)
	children := []woxwidget.Widget{}
	bodyHeight := float32(0)
	meta := role
	if conversation.Timestamp > 0 {
		meta += "  " + time.UnixMilli(conversation.Timestamp).Local().Format("15:04")
	}
	copyText := chatConversationClipboardText(conversation)
	actionWidgets := make([]woxwidget.Widget, 0, 2)
	actionWidth := float32(0)
	appendAction := func(name, label string, width float32, action func()) {
		if len(actionWidgets) > 0 {
			actionWidth += 6
		}
		actionWidth += width
		actionWidgets = append(actionWidgets, woxwidget.Gesture{
			ID:    fmt.Sprintf("chat-%s-%s-%d", name, key, index),
			OnTap: action,
			Child: woxwidget.Container{Width: width, Height: 18, Radius: 6, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 7, Top: 3}, Child: woxwidget.Text{
				Value: label, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: palette.previewText,
			}},
		})
	}
	if copyText != "" {
		appendAction("copy", "Copy", 38, func() { a.copyChatText(copyText) })
	}
	if actionsEnabled && conversation.ID != "" {
		switch conversation.Role {
		case "user":
			appendAction("edit", "Edit", 34, func() { a.editChatConversation(conversation.ID) })
		case "assistant":
			appendAction("retry", "Retry", 40, func() { a.regenerateChatConversation(conversation.ID) })
		}
	}
	showRoleHeader := conversation.Role == "tool" || conversation.Role == "system" || conversation.ToolCallInfo.Name != ""
	if showRoleHeader {
		metaWidth := innerWidth
		if len(actionWidgets) > 0 {
			metaWidth = max(float32(0), innerWidth-actionWidth-8)
		}
		headerChildren := []woxwidget.StackChild{{Child: woxwidget.Container{Width: metaWidth, Height: 18, Child: woxwidget.Text{Value: meta, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: palette.resultSubtitle}}}}
		if len(actionWidgets) > 0 {
			headerChildren = append(headerChildren, woxwidget.StackChild{Left: innerWidth - actionWidth, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: actionWidgets}})
		}
		children = append(children, woxwidget.Stack{Width: innerWidth, Height: 18, Children: headerChildren})
		bodyHeight += 18
	}

	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		toolText := formatChatToolCall(conversation)
		layout := a.previewTextLayout(fmt.Sprintf("chat-tool\x00%s\x00%d", key, index), toolText, woxui.TextStyle{Size: 11}, innerWidth, 17)
		children = append(children, woxwidget.TextBlock{Value: toolText, Width: innerWidth, Height: layout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: textColor, Layout: &layout})
		bodyHeight += layout.Size.Height
	} else {
		if reasoning := strings.TrimSpace(conversation.Reasoning); reasoning != "" {
			value := "Reasoning\n" + reasoning
			reasoningWidth := max(float32(20), innerWidth-16)
			layout := a.previewTextLayout(fmt.Sprintf("chat-reasoning\x00%s\x00%d", key, index), value, woxui.TextStyle{Size: 11}, reasoningWidth, 17)
			children = append(children, woxwidget.Container{Width: innerWidth, Height: layout.Size.Height + 12, Radius: 7, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 8, Top: 6, Right: 8, Bottom: 6}, Child: woxwidget.TextBlock{
				Value: value, Width: reasoningWidth, Height: layout.Size.Height, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: palette.resultSubtitle, Layout: &layout,
			}})
			bodyHeight += layout.Size.Height + 12
		}
		value := strings.TrimSpace(conversation.Text)
		if value == "" && conversation.Role == "assistant" {
			value = "Thinking…"
		}
		if value != "" {
			layout := a.previewTextLayout(fmt.Sprintf("chat-text\x00%s\x00%d", key, index), value, woxui.TextStyle{Size: 13}, innerWidth, 19)
			children = append(children, woxwidget.TextBlock{Value: value, Width: innerWidth, Height: layout.Size.Height, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: textColor, Layout: &layout})
			bodyHeight += layout.Size.Height
		}
	}
	if len(conversation.SkillRefs) > 0 {
		names := make([]string, 0, len(conversation.SkillRefs))
		for _, skill := range conversation.SkillRefs {
			if skill.Name != "" {
				names = append(names, "#"+skill.Name)
			}
		}
		if len(names) > 0 {
			children = append(children, woxwidget.Container{Width: innerWidth, Height: 18, Child: woxwidget.Text{Value: strings.Join(names, "  "), Style: woxui.TextStyle{Size: 10}, Color: palette.resultSubtitle}})
			bodyHeight += 18
		}
	}
	if len(conversation.Images) > 0 {
		imageChildren := make([]woxwidget.Widget, 0, min(3, len(conversation.Images)))
		for _, source := range conversation.Images[:min(3, len(conversation.Images))] {
			var child woxwidget.Widget = woxwidget.Container{Width: 82, Height: 82, Radius: 8, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 13, Top: 31}, Child: woxwidget.Text{Value: "Image", Style: woxui.TextStyle{Size: 10}, Color: palette.resultSubtitle}}
			if image := a.imageFor(source); image != nil {
				child = woxwidget.Image{Source: image, Width: 82, Height: 82}
			}
			imageChildren = append(imageChildren, child)
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: imageChildren})
		bodyHeight += 82
	}
	if !showRoleHeader && (conversation.Timestamp > 0 || len(actionWidgets) > 0) {
		footerMeta := ""
		if conversation.Timestamp > 0 {
			footerMeta = time.UnixMilli(conversation.Timestamp).Local().Format("15:04")
		}
		metaWidth := innerWidth
		if len(actionWidgets) > 0 {
			metaWidth = max(float32(0), innerWidth-actionWidth-8)
		}
		footerChildren := []woxwidget.StackChild{{Top: 3, Child: woxwidget.Container{Width: metaWidth, Height: 15, Child: woxwidget.Text{Value: footerMeta, Style: woxui.TextStyle{Size: 9}, Color: palette.resultSubtitle}}}}
		if len(actionWidgets) > 0 {
			footerChildren = append(footerChildren, woxwidget.StackChild{Left: innerWidth - actionWidth, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: actionWidgets}})
		}
		children = append(children, woxwidget.Stack{Width: innerWidth, Height: 18, Children: footerChildren})
		bodyHeight += 18
	}

	gapHeight := float32(max(0, len(children)-1)) * 6
	cardHeight := bodyHeight + gapHeight + 20
	card := woxwidget.Container{Width: cardWidth, Height: cardHeight, Radius: 10, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12, Bottom: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children}}
	return woxwidget.Stack{Width: width, Height: cardHeight, Children: []woxwidget.StackChild{{Left: left, Child: card}}}, cardHeight
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

// buildChatInput mirrors Flutter's input card: multiline editor above a compact model and send toolbar.
func (a *App) buildChatInput(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	const toolbarHeight = float32(42)
	cardHeight := max(float32(78), height-14)
	editorHeight := max(float32(36), cardHeight-toolbarHeight-1)
	style := woxui.TextStyle{Size: 13}
	active := snapshot.active && snapshot.question == nil && snapshot.panel == ""
	input := woxwidget.Gesture{
		ID: "chat-input-" + snapshot.key,
		OnTapAt: func(position woxui.Point) {
			offset := formTextOffsetAt(snapshot.editing, a.window, style, 5, width-28, woxui.Point{X: max(float32(0), position.X-14), Y: max(float32(0), position.Y-8)})
			a.setChatCaret(offset)
		},
		Child: woxwidget.Container{Width: width, Height: editorHeight, Padding: woxwidget.Insets{Left: 14, Top: 8, Right: 14, Bottom: 7}, Child: woxwidget.Painter{
			Width: max(float32(0), width-28), Height: max(float32(0), editorHeight-15), Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				if snapshot.editing.Text == "" && snapshot.editing.Composition == "" {
					hint := a.translate("i18n:ui_ai_chat_input_hint")
					if strings.TrimSpace(hint) == "" || hint == "i18n:ui_ai_chat_input_hint" {
						hint = "Type a message. Use / to switch models or insert skills"
					}
					displayList.DrawText(hint, bounds, style, palette.resultSubtitle)
				}
				drawFormEditor(displayList, bounds, snapshot.editing, style, palette, active, 5, a.window)
			},
		}},
	}
	divider := palette.resultSubtitle
	divider.A = uint8(float32(divider.A) * 0.14)
	model := strings.TrimSpace(snapshot.chat.Model.Name)
	if model == "" {
		model = a.translate("i18n:ui_ai_chat_select_model")
	}
	if strings.TrimSpace(model) == "" || model == "i18n:ui_ai_chat_select_model" {
		model = "Select model"
	}
	modelMetrics, _ := a.window.MeasureText(model, woxui.TextStyle{Size: 11})
	modelWidth := min(float32(230), max(float32(110), modelMetrics.Size.Width+34))
	modelButton := woxwidget.Gesture{ID: "chat-models-" + snapshot.key, OnTap: func() { a.toggleChatPanel("models") }, Child: woxwidget.Container{
		Width: modelWidth, Height: 34, Radius: 5, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 9, Top: 9, Right: 8}, Child: woxwidget.Text{
			Value: model + "  ▾", Style: woxui.TextStyle{Size: 11}, Color: palette.previewText,
		},
	}}
	streaming := snapshot.chat.IsStreaming || snapshot.sending
	label := "Send"
	action := a.sendChatMessage
	buttonColor := palette.actionSelected
	foreground := palette.actionSelectedText
	if streaming {
		label = "Stop"
		action = a.stopChatMessage
		buttonColor = palette.actionBackground
		foreground = palette.previewText
	}
	sendButton := a.buildChatPanelButton("chat-send-"+snapshot.key, label, 82, buttonColor, foreground, action)
	status := ""
	statusColor := palette.resultSubtitle
	if snapshot.error != "" {
		status = snapshot.error
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	} else if snapshot.loading {
		status = "Loading…"
	} else if streaming {
		status = "Streaming…"
		statusColor = woxui.Color{R: 68, G: 196, B: 120, A: 255}
	}
	statusLeft := modelWidth + 18
	statusWidth := max(float32(0), width-statusLeft-100)
	toolbarChildren := []woxwidget.StackChild{
		{Left: 8, Top: 4, Child: modelButton},
		{Left: width - 90, Top: 6, Child: sendButton},
	}
	if status != "" && statusWidth > 30 {
		toolbarChildren = append(toolbarChildren, woxwidget.StackChild{Left: statusLeft, Top: 14, Child: woxwidget.Container{Width: statusWidth, Height: 16, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 9}, Color: statusColor}}})
	}
	card := woxwidget.Container{Width: width, Height: cardHeight, Radius: 9, Color: palette.queryBackground, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		input,
		woxwidget.Container{Width: width, Height: 1, Color: divider},
		woxwidget.Stack{Width: width, Height: toolbarHeight, Children: toolbarChildren},
	}}}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 6, Bottom: 8}, Child: card}
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

// buildAIQuestionPanel renders selectable and free-text ask_user requests in the chat surface.
func (a *App) buildAIQuestionPanel(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	question := snapshot.question
	if question == nil {
		return woxwidget.Painter{Width: width, Height: 0}
	}
	innerWidth := max(float32(0), width-24)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 34, Child: woxwidget.TextBlock{Value: question.Question, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, LineHeight: 17, Color: palette.previewText}},
	}
	if len(question.Options) > 0 {
		for index, option := range question.Options {
			index := index
			background := palette.queryBackground
			if index == snapshot.questionSelected {
				background = palette.selectedBackground
			}
			label := option.Title
			if option.SubTitle != "" {
				label += " — " + option.SubTitle
			}
			if option.Recommended {
				label += "  · Recommended"
			}
			children = append(children, woxwidget.Gesture{ID: fmt.Sprintf("chat-question-%s-%d", question.QuestionID, index), OnTap: func() { a.selectAIQuestionOption(index) }, Child: woxwidget.Container{
				Width: innerWidth, Height: 40, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 11, Right: 10}, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11}, Color: palette.previewText},
			}})
		}
		if snapshot.questionSelected == len(question.Options)-1 {
			children = append(children, a.buildAIQuestionInput(snapshot, palette, innerWidth, 48))
		}
	} else {
		inputHeight := max(float32(42), height-92)
		children = append(children, a.buildAIQuestionInput(snapshot, palette, innerWidth, inputHeight))
	}
	buttonWidth := float32(76)
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-buttonWidth*2-8), Height: 30},
		a.buildChatPanelButton("chat-question-cancel-"+question.QuestionID, "Cancel", buttonWidth, palette.queryBackground, palette.previewText, func() { a.submitAIQuestionAnswer("User cancelled") }),
		a.buildChatPanelButton("chat-question-submit-"+question.QuestionID, "Submit", buttonWidth, palette.actionSelected, palette.actionSelectedText, a.submitSelectedAIQuestionAnswer),
	}}
	children = append(children, buttons)
	return woxwidget.Container{Width: width, Height: height, Radius: 9, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 8}, Child: woxwidget.Clip{
		Width: innerWidth, Height: max(float32(0), height-16), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: children},
	}}
}

// buildAIQuestionInput reuses one IME-aware editor for free-form and "Other" answers.
func (a *App) buildAIQuestionInput(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	style := woxui.TextStyle{Size: 12}
	return woxwidget.Gesture{ID: "chat-question-input-" + snapshot.question.QuestionID, OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.questionEditing, a.window, style, 4, width-20, woxui.Point{X: max(float32(0), position.X-10), Y: max(float32(0), position.Y-8)})
		a.setAIQuestionCaret(offset)
	}, Child: woxwidget.Container{Width: width, Height: height, Radius: 7, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Painter{
		Width: width - 20, Height: height - 16, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if snapshot.questionEditing.Text == "" && snapshot.questionEditing.Composition == "" {
				displayList.DrawText("Type an answer…", bounds, style, palette.resultSubtitle)
			}
			drawFormEditor(displayList, bounds, snapshot.questionEditing, style, palette, snapshot.active, 4, a.window)
		},
	}}}
}

func (a *App) buildChatPanelButton(id, label string, width float32, background, foreground woxui.Color, action func()) woxwidget.Widget {
	return woxwidget.Gesture{ID: id, OnTap: action, Child: woxwidget.Container{Width: width, Height: 30, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 16, Top: 8}, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: foreground,
	}}}
}
