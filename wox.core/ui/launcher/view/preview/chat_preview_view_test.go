package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatDebugCopyFeedbackRequiresSuccess(t *testing.T) {
	success := false
	props := ChatHeaderProps{
		Width: 500, Height: 52, Key: "test", ShowDebug: true, DebugCopiedLabel: "Copied",
		OnDebug: func() bool { return success },
	}
	state := &chatHeaderState{}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		header := ChatHeader(props).(woxwidget.Stateful)
		header.CreateState = func() woxwidget.State { return state }
		return header
	})
	host.AttachServices(terminalPreviewHostServices{})
	t.Cleanup(host.Dispose)
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 500, Height: 52}, PixelSize: woxui.PixelSize{Width: 500, Height: 52}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	for _, success = range []bool{false, true} {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Position: woxui.Point{X: 474, Y: 26}, Button: woxui.PointerButtonPrimary})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Position: woxui.Point{X: 474, Y: 26}, Button: woxui.PointerButtonPrimary})
		if state.copied != success || (state.copyReset != nil) != success {
			t.Fatalf("copy success=%v: feedback=%v timer=%v", success, state.copied, state.copyReset != nil)
		}
		header := state.Build(woxwidget.StateContext{}, props).(woxwidget.Container)
		button := header.Child.(woxwidget.Stack).Children[3].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		wantLabel := "Debug trace"
		wantIcon := woxcomponent.DebugGlyph(16, props.Theme.ResultSubtitle).(woxwidget.Image)
		if success {
			wantLabel = "Copied"
			wantIcon = woxcomponent.CheckGlyph(16, props.Theme.ResultSubtitle).(woxwidget.Image)
		}
		if button.Label != wantLabel || button.Icon.(woxwidget.Image).Source != wantIcon.Source {
			t.Fatalf("copy success=%v: label=%q icon=%#v", success, button.Label, button.Icon)
		}
	}
}

// TestFramelessChatTextUsesPreparedWidth prevents a full reflow inside TextBlock on every frame.
func TestFramelessChatTextUsesPreparedWidth(t *testing.T) {
	for _, width := range []float32{320, 560, 1000} {
		layout := woxwidget.TextBlockLayout{ConstraintWidth: width - 8, HasConstraintWidth: true, Size: woxui.Size{Height: 160}, LineHeight: 16}
		props := ChatMessageProps{Role: "assistant", Reasoning: "thinking", ReasoningLayout: layout}
		view := chatMessageContent(props, width, false, nil, nil).(woxwidget.Gesture)
		card := view.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Flex)
		body := card.Children[0].(woxwidget.Container)
		text := body.Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock)
		if text.Width != layout.ConstraintWidth || text.Width != body.Width-body.Padding.Right {
			t.Fatalf("viewport %.0f: prepared width %.0f, rendered %.0f", width, layout.ConstraintWidth, text.Width)
		}
	}
}

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

func TestChatMessageHoverCoversWholeAnswer(t *testing.T) {
	view := chatMessageContent(ChatMessageProps{
		Key: "assistant", Role: "assistant", ShowMeta: true, Text: "hello",
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}},
	}, 1000, false, func(bool) {}, nil).(woxwidget.Gesture)
	stack, ok := view.Child.(woxwidget.Stack)
	if !view.CoverHover || view.ID != "chat-message-hover-assistant" || !ok || stack.Width != 1000 || stack.Height <= 0 {
		t.Fatalf("message hover = cover %v id %q size %.0fx%.0f, want the whole answer", view.CoverHover, view.ID, stack.Width, stack.Height)
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
		Input:    ChatInputProps{Width: 480, Height: ChatComposerHeight(0)},
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
		Input:    ChatInputProps{Width: 520, Height: ChatComposerHeight(0)},
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
		Input:    ChatInputProps{Width: 520, Height: ChatComposerHeight(0)},
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

func TestDedicatedChatPickerFollowsCenteredComposer(t *testing.T) {
	for _, sidebar := range []bool{false, true} {
		props := ChatPreviewProps{
			Dedicated: true, Width: 1400, Height: 600, Key: "dedicated",
			Messages: ChatMessagesProps{Width: 760, Height: 480},
			Input:    ChatInputProps{Width: 760, Height: ChatComposerHeight(0)},
			Catalog:  &ChatCatalogProps{Width: 560, Height: 80, Picker: true},
		}
		if sidebar {
			props.History = &ChatCatalogProps{Width: 240, Height: 600, ShowNew: true}
		}
		var stack woxwidget.Stack
		if sidebar {
			stack = ChatPreview(props).(woxwidget.Flex).Children[1].(woxwidget.Stack)
		} else {
			stack = ChatPreview(props).(woxwidget.Stack)
		}
		want := float32(320)
		if sidebar {
			want = 200
		}
		if got := stack.Children[len(stack.Children)-1].Left; got != want {
			t.Fatalf("sidebar=%v: picker left=%v, want %v", sidebar, got, want)
		}
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

func TestChatCatalogLoadingPanelFitsTitleAndPlaceholder(t *testing.T) {
	catalog := ChatCatalog(ChatCatalogProps{
		Width: 400, Height: 82, Key: "models", Label: "Select Model", ContentHeight: 40,
		Items: []ChatCatalogItemProps{{SelectID: "loading", Kind: "models", Title: "Loading models…", Placeholder: true}},
	}).(woxwidget.Container)
	if catalog.Height != 82 || catalog.Padding.Top+catalog.Padding.Bottom != 14 {
		t.Fatalf("catalog chrome = height %.0f padding %+v", catalog.Height, catalog.Padding)
	}
	body := catalog.Child.(woxwidget.Flex)
	if len(body.Children) != 2 {
		t.Fatalf("catalog children = %d, want title and rows", len(body.Children))
	}
	header := body.Children[0].(woxwidget.Container)
	if header.Height != 28 {
		t.Fatalf("catalog title = %.0f, want 28", header.Height)
	}
	if remaining := catalog.Height - catalog.Padding.Top - catalog.Padding.Bottom - header.Height; remaining != 40 {
		t.Fatalf("rows slot = %.0f, want 40 so the loading placeholder stays inside the panel", remaining)
	}
}

func TestChatHistoryUsesInsetRoundedPanel(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{R: 220, G: 220, B: 220, A: 255}}
	drawer := ChatCatalog(ChatCatalogProps{
		Width: 240, Height: 600, ShowNew: true, ContentHeight: 900, Theme: theme,
	}).(woxwidget.Stack)
	if len(drawer.Children) != 1 || drawer.Children[0].Left != ChatHistoryFloatingInset || drawer.Children[0].Top != ChatHistoryFloatingInset {
		t.Fatalf("floating sidebar position = %+v", drawer.Children)
	}
	panel := drawer.Children[0].Child.(woxwidget.Container)
	if panel.Width != 224 || panel.Height != 576 || panel.Radius != 12 || panel.Color.A == 0 || panel.BorderWidth != 1 || panel.BorderColor.A == 0 {
		t.Fatalf("floating sidebar surface = %+v", panel)
	}
	scroll := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if scroll.Width != 208 || scroll.Height != 552 || scroll.ContentHeight != 900 {
		t.Fatalf("floating sidebar scroll geometry = %+v", scroll)
	}
}

func TestChatHistorySearchStaysAboveScrollAndCanClear(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		query := "roadmap"
		editor := woxwidget.NewTextEditingController(query)
		props := ChatCatalogProps{Width: 240, Height: 600, Key: "history", DensityScale: density, ShowNew: true, NewLabel: "New Chat", ContentHeight: 1000, SearchClearLabel: "Clear search"}
		props.Search = &woxcomponent.TextFieldProps{ID: "chat-history-search", Label: "Search chat titles", Value: query, Controller: editor}
		props.OnSearchClear = func() { query = ""; editor.SetText("", false) }
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { props.Search.Value = query; return ChatCatalog(props) })
		host.AttachServices(terminalPreviewHostServices{})
		frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 600}, Scale: 1}
		host.Frame(&woxui.DisplayList{}, frame)
		var clear woxui.Rect
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "chat-history-search-clear" {
				clear = node.Bounds
			}
		}
		if clear.Width == 0 {
			t.Fatalf("density %v: search clear is missing", density)
		}
		point := woxui.Point{X: clear.X + clear.Width/2, Y: clear.Y + clear.Height/2}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		if query != "" || editor.Text() != "" {
			t.Fatal("clear did not reset the search field")
		}
		drawer := ChatCatalog(props).(woxwidget.Stack)
		panel := drawer.Children[0].Child.(woxwidget.Container)
		content := panel.Child.(woxwidget.Flex)
		if len(content.Children) != 5 {
			t.Fatalf("density %v: search header is not separate from scroll", density)
		}
		scroll := content.Children[4].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		if scroll.Height != panel.Height-2*chatSidebarDrawerPaddingY-ChatHistorySearchHeaderHeight(density) {
			t.Fatalf("density %v: incorrect scroll viewport %v", density, scroll.Height)
		}
		host.Dispose()
	}
}

func TestDedicatedChatSidebarAndComposerShareBottomEdge(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		drawer := ChatCatalog(ChatCatalogProps{Width: 240, Height: 600, DensityScale: scale, ShowNew: true}).(woxwidget.Stack)
		panel := drawer.Children[0].Child.(woxwidget.Container)
		sidebarBottom := drawer.Children[0].Top + panel.Height
		composer := ChatInput(ChatInputProps{Width: 300, DensityScale: scale}).(woxwidget.Container)
		composerBottomInset := chatContentBottomInset + composer.Padding.Bottom
		if sidebarBottom != drawer.Height-composerBottomInset {
			t.Fatalf("density %v: sidebar bottom %v, composer bottom %v", scale, sidebarBottom, drawer.Height-composerBottomInset)
		}
	}
}

func TestChatHistoryGroupLabelIsVerticallyCentered(t *testing.T) {
	drawer := ChatCatalog(ChatCatalogProps{
		Width: 260, Height: 600, Key: "history", ShowNew: true, NewLabel: "New Chat", ContentHeight: 800,
		Items: []ChatCatalogItemProps{{SelectID: "chat", Kind: "history", Title: "Suzhou", GroupLabel: "Today"}},
	}).(woxwidget.Stack)
	panel := drawer.Children[0].Child.(woxwidget.Container)
	scroll := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	group := scroll.Content.(woxwidget.Flex).Children[2].(woxwidget.Container)
	align := group.Child.(woxwidget.Align)
	label := align.Child.(woxwidget.TextBlock)
	if group.Padding.Top != 0 || align.Vertical != 0.5 || align.Height != group.Height || label.AlignmentY != 0.5 || label.Value != "Today" {
		t.Fatalf("history group label is not vertically centered: padding %+v align %+v", group.Padding, align)
	}
}

func TestChatHistoryEmptyMessageIsVerticallyCentered(t *testing.T) {
	drawer := ChatCatalog(ChatCatalogProps{
		Width: 260, Height: 600, Key: "history", ShowNew: true, NewLabel: "New Chat", EmptyMessage: "No conversations",
	}).(woxwidget.Stack)
	panel := drawer.Children[0].Child.(woxwidget.Container)
	scroll := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	empty := scroll.Content.(woxwidget.Flex).Children[2].(woxwidget.Container)
	align := empty.Child.(woxwidget.Align)
	label := align.Child.(woxwidget.TextBlock)
	if empty.Padding.Top != 0 || empty.Height != ChatHistoryRowHeight || align.Vertical != 0.5 || align.Height != empty.Height || label.AlignmentY != 0.5 || label.Value != "No conversations" {
		t.Fatalf("empty history message is not centered in the row slot: padding %+v align %+v", empty.Padding, align)
	}
}

func TestChatHistorySectionSpacing(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		drawer := ChatCatalog(ChatCatalogProps{
			Width: 260, Height: 600, Key: "history", ShowNew: true, DensityScale: scale,
			Items: []ChatCatalogItemProps{
				{SelectID: "projects", Kind: "projects-section"},
				{SelectID: "project", Kind: "project-group"},
				{SelectID: "recents", Kind: "projects-section", SpacingBefore: ChatHistorySectionGap},
			},
		}).(woxwidget.Stack)
		scroll := drawer.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		rows := scroll.Content.(woxwidget.Flex).Children
		gap := rows[4].(woxwidget.Container)
		if gap.Height != ChatScaledSize(scale, ChatHistorySectionGap) || rows[5].(woxwidget.Stateful).Widget.(chatCatalogItemWidget).item.SelectID != "recents" {
			t.Fatalf("density %v: recents spacing = %v", scale, gap.Height)
		}
	}
}

func TestIndentedChatHistoryHighlightMatchesProjectWidth(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		theme := woxcomponent.Theme{SelectedBackground: woxui.Color{R: 30, G: 40, B: 50, A: 255}}
		drawer := ChatCatalog(ChatCatalogProps{
			Width: 260, Height: 600, Key: "history", ShowNew: true, DensityScale: scale, Theme: theme,
			Items: []ChatCatalogItemProps{
				{SelectID: "project", Kind: "project-group", Title: "Project", OnSelect: func() {}},
				{SelectID: "chat", Kind: "history", Title: "Chat", Indented: true, Selected: true, OnSelect: func() {}},
			},
		}).(woxwidget.Stack)
		scroll := drawer.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		rows := scroll.Content.(woxwidget.Flex).Children
		project := rows[2].(woxwidget.Stateful).Widget.(chatCatalogItemWidget)
		chat := rows[3].(woxwidget.Stateful).Widget.(chatCatalogItemWidget)
		if !scroll.ReserveScrollbarSpace || chat.width != scroll.ContentViewportWidth() || chat.width >= scroll.Width {
			t.Fatalf("density %v: history width %v overlaps scrollbar viewport %v", scale, chat.width, scroll.Width)
		}
		if chat.width != project.width {
			t.Fatalf("density %v: chat width %v differs from project width %v", scale, chat.width, project.width)
		}
		for _, hovered := range []bool{false, true} {
			chat.item.Selected = !hovered
			view := chatHistoryItem(chat.item, chat.width, chat.height, theme, hovered, nil).(woxwidget.Container)
			row := chatHistoryTestRow(chatHistoryTestStack(view).Children[0].Child)
			if row.Width != project.width || row.Color != theme.SelectedBackground || row.Padding.Left != ChatScaledSize(scale, chatSidebarRowInset)+ChatScaledSize(scale, 20) {
				t.Fatalf("density %v hovered %v: row width %v color %v inset %v", scale, hovered, row.Width, row.Color, row.Padding.Left)
			}
		}
	}
}

func TestChatHistoryDeleteScalesWithDensity(t *testing.T) {
	const scale = float32(1.1)
	theme := woxcomponent.Theme{ErrorText: woxui.Color{R: 210, G: 45, B: 55, A: 255}, SelectedTitle: woxui.Color{A: 255}}
	item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Suzhou", DensityScale: scale, OnSelect: func() {}, OnDelete: func() {}}
	height := ChatScaledSize(scale, ChatHistoryRowHeight)
	view := chatHistoryItemWithDeleteState(item, 240, height, theme, true, true, false, nil, nil, func() {}).(woxwidget.Container)
	stack := chatHistoryTestStack(view)
	row := chatHistoryTestRow(stack.Children[0].Child)
	button := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	glyph := button.Icon.(woxwidget.Image)
	wantGlyph := woxcomponent.DeleteGlyph(ChatScaledSize(scale, chatSidebarActionGlyph), theme.ErrorText).(woxwidget.Image)
	if view.Height != height || view.Padding.Bottom != 0 || stack.Height != height || row.Height != height || button.Width != ChatScaledSize(scale, chatSidebarActionSize) || button.Height != button.Width || glyph.Source != wantGlyph.Source {
		t.Fatalf("scaled history row = height %.0f button %.0fx%.0f, want the slot and a density-scaled delete control", view.Height, button.Width, button.Height)
	}
}

// TestChatHistoryHoverCoversDelete checks the row and its overlapping action as one hover target.
func TestChatHistoryHoverCoversDelete(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "row", Kind: "history", Title: "Chat", DeleteID: "delete", OnSelect: func() {}, OnDelete: func() {}}
	var hovered, deleteHovered bool
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return chatHistoryItemWithDeleteState(item, 260, ChatHistoryRowHeight, woxcomponent.Theme{}, hovered, deleteHovered, false,
			func(inside bool) { hovered = inside }, func(inside bool) { deleteHovered = inside }, item.OnDelete)
	})
	host.AttachServices(terminalPreviewHostServices{})
	defer host.Dispose()
	paint := func() {
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 260, Height: 100}, Scale: 1})
	}
	paint()
	for _, x := range []float32{245, 60, 245, 60} {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: x, Y: 16}})
		paint()
		if !hovered {
			t.Fatalf("pointer at %v did not highlight the whole history row", x)
		}
		if deleteHovered != (x == 245) {
			t.Fatalf("pointer at %v: delete hover = %v", x, deleteHovered)
		}
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave})
	paint()
	if hovered {
		t.Fatal("history row remains hovered after the pointer leaves")
	}
}

func chatHistoryTestStack(view woxwidget.Container) woxwidget.Stack {
	return view.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
}

func TestChatHistoryStreamingIndicatorKeepsDeleteAvailable(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Chat", Streaming: true, OnDelete: func() {}}
	view := chatHistoryItem(item, 260, ChatHistoryRowHeight, woxcomponent.Theme{}, true, nil).(woxwidget.Container)
	stack := chatHistoryTestStack(view)
	if len(stack.Children) != 3 || stack.Children[1].Right <= stack.Children[2].Right {
		t.Fatal("stream indicator and delete action must have separate trailing slots")
	}
	button := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if button.Icon == nil || button.OnTap == nil {
		t.Fatal("streaming history must keep its delete action available on hover")
	}
	indicator := stack.Children[2].Child.(woxwidget.Align).Child.(woxwidget.LoopAnimation)
	if indicator.Duration <= 0 {
		t.Fatal("streaming history indicator must animate")
	}
	item.Streaming = false
	idle := chatHistoryTestStack(chatHistoryItem(item, 260, ChatHistoryRowHeight, woxcomponent.Theme{}, false, nil).(woxwidget.Container))
	if len(idle.Children) != 2 {
		t.Fatal("finished chat still shows an animation")
	}
}

func TestChatHistoryItemOmitsBubbleIcon(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "row", Kind: "history", Title: "Suzhou", DeleteID: "delete", OnSelect: func() {}, OnDelete: func() {}}
	view := chatHistoryItem(item, 260, 46, woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}, false, func(bool) {}).(woxwidget.Container)
	stack := chatHistoryTestStack(view)
	row := chatHistoryTestRow(stack.Children[0].Child)
	title, ok := row.Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if !ok || title.AlignmentY != 0.5 || row.Child.(woxwidget.Align).Vertical != 0.5 || row.Padding.Left != 12 {
		t.Fatal("history row should contain an indented, vertically centered title without a bubble icon")
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
	semantics := chatHistoryTestStack(row).Children[0].Child.(woxwidget.Gesture).Child.(woxwidget.Semantics)
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
	stack := chatHistoryTestStack(view)
	row := chatHistoryTestRow(stack.Children[0].Child)
	title := row.Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if row.Color != theme.SelectedBackground || title.Color != theme.SelectedTitle || title.AlignmentY != 0.5 {
		t.Fatalf("hovered history row = background %#v title %#v, want selected palette", row.Color, title.Color)
	}
	delete := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	deleteGlyph := delete.Icon.(woxwidget.Image)
	expectedGlyph := woxcomponent.DeleteGlyph(chatSidebarActionGlyph, theme.ErrorText).(woxwidget.Image)
	idleGlyph := woxcomponent.DeleteGlyph(chatSidebarActionGlyph, theme.PreviewText).(woxwidget.Image)
	if deleteGlyph.Source != expectedGlyph.Source || delete.IdleIcon.(woxwidget.Image).Source != idleGlyph.Source {
		t.Fatal("history delete icon should turn red on button hover or focus")
	}

	newChat := chatHistoryItem(ChatCatalogItemProps{SelectID: "new", Kind: "history-new", Title: "New Chat"}, 260, 38, theme, true, func(bool) {}).(woxwidget.Gesture)
	newChatRow := chatHistoryTestRow(newChat)
	newChatContent := newChatRow.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	newChatTitle := newChatContent.Children[1].(woxwidget.TextBlock)
	if newChatRow.Child.(woxwidget.Align).Vertical != 0.5 || newChatContent.CrossAxisAlignment != woxwidget.CrossAxisCenter || newChatTitle.AlignmentY != 0.5 || newChatRow.Color != theme.SelectedBackground || newChatTitle.Color != theme.SelectedTitle {
		t.Fatal("hovered new-chat row should center its icon and title on the selected row")
	}
}

func TestChatHistoryDeleteUsesDangerHoverAndConfirmation(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultSubtitle: woxui.Color{R: 80, G: 80, B: 80, A: 255}, ErrorText: woxui.Color{R: 210, G: 45, B: 55, A: 255},
		SelectedTitle: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Cursor: woxui.Color{R: 30, G: 110, B: 220, A: 255},
	}
	item := ChatCatalogItemProps{SelectID: "row", DeleteID: "delete", Kind: "history", Title: "Suzhou", DeleteLabel: "Delete", ConfirmDeleteLabel: "Confirm delete", OnSelect: func() {}, OnDelete: func() {}}
	deleteControl := func(deleteHovered, deleteConfirm bool) woxwidget.Align {
		view := chatHistoryItemWithDeleteState(item, 240, 46, theme, true, deleteHovered, deleteConfirm, func(bool) {}, func(bool) {}, func() {}).(woxwidget.Container)
		stack := chatHistoryTestStack(view)
		return stack.Children[1].Child.(woxwidget.Align)
	}

	hovered := deleteControl(true, false).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	hoveredGlyph := hovered.Icon.(woxwidget.Image)
	expectedHoverGlyph := woxcomponent.DeleteGlyph(chatSidebarActionGlyph, theme.ErrorText).(woxwidget.Image)
	if hoveredGlyph.Source != expectedHoverGlyph.Source || hovered.HoverBackground.A != 0 || hovered.Background.A != 0 {
		t.Fatal("delete hover should use a red icon without a button background")
	}

	confirm := deleteControl(true, true)
	confirmButton := confirm.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	confirmGlyph := confirmButton.Icon.(woxwidget.Image)
	expectedConfirmGlyph := woxcomponent.CheckGlyph(chatSidebarActionGlyph, theme.ErrorText).(woxwidget.Image)
	if confirm.Width != chatSidebarActionSize || confirmButton.Width != chatSidebarActionSize || confirmButton.Height != chatSidebarActionSize || confirmButton.Label != item.ConfirmDeleteLabel || confirmButton.Background.A != 0 || confirmButton.HoverBackground.A != 0 || confirmGlyph.Source != expectedConfirmGlyph.Source {
		t.Fatalf("delete confirmation = width %.0f label %q background %#v, want red confirmation icon without background", confirm.Width, confirmButton.Label, confirmButton.Background)
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

func TestChatCatalogItemShowsPluginIconAndName(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}
	row := chatCatalogItem(ChatCatalogItemProps{SelectID: "notes", Kind: "plugin", Title: "Notes"}, 400, 38, theme, false, func(bool) {}).(woxwidget.Gesture)
	aligned := row.Child.(woxwidget.Container).Child.(woxwidget.Align)
	content := aligned.Child.(woxwidget.Flex)
	title := content.Children[1].(woxwidget.TextBlock)
	subtitle := content.Children[2].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if aligned.Vertical != 0.5 || title.Value != "Notes" || title.AlignmentY != 0.5 || subtitle.Value != "" {
		t.Fatalf("plugin row = title %#v subtitle %q", title, subtitle.Value)
	}
}

func TestChatCatalogItemOnlyShowsCheckForCurrentModel(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}

	notCurrent := chatCatalogItem(ChatCatalogItemProps{SelectID: "flash", Kind: "models", Title: "flash", Selected: true}, 400, 38, theme, false, func(bool) {}).(woxwidget.Gesture)
	notCurrentRow := notCurrent.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(notCurrentRow.Children) != 3 {
		t.Fatalf("non-current model children = %d, want icon, title, and subtitle", len(notCurrentRow.Children))
	}

	current := chatCatalogItem(ChatCatalogItemProps{SelectID: "flash", Kind: "models", Title: "flash", Current: true}, 400, 38, theme, false, func(bool) {}).(woxwidget.Gesture)
	currentRow := current.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(currentRow.Children) != 4 || currentRow.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("current model children = %d, want a centered check", len(currentRow.Children))
	}
}

func TestChatMessageRendersMentionChipsInline(t *testing.T) {
	tag := "{plugin:notes}"
	end := len([]rune(tag))
	theme := woxcomponent.Theme{
		SelectedBackground: woxui.Color{A: 255},
		SelectedTitle:      woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Controls:           woxcomponent.ControlTheme{Text: woxui.Color{A: 255}, TextSecondary: woxui.Color{A: 200}},
	}
	run := woxcomponent.NewTokenChipRunWithIcon(0, end, "@笔记", nil, nil, theme.Controls)
	text := tag + " 现在有几个笔记?"
	props := ChatMessageProps{
		Key: "user-mention", Role: "user", Text: text, RichRuns: []woxcomponent.TextFieldRichRun{run},
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 160, Height: 20}}, ContentWidth: 160, Theme: theme,
	}
	view := chatMessageContent(props, 1000, false, nil, nil).(woxwidget.Gesture)
	body := view.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	children := body.Child.(woxwidget.Flex).Children
	if len(children) != 1 {
		t.Fatalf("user mention body children = %d, want only inline chips (no skills footer)", len(children))
	}
	field := children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if !field.Disabled || !field.Transparent || !field.ReadOnly || field.Value != text {
		t.Fatalf("mention field = disabled %v transparent %v readOnly %v value %q", field.Disabled, field.Transparent, field.ReadOnly, field.Value)
	}
	if len(field.RichRuns) != 1 || field.RichRuns[0].ChipLabel != "@笔记" || field.RichRuns[0].Paint == nil {
		t.Fatalf("mention chips = %#v", field.RichRuns)
	}
	if field.TextColor != theme.SelectedTitle {
		t.Fatalf("mention text color = %#v, want selected title", field.TextColor)
	}
}

func TestMeasureChatMessageUsesChipAdvanceForUserWidth(t *testing.T) {
	tag := "{plugin:notes}"
	run := woxcomponent.NewTokenChipRun(0, len([]rune(tag)), "@Notes", nil, woxcomponent.ControlTheme{})
	props := MeasureChatMessage(ChatMessageProps{
		Role: "user", Text: tag + " hi", RichRuns: []woxcomponent.TextFieldRichRun{run},
	}, nil, 400, nil)
	if props.TextLayout.Size.Height != chatMessageLineHeight+1 {
		t.Fatalf("chip message height = %.0f, want %.0f", props.TextLayout.Size.Height, chatMessageLineHeight+1)
	}
	if props.ContentWidth < run.Advance {
		t.Fatalf("content width = %.0f, want at least chip advance %.0f", props.ContentWidth, run.Advance)
	}
}

func TestChatMessageHeightOmitsSkillsFooterWhenChipsInline(t *testing.T) {
	withChips := ChatMessageProps{Role: "user", Text: "{plugin:notes} hi", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 20}}}
	withFooter := ChatMessageProps{Role: "user", Text: "hi", Skills: "@Notes", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 20}}}
	if diff := chatMessageHeight(withFooter) - chatMessageHeight(withChips); diff != 21 {
		t.Fatalf("skills footer extra height = %.0f, want 21", diff)
	}
}

func TestChatInputRendersSkillTagsAsAtomicChips(t *testing.T) {
	theme := woxcomponent.Theme{ResultTitle: woxui.Color{A: 255}, ResultSubtitle: woxui.Color{A: 200}, QueryBackground: woxui.Color{A: 255}}
	tag := "{skill:wox-plugin-creator}"
	end := len([]rune(tag))
	run := woxcomponent.NewTokenChipRun(0, end, "wox-plugin-creator", nil, theme.Controls)
	input := ChatInput(ChatInputProps{
		Width: 400, Height: ChatComposerHeight(0), Key: "skills", Editing: woxui.TextEditingState{Text: tag + " 士大夫"},
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

func TestChatInputUsesSharedSurfaceAndFocusTreatment(t *testing.T) {
	for _, color := range []woxui.Color{{R: 220, G: 220, B: 220, A: 255}, {R: 30, G: 30, B: 30, A: 255}} {
		theme := woxcomponent.Theme{ResultSubtitle: color, Cursor: woxui.Color{R: 70, G: 140, B: 220, A: 255}}
		var borders [2]woxui.Color
		for i, focused := range []bool{false, true} {
			input := ChatInput(ChatInputProps{Width: 400, Focused: focused, Theme: theme}).(woxwidget.Container)
			card := input.Child.(woxwidget.Container)
			field := card.Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
			if card.Radius != 12 || card.Color.A == 0 || card.BorderWidth != 1 || !field.Transparent || !field.DisableHover {
				t.Fatalf("focused=%v: composer=%+v, field=%+v", focused, card, field)
			}
			borders[i] = card.BorderColor
		}
		if borders[0].A == 0 || borders[1].A <= borders[0].A || borders[1].R != theme.Cursor.R {
			t.Fatalf("focus border = %#v -> %#v", borders[0], borders[1])
		}
	}
}

func TestChatInputWiresOnPasteToTextField(t *testing.T) {
	pasted := false
	theme := woxcomponent.Theme{QueryBackground: woxui.Color{A: 255}}
	input := ChatInput(ChatInputProps{
		Width: 400, Height: ChatComposerHeight(0), Key: "paste", Theme: theme,
		OnPaste: func(string) bool {
			pasted = true
			return true
		},
		Importing: true,
	}).(woxwidget.Container)
	field := input.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if field.OnPaste == nil || !field.OnPaste("ignored") || !pasted {
		t.Fatal("chat composer OnPaste must reach the shared text field")
	}
}

func TestChatReadingTextFollowsInterfaceSize(t *testing.T) {
	if ChatScaledSize(0, 13) != 13 || ChatScaledSize(1, 13) != 13 || ChatMessageFontSize(1) != 13 || ChatMessageLineHeight(1) != 19 {
		t.Fatal("normal interface size must keep authored chat text")
	}
	if ChatMessageFontSize(1.1) != 14 || ChatMessageLineHeight(1.1) != 21 || ChatScaledSize(1.1, 20) != 22 {
		t.Fatalf("comfortable chat text = %v/%v/%v", ChatMessageFontSize(1.1), ChatMessageLineHeight(1.1), ChatScaledSize(1.1, 20))
	}
	if ChatMessageFontSize(0.9) != 12 || ChatMessageLineHeight(0.9) != 17 {
		t.Fatalf("compact chat text = %v/%v", ChatMessageFontSize(0.9), ChatMessageLineHeight(0.9))
	}
	theme := woxcomponent.Theme{QueryBackground: woxui.Color{R: 30, G: 30, B: 30, A: 255}, ResultTitle: woxui.Color{A: 255}}
	input := ChatInput(ChatInputProps{Width: 400, DensityScale: 1.1, Key: "scaled", Theme: theme}).(woxwidget.Container)
	field := input.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.Height != 94 || field.Style.Size != 14 || field.LineHeight != 22 || field.Height != 37 {
		t.Fatalf("comfortable composer = pane %.0f font %.0f line %.0f editor %.0f", input.Height, field.Style.Size, field.LineHeight, field.Height)
	}
}

func TestChatInputShowsQuoteCardAboveComposer(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewText:     woxui.Color{R: 220, G: 225, B: 230, A: 255},
		ResultSubtitle:  woxui.Color{R: 180, G: 180, B: 180, A: 200},
		QueryBackground: woxui.Color{R: 30, G: 30, B: 30, A: 255},
	}
	if ChatComposerHeight(0) != 92 || ChatComposerHeight(1) != 148 {
		t.Fatalf("composer height = %.0f/%.0f", ChatComposerHeight(0), ChatComposerHeight(1))
	}

	plain := ChatInput(ChatInputProps{Width: 400, Height: ChatComposerHeight(0), Key: "plain", Theme: theme}).(woxwidget.Container)
	plainField := plain.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if plainField.MaxLines != 5 || plainField.Height != 35 || plainField.LineHeight != 20 {
		t.Fatalf("composer editor = lines %d height %.0f line height %.0f, want a 1-line 35-unit field that can scroll at 5", plainField.MaxLines, plainField.Height, plainField.LineHeight)
	}
	plainCard := plain.Child.(woxwidget.Container)
	if len(plainCard.Child.(woxwidget.Flex).Children) != 3 {
		t.Fatalf("plain composer children = %d, want editor, gap, toolbar", len(plainCard.Child.(woxwidget.Flex).Children))
	}

	quoted := ChatInput(ChatInputProps{
		Width: 400, Height: ChatComposerHeight(1), Key: "quoted", Attachments: []ChatAttachmentProps{{ID: "quoted", Label: "Quote", Text: "selected text"}},
		QuoteDismissLabel: "Remove quote", Theme: theme, OnDismissAttachment: func(string) {},
	}).(woxwidget.Container)
	quotedCard := quoted.Child.(woxwidget.Container)
	children := quotedCard.Child.(woxwidget.Flex).Children
	if len(children) != 4 {
		t.Fatalf("quoted composer children = %d, want quote, editor, gap, toolbar", len(children))
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

func TestChatComposerGrowsFromOneToFiveLinesThenScrolls(t *testing.T) {
	if ChatComposerVisibleLines("", 400, nil, nil) != 1 || ChatComposerVisibleLines("a\nb", 400, nil, nil) != 2 {
		t.Fatal("composer should start at one visible line and grow with hard breaks")
	}
	if ChatComposerVisibleLines("1\n2\n3\n4\n5\n6", 400, nil, nil) != 5 {
		t.Fatal("composer should stop growing after five visible lines")
	}
	if ChatComposerHeightForLines(0, 1) != 92 || ChatComposerHeightForLines(0, 2) != 112 || ChatComposerHeightForLines(0, 5) != 172 {
		t.Fatalf("composer heights = %.0f/%.0f/%.0f, want 92/112/172", ChatComposerHeightForLines(0, 1), ChatComposerHeightForLines(0, 2), ChatComposerHeightForLines(0, 5))
	}

	two := ChatInput(ChatInputProps{Width: 400, Editing: woxui.TextEditingState{Text: "a\nb"}}).(woxwidget.Container)
	twoField := two.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if two.Height != 112 || twoField.Height != 55 || twoField.MaxLines != 5 {
		t.Fatalf("two-line composer = pane %.0f editor %.0f max %d", two.Height, twoField.Height, twoField.MaxLines)
	}

	six := ChatInput(ChatInputProps{
		Width: 400, Editing: woxui.TextEditingState{Text: "1\n2\n3\n4\n5\n6"},
		Theme: woxcomponent.Theme{ResultTitle: woxui.Color{R: 220, G: 225, B: 230, A: 255}},
	}).(woxwidget.Container)
	scroll, ok := six.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful)
	if !ok {
		t.Fatal("overflowing composer should use the shared WoxScrollView")
	}
	scrollProps := scroll.Widget.(woxcomponent.ScrollViewProps)
	sixField := scrollProps.Content.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if six.Height != 172 || scrollProps.Height != 115 || scrollProps.ContentHeight != 135 || !scrollProps.AlwaysShowScrollbar {
		t.Fatalf("overflow scroll = pane %.0f viewport %.0f content %.0f always %v", six.Height, scrollProps.Height, scrollProps.ContentHeight, scrollProps.AlwaysShowScrollbar)
	}
	if sixField.Height != 135 || sixField.MaxLines != 6 || scrollProps.ThumbColor.A == 0 || scrollProps.KeepVisible == nil {
		t.Fatalf("overflow field = height %.0f max %d thumb %#v keep %#v", sixField.Height, sixField.MaxLines, scrollProps.ThumbColor, scrollProps.KeepVisible)
	}
}

func TestSentChatQuotePreservesLinesAndMessageHeight(t *testing.T) {
	attachment := ChatAttachmentProps{ID: "quote", Label: "Quote", Text: "  first line\nsecond line", Layout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 36}}}
	for _, width := range []float32{180, 600} {
		quote := chatQuoteCard(attachment, width, woxcomponent.Theme{}, "", nil, true, 0).(woxwidget.Semantics)
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
	if ChatComposerHeight(2) != 204 {
		t.Fatal("composer must reserve space for every attachment")
	}
}

func TestChatFileAndImageAttachmentsUseCompactTiles(t *testing.T) {
	theme := woxcomponent.Theme{
		ResultSubtitle:  woxui.Color{R: 180, G: 180, B: 180, A: 200},
		QueryBackground: woxui.Color{R: 30, G: 30, B: 30, A: 255},
		Cursor:          woxui.Color{R: 30, G: 110, B: 220, A: 255},
	}
	file := ChatAttachmentProps{ID: "file", Kind: "file", Label: "a very long attachment name", Text: "/original/path", Image: &woxui.Image{Width: 24, Height: 24}}
	folder := ChatAttachmentProps{ID: "folder", Kind: "folder", Label: "project", Text: "/original/dir", Image: &woxui.Image{Width: 24, Height: 24}}
	image := ChatAttachmentProps{ID: "image", Kind: "image", Label: "snapshot.png", Text: "Image", Image: &woxui.Image{Width: 400, Height: 100}}
	for _, sent := range []bool{false, true} {
		fileView := chatAttachmentCard(file, 400, theme, "Remove", nil, sent, 0).(woxwidget.Semantics)
		fileTile := fileView.Child.(woxwidget.Stack)
		if fileTile.Width != chatAttachmentTileSize || fileTile.Height != chatAttachmentTileSize || fileView.Label != file.Label+": "+file.Text {
			t.Fatalf("file tile = %#v", fileView)
		}
		fileIcon := fileTile.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Image)
		if fileIcon.Fit != woxwidget.ImageFitContain || fileIcon.Width != chatAttachmentTileIconSize {
			t.Fatal("file icons must stay contained inside the tile")
		}
		folderView := chatAttachmentCard(folder, 400, theme, "Remove", nil, sent, 0).(woxwidget.Semantics)
		if folderView.Child.(woxwidget.Stack).Width != chatAttachmentTileSize || folderView.Label != folder.Label+": "+folder.Text {
			t.Fatalf("folder tile = %#v", folderView)
		}

		imageView := chatAttachmentCard(image, 180, theme, "Remove", nil, sent, 0).(woxwidget.Semantics)
		thumbnail := imageView.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Image)
		if thumbnail.Fit != woxwidget.ImageFitCover || thumbnail.Width != chatAttachmentTileSize || thumbnail.Height != chatAttachmentTileSize {
			t.Fatalf("image tile = %#v", thumbnail)
		}
	}
	dismissed := chatAttachmentCard(file, 400, theme, "Remove file", func() {}, false, 0).(woxwidget.Semantics)
	dismiss := dismissed.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if dismiss.ID != "chat-attachment-dismiss-file" || dismiss.Width != chatAttachmentTileDismissSize || dismiss.Background != chatAttachmentTileDismissBackground(theme) || dismiss.HoverBackground != chatIconHoverBackground(theme) || dismiss.HoverBackground.A == 0 {
		t.Fatalf("attachment dismiss = %#v, want compact overlay hover", dismiss)
	}

	files := []ChatAttachmentProps{file, file, file, file, file}
	if ChatComposerHeightForAttachments(files, 1) != ChatComposerHeightForAttachments(files[:1], 1) {
		t.Fatal("file tiles must share one composer row")
	}
	if ChatComposerHeight(100) != ChatComposerHeight(3) {
		t.Fatal("large quote selections must leave the editor visible")
	}
	input := ChatInput(ChatInputProps{Width: 200, Height: ChatComposerHeightForAttachments(files, 1), Key: "files", Attachments: files, Theme: theme}).(woxwidget.Container)
	card := input.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(card.Children) != 4 {
		t.Fatal("many file tiles should use one strip above the editor")
	}
	strip := card.Children[0].(woxwidget.Container)
	if strip.Height != chatComposerAttachmentStripHeight {
		t.Fatalf("tile strip height = %.0f, want %.0f", strip.Height, chatComposerAttachmentStripHeight)
	}
	if _, ok := strip.Child.(woxwidget.Stateful); !ok {
		t.Fatal("overflowing tiles must own retained horizontal scroll state")
	}

	quotes := make([]ChatAttachmentProps, 5)
	quoted := ChatInput(ChatInputProps{Width: 400, Height: ChatComposerHeight(5), Attachments: quotes}).(woxwidget.Container)
	quotedCard := quoted.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if _, ok := quotedCard.Children[0].(woxwidget.Stateful); !ok {
		t.Fatal("many quotes should keep one vertical scroll pane")
	}

	plain := ChatMessageProps{Role: "user", Text: "Explain", TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 19}}}
	oneTile := plain
	oneTile.Attachments = []ChatAttachmentProps{file}
	twoTiles := plain
	twoTiles.Attachments = []ChatAttachmentProps{file, image}
	if chatMessageHeight(oneTile)-chatMessageHeight(plain) != chatAttachmentTileSize+3 {
		t.Fatal("sent file tiles must use the compact square height")
	}
	if chatMessageHeight(twoTiles) != chatMessageHeight(oneTile) {
		t.Fatal("multiple sent file tiles must share one row")
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
			stack := chatHistoryTestStack(view)
			button := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
			if (button.Icon != nil) != state.visible || button.Disabled || button.OnTap == nil {
				t.Fatalf("delete visibility/accessibility = %+v, want visible %v and enabled", button, state.visible)
			}
			row := chatHistoryTestRow(stack.Children[0].Child)
			title := row.Child.(woxwidget.Align)
			label := title.Child.(woxwidget.TextBlock)
			if view.Height != ChatHistoryRowHeight || view.Padding.Bottom != 0 || row.Height != view.Height || title.Height != row.Height || title.Vertical != 0.5 || label.AlignmentY != 0.5 || label.Style.Weight == woxui.FontWeightSemibold {
				t.Fatal("history title must be regular and centered in the full row slot")
			}
		})
	}
}
