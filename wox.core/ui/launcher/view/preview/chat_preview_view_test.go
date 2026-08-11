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
	theme := woxcomponent.Theme{ActionBackground: woxui.Color{R: 20, G: 21, B: 22, A: 255}, PreviewText: woxui.Color{A: 255}}
	drawer := ChatCatalog(ChatCatalogProps{
		Width: 260, Height: 600, Key: "history", ShowNew: true, NewLabel: "New Chat", ContentHeight: 576, Theme: theme,
		Items: []ChatCatalogItemProps{{SelectID: "chat", DeleteID: "delete", Kind: "history", Title: "Suzhou", GroupLabel: "Today", Selected: true, OnSelect: func() {}, OnDelete: func() {}}},
	}).(woxwidget.Stack)
	if drawer.Width != 260 || drawer.Height != 600 || len(drawer.Children) != 2 || !drawer.Children[1].AnchorRight {
		t.Fatalf("history drawer = width %.0f, height %.0f, children %+v", drawer.Width, drawer.Height, drawer.Children)
	}
	panel := drawer.Children[0].Child.(woxwidget.Container)
	if panel.Radius != 0 || panel.Padding.Left != 10 || panel.Padding.Top != 12 || panel.Color != theme.ActionBackground {
		t.Fatalf("history panel = radius %.0f, padding %+v, color %#v", panel.Radius, panel.Padding, panel.Color)
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
