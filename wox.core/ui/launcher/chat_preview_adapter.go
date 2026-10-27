package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"wox/common"
	"wox/common/icons"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const chatHistorySidebarWidth = float32(240)

// buildChatPreview prepares chat view props while retaining lifecycle and actions in the controller.
func (a *App) buildChatPreview(result queryResult, preview queryPreview, palette uiPalette, width, height, imageScale float32, showHeader bool) woxwidget.Widget {
	snapshot, err := a.chat.chatPreviewSnapshotFor(result, preview)
	if err != nil {
		return previewview.PreviewError(fmt.Sprintf("Invalid chat preview: %v", err), width, height, palette.componentTheme())
	}
	return a.buildChatPreviewFromSnapshot(snapshot, palette, width, height, imageScale, showHeader, true)
}

// buildChatPreviewFromSnapshot mounts one conversation snapshot into the shared chat panes.
func (a *App) buildChatPreviewFromSnapshot(snapshot *chatPreviewSnapshot, palette uiPalette, width, height, imageScale float32, showHeader, showOpenWindow bool) woxwidget.Widget {
	// Each host mounts its own editor and IME target while sharing committed chat data.
	window, onKey := a.window, a.chat.onChatPreviewKey
	if !showOpenWindow {
		window, onKey = a.chatWindow.chatNativeWindow(), a.chat.onDedicatedChatKey
	}
	headerHeight := float32(0)
	if showHeader {
		headerHeight = 52
	}
	contentWidth := width
	catalogWidth := float32(0)
	historyOpen := chatHistoryVisible(snapshot.panel, snapshot.sidebarOpen)
	if historyOpen {
		catalogWidth = min(chatHistorySidebarWidth, width/2)
		contentWidth = max(float32(0), width-catalogWidth)
	}
	innerWidth := max(float32(0), contentWidth-20)
	innerHeight := max(float32(0), height-14)
	questionHeight := chatQuestionPanelHeight(snapshot, innerHeight, a.chatDensityScale())
	var question *previewview.ChatQuestionProps
	if questionHeight > 0 {
		props := a.chatQuestionProps(snapshot, palette, innerWidth, questionHeight, window, onKey)
		question = &props
	}
	var history *previewview.ChatCatalogProps
	if historyOpen {
		// The controller's scroll viewport must exclude the floating panel's outer margin.
		historyHeight := max(float32(0), height-previewview.ChatScaledSize(a.chatDensityScale(), previewview.ChatHistoryFloatingInset)-previewview.ChatHistoryFloatingBottomInset)
		props := a.chatHistoryCatalogProps(snapshot, palette, catalogWidth, historyHeight)
		props.Height = height
		history = &props
	}
	panel := snapshot.panel
	var header *previewview.ChatHeaderProps
	if showHeader {
		props := a.chatHeaderProps(snapshot, palette, innerWidth, headerHeight, true, showOpenWindow)
		header = &props
	}
	body := previewview.ChatConversation(previewview.ChatConversationProps{
		PrepareMessages: func(width, height float32) previewview.ChatMessagesProps {
			return a.chatMessagesProps(window, snapshot, palette, width, height, imageScale)
		},
		PrepareCatalog: func(width, available float32) *previewview.ChatCatalogProps {
			if panel != "models" && panel != "skills" && panel != chatCommandPanel && panel != chatMentionPanel && panel != chatProjectPanel {
				return nil
			}
			props := a.chatCatalogProps(snapshot, palette, width, available)
			return &props
		},
		ChatPreviewProps: previewview.ChatPreviewProps{
			Width: width, Height: height, Key: snapshot.key, Panel: panel, Dedicated: !showOpenWindow,
			Header:   header,
			Question: question,
			Input:    a.chatInputProps(snapshot, palette, 0, 0, window, onKey),
			History:  history, OnDismiss: func() { a.chat.toggleChatPanel(panel) },
		}})
	return a.chatProjectOverlay(snapshot, palette, width, height, window, body)
}

// chatHeaderProps resolves the current title and available controller actions.
func (a *App) chatHeaderProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32, showExit, showOpenWindow bool) previewview.ChatHeaderProps {
	title := strings.TrimSpace(snapshot.chat.Title)
	if title == "" {
		title = a.language.translate("i18n:ui_ai_chat_new_chat")
	}
	if strings.TrimSpace(title) == "" || title == "i18n:ui_ai_chat_new_chat" {
		title = "New chat"
	}
	hasDebug := len(bytes.TrimSpace(snapshot.chat.DebugTrace)) > 0 && !bytes.Equal(bytes.TrimSpace(snapshot.chat.DebugTrace), []byte("null"))
	exitLabel := a.language.translate("i18n:ui_close")
	if strings.TrimSpace(exitLabel) == "" || exitLabel == "i18n:ui_close" {
		exitLabel = "Close"
	}
	exitLabel += " (Esc)"
	openWindowLabel := a.language.translate("i18n:ui_ai_chat_open_window")
	if strings.TrimSpace(openWindowLabel) == "" || openWindowLabel == "i18n:ui_ai_chat_open_window" {
		openWindowLabel = "Open in dedicated window"
	}
	openWindowLabel += " (" + strings.Join(formatHotkeyLabels(chatOpenWindowHotkey()), "+") + ")"
	historyLabel := a.language.translate("i18n:ui_action_toggle_sidebar")
	// The sidebar toggle advertises the same Ctrl/Cmd+B shortcut Flutter binds to preview fullscreen.
	historyTooltip := historyLabel + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("b")), "+") + ")"
	return previewview.ChatHeaderProps{
		Width: width, Height: height, DensityScale: a.chatDensityScale(), Key: snapshot.key, Title: title,
		ShowDebug:        hasDebug,
		DebugCopiedLabel: a.language.translate("i18n:toolbar_copied"),
		ShowExit:         showExit && launcherChromeHidden(a.show, a.chat.chatFullscreen), ExitLabel: exitLabel,
		ShowOpenWindow: showOpenWindow, OpenWindowLabel: openWindowLabel,
		HistoryLabel: historyLabel, HistoryTooltip: historyTooltip, Theme: palette.componentTheme(),
		OnHistory: func() { a.chat.toggleChatPanel("history") }, OnHistoryHover: a.previews.setPreviewTooltip, OnDebug: func() bool {
			value := formatChatDebugTrace(snapshot.chat.DebugTrace)
			return a.chat.copyChatText(value)
		},
		OnExit: a.closePreviewWindow, OnOpenWindow: a.requestOpenDedicatedChatWindow, OnDrag: func() {
			if err := a.window.StartDragging(); err != nil {
				log.Printf("start chat preview window drag: %v", err)
			}
		}, OnExitHover: a.previews.setPreviewTooltip, OnOpenWindowHover: a.previews.setPreviewTooltip,
	}
}

// chatCatalogPanelHeight gives catalogs room without permanently shrinking the message pane.
func chatCatalogPanelHeight(snapshot *chatPreviewSnapshot, available float32) float32 {
	if snapshot == nil || snapshot.panel == "" || snapshot.question != nil {
		return 0
	}
	if snapshot.panel == chatProjectPanel {
		items := chatProjectPaletteItems(snapshot.projects.Projects, snapshot.chat.ProjectID, snapshot.panelQuery, "", snapshot.usePinYin)
		return previewview.ChatPickerHeight(chatCommandContentHeight(items), 1, available)
	}
	if snapshot.panel == chatMentionPanel {
		items := chatMentionPaletteItems(snapshot.mentions, snapshot.panelQuery, snapshot.usePinYin, snapshot.panelFilter)
		contentHeight := chatCommandContentHeight(items)
		if snapshot.panelFilter != chatMentionKindFiles {
			contentHeight += chatMentionStatusHeight(items, snapshot.pluginMentionsLoading, snapshot.pluginMentionsError)
		}
		return previewview.ChatPickerHeight(contentHeight, 1, available)
	}
	if snapshot.panel == "models" || snapshot.panel == "skills" || snapshot.panel == chatCommandPanel {
		items := chatSnapshotCommandItems(snapshot)
		filter := chatCommandFilter(snapshot.panel, snapshot.panelFilter)
		loadingHeight := chatCommandPaletteLoadingHeight(items, filter, snapshot.modelsLoading, snapshot.skillsLoading)
		return previewview.ChatPickerHeight(chatCommandContentHeight(items)+loadingHeight, 1, available)
	}
	return min(float32(270), max(float32(150), available*0.44))
}

func chatSnapshotCommandItems(snapshot *chatPreviewSnapshot) []chatCommandPaletteItem {
	filter := chatCommandFilter(snapshot.panel, snapshot.panelFilter)
	if snapshot.panel != chatCommandPanel {
		return chatCommandPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.panelQuery, filter, snapshot.usePinYin)
	}
	return chatSlashPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.projects, snapshot.chat.ProjectID, snapshot.projectNoneLabel, snapshot.panelQuery, filter, snapshot.usePinYin)
}

// chatCatalogProps prepares history, model, and skill rows without constructing widgets.
func (a *App) chatCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	if snapshot.panel == chatProjectPanel {
		return a.chatProjectPickerProps(snapshot, palette, width, height)
	}
	if snapshot.panel == "history" {
		return a.chatHistoryCatalogProps(snapshot, palette, width, height)
	}
	if snapshot.panel == chatMentionPanel {
		return a.chatMentionCatalogProps(snapshot, palette, width, height)
	}
	grouped := snapshot.panel == chatCommandPanel
	scale := a.chatDensityScale()
	filter := chatCommandFilter(snapshot.panel, snapshot.panelFilter)
	commands := chatSnapshotCommandItems(snapshot)
	loadingHeight := chatCommandPaletteLoadingHeight(commands, filter, snapshot.modelsLoading, snapshot.skillsLoading, scale)
	baseContentHeight := chatCommandContentHeight(commands, scale) + loadingHeight
	height = previewview.ChatPickerHeight(baseContentHeight, scale, height)
	viewportHeight := max(float32(0), height-previewview.ChatPickerChromeHeight(scale))
	count := len(commands)
	label := ""
	if snapshot.panel == "models" {
		label = a.language.translate("i18n:ui_ai_chat_select_model_title")
	} else if snapshot.panel == "skills" {
		label = a.language.translate("i18n:ui_ai_skills")
	} else if grouped {
		label = ""
	}
	showModelLoading := chatCommandPaletteShowsGroup(filter, "models") && chatCommandPaletteShowsLoading(commands, "models", snapshot.modelsLoading)
	showSkillLoading := chatCommandPaletteShowsGroup(filter, "skills") && chatCommandPaletteShowsLoading(commands, "skills", snapshot.skillsLoading)
	contentHeight := chatCommandContentHeight(commands, scale)
	contentHeight += loadingHeight
	contentHeight = max(viewportHeight, contentHeight)
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	if count > 0 && snapshot.panelViewport <= 0 {
		selected := min(max(0, snapshot.panelSelected), count-1)
		rowTop := chatCommandItemOffset(commands, selected, scale)
		if showModelLoading {
			rowTop += previewview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight) + previewview.ChatScaledSize(scale, chatCatalogRowHeight) + previewview.ChatScaledSize(scale, previewview.ChatPickerSectionGap)
		}
		rowBottom := rowTop + previewview.ChatScaledSize(scale, chatCommandItemHeight(commands[selected]))
		if rowTop < offset {
			offset = rowTop
		} else if rowBottom > offset+viewportHeight {
			offset = rowBottom - viewportHeight
		}
	}
	if state := a.chat.chatPreview; state != nil {
		state.panelDensityScale = scale
		state.panelLoadingHeight = loadingHeight
		state.panelLeadingHeight = 0
		if showModelLoading {
			state.panelLeadingHeight = previewview.ChatScaledSize(scale, chatCatalogGroupHeaderHeight) + previewview.ChatScaledSize(scale, chatCatalogRowHeight)
			if count > 0 {
				state.panelLeadingHeight += previewview.ChatScaledSize(scale, previewview.ChatPickerSectionGap)
			}
		}
	}
	a.chat.setChatPanelViewport(viewportHeight)

	items := make([]previewview.ChatCatalogItemProps, 0, count+2)
	if showModelLoading {
		items = append(items, a.chatCatalogLoadingItem(snapshot, "models", true))
	}
	for index, command := range commands {
		sectionLabel := a.language.translate("i18n:ui_ai_skills")
		groupLabel := sectionLabel
		var groupIcon *woxui.Image
		var itemIcon *woxui.Image
		if command.group == "models" {
			sectionLabel = a.language.translate("i18n:ui_ai_chat_models")
			groupLabel = command.subtitle
			if groupLabel == "" {
				groupLabel = a.language.translate("i18n:ui_ai_chat_select_model_title")
			}
			if a.aiSettings != nil {
				for _, provider := range a.aiSettings.ProviderCatalog() {
					if provider.Name == snapshot.models[command.sourceIndex].Provider {
						groupIcon = a.imageForSurface(provider.Icon, 64, palette.background)
						break
					}
				}
			}
		} else if command.group == chatProjectPanel {
			sectionLabel = a.language.translate("i18n:plugin_ai_chat_project_section")
			groupLabel = sectionLabel
			itemIcon = a.imageForSurface(fromCoreImage(icons.Get(icons.ControlFolderOpen)), 64, palette.background)
		}
		items = append(items, previewview.ChatCatalogItemProps{
			SelectID: fmt.Sprintf("chat-%s-row-%s-%d", command.group, snapshot.key, index), GroupLabel: groupLabel, GroupIcon: groupIcon,
			SectionLabel: sectionLabel,
			Picker:       true, RowHeight: chatCommandItemHeight(command), Kind: command.group, Title: command.title, Subtitle: command.subtitle, Selected: index == snapshot.panelSelected, Current: command.current, Icon: itemIcon,
			OnSelect: func() {
				if command.group == "models" {
					a.chat.selectChatModel(command.sourceIndex)
				} else if command.group == chatProjectPanel {
					id := ""
					if command.sourceIndex >= 0 {
						id = snapshot.projects.Projects[command.sourceIndex].Id
					}
					a.chat.chooseChatProject(id)
				} else {
					a.chat.insertChatSkill(command.sourceIndex)
				}
				a.focusChatCatalogComposer(snapshot.key)
			},
		})
	}
	if showSkillLoading {
		items = append(items, a.chatCatalogLoadingItem(snapshot, "skills", true))
	}
	emptyMessage := a.language.translate("i18n:ui_no_data")
	if filter == "models" {
		if snapshot.modelsLoading {
			emptyMessage = a.language.translate("i18n:ui_ai_chat_loading_models")
		} else if snapshot.modelsError != "" {
			emptyMessage = snapshot.modelsError
		}
	} else if filter == "skills" {
		if snapshot.skillsLoading {
			emptyMessage = a.language.translate("i18n:ui_ai_chat_loading_skills")
		} else if snapshot.skillsError != "" {
			emptyMessage = snapshot.skillsError
		}
	} else if grouped {
		emptyMessage = a.language.translate("i18n:ui_no_data")
		if snapshot.modelsLoading || snapshot.skillsLoading {
			emptyMessage = a.language.translate("i18n:ui_ai_chat_loading_models")
		} else if snapshot.modelsError != "" && snapshot.skillsError != "" {
			emptyMessage = snapshot.modelsError + "; " + snapshot.skillsError
		} else if snapshot.modelsError != "" {
			emptyMessage = snapshot.modelsError
		} else if snapshot.skillsError != "" {
			emptyMessage = snapshot.skillsError
		}
	}
	props := previewview.ChatCatalogProps{
		Width: width, Height: height, DensityScale: scale, Key: snapshot.key, Label: label, Items: items, EmptyMessage: emptyMessage,
		Picker: true, Scroll: offset, ContentHeight: contentHeight, Theme: palette.componentTheme(),
		OnScroll: a.chat.scrollChatPanel,
	}
	for i := range props.Items {
		if props.Items[i].Placeholder {
			props.Items[i].RowHeight = chatCatalogRowHeight
			props.Items[i].SectionLabel = props.Items[i].GroupLabel
			props.Items[i].GroupLabel = ""
			if props.Items[i].Kind == "models" {
				props.Items[i].SectionLabel = a.language.translate("i18n:ui_ai_chat_models")
			}
		}
	}
	if grouped {
		props.Filters = a.chatCatalogFilters(snapshot)
	} else {
		props.Search = &woxcomponent.TextFieldProps{
			ID: "chat-model-search-" + snapshot.key, Value: snapshot.panelQuery,
			Hint: a.language.translate("i18n:ui_ai_chat_search_models"), Theme: props.Theme.Controls,
			Window: a.chat.chatTextInputWindow(), Focused: true, OnChanged: a.chat.setChatCatalogQuery,
			OnKey: func(event woxui.KeyEvent) bool {
				return previewview.ChatCatalogKey(event, a.chat.moveChatPanelSelection, a.chat.activateChatPanelSelection, a.chat.closeChatPanel)
			},
		}
	}
	return props
}

// chatCatalogFilters gives both token pickers the same category callbacks and focus behavior.
func (a *App) chatCatalogFilters(snapshot *chatPreviewSnapshot) []previewview.ChatCatalogFilter {
	var filters []previewview.ChatCatalogFilter
	for _, category := range chatCatalogCategories(snapshot.panel) {
		filters = append(filters, previewview.ChatCatalogFilter{
			ID: "chat-filter-" + category.id + "-" + snapshot.key, Label: a.language.translate("i18n:" + category.key),
			Selected: snapshot.panelFilter == category.id, OnSelect: func() {
				a.chat.setChatCatalogFilter(category.id)
				a.focusChatCatalogComposer(snapshot.key)
			},
		})
	}
	return filters
}

// focusChatCatalogComposer returns typing to the presenting host after a pointer selection.
func (a *App) focusChatCatalogComposer(key string) {
	host := a.host
	if a.chat.chatTextInputWindow() != a.window {
		host = a.chatWindow.chatHost
	}
	if host != nil {
		host.RequestFocus(woxwidget.Key("chat-input-" + key))
	}
}

func (a *App) chatMentionCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	scale := a.chatDensityScale()
	commands := chatMentionPaletteItems(snapshot.mentions, snapshot.panelQuery, snapshot.usePinYin, snapshot.panelFilter)
	statusHeight := chatMentionStatusHeight(commands, snapshot.pluginMentionsLoading, snapshot.pluginMentionsError, scale)
	if snapshot.panelFilter == chatMentionKindFiles {
		statusHeight = 0
	}
	contentHeight := chatCommandContentHeight(commands, scale) + statusHeight
	height = previewview.ChatPickerHeight(contentHeight, scale, height)
	if state := a.chat.chatPreview; state != nil {
		state.panelLoadingHeight, state.panelLeadingHeight = statusHeight, 0
		state.panelDensityScale = scale
	}
	viewportHeight := max(float32(0), height-previewview.ChatPickerChromeHeight(scale))
	maxOffset := max(float32(0), contentHeight-viewportHeight)
	offset := min(max(float32(0), snapshot.panelScroll), maxOffset)
	if len(commands) > 0 && snapshot.panelViewport <= 0 {
		selected := min(max(0, snapshot.panelSelected), len(commands)-1)
		rowTop := chatCommandItemOffset(commands, selected, scale)
		rowBottom := rowTop + previewview.ChatScaledSize(scale, chatCatalogRowHeight)
		if rowTop < offset {
			offset = rowTop
		} else if rowBottom > offset+viewportHeight {
			offset = rowBottom - viewportHeight
		}
	}
	a.chat.setChatPanelViewport(viewportHeight)
	rows := make([]previewview.ChatCatalogItemProps, 0, len(commands)+1)
	for index, command := range commands {
		item := command
		mention := snapshot.mentions[item.sourceIndex]
		rows = append(rows, previewview.ChatCatalogItemProps{
			SelectID:   fmt.Sprintf("chat-mention-row-%s-%s-%d", item.group, snapshot.key, index),
			GroupLabel: a.chatMentionGroupLabel(item.group), Kind: item.group, Title: item.title, Selected: index == snapshot.panelSelected,
			SectionLabel: a.chatMentionGroupLabel(item.group), Picker: true, RowHeight: chatCatalogRowHeight,
			Icon:     a.imageForSurface(mention.Icon, 256, palette.background),
			OnSelect: func() { a.chat.insertChatMention(item) },
		})
	}
	if statusHeight > 0 {
		status := a.chatCatalogLoadingItem(snapshot, chatMentionKindPlugin, true)
		status.Picker, status.RowHeight, status.SectionLabel = true, chatCatalogRowHeight, a.chatMentionGroupLabel(chatMentionKindPlugin)
		if !snapshot.pluginMentionsLoading {
			status.SelectID = "chat-plugins-error-" + snapshot.key
			status.Title = snapshot.pluginMentionsError
		}
		rows = append(rows, status)
	}
	emptyMessage := a.language.translate("i18n:ui_no_data")
	return previewview.ChatCatalogProps{
		Width: width, Height: height, DensityScale: a.chatDensityScale(), Key: snapshot.key, Items: rows, EmptyMessage: emptyMessage,
		Picker: true, Filters: a.chatCatalogFilters(snapshot),
		Scroll: offset, ContentHeight: contentHeight, Theme: palette.componentTheme(),
		OnScroll: a.chat.scrollChatPanel,
	}
}

func (a *App) chatMentionGroupLabel(kind string) string {
	switch kind {
	case chatMentionKindPlugin:
		return a.language.translate("i18n:ui_ai_chat_mention_plugins")
	case chatMentionKindFiles:
		return a.language.translate("i18n:ui_ai_chat_mention_files")
	default:
		return kind
	}
}

// chatCatalogLoadingItem keeps a Models or Skills group visible while its catalog is still loading.
func (a *App) chatCatalogLoadingItem(snapshot *chatPreviewSnapshot, group string, grouped bool) previewview.ChatCatalogItemProps {
	title := a.language.translate("i18n:ui_ai_chat_loading_models")
	groupLabel := ""
	if group == "skills" {
		title = a.language.translate("i18n:ui_ai_chat_loading_skills")
		if grouped {
			groupLabel = a.language.translate("i18n:ui_ai_skills")
		}
	} else if group == chatMentionKindPlugin {
		title = a.language.translate("i18n:ui_ai_chat_loading_plugins")
		if grouped {
			groupLabel = a.language.translate("i18n:ui_ai_chat_mention_plugins")
		}
	} else if grouped {
		groupLabel = a.language.translate("i18n:ui_ai_chat_select_model_title")
	}
	return previewview.ChatCatalogItemProps{
		SelectID: fmt.Sprintf("chat-%s-loading-%s", group, snapshot.key), GroupLabel: groupLabel,
		Kind: group, Title: title, Placeholder: true,
	}
}

// chatHistoryCatalogProps groups visible conversations using Flutter's local-day boundaries.
func (a *App) chatHistoryCatalogProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32) previewview.ChatCatalogProps {
	return a.chatProjectHistoryProps(snapshot, palette, width, height)
}

// formatChatDebugTrace formats the protocol payload for copying without changing its fields.
func formatChatDebugTrace(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "No debug trace is available."
	}
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, trimmed, "", "  "); err != nil {
		return string(trimmed)
	}
	return formatted.String()
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
	var lastUserTimestamp int64
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
			// A typical reply is one assistant message, so first==last timestamp.
			// Measure from the user turn that started this round instead.
			start := lastUserTimestamp
			if start <= 0 {
				for _, message := range round {
					if message.Timestamp > 0 {
						start = message.Timestamp
						break
					}
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
			lastUserTimestamp = conversation.Timestamp
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
func (a *App) chatMessagesProps(window *woxui.Window, snapshot *chatPreviewSnapshot, palette uiPalette, width, height, imageScale float32) previewview.ChatMessagesProps {
	// LazyList needs every row's height, but unchanged historical Markdown must not
	// rebuild a widget tree just to supply that height on each streamed frame.
	if a.chat.chatMarkdown.chatID != snapshot.chat.ID {
		a.chat.chatMarkdown = chatMarkdownCache{chatID: snapshot.chat.ID}
	}
	innerWidth := max(float32(0), width-4)
	innerHeight := max(float32(0), height-14)
	emptyMessage := a.language.translate("i18n:ui_ai_chat_empty_prompt")
	if strings.TrimSpace(emptyMessage) == "" || emptyMessage == "i18n:ui_ai_chat_empty_prompt" {
		emptyMessage = "What do you want to ask Wox today?"
	}
	if snapshot.loading {
		emptyMessage = "Loading conversation…"
	}
	scale := a.chatDensityScale()
	emptySize := previewview.ChatScaledSize(scale, 22)
	emptyMetrics, _ := window.MeasureText(emptyMessage, woxui.TextStyle{Size: emptySize, Weight: woxui.FontWeightSemibold})
	props := previewview.ChatMessagesProps{
		Width: width, Height: height, DensityScale: scale, Key: snapshot.key, EmptyMessage: emptyMessage,
		EmptyTextStyle: woxui.TextStyle{Size: emptySize, Weight: woxui.FontWeightSemibold}, EmptyLineHeight: previewview.ChatScaledSize(scale, 36),
		EmptyTextWidth: emptyMetrics.Size.Width, EmptyTextHeight: emptyMetrics.Size.Height,
		ContentHeight: innerHeight, Scroll: snapshot.scroll, Theme: palette.componentTheme(), OnScroll: a.chat.scrollChatPreview,
	}
	if len(snapshot.chat.Conversations) == 0 {
		return props
	}
	renderItems := chatRenderItems(snapshot.chat.Conversations, snapshot.chat.IsStreaming || snapshot.sending, snapshot.expandedRounds)
	props.Messages = make([]previewview.ChatMessageProps, 0, len(renderItems))
	for index, item := range renderItems {
		if item.kind == "round" {
			roundID := item.roundID
			props.Messages = append(props.Messages, a.chatRoundProps(item, palette.componentTheme(), func() { a.chat.toggleChatDisclosure(roundID) }))
			continue
		}
		if item.kind == "tool-activity" {
			props.Messages = append(props.Messages, a.chatToolActivityProps(window, item, snapshot.expandedRounds, palette, innerWidth))
			continue
		}
		props.Messages = append(props.Messages, a.chatMessageProps(window, snapshot.key, index, item.conversation, palette, width, item.showMeta, item.hideReasoning, imageScale))
	}
	props.ContentHeight, props.ExtentRevision = previewview.ChatMessagesScrollMetrics(props.Messages, innerHeight)
	maxOffset := max(float32(0), props.ContentHeight-innerHeight)
	a.chat.clampChatPreviewScroll(maxOffset)
	return props
}

// chatToolActivityProps prepares Flutter's grouped tool summary and nested detail rows.
func (a *App) chatToolActivityProps(window *woxui.Window, item chatRenderItem, expanded map[string]bool, palette uiPalette, width float32) previewview.ChatMessageProps {
	status := chatToolActivityStatus(item.tools)
	statusColor := chatToolStatusColor(status, palette.componentTheme())
	actions := make([]string, 0, len(item.tools))
	seen := make(map[string]bool)
	leading := "tool"
	for _, conversation := range item.tools {
		action := a.chatToolActionLabel(conversation.ToolCallInfo)
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
	count := a.language.translate("i18n:ui_ai_chat_tool_activity_count_one")
	if len(item.tools) != 1 {
		count = fmt.Sprintf(a.language.translate("i18n:ui_ai_chat_tool_activity_count_many"), strconv.Itoa(len(item.tools)))
	}
	separator := a.language.translate("i18n:ui_ai_chat_tool_activity_action_separator")
	summary := strings.Join([]string{a.chatToolActivityStatusLabel(status), strings.Join(actions, separator), count}, " · ")
	scale := a.chatDensityScale()
	summaryWidth := float32(0)
	if metrics, err := window.MeasureText(summary, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11), Weight: woxui.FontWeightSemibold}); err == nil {
		summaryWidth = metrics.Size.Width
	}
	activityID := item.roundID
	props := previewview.ChatMessageProps{
		Key: activityID, DensityScale: scale, Kind: "tool-activity", RoundExpanded: item.roundExpanded,
		ToolSummary: summary, ToolSummaryWidth: summaryWidth, ToolLeading: leading, ToolStatus: status, ToolStatusColor: statusColor,
		Theme: palette.componentTheme(), OnToggleRound: func() { a.chat.toggleChatDisclosure(activityID) },
	}
	// Collapsed groups only display the summary. Preparing hidden responses can
	// overflow the text-layout cache and remeasure every tool on each scroll frame.
	if !props.RoundExpanded {
		return props
	}
	props.Tools = make([]previewview.ChatToolCallProps, 0, len(item.tools))
	for index, conversation := range item.tools {
		props.Tools = append(props.Tools, a.chatToolCallProps(window, activityID, index, conversation, expanded, palette, width-24))
	}
	return props
}

// chatToolCallProps measures one tool badge and its expandable details.
func (a *App) chatToolCallProps(window *woxui.Window, activityID string, index int, conversation chatConversation, expanded map[string]bool, palette uiPalette, width float32) previewview.ChatToolCallProps {
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
	scale := a.chatDensityScale()
	durationWidth := float32(0)
	if metrics, err := window.MeasureText(duration, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11)}); err == nil {
		durationWidth = metrics.Size.Width
	}
	name := chatToolOriginLabel(tool)
	if name == "" {
		name = a.language.translate("i18n:ui_ai_chat_tools")
	}
	nameWidth := float32(0)
	if metrics, err := window.MeasureText(name, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11), Weight: woxui.FontWeightSemibold}); err == nil {
		nameWidth = metrics.Size.Width
	}
	callID := conversation.ID
	if callID == "" {
		callID = fmt.Sprintf("%s:%d", activityID, index)
	}
	props := previewview.ChatToolCallProps{
		Key: callID, DensityScale: scale, Name: name, NameWidth: nameWidth, Duration: duration, DurationWidth: durationWidth,
		Status: status, StatusColor: chatToolStatusColor(status, palette.componentTheme()), Expanded: expanded[callID],
		OnToggle: func() { a.chat.toggleChatDisclosure(callID) },
	}
	if !props.Expanded {
		return props
	}
	params := tool.Delta
	if status != "streaming" {
		if encoded, err := json.Marshal(tool.Arguments); err == nil {
			params = string(encoded)
		}
	}
	details := [][2]string{
		{a.language.translate("i18n:ui_ai_chat_tool_detail_id"), tool.ID},
	}
	if source := a.chatToolSourceLabel(tool.Source); source != "" {
		details = append(details, [2]string{a.language.translate("i18n:ui_ai_chat_tool_detail_source"), source})
	}
	if strings.TrimSpace(tool.Server) != "" {
		details = append(details, [2]string{a.language.translate("i18n:ui_ai_chat_tool_detail_server"), tool.Server})
	}
	details = append(details,
		[2]string{a.language.translate("i18n:ui_ai_chat_tool_detail_name"), tool.Name},
		[2]string{a.language.translate("i18n:ui_ai_chat_tool_detail_params"), params},
	)
	if tool.Response != "" {
		details = append(details, [2]string{a.language.translate("i18n:ui_ai_chat_tool_detail_response"), tool.Response})
	}
	detailTextWidth := max(float32(40), width-52)
	props.Details = make([]previewview.ChatToolDetailProps, 0, len(details))
	props.DetailsHeight = 16
	for detailIndex, detail := range details {
		layout := a.previews.previewTextLayout(window, fmt.Sprintf("chat-tool-detail\x00%s\x00%d", callID, detailIndex), detail[1], woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11)}, detailTextWidth, previewview.ChatScaledSize(scale, 16))
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
func (a *App) chatToolActionLabel(tool chatToolCallInfo) string {
	if tool.Source != "" {
		if label := chatToolOriginLabel(tool); label != "" {
			return label
		}
	}
	key := ""
	switch tool.Name {
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
		if tool.Name != "" {
			return tool.Name
		}
		return a.language.translate("i18n:ui_ai_chat_tools")
	}
	return a.language.translate("i18n:" + key)
}

func (a *App) chatToolSourceLabel(source string) string {
	switch source {
	case string(common.ToolSourceBuiltin):
		return a.language.translate("i18n:ui_ai_chat_tool_source_builtin")
	case string(common.ToolSourceMCP):
		return a.language.translate("i18n:ui_ai_chat_tool_source_mcp")
	default:
		return source
	}
}

// chatToolOriginLabel qualifies a chat tool name with builtin or MCP server identity.
func chatToolOriginLabel(tool chatToolCallInfo) string {
	return common.FormatToolOrigin(common.ToolSource(tool.Source), tool.Server, tool.Name)
}

func (a *App) chatToolActivityStatusLabel(status string) string {
	return a.language.translate("i18n:ui_ai_chat_tool_activity_status_" + status)
}

// chatMessageProps resolves text layouts, images, and controller actions for one conversation.
func (a *App) chatMessageProps(window *woxui.Window, key string, index int, conversation chatConversation, palette uiPalette, width float32, showMeta, hideReasoning bool, imageScale float32) previewview.ChatMessageProps {
	scale := a.chatDensityScale()
	innerWidth := previewview.ChatMessageTextWidth(width, conversation.Role)
	props := previewview.ChatMessageProps{
		Key: fmt.Sprintf("%s-%d", key, index), DensityScale: scale, Role: conversation.Role, ShowMeta: showMeta, Theme: palette.componentTheme(),
		CopyLabel: a.language.translate("i18n:ui_ai_chat_copy_message"), CopiedLabel: a.language.translate("i18n:ui_ai_chat_message_copied"),
		EditLabel: a.language.translate("i18n:ui_ai_chat_edit_message"), RetryLabel: a.language.translate("i18n:ui_ai_chat_regenerate_response"),
		OnTooltip: a.previews.setPreviewTooltip,
	}
	if conversation.Timestamp > 0 {
		props.Timestamp = time.UnixMilli(conversation.Timestamp).Local().Format("15:04")
		if metrics, err := window.MeasureText(props.Timestamp, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11)}); err == nil {
			props.TimestampWidth = metrics.Size.Width
		}
	}
	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		props.ToolText = formatChatToolCall(conversation)
		props.ToolLayout = a.previews.previewTextLayout(window, fmt.Sprintf("chat-tool\x00%s\x00%d", key, index), props.ToolText, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, 11)}, innerWidth, previewview.ChatScaledSize(scale, 17))
	} else {
		if reasoning := strings.TrimSpace(conversation.Reasoning); reasoning != "" && !hideReasoning {
			props.Reasoning = reasoning
		}
		props.Text = strings.TrimSpace(conversation.Text)
	}
	if conversation.Role == "user" && props.Text != "" {
		props.RichRuns, _ = chatTokenChipDecorations(props.Text, a.chat.chatMentionCatalog(), window, chatMessageChipTheme(props.Theme, conversation.Role), func(mention chatMention) *woxui.Image {
			return a.imageForSurface(mention.Icon, 256, palette.background)
		})
	}
	if len(props.RichRuns) == 0 && (len(conversation.SkillRefs) > 0 || len(conversation.Mentions) > 0) {
		names := make([]string, 0, len(conversation.SkillRefs)+len(conversation.Mentions))
		for _, skill := range conversation.SkillRefs {
			if skill.Name != "" {
				names = append(names, "#"+skill.Name)
			}
		}
		for _, mention := range conversation.Mentions {
			if mention.Name != "" {
				names = append(names, chatMentionChipLabel(mention.Name))
			}
		}
		props.Skills = strings.Join(names, "  ")
	}
	props.Attachments = a.chatAttachmentProps(conversation.Attachments, a.language.translate("i18n:ui_ai_chat_quote_label"))
	for i := range props.Attachments {
		attachment := &props.Attachments[i]
		if attachment.Kind != common.AIChatAttachmentQuote {
			continue
		}
		attachment.Layout = a.previews.previewTextLayout(window, fmt.Sprintf("chat-attachment\x00%s\x00%d\x00%d", key, index, i), attachment.Text, woxui.TextStyle{Size: previewview.ChatScaledSize(scale, woxcomponent.SettingsHelpFontSize)}, max(float32(0), innerWidth-32), previewview.ChatScaledSize(scale, 18))
	}
	if len(conversation.Images) > 0 {
		props.Images = make([]*woxui.Image, 0, min(3, len(conversation.Images)))
		for _, source := range conversation.Images[:min(3, len(conversation.Images))] {
			props.Images = append(props.Images, a.imageFor(source))
		}
	}
	if copyText := chatConversationClipboardText(conversation); copyText != "" {
		props.OnCopy = func() bool { return a.chat.copyChatText(copyText) }
	}
	if conversation.ID != "" {
		conversationID := conversation.ID
		switch conversation.Role {
		case "user":
			props.OnEdit = func() { a.chat.editChatConversation(conversationID) }
		case "assistant":
			props.OnRetry = func() { a.chat.regenerateChatConversation(conversationID) }
		}
	}
	return a.prepareChatMessage(props, window, width, imageScale)
}

// chatConversationClipboardText keeps platform clipboard behavior independent from chat rendering.
func chatConversationClipboardText(conversation chatConversation) string {
	if conversation.Role == "tool" || conversation.ToolCallInfo.Name != "" {
		return strings.TrimSpace(formatChatToolCall(conversation))
	}
	return common.ChatMessageText(conversation.Text, conversation.Attachments)
}

// formatChatToolCall keeps tool name, state, arguments, and response visible in the first vertical slice.
func formatChatToolCall(conversation chatConversation) string {
	tool := conversation.ToolCallInfo
	name := chatToolOriginLabel(tool)
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
func (a *App) chatInputProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32, window *woxui.Window, onKey func(woxui.KeyEvent) bool) previewview.ChatInputProps {
	hintKey := "i18n:ui_ai_chat_input_hint"
	hintFallback := "Type a message. Use / for models and skills, @ to mention a plugin or attach a file"
	if len(snapshot.attachments) > 0 {
		hintKey = "i18n:plugin_ai_chat_input_hint_with_attachments"
		hintFallback = "Ask about the attached files or images"
		if snapshot.attachments[0].Kind == common.AIChatAttachmentQuote {
			hintKey = "i18n:ui_ai_chat_input_hint_with_quote"
			hintFallback = "Ask a follow-up about the quoted text"
		}
	}
	hint := a.language.translate(hintKey)
	if strings.TrimSpace(hint) == "" || hint == hintKey {
		hint = hintFallback
	}
	model := strings.TrimSpace(snapshot.chat.Model.Name)
	if model == "" {
		model = a.language.translate("i18n:ui_ai_chat_select_model")
	}
	if strings.TrimSpace(model) == "" || model == "i18n:ui_ai_chat_select_model" {
		model = "Select model"
	}
	streaming := snapshot.chat.IsStreaming || snapshot.sending
	status := ""
	statusColor := palette.resultSubtitle
	if snapshot.error != "" {
		status = snapshot.error
		statusColor = palette.componentTheme().ErrorText
	} else if snapshot.importing {
		status = a.language.translate("i18n:ui_ai_chat_importing_attachments")
		if strings.TrimSpace(status) == "" || status == "i18n:ui_ai_chat_importing_attachments" {
			status = "Adding attachments…"
		}
	} else if snapshot.loading {
		status = "Loading…"
	}
	quoteLabel := a.language.translate("i18n:ui_ai_chat_quote_label")
	if strings.TrimSpace(quoteLabel) == "" || quoteLabel == "i18n:ui_ai_chat_quote_label" {
		quoteLabel = "Quote"
	}
	quoteDismiss := a.language.translate("i18n:plugin_ai_chat_attachment_dismiss")
	if strings.TrimSpace(quoteDismiss) == "" || quoteDismiss == "i18n:plugin_ai_chat_attachment_dismiss" {
		quoteDismiss = "Remove attachment"
	}
	attachLabel := a.language.translate("i18n:ui_ai_chat_attach_file")
	if strings.TrimSpace(attachLabel) == "" || attachLabel == "i18n:ui_ai_chat_attach_file" {
		attachLabel = "Attach file"
	}
	theme := palette.componentTheme()
	richRuns, atomicTokens := chatTokenChipDecorations(snapshot.editing.Text, snapshot.mentions, window, theme.Controls, func(mention chatMention) *woxui.Image {
		return a.imageForSurface(mention.Icon, 256, palette.background)
	})
	var projectIcon *woxui.Image
	if len(snapshot.projects.Projects) > 0 {
		projectIcon = a.imageForSurface(fromCoreImage(icons.Get(icons.ControlFolderOpen)), 64, palette.background)
	}
	return previewview.ChatInputProps{
		Width: width, Height: height, DensityScale: a.chatDensityScale(), Key: snapshot.key, Editing: snapshot.editing,
		Focused: snapshot.active && snapshot.projects.Editor == nil && snapshot.projects.DeleteID == "" && snapshot.question == nil && snapshot.panel != "models" && snapshot.panel != "skills" && snapshot.panel != chatProjectPanel, Hint: hint, Window: window,
		ProjectVisible: len(snapshot.projects.Projects) > 0, ProjectLabel: a.chatProjectLabel(snapshot), ProjectIcon: projectIcon, OnProjects: a.chat.openChatProjects, ProjectBusy: snapshot.projects.Busy,
		Model: model, Status: status, StatusColor: statusColor, SendLabel: a.language.translate("i18n:ui_ai_chat_send"), StopLabel: a.language.translate("i18n:ui_ai_chat_stop"), OnStop: a.chat.stopChatMessage, Sending: streaming, Importing: snapshot.importing, Theme: theme,
		Attachments: a.chatAttachmentProps(snapshot.attachments, quoteLabel), QuoteDismissLabel: quoteDismiss,
		RichRuns: richRuns, AtomicTokens: atomicTokens,
		OnFocus: a.chat.focusChatInput, OnChanged: a.chat.setChatText, OnKey: onKey, OnPaste: a.chat.pasteChatComposer,
		OnAttach: a.chat.attachChatComposerFile, AttachLabel: attachLabel,
		OnModels: func() { a.chat.toggleChatPanel("models") }, OnSend: a.chat.sendChatMessage, OnDismissAttachment: a.chat.dismissChatAttachment,
	}
}

// chatQuestionPanelHeight bounds the tool question without starving the conversation viewport.
func chatQuestionPanelHeight(snapshot *chatPreviewSnapshot, available, scale float32) float32 {
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
	// The question title grows with interface size; keep the same slack the normal panel reserved.
	height += max(float32(0), previewview.ChatScaledSize(scale, 17)*2-34)
	return min(max(float32(140), height), max(float32(140), available*0.48))
}

// chatQuestionProps prepares ask-user options and keeps selection and submission in the controller.
func (a *App) chatQuestionProps(snapshot *chatPreviewSnapshot, palette uiPalette, width, height float32, window *woxui.Window, onKey func(woxui.KeyEvent) bool) previewview.ChatQuestionProps {
	question := snapshot.question
	props := previewview.ChatQuestionProps{
		Width: width, Height: height, DensityScale: a.chatDensityScale(), Question: question.Question, Theme: palette.componentTheme(),
		OnCancel: func() { a.chat.submitAIQuestionAnswer("User cancelled") }, OnSubmit: a.chat.submitSelectedAIQuestionAnswer,
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
			Selected: index == snapshot.questionSelected, OnSelect: func() { a.chat.selectAIQuestionOption(index) },
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
			Focused: snapshot.active, Window: window, OnFocus: a.chat.focusAIQuestionInput, OnChanged: a.chat.setAIQuestionText, OnKey: onKey,
		}
	}
	return props
}

// chatAttachmentProps keeps attachment identity while translating presentation labels.
func (a *App) chatAttachmentProps(attachments []common.AIChatAttachment, quoteLabel string) []previewview.ChatAttachmentProps {
	props := make([]previewview.ChatAttachmentProps, 0, len(attachments))
	for _, attachment := range attachments {
		label := attachment.Name
		if attachment.Kind == common.AIChatAttachmentQuote {
			label = quoteLabel
		}
		item := previewview.ChatAttachmentProps{ID: attachment.ID, Kind: attachment.Kind, Label: label, Text: attachment.Text}
		switch attachment.Kind {
		case common.AIChatAttachmentFile:
			item.Text = attachment.URL
			item.Image = a.imageFor(fromCoreImage(icons.Get(icons.PluginFile)))
		case common.AIChatAttachmentFolder:
			item.Text = attachment.URL
			item.Image = a.imageFor(fromCoreImage(icons.Get(icons.PluginFolder)))
		case common.AIChatAttachmentImage:
			item.Text = a.language.translate("i18n:plugin_ai_chat_image_label")
			if path := common.ChatAttachmentPath(attachment); path != "" {
				item.Image = a.imageFor(fromCoreImage(common.NewWoxImageAbsolutePath(path)))
			}
		}
		props = append(props, item)
	}
	return props
}

// chatRoundProps keeps completed-turn labels and disclosure controls identical in all chat hosts.
func (a *App) chatRoundProps(item chatRenderItem, theme woxcomponent.Theme, toggle func()) previewview.ChatMessageProps {
	label := a.language.translate("i18n:ui_ai_chat_round_worked_duration")
	if strings.TrimSpace(label) == "" || label == "i18n:ui_ai_chat_round_worked_duration" {
		label = "Worked for %s"
	}
	return previewview.ChatMessageProps{Key: item.roundID, DensityScale: a.chatDensityScale(), Kind: "round", RoundLabel: fmt.Sprintf(label, formatChatRoundDuration(item.roundStart, item.roundEnd)), RoundExpanded: item.roundExpanded, Theme: theme, OnToggleRound: toggle}
}

func chatMessageChipTheme(theme woxcomponent.Theme, role string) woxcomponent.ControlTheme {
	chip := theme.Controls
	if role == "user" && theme.SelectedTitle.A > 0 {
		chip.Text = theme.SelectedTitle
		chip.TextSecondary = theme.SelectedTitle
	}
	return chip
}

func chatTokenChipDecorations(text string, mentions []chatMention, window *woxui.Window, chipTheme woxcomponent.ControlTheme, iconFor func(chatMention) *woxui.Image) ([]woxcomponent.TextFieldRichRun, []woxcomponent.TextFieldTokenRange) {
	skillTags := chatSkillTagRanges(text)
	mentionTags := chatMentionTagRanges(text)
	if len(skillTags) == 0 && len(mentionTags) == 0 {
		return nil, nil
	}
	runs := make([]woxcomponent.TextFieldRichRun, 0, len(skillTags)+len(mentionTags))
	tokens := make([]woxcomponent.TextFieldTokenRange, 0, len(skillTags)+len(mentionTags))
	for _, tag := range skillTags {
		runs = append(runs, woxcomponent.NewTokenChipRun(tag.start, tag.end, tag.name, window, chipTheme))
		tokens = append(tokens, woxcomponent.TextFieldTokenRange{Start: tag.start, End: tag.end})
	}
	for _, tag := range mentionTags {
		label := chatMentionChipLabel(tag.name)
		var icon *woxui.Image
		if mention, ok := lookupChatMention(tag.kind, tag.name, mentions); ok {
			label = chatMentionChipLabel(mention.Name)
			if iconFor != nil {
				icon = iconFor(mention)
			}
		}
		runs = append(runs, woxcomponent.NewTokenChipRunWithIcon(tag.start, tag.end, label, icon, window, chipTheme))
		tokens = append(tokens, woxcomponent.TextFieldTokenRange{Start: tag.start, End: tag.end})
	}
	return runs, tokens
}

// prepareChatMessage applies the same rich reply rendering and geometry to every chat host.
// chatDensityScale is the interface-size multiplier shared by chat text and its line boxes.
func (a *App) chatDensityScale() float32 {
	if a == nil {
		return 1
	}
	return a.appearance.densityMetrics.normalized().scale
}

func (a *App) prepareChatMessage(props previewview.ChatMessageProps, window *woxui.Window, width, imageScale float32) previewview.ChatMessageProps {
	props.Window = window
	if props.DensityScale <= 0 {
		props.DensityScale = a.chatDensityScale()
	}
	if props.Role == "assistant" && props.Text != "" {
		innerWidth := previewview.ChatMessageTextWidth(width, props.Role)
		markdown := a.previews.markdownPropsWithDocument("chat-markdown-"+props.Key, woxcomponent.MarkdownDocument{}, "", uiPalette{}, innerWidth, imageScale)
		markdown.Theme, markdown.Window, markdown.FontSize = props.Theme.Controls, window, previewview.ChatMessageFontSize(props.DensityScale)
		markdown.InlineTrailing = props.TextTrailing
		font := ""
		if a.generalSettings != nil {
			font = a.generalSettings.Data().AppFontFamily
		}
		markdown, props.TextLayout.Size = a.chat.chatMarkdown.measure(props.Key, props.Text, chatMarkdownLayoutKey{
			width: innerWidth, scale: imageScale, fontSize: markdown.FontSize, font: font, images: a.imageStore.imagesRevision.Load(), window: window, trailing: props.TextTrailing != nil,
		}, markdown)
		props.Markdown = &markdown
	}
	// Bind cached layouts to the presenting host; the hidden launcher may have
	// already released its native text measurer while another window stays open.
	return previewview.MeasureChatMessage(props, window, width, func(key, text string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout {
		return a.previews.previewTextLayout(window, key, text, style, width, lineHeight)
	})
}
