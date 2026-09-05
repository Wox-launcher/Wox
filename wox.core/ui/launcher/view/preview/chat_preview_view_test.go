package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatMessageUsesContentWidthAndCenteredDisclosureIcon(t *testing.T) {
	action := func() {}
	copyAction := func() bool { return true }
	actions, actionWidth := chatMessageActions(ChatMessageProps{Key: "user", OnCopy: copyAction, OnEdit: action}, true, func(bool, woxui.Rect) {})
	if len(actions) != 2 || actionWidth != chatMessageActionWidth(ChatMessageProps{OnCopy: copyAction, OnEdit: action}) {
		t.Fatalf("actions = %d, width = %.0f", len(actions), actionWidth)
	}

	user := chatMessageContent(ChatMessageProps{
		Key: "user", Role: "user", ContentWidth: 26, Text: "你好",
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 800, Height: 19}},
		Theme:      woxcomponent.Theme{SelectedBackground: woxui.Color{A: 255}},
	}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	stack := user.Child.(woxwidget.Stack)
	if stack.Children[0].Left != 948 {
		t.Fatalf("user bubble left = %.0f, want 948", stack.Children[0].Left)
	}
	card := stack.Children[0].Child.(woxwidget.Flex)
	body := card.Children[0].(woxwidget.Container)
	if body.Width != 50 || body.Color.A != 255 {
		t.Fatalf("user bubble = width %.0f, color %#v", body.Width, body.Color)
	}

	collapsed := chatMessageContent(ChatMessageProps{Key: "round", Kind: "round", RoundLabel: "Worked for 0s"}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	expanded := chatMessageContent(ChatMessageProps{Key: "round", Kind: "round", RoundLabel: "Worked for 0s", RoundExpanded: true}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	row := collapsed.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("round alignment = %v", row.CrossAxisAlignment)
	}
	icon, ok := row.Children[0].(woxwidget.Image)
	if !ok || icon.Source == nil {
		t.Fatalf("round icon = %#v", row.Children[0])
	}
	expandedIcon := expanded.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Image)
	if icon.Source == expandedIcon.Source {
		t.Fatal("collapsed and expanded round icons should point right and down")
	}

	assistant := chatMessageContent(ChatMessageProps{Key: "assistant", Role: "assistant", Text: "hello", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}}}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	assistantStack := assistant.Child.(woxwidget.Stack)
	assistantBody := assistantStack.Children[0].Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	if assistantStack.Children[0].Left != 2 || assistantBody.Width != 996 || assistantBody.Padding.Left != 0 {
		t.Fatalf("assistant gutter = left %.0f, width %.0f, padding %.0f", assistantStack.Children[0].Left, assistantBody.Width, assistantBody.Padding.Left)
	}

	markdown := woxcomponent.MarkdownProps{ID: "assistant-markdown", Document: woxcomponent.ParseMarkdown("**bold**")}
	markdownAssistant := chatMessageContent(ChatMessageProps{
		Key: "assistant-markdown", Role: "assistant", Text: "**bold**", Markdown: &markdown,
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}},
	}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	markdownBody := markdownAssistant.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	markdownContent, ok := markdownBody.Child.(woxwidget.Flex)
	if !ok || len(markdownContent.Children) != 1 {
		t.Fatalf("assistant Markdown body = %#v, want shared WoxMarkdown content", markdownBody.Child)
	}
	sharedMarkdown, ok := markdownContent.Children[0].(woxwidget.Flex)
	if !ok || len(sharedMarkdown.Children) != 1 {
		t.Fatalf("assistant Markdown content = %#v, want shared WoxMarkdown tree", markdownContent.Children[0])
	}
	if _, ok := sharedMarkdown.Children[0].(woxwidget.Wrap); !ok {
		t.Fatalf("assistant Markdown block = %#v, want parsed inline runs", sharedMarkdown.Children[0])
	}
}

func TestChatMessageActionsUseSharedIconButtonHover(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultSubtitle: woxui.Color{R: 180, G: 180, B: 180, A: 200},
		Cursor:         woxui.Color{R: 30, G: 110, B: 220, A: 255},
	}
	wantHover := theme.ResultSubtitle
	wantHover.A = uint8(float32(wantHover.A) * 0.1)
	action := func() {}
	copyAction := func() bool { return true }
	actions, _ := chatMessageActions(ChatMessageProps{
		Key: "assistant", Theme: theme, CopyLabel: "Copy", RetryLabel: "Regenerate",
		OnCopy: copyAction, OnRetry: action,
	}, true, func(bool, woxui.Rect) {})
	if len(actions) != 2 {
		t.Fatalf("actions = %d, want copy and retry", len(actions))
	}
	for index, name := range []string{"copy", "retry"} {
		button := actions[index].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if button.ID != "chat-"+name+"-assistant" || button.HoverBackground != wantHover || button.OnHoverAt == nil {
			t.Fatalf("%s = id %q hover %#v hoverAt %v, want shared icon-button hover", name, button.ID, button.HoverBackground, button.OnHoverAt != nil)
		}
		if button.Width != 18 || button.Height != 18 || button.Radius != 5 {
			t.Fatalf("%s geometry = %.0fx%.0f radius %.0f, want 18x18 at 5", name, button.Width, button.Height, button.Radius)
		}
	}

	view := chatMessageContent(ChatMessageProps{
		Key: "assistant", Role: "assistant", ShowMeta: true, Timestamp: "19:41", TimestampWidth: 20,
		Text: "hello", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}},
		CopyLabel: "Copy", RetryLabel: "Regenerate", OnCopy: copyAction, OnRetry: action, Theme: theme,
	}, 1000, true, func(bool) {}, func(bool, woxui.Rect) {}).(woxwidget.Gesture)
	footer := view.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Flex).Children[1].(woxwidget.Container)
	if _, isAlign := footer.Child.(woxwidget.Align); isAlign || footer.Width != 0 || footer.Height != 18 {
		t.Fatalf("footer = %#v, want shrink-wrapped 18-high container so the last action stays hittable", footer)
	}
}

func TestChatMessageCopyActionShowsCopiedFeedback(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 180, G: 180, B: 180, A: 200}}
	idleIcon := woxcomponent.CopyGlyph(14, theme.ResultSubtitle).(woxwidget.Image)
	copiedIcon := woxcomponent.CheckGlyph(14, theme.ResultSubtitle).(woxwidget.Image)
	copyAction := func() bool { return true }

	idleActions, _ := chatMessageActions(ChatMessageProps{
		Key: "user", Theme: theme, CopyLabel: "Copy message", CopiedLabel: "Message copied to clipboard",
		OnCopy: copyAction,
	}, true, nil)
	idle := idleActions[0].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if idle.Label != "Copy message" || idle.Icon.(woxwidget.Image).Source != idleIcon.Source {
		t.Fatalf("idle copy = label %q icon %#v, want copy glyph", idle.Label, idle.Icon)
	}

	copiedActions, _ := chatMessageActions(ChatMessageProps{
		Key: "user", Theme: theme, Copied: true, CopyLabel: "Copy message", CopiedLabel: "Message copied to clipboard",
		OnCopy: copyAction,
	}, true, nil)
	copied := copiedActions[0].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if copied.Label != "Message copied to clipboard" || copied.Icon.(woxwidget.Image).Source != copiedIcon.Source {
		t.Fatalf("copied = label %q icon %#v, want check glyph and copied label", copied.Label, copied.Icon)
	}
}

func TestChatToolActivityUsesNestedDisclosureHeights(t *testing.T) {
	tool := ChatToolCallProps{Key: "tool", Name: "web_search", NameWidth: 72, Duration: "3273ms", DurationWidth: 42, Status: "succeeded", DetailsHeight: 72}
	collapsed := ChatMessageProps{Key: "activity", Kind: "tool-activity", ToolSummary: "Completed · Search web · 1 tool", ToolSummaryWidth: 174, Tools: []ChatToolCallProps{tool}}
	if height := chatMessageHeight(collapsed); height != 34 {
		t.Fatalf("collapsed activity height = %.0f, want 34", height)
	}
	collapsed.RoundExpanded = true
	if height := chatMessageHeight(collapsed); height != 74 {
		t.Fatalf("expanded activity height = %.0f, want 74", height)
	}
	collapsed.Tools[0].Expanded = true
	if height := chatMessageHeight(collapsed); height != 160 {
		t.Fatalf("expanded tool detail height = %.0f, want 160", height)
	}
	view := chatToolActivity(collapsed, 1000).(woxwidget.Container)
	if view.Padding.Left != 2 || view.Height != 160 {
		t.Fatalf("tool activity frame = padding %.0f, height %.0f", view.Padding.Left, view.Height)
	}
	header := view.Child.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if summary := header.Children[2].(woxwidget.Container); summary.Width != 174 {
		t.Fatalf("tool summary width = %.0f, want measured width 174", summary.Width)
	}
	toolRow := chatToolCall(tool, 900, woxcomponent.Theme{}).(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if name := toolRow.Children[2].(woxwidget.Container); name.Width != 72 {
		t.Fatalf("tool name width = %.0f, want measured width 72", name.Width)
	}
}

func TestChatMessagesScrollMetricsHashesEveryItemHeight(t *testing.T) {
	first := []ChatMessageProps{{Kind: "round"}, {Kind: "tool-activity"}, {Kind: "round"}}
	second := []ChatMessageProps{{Kind: "round"}, {Kind: "round"}, {Kind: "tool-activity"}}
	heightA, revisionA := ChatMessagesScrollMetrics(first, 0)
	heightB, revisionB := ChatMessagesScrollMetrics(second, 0)
	if heightA != heightB {
		t.Fatalf("reordered heights = %.0f and %.0f, want the same total", heightA, heightB)
	}
	if revisionA == revisionB {
		t.Fatal("reordered item heights kept the same extent revision")
	}
}

func TestChatMessagesUsesSharedScrollView(t *testing.T) {
	theme := woxcomponent.Theme{ResultTitle: woxui.Color{R: 120, G: 130, B: 140, A: 255}}
	maxOffset := float32(0)
	view := ChatMessages(ChatMessagesProps{
		Width: 500, Height: 300, Key: "test", ContentHeight: 600,
		Messages: []ChatMessageProps{{Key: "user", Role: "user", Text: "hello", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}}}},
		Theme:    theme,
		OnScroll: func(delta, offset float32) { maxOffset = offset },
	}).(woxwidget.Container)

	scroll, ok := view.Child.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("chat messages child = %#v, want shared WoxScrollView", view.Child)
	}
	props := scroll.Widget.(woxcomponent.ScrollViewProps)
	if props.Height != 286 || props.ContentHeight != 600 {
		t.Fatalf("scroll geometry = height %.0f content %.0f, want 286/600", props.Height, props.ContentHeight)
	}
	if props.ThumbColor != theme.ResultTitle {
		t.Fatalf("scrollbar thumb color = %#v, want ResultTitle so ResultSubtitle cannot restyle it", props.ThumbColor)
	}
	if props.OnScroll != nil {
		props.OnScroll(20)
	}
	if maxOffset != 314 {
		t.Fatalf("scroll callback maxOffset = %.0f, want 314", maxOffset)
	}
}

func TestChatPreviewCanHostHeaderOutsideContent(t *testing.T) {
	view := ChatPreview(ChatPreviewProps{
		Width: 500, Height: 400, Key: "external-header",
		Messages: ChatMessagesProps{Width: 480, Height: 288},
		Input:    ChatInputProps{Width: 480, Height: 98},
	}).(woxwidget.Stack)

	body := view.Children[0].Child.(woxwidget.Flex)
	if len(body.Children) != 2 {
		t.Fatalf("chat body children = %d, want messages and input without an internal header", len(body.Children))
	}
}

func TestChatPreviewHistorySidebarPushesContent(t *testing.T) {
	view := ChatPreview(ChatPreviewProps{
		Width: 800, Height: 600, Key: "history", Panel: "history",
		Messages: ChatMessagesProps{Width: 520, Height: 488},
		Input:    ChatInputProps{Width: 520, Height: 98},
		Catalog:  &ChatCatalogProps{Width: 260, Height: 600, Key: "history", ShowNew: true},
	}).(woxwidget.Flex)

	if view.Axis != woxwidget.Horizontal || len(view.Children) != 2 {
		t.Fatalf("history split = %+v, want two horizontal columns", view)
	}
	if _, ok := view.Children[0].(woxwidget.Stack); !ok {
		t.Fatalf("history sidebar = %T, want an in-flow catalog without a dismiss overlay", view.Children[0])
	}
	content := view.Children[1].(woxwidget.Container)
	if content.Width != 540 || content.Padding.Left != 10 {
		t.Fatalf("history content = width %.0f padding %+v, want 540-wide pushed content", content.Width, content.Padding)
	}
}

func TestChatPreviewKeepsHistoryWhileCommandOverlayIsOpen(t *testing.T) {
	view := ChatPreview(ChatPreviewProps{
		Width: 800, Height: 600, Key: "both", Panel: "commands",
		Messages: ChatMessagesProps{Width: 520, Height: 488},
		Input:    ChatInputProps{Width: 520, Height: 98},
		History:  &ChatCatalogProps{Width: 260, Height: 600, Key: "history", ShowNew: true},
		Catalog:  &ChatCatalogProps{Width: 400, Height: 80, Key: "commands", EmptyMessage: "No data"},
	}).(woxwidget.Flex)

	if view.Axis != woxwidget.Horizontal || len(view.Children) != 2 {
		t.Fatalf("slash overlay split = %+v, want the sidebar to stay in flow", view)
	}
	content, ok := view.Children[1].(woxwidget.Stack)
	if !ok || content.Width != 540 {
		t.Fatalf("conversation column = %#v, want a 540-wide overlay stack", view.Children[1])
	}
}

func TestChatMessagesCentersEmptyStateWithAlign(t *testing.T) {
	view := ChatMessages(ChatMessagesProps{
		Width: 500, Height: 300, EmptyMessage: "Ask anything", EmptyTextWidth: 180, EmptyTextHeight: 36,
	}).(woxwidget.Container)
	alignment := view.Child.(woxwidget.Align)
	if alignment.Width != 500 || alignment.Height != 286 || alignment.Horizontal != 0.5 || alignment.Vertical != 0.5 {
		t.Fatalf("chat empty alignment = %#v, want centered 500x286 viewport", alignment)
	}
}

func TestChatDebugUsesMeasuredControlledScrollGeometry(t *testing.T) {
	view := ChatDebug(ChatDebugProps{
		Width: 500, Height: 300, Key: "debug", Value: "trace", Layout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 400, Height: 180}},
		OnScroll: func(float32) {}, OnGeometryChanged: func(float32, float32) {},
	}).(woxwidget.Container)
	bodyWidget := view.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child
	body := resolvedScrollViewProps(bodyWidget, woxui.Size{Width: 480, Height: 258})
	content := body.Content.(woxwidget.Constrained).Child.(woxwidget.Container)

	if body.ContentHeight != 0 || body.OnGeometryChanged == nil || content.Height != 0 {
		t.Fatalf("debug scroll = content hint %.0f callback %v container height %.0f, want measured controlled geometry", body.ContentHeight, body.OnGeometryChanged != nil, content.Height)
	}
}

func TestChatHistoryCatalogUsesFullHeightDrawerGeometry(t *testing.T) {
	theme := woxcomponent.Theme{Background: woxui.Color{R: 20, G: 21, B: 22, A: 255}, ActionBackground: woxui.Color{R: 30, G: 31, B: 32, A: 255}, PreviewText: woxui.Color{A: 255}, PreviewSplit: woxui.Color{R: 90, G: 91, B: 92, A: 80}}
	drawer := ChatCatalog(ChatCatalogProps{
		Width: 260, Height: 600, Key: "history", ShowNew: true, NewLabel: "New Chat", ContentHeight: 576, Theme: theme,
		Items: []ChatCatalogItemProps{{SelectID: "chat", DeleteID: "delete", Kind: "history", Title: "Suzhou", GroupLabel: "Today", Selected: true, OnSelect: func() {}, OnDelete: func() {}}},
	}).(woxwidget.Stack)
	if drawer.Width != 260 || drawer.Height != 600 || len(drawer.Children) != 2 || !drawer.Children[1].AnchorRight {
		t.Fatalf("history drawer = width %.0f, height %.0f, children %+v", drawer.Width, drawer.Height, drawer.Children)
	}
	panel := drawer.Children[0].Child.(woxwidget.Container)
	if panel.Radius != 0 || panel.Padding.Left != 10 || panel.Padding.Top != 12 || panel.Color.A != 0 {
		t.Fatalf("history panel = radius %.0f, padding %+v, color %#v", panel.Radius, panel.Padding, panel.Color)
	}
	if divider := drawer.Children[1].Child.(woxwidget.Container); divider.Width != 1 || divider.Color != theme.PreviewSplit {
		t.Fatalf("history divider = width %.0f color %#v, want semantic preview divider", divider.Width, divider.Color)
	}
}

func TestChatHistoryItemOmitsBubbleIcon(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "row", Kind: "history", Title: "Suzhou", DeleteID: "delete", OnSelect: func() {}, OnDelete: func() {}}
	view := chatHistoryItem(item, 260, 46, woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}, false, func(bool) {}).(woxwidget.Container)
	stack := view.Child.(woxwidget.Stack)
	row := chatHistoryTestRow(stack.Children[0].Child)
	if _, ok := row.Child.(woxwidget.Align).Child.(woxwidget.Text); !ok || row.Padding.Left != 12 {
		t.Fatal("history row should contain an indented title without a bubble icon")
	}
}

// chatHistoryTestRow resolves the shared list item's retained hover wrapper for visual assertions.
func chatHistoryTestRow(row woxwidget.Widget) woxwidget.Container {
	content := row.(woxwidget.Gesture).Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child
	if stateful, ok := content.(woxwidget.Stateful); ok {
		content = stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget)
	}
	return content.(woxwidget.Gesture).Child.(woxwidget.Container)
}

func TestChatHistoryRowOwnsKeyboardActivation(t *testing.T) {
	selected := 0
	deleted := 0
	item := ChatCatalogItemProps{SelectID: "row", Kind: "history", Title: "Chat", DeleteID: "delete",
		OnSelect: func() { selected++ }, OnDelete: func() { deleted++ },
	}
	row := chatHistoryItem(item, 260, ChatHistoryRowHeight, woxcomponent.Theme{}, false, nil).(woxwidget.Container)
	semantics := row.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Gesture).Child.(woxwidget.Semantics)
	focus := semantics.Child.(woxwidget.Focusable)
	for _, key := range []woxui.Key{woxui.KeyEnter, woxui.KeySpace} {
		if !focus.OnKey(woxui.KeyEvent{Key: key, Down: true}) {
			t.Fatalf("focused history row ignored %s", key)
		}
	}
	if selected != 2 || deleted != 0 || semantics.Role != woxui.AccessibilityRoleListItem {
		t.Fatal("history keyboard activation must select its own row without deleting it")
	}
}

func TestChatHistoryItemUsesSelectedForegroundOnHover(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewText: woxui.Color{R: 40, G: 40, B: 40, A: 255}, ResultSubtitle: woxui.Color{R: 80, G: 80, B: 80, A: 255},
		SelectedBackground: woxui.Color{R: 30, G: 110, B: 220, A: 255}, SelectedTitle: woxui.Color{R: 255, G: 255, B: 255, A: 255}, SelectedSubtitle: woxui.Color{R: 230, G: 235, B: 245, A: 255},
	}
	item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Suzhou", OnSelect: func() {}, OnDelete: func() {}}
	view := chatHistoryItem(item, 260, 46, theme, true, func(bool) {}).(woxwidget.Container)
	stack := view.Child.(woxwidget.Stack)
	row := chatHistoryTestRow(stack.Children[0].Child)
	title := row.Child.(woxwidget.Align).Child.(woxwidget.Text)
	if row.Color != theme.SelectedBackground || title.Color != theme.SelectedTitle {
		t.Fatalf("hovered history row = background %#v title %#v, want selected palette", row.Color, title.Color)
	}
	delete := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	deleteGlyph := delete.Icon.(woxwidget.Image)
	expectedGlyph := woxcomponent.DeleteGlyph(15, theme.SelectedSubtitle).(woxwidget.Image)
	if deleteGlyph.Source != expectedGlyph.Source {
		t.Fatal("hovered history delete icon should use the selected subtitle color")
	}

	newChat := chatHistoryItem(ChatCatalogItemProps{SelectID: "new", Kind: "history-new", Title: "New Chat"}, 260, 38, theme, true, func(bool) {}).(woxwidget.Gesture)
	newChatRow := chatHistoryTestRow(newChat)
	newChatContent := newChatRow.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if newChatRow.Color != theme.SelectedBackground || newChatContent.Children[1].(woxwidget.Text).Color != theme.SelectedTitle {
		t.Fatal("hovered new-chat row should use the selected background and title colors")
	}
}

func TestChatHistoryDeleteUsesDangerHoverAndConfirmation(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultSubtitle: woxui.Color{R: 80, G: 80, B: 80, A: 255}, ErrorText: woxui.Color{R: 210, G: 45, B: 55, A: 255},
		SelectedTitle: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Cursor: woxui.Color{R: 30, G: 110, B: 220, A: 255},
	}
	item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Suzhou", DeleteLabel: "Delete", ConfirmDeleteLabel: "Confirm delete", OnSelect: func() {}, OnDelete: func() {}}
	deleteControl := func(deleteHovered, deleteConfirm bool) woxwidget.Align {
		view := chatHistoryItemWithDeleteState(item, 240, 46, theme, false, deleteHovered, deleteConfirm, func(bool) {}, func(bool) {}, func() {}).(woxwidget.Container)
		stack := view.Child.(woxwidget.Stack)
		return stack.Children[1].Child.(woxwidget.Align)
	}

	hovered := deleteControl(true, false).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	hoveredGlyph := hovered.Icon.(woxwidget.Image)
	expectedDangerGlyph := woxcomponent.DeleteGlyph(15, theme.ErrorText).(woxwidget.Image)
	if hoveredGlyph.Source != expectedDangerGlyph.Source || hovered.HoverBackground.R != theme.ErrorText.R || hovered.HoverBackground.A == 0 {
		t.Fatalf("delete hover = icon %#v background %#v, want danger treatment", hoveredGlyph.Source, hovered.HoverBackground)
	}

	confirm := deleteControl(true, true)
	confirmButton := confirm.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	confirmGlyph := confirmButton.Icon.(woxwidget.Image)
	expectedConfirmGlyph := woxcomponent.CheckGlyph(14, theme.SelectedTitle).(woxwidget.Image)
	if confirm.Width != 28 || confirmButton.Label != item.ConfirmDeleteLabel || confirmButton.Background != theme.ErrorText || confirmGlyph.Source != expectedConfirmGlyph.Source {
		t.Fatalf("delete confirmation = width %.0f label %q background %#v, want danger confirmation icon", confirm.Width, confirmButton.Label, confirmButton.Background)
	}
}

func TestChatHistoryDeleteRequiresTwoActivations(t *testing.T) {
	state := &chatCatalogItemState{}
	if state.advanceDeleteConfirmation() {
		t.Fatal("first delete activation must only enter confirmation")
	}
	if !state.deleteConfirm {
		t.Fatal("first delete activation did not retain confirmation state")
	}
	if !state.advanceDeleteConfirmation() {
		t.Fatal("second delete activation must confirm deletion")
	}
	if state.deleteConfirm {
		t.Fatal("confirmed deletion must clear confirmation state")
	}
}

func TestChatHistoryDeleteConfirmationClearsOnMouseLeave(t *testing.T) {
	state := &chatCatalogItemState{deleteHovered: true, deleteConfirm: true}
	state.setDeleteHovered(false)
	if state.deleteHovered || state.deleteConfirm {
		t.Fatalf("delete state after mouse leave = hovered %v confirm %v, want both cleared", state.deleteHovered, state.deleteConfirm)
	}
}

func TestChatCatalogItemOnlyShowsCheckForCurrentModel(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}

	notCurrent := chatCatalogItem(ChatCatalogItemProps{SelectID: "flash", Kind: "models", Title: "flash", Selected: true}, 400, 38, theme, false, func(bool) {}).(woxwidget.Gesture)
	notCurrentStack := notCurrent.Child.(woxwidget.Container).Child.(woxwidget.Stack)
	notCurrentCheck := notCurrentStack.Children[3].Child.(woxwidget.Container)
	if notCurrentCheck.Width != 0 || notCurrentCheck.Child != nil {
		t.Fatalf("non-current model check slot = width %.0f, child %#v; want empty", notCurrentCheck.Width, notCurrentCheck.Child)
	}

	current := chatCatalogItem(ChatCatalogItemProps{SelectID: "flash", Kind: "models", Title: "flash", Current: true}, 400, 38, theme, false, func(bool) {}).(woxwidget.Gesture)
	currentStack := current.Child.(woxwidget.Container).Child.(woxwidget.Stack)
	currentCheck := currentStack.Children[3].Child.(woxwidget.Container)
	if currentCheck.Width != 28 || currentCheck.Child == nil {
		t.Fatalf("current model check slot = width %.0f, child %#v; want check glyph", currentCheck.Width, currentCheck.Child)
	}
}

func TestChatInputRendersSkillTagsAsAtomicChips(t *testing.T) {
	theme := woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}, ResultSubtitle: woxui.Color{A: 200}, QueryBackground: woxui.Color{A: 255}}
	tag := "{skill:wox-plugin-creator}"
	end := len([]rune(tag))
	run := woxcomponent.NewTokenChipRun(0, end, "wox-plugin-creator", nil, theme)
	input := ChatInput(ChatInputProps{
		Width: 400, Height: 98, Key: "skills", Editing: woxui.TextEditingState{Text: tag + " 士大夫"},
		RichRuns: []woxcomponent.TextFieldRichRun{run}, AtomicTokens: []woxcomponent.TextFieldTokenRange{{Start: 0, End: end}},
		Theme: theme,
	}).(woxwidget.Container)
	field := input.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if len(field.RichRuns) != 1 || !field.RichRuns[0].HideText || field.RichRuns[0].Paint == nil {
		t.Fatalf("chat input rich runs = %#v, want a painted skill chip", field.RichRuns)
	}
	if len(field.AtomicTokens) != 1 || field.AtomicTokens[0] != (woxcomponent.TextFieldTokenRange{Start: 0, End: end}) {
		t.Fatalf("chat input atomic tokens = %#v, want the complete skill tag", field.AtomicTokens)
	}
}

func TestChatInputShowsQuoteCardAboveComposer(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewText:     woxui.Color{R: 220, G: 225, B: 230, A: 255},
		ResultSubtitle:  woxui.Color{R: 180, G: 180, B: 180, A: 200},
		QueryBackground: woxui.Color{R: 30, G: 30, B: 30, A: 255},
	}
	if ChatComposerHeight(0) != 98 || ChatComposerHeight(1) != 154 {
		t.Fatalf("composer height = %.0f/%.0f", ChatComposerHeight(0), ChatComposerHeight(1))
	}

	plain := ChatInput(ChatInputProps{Width: 400, Height: 98, Key: "plain", Theme: theme}).(woxwidget.Container)
	plainCard := plain.Child.(woxwidget.Container)
	if len(plainCard.Child.(woxwidget.Flex).Children) != 3 {
		t.Fatalf("plain composer children = %d, want editor, divider, toolbar", len(plainCard.Child.(woxwidget.Flex).Children))
	}

	quoted := ChatInput(ChatInputProps{
		Width: 400, Height: ChatComposerHeight(1), Key: "quoted", Attachments: []ChatAttachmentProps{{ID: "quoted", Label: "Quote", Text: "selected text"}},
		QuoteDismissLabel: "Remove quote", Theme: theme, OnDismissAttachment: func(string) {},
	}).(woxwidget.Container)
	quotedCard := quoted.Child.(woxwidget.Container)
	children := quotedCard.Child.(woxwidget.Flex).Children
	if len(children) != 4 {
		t.Fatalf("quoted composer children = %d, want quote, editor, divider, toolbar", len(children))
	}
	quote := children[0].(woxwidget.Semantics)
	if quote.AutomationID != "chat-quote-quoted" || quote.Role != woxui.AccessibilityRoleGroup {
		t.Fatalf("quote card = %#v", quote)
	}
	dismiss := quote.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if dismiss.ID != "chat-quote-dismiss-quoted" || dismiss.HoverBackground != chatIconHoverBackground(theme) || dismiss.HoverBackground.A == 0 {
		t.Fatalf("quote dismiss = %#v, want shared icon-button hover", dismiss)
	}
}

func TestSentChatQuotePreservesLinesAndMessageHeight(t *testing.T) {
	attachment := ChatAttachmentProps{ID: "quote", Label: "Quote", Text: "  first line\nsecond line", Layout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 36}}}
	for _, width := range []float32{180, 600} {
		quote := chatQuoteCard(attachment, width, woxcomponent.Theme{}, "", nil, true).(woxwidget.Semantics)
		card := quote.Child.(woxwidget.Container)
		row := card.Child.(woxwidget.Flex)
		if len(row.Children) != 2 {
			t.Fatal("sent quote must not have a dismiss action")
		}
		text := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex).Children[1].(woxwidget.TextBlock)
		if text.Value != attachment.Text || text.MaxLines != 0 || text.Layout == nil {
			t.Fatalf("sent reference was flattened or truncated: %#v", text)
		}
		if text.Width > width-32 || card.Height != 72 {
			t.Fatalf("quote geometry = %#v", card)
		}
	}
	plain := ChatMessageProps{Role: "user", Text: "Explain", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}}}
	quoted := plain
	quoted.Attachments = []ChatAttachmentProps{attachment}
	if chatMessageHeight(quoted)-chatMessageHeight(plain) != 75 {
		t.Fatal("message scroll extent does not include the quote and gap")
	}
	if ChatComposerHeight(2) != 210 {
		t.Fatal("composer must reserve space for every attachment")
	}
}

func TestChatFileAndImageAttachmentsUseCompactCards(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 180, G: 180, B: 180, A: 200}}
	for _, kind := range []string{"file", "image"} {
		attachment := ChatAttachmentProps{ID: "a", Kind: kind, Label: "a very long attachment name", Text: "/original/path", Image: &woxui.Image{Width: 400, Height: 100}}
		for _, sent := range []bool{false, true} {
			for _, width := range []float32{180, 600} {
				view := chatAttachmentCard(attachment, width, woxcomponent.Theme{}, "Remove", nil, sent).(woxwidget.Semantics)
				container := view.Child.(woxwidget.Container)
				row := container.Child.(woxwidget.Flex)
				thumbnail := row.Children[0].(woxwidget.Image)
				if thumbnail.Fit != woxwidget.ImageFitContain {
					t.Fatal("image aspect ratio must be preserved")
				}
				expected := chatQuoteCardHeight
				if sent {
					expected = chatAttachmentHeight(attachment)
				}
				if container.Height != expected || view.Label != attachment.Label+": "+attachment.Text {
					t.Fatalf("attachment card = %#v", view)
				}
			}
		}
	}
	dismissed := chatAttachmentCard(ChatAttachmentProps{ID: "file", Kind: "file", Label: "notes.txt", Text: "/tmp/notes.txt"}, 400, theme, "Remove file", func() {}, false).(woxwidget.Semantics)
	dismiss := dismissed.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if dismiss.ID != "chat-attachment-dismiss-file" || dismiss.HoverBackground != chatIconHoverBackground(theme) || dismiss.HoverBackground.A == 0 {
		t.Fatalf("attachment dismiss = %#v, want shared icon-button hover", dismiss)
	}
	if ChatComposerHeight(100) != ChatComposerHeight(3) {
		t.Fatal("large selections must leave the editor visible")
	}
	attachments := make([]ChatAttachmentProps, 5)
	input := ChatInput(ChatInputProps{Width: 400, Height: ChatComposerHeight(5), Attachments: attachments}).(woxwidget.Container)
	card := input.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(card.Children) != 4 {
		t.Fatal("many attachments should use one scroll pane above the editor")
	}
	if _, ok := card.Children[0].(woxwidget.Stateful); !ok {
		t.Fatal("attachment pane must own retained scroll state")
	}
}

// TestChatHistoryDeleteVisibility preserves keyboard access without a permanent icon column.
func TestChatHistoryDeleteVisibility(t *testing.T) {
	for _, state := range []struct {
		name                                                 string
		hovered, deleteHovered, focused, confirming, visible bool
	}{
		{name: "idle"},
		{name: "row hover", hovered: true, visible: true},
		{name: "delete hover", deleteHovered: true, visible: true},
		{name: "keyboard focus", focused: true, visible: true},
		{name: "confirmation", confirming: true, visible: true},
	} {
		t.Run(state.name, func(t *testing.T) {
			item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Conversation", Selected: true, deleteFocused: state.focused}
			view := chatHistoryItemWithDeleteState(item, 240, ChatHistoryRowHeight, woxcomponent.Theme{}, state.hovered, state.deleteHovered, state.confirming, nil, nil, func() {}).(woxwidget.Container)
			stack := view.Child.(woxwidget.Stack)
			button := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
			if (button.Icon != nil) != state.visible || button.Disabled || button.OnTap == nil {
				t.Fatalf("delete visibility/accessibility = %+v, want visible %v and enabled", button, state.visible)
			}
			row := chatHistoryTestRow(stack.Children[0].Child)
			title := row.Child.(woxwidget.Align)
			if view.Height != 38 || title.Height != row.Height || title.Vertical != 0.5 || title.Child.(woxwidget.Text).Style.Weight == woxui.FontWeightSemibold {
				t.Fatal("history title must be regular and centered in a compact row")
			}
		})
	}
}
