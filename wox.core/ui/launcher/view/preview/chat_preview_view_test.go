package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatMessageUsesContentWidthAndCenteredDisclosureIcon(t *testing.T) {
	action := func() {}
	actions, actionWidth := chatMessageActions(ChatMessageProps{Key: "user", OnCopy: action, OnEdit: action}, true, func(bool) {})
	if len(actions) != 2 || actionWidth != chatMessageActionWidth(ChatMessageProps{OnCopy: action, OnEdit: action}) {
		t.Fatalf("actions = %d, width = %.0f", len(actions), actionWidth)
	}

	user := chatMessageContent(ChatMessageProps{
		Key: "user", Role: "user", ContentWidth: 26, Text: "你好",
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 800, Height: 19}},
		Theme:      woxcomponent.Theme{SelectedBackground: woxui.Color{A: 255}},
	}, 1000, false, func(bool) {}).(woxwidget.Gesture)
	stack := user.Child.(woxwidget.Stack)
	if stack.Children[0].Left != 948 {
		t.Fatalf("user bubble left = %.0f, want 948", stack.Children[0].Left)
	}
	card := stack.Children[0].Child.(woxwidget.Flex)
	body := card.Children[0].(woxwidget.Container)
	if body.Width != 50 || body.Color.A != 255 {
		t.Fatalf("user bubble = width %.0f, color %#v", body.Width, body.Color)
	}

	collapsed := chatMessageContent(ChatMessageProps{Key: "round", Kind: "round", RoundLabel: "Worked for 0s"}, 1000, false, func(bool) {}).(woxwidget.Gesture)
	expanded := chatMessageContent(ChatMessageProps{Key: "round", Kind: "round", RoundLabel: "Worked for 0s", RoundExpanded: true}, 1000, false, func(bool) {}).(woxwidget.Gesture)
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

	assistant := chatMessageContent(ChatMessageProps{Key: "assistant", Role: "assistant", Text: "hello", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}}}, 1000, false, func(bool) {}).(woxwidget.Gesture)
	assistantStack := assistant.Child.(woxwidget.Stack)
	assistantBody := assistantStack.Children[0].Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	if assistantStack.Children[0].Left != 2 || assistantBody.Width != 996 || assistantBody.Padding.Left != 0 {
		t.Fatalf("assistant gutter = left %.0f, width %.0f, padding %.0f", assistantStack.Children[0].Left, assistantBody.Width, assistantBody.Padding.Left)
	}

	markdown := woxcomponent.MarkdownProps{ID: "assistant-markdown", Document: woxcomponent.ParseMarkdown("**bold**")}
	markdownAssistant := chatMessageContent(ChatMessageProps{
		Key: "assistant-markdown", Role: "assistant", Text: "**bold**", Markdown: &markdown,
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}},
	}, 1000, false, func(bool) {}).(woxwidget.Gesture)
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

func TestChatMessagesUsesSharedScrollView(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 120, G: 130, B: 140, A: 255}}
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
	if props.ThumbColor != theme.ResultSubtitle {
		t.Fatalf("scrollbar thumb color = %#v, want theme subtitle", props.ThumbColor)
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
	row := stack.Children[0].Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Stack)
	if len(row.Children) != 1 {
		t.Fatalf("history row children = %d, want title only (no bubble icon)", len(row.Children))
	}
	if row.Children[0].Left != 12 {
		t.Fatalf("history title left = %.0f, want indented 12", row.Children[0].Left)
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
	row := stack.Children[0].Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	title := row.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container).Child.(woxwidget.Text)
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
	newChatRow := newChat.Child.(woxwidget.Container)
	newChatContent := newChatRow.Child.(woxwidget.Flex)
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
	if confirm.Width != 26 || confirmButton.Label != item.ConfirmDeleteLabel || confirmButton.Background != theme.ErrorText || confirmGlyph.Source != expectedConfirmGlyph.Source {
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
