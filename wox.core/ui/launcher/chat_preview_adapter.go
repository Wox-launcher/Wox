package launcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildChatPreview prepares chat view props while retaining lifecycle and actions in the controller.
func (a *App) buildChatPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
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
		catalogWidth := min(float32(260), max(float32(220), innerWidth*0.46))
		props := a.chatCatalogProps(snapshot, palette, catalogWidth, innerHeight)
		catalog = &props
	} else if snapshot.panel == "models" || snapshot.panel == "skills" {
		catalogWidth := min(float32(440), innerWidth)
		catalogHeight := chatCatalogPanelHeight(snapshot, innerHeight-questionHeight)
		props := a.chatCatalogProps(snapshot, palette, catalogWidth, catalogHeight)
		catalog = &props
	}
	panel := snapshot.panel
	return previewview.ChatPreview(previewview.ChatPreviewProps{
		Width: width, Height: height, Key: snapshot.key, Panel: panel,
		Header:   a.chatHeaderProps(snapshot, palette, innerWidth, headerHeight),
		Messages: a.chatMessagesProps(snapshot, palette, innerWidth, messagesHeight),
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
	return previewview.ChatHeaderProps{
		Width: width, Height: height, Key: snapshot.key, Title: title, HistoryOpen: snapshot.panel == "history",
		ShowDebug: hasDebug, DebugOpen: snapshot.panel == "debug", Theme: palette.componentTheme(),
		OnHistory: func() { a.toggleChatPanel("history") }, OnDebug: func() { a.toggleChatPanel("debug") },
	}
}

// chatCatalogPanelHeight gives catalogs room without permanently shrinking the message pane.
func chatCatalogPanelHeight(snapshot *chatPreviewSnapshot, available float32) float32 {
	if snapshot == nil || snapshot.panel == "" || snapshot.question != nil {
		return 0
	}
	return min(float32(270), max(float32(150), available*0.44))
}

// chatCatalogProps prepares history, model, and skill rows without constructing widgets.
func (a *App) chatCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	const rowHeight = float32(44)
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

	items := make([]previewview.ChatCatalogItemProps, 0, count)
	if snapshot.panel == "history" {
		for index, chat := range snapshot.chats {
			index := index
			chatID := chat.ID
			title := strings.TrimSpace(chat.Title)
			if title == "" {
				title = "Untitled chat"
			}
			subtitle := "Saved conversation"
			if chat.UpdatedAt > 0 {
				subtitle = time.UnixMilli(chat.UpdatedAt).Local().Format("2006-01-02 15:04")
			}
			if chatID == snapshot.chat.ID {
				subtitle += " · Active"
			}
			items = append(items, previewview.ChatCatalogItemProps{
				SelectID: fmt.Sprintf("chat-history-row-%s-%d", snapshot.key, index), DeleteID: fmt.Sprintf("chat-history-delete-%s-%d", snapshot.key, index),
				Title: title, Subtitle: subtitle, Selected: index == snapshot.panelSelected,
				OnSelect: func() { a.selectChatHistory(chatID) }, OnDelete: func() { a.deleteChatHistory(chatID) },
			})
		}
	} else if snapshot.panel == "models" {
		for index, model := range snapshot.models {
			index := index
			provider := model.Provider
			if model.ProviderAlias != "" {
				provider += " (" + model.ProviderAlias + ")"
			}
			if model == snapshot.chat.Model {
				provider += " · Selected"
			}
			items = append(items, previewview.ChatCatalogItemProps{
				SelectID: fmt.Sprintf("chat-model-row-%s-%d", snapshot.key, index), Title: model.Name, Subtitle: provider,
				Selected: index == snapshot.panelSelected, OnSelect: func() { a.selectChatModel(index) },
			})
		}
	} else {
		for index, skill := range snapshot.skills {
			index := index
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
			items = append(items, previewview.ChatCatalogItemProps{
				SelectID: fmt.Sprintf("chat-skill-row-%s-%d", snapshot.key, index), Title: skill.Name, Subtitle: subtitle,
				Selected: index == snapshot.panelSelected, OnSelect: func() { a.insertChatSkill(index) },
			})
		}
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
	}
	return previewview.ChatCatalogProps{
		Width: width, Height: height, Key: snapshot.key, Label: label, Items: items, EmptyMessage: emptyMessage,
		Scroll: offset, ContentHeight: contentHeight, ShowNew: snapshot.panel == "history", Theme: palette.componentTheme(),
		OnScroll: a.scrollChatPanel, OnNew: a.startNewChat,
	}
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

// chatMessagesProps prepares semantic messages and leaves their widget composition to the view.
func (a *App) chatMessagesProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatMessagesProps {
	// ponytail: Add viewport virtualization after profiling a real long chat; the current full list preserves exact scroll height with less state.
	innerWidth := max(float32(0), width-20)
	innerHeight := max(float32(0), height-16)
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
	actionsEnabled := !snapshot.chat.IsStreaming && !snapshot.sending && snapshot.question == nil
	props.Messages = make([]previewview.ChatMessageProps, 0, len(snapshot.chat.Conversations))
	for index, conversation := range snapshot.chat.Conversations {
		props.Messages = append(props.Messages, a.chatMessageProps(snapshot.key, index, conversation, palette, innerWidth, actionsEnabled))
	}
	props.ContentHeight = previewview.ChatMessagesContentHeight(props.Messages, innerHeight)
	maxOffset := max(float32(0), props.ContentHeight-innerHeight)
	a.clampChatPreviewScroll(maxOffset)
	return props
}

// chatMessageProps resolves text layouts, images, and controller actions for one conversation.
func (a *App) chatMessageProps(key string, index int, conversation chatConversation, palette uiPalette, width float32, actionsEnabled bool) previewview.ChatMessageProps {
	cardWidth := width
	if conversation.Role == "user" {
		cardWidth = max(float32(180), width*0.82)
	}
	innerWidth := max(float32(40), cardWidth-24)
	props := previewview.ChatMessageProps{
		Key: fmt.Sprintf("%s-%d", key, index), Role: conversation.Role, Theme: palette.componentTheme(),
	}
	if conversation.Timestamp > 0 {
		props.Timestamp = time.UnixMilli(conversation.Timestamp).Local().Format("15:04")
	}
	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		props.ToolText = formatChatToolCall(conversation)
		props.ToolLayout = a.previewTextLayout(fmt.Sprintf("chat-tool\x00%s\x00%d", key, index), props.ToolText, woxui.TextStyle{Size: 11}, innerWidth, 17)
	} else {
		if reasoning := strings.TrimSpace(conversation.Reasoning); reasoning != "" {
			props.Reasoning = "Reasoning\n" + reasoning
			reasoningWidth := max(float32(20), innerWidth-16)
			props.ReasoningLayout = a.previewTextLayout(fmt.Sprintf("chat-reasoning\x00%s\x00%d", key, index), props.Reasoning, woxui.TextStyle{Size: 11}, reasoningWidth, 17)
		}
		props.Text = strings.TrimSpace(conversation.Text)
		if props.Text == "" && conversation.Role == "assistant" {
			props.Text = "Thinking…"
		}
		if props.Text != "" {
			props.TextLayout = a.previewTextLayout(fmt.Sprintf("chat-text\x00%s\x00%d", key, index), props.Text, woxui.TextStyle{Size: 13}, innerWidth, 19)
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
	if copyText := chatConversationClipboardText(conversation); copyText != "" {
		props.OnCopy = func() { a.copyChatText(copyText) }
	}
	if actionsEnabled && conversation.ID != "" {
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
	modelWidth := min(float32(230), max(float32(110), modelMetrics.Size.Width+34))
	streaming := snapshot.chat.IsStreaming || snapshot.sending
	action := a.sendChatMessage
	if streaming {
		action = a.stopChatMessage
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
		Focused: snapshot.active && snapshot.question == nil && snapshot.panel == "", Hint: hint, Window: a.window,
		Model: model, ModelWidth: modelWidth, Status: status, StatusColor: statusColor, Sending: streaming, Theme: palette.componentTheme(),
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
		index := index
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
