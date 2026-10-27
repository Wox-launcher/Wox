package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatProjectToolbarWrapsAndMatchesConversationLayout(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		for _, width := range []float32{320, 760} {
			props := ChatInputProps{Key: "project", Width: width, DensityScale: scale, ProjectVisible: true, ProjectLabel: "A long project", Model: "model"}
			container := ChatInput(props).(woxwidget.Container)
			conversation := prepareChatConversation(ChatConversationProps{ChatPreviewProps: ChatPreviewProps{
				Width: width + 20, Height: 600, Input: props,
			}})
			if container.Height != conversation.Input.Height {
				t.Fatalf("density %v width %v: measured %.0f rendered %.0f", scale, width, conversation.Input.Height, container.Height)
			}
			if width == 320 && ChatProjectToolbarExtra(props, width) == 0 {
				t.Fatal("narrow project selector did not wrap")
			}
			if width == 760 && ChatProjectToolbarExtra(props, width) != 0 {
				t.Fatal("wide project selector wrapped")
			}
		}
	}
}

func TestChatProjectSelectorDisablesDuringStreaming(t *testing.T) {
	props := ChatInputProps{Key: "project", ProjectLabel: "No project", Sending: true, OnProjects: func() {}}
	selector := ChatProjectSelector(props, 150).(woxwidget.Stateful)
	state := chatModelSelectorState{}
	button := state.Build(woxwidget.StateContext{}, selector.Widget).(woxwidget.Semantics)
	if !button.Disabled || button.Label != "No project" {
		t.Fatalf("%+v", button)
	}
}

func TestChatProjectDialogKeepsDeleteSeparate(t *testing.T) {
	dialog := ChatProjectDialog(ChatProjectDialogProps{Width: 600, Height: 600}).(woxwidget.Stateful).Widget.(woxcomponent.DialogProps)
	body := dialog.Child.(woxwidget.Flex)
	actions := body.Children[len(body.Children)-1].(woxwidget.Align).Child.(woxwidget.Flex)
	if len(actions.Children) != 2 || dialog.InitialFocus != "chat-project-name" {
		t.Fatal("editor must contain only cancel and save actions")
	}
	confirm := ChatProjectDeleteDialog(ChatProjectDeleteDialogProps{Width: 600, Height: 600}).(woxwidget.Stateful).Widget.(woxcomponent.DialogProps)
	if confirm.InitialFocus != "chat-project-delete-cancel" {
		t.Fatal("delete confirmation must focus cancel initially")
	}
}

func TestChatProjectDirectoriesAlignActionsInOneRow(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		theme := woxcomponent.ControlTheme{DensityScale: scale, Error: woxui.Color{R: 200, A: 255}}
		dialog := ChatProjectDialog(ChatProjectDialogProps{
			Width: 800, Height: 600, Theme: theme, RemoveLabel: "Remove",
			Rows: []ChatProjectRow{{ID: "directory", Label: "A long path", Selected: true, OnSelect: func() {}, OnRemove: func() {}}},
		}).(woxwidget.Stateful).Widget.(woxcomponent.DialogProps)
		body := dialog.Child.(woxwidget.Flex)
		scroll := body.Children[3].(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		row := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
		entry := row.Children[0].Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child
		if retained, ok := entry.(woxwidget.Stateful); ok {
			entry = retained.CreateState().Build(woxwidget.StateContext{}, retained.Widget)
		}
		container := entry.(woxwidget.Gesture).Child.(woxwidget.Container)
		alignment := container.Child.(woxwidget.Align)
		content := alignment.Child.(woxwidget.Flex)
		removeAlign := row.Children[1].Child.(woxwidget.Align)
		remove := removeAlign.Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if alignment.Vertical != 0.5 || content.CrossAxisAlignment != woxwidget.CrossAxisCenter || removeAlign.Vertical != 0.5 || remove.Height != theme.Scaled(28) || remove.OnTap == nil || remove.HoverBackground.A != 0 {
			t.Fatalf("density %v: directory content and remove control are not centered in one row", scale)
		}
		if remove.Icon.(woxwidget.Image).Source != woxcomponent.DeleteGlyph(theme.Scaled(16), theme.Error).(woxwidget.Image).Source {
			t.Fatal("directory remove should use a red icon on hover")
		}
	}
}

// TestChatProjectActionsAppearOnHoverOrFocus keeps quiet rows operable with either input method.
func TestChatProjectActionsAppearOnHoverOrFocus(t *testing.T) {
	for _, kind := range []string{"project-group", "projects-section"} {
		for _, state := range []struct {
			hovered, focused bool
		}{{}, {hovered: true}, {focused: true}} {
			item := ChatCatalogItemProps{SelectID: "project", Kind: kind, Title: "Project", OnNew: func() {}}
			if kind == "project-group" {
				item.OnEdit = func() {}
				item.OnDelete = func() {}
			}
			view := chatProjectHistoryGroup(item, 240, woxcomponent.Theme{}, state.hovered, state.focused, state.focused, state.focused, nil, nil, nil, nil).(woxwidget.Semantics).Child.(woxwidget.Gesture)
			if !view.CoverHover || view.ID == "" {
				t.Fatal("row hover must survive pointer movement over nested buttons")
			}
			row := view.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
			aligned := row.Child.(woxwidget.Align)
			labelRow := aligned.Child.(woxwidget.Flex)
			if aligned.Vertical != 0.5 || aligned.Height != ChatHistoryRowHeight || labelRow.CrossAxisAlignment != woxwidget.CrossAxisCenter {
				t.Fatalf("kind %s: project row is not vertically centered: %+v", kind, aligned)
			}
			actions := view.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Align).Child.(woxwidget.Flex)
			for _, action := range actions.Children {
				props := action.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
				if (props.Icon != nil) != (state.hovered || state.focused) || props.OnTap == nil {
					t.Fatalf("kind %s: unexpected action visibility/accessibility: %+v", kind, props)
				}
			}
		}
	}
}

func TestChatProjectArrowAndActionColors(t *testing.T) {
	theme := woxcomponent.Theme{
		PreviewText:    woxui.Color{R: 220, G: 220, B: 220, A: 255},
		ResultSubtitle: woxui.Color{R: 110, G: 110, B: 110, A: 255},
		SelectedTitle:  woxui.Color{R: 255, G: 255, B: 255, A: 255},
		ErrorText:      woxui.Color{R: 220, G: 45, B: 55, A: 255},
	}
	for _, collapsed := range []bool{false, true} {
		for _, hovered := range []bool{false, true} {
			item := ChatCatalogItemProps{SelectID: "project", Kind: "project-group", Title: "Project", Collapsed: collapsed, OnNew: func() {}, OnEdit: func() {}, OnDelete: func() {}}
			group := chatProjectHistoryGroup(item, 240, theme, hovered, false, false, false, nil, nil, nil, nil).(woxwidget.Semantics)
			stack := group.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
			row := stack.Children[0].Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
			label := row.Child.(woxwidget.Align).Child.(woxwidget.Flex)
			arrow := label.Children[len(label.Children)-1].(woxwidget.Container)
			if arrow.Width != 16 || (arrow.Child != nil) != hovered {
				t.Fatalf("collapsed %v hovered %v: arrow slot = %+v", collapsed, hovered, arrow)
			}
			if hovered {
				want := woxcomponent.DisclosureChevronGlyph(16, theme.PreviewText, !collapsed).(woxwidget.Image)
				if arrow.Child.(woxwidget.Image).Source != want.Source {
					t.Fatalf("collapsed %v: incorrect arrow", collapsed)
				}
				actions := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Flex)
				for _, action := range actions.Children {
					props := action.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
					if props.Icon == nil || props.IdleIcon == nil || props.Icon.(woxwidget.Image).Source == props.IdleIcon.(woxwidget.Image).Source {
						t.Fatalf("action %s should have distinct active and idle glyphs", props.ID)
					}
					if props.ID == "project-delete" {
						want := woxcomponent.DeleteGlyph(chatSidebarActionGlyph, theme.ErrorText).(woxwidget.Image)
						idle := woxcomponent.DeleteGlyph(chatSidebarActionGlyph, theme.PreviewText).(woxwidget.Image)
						if props.Icon.(woxwidget.Image).Source != want.Source || props.IdleIcon.(woxwidget.Image).Source != idle.Source || props.Background.A != 0 || props.HoverBackground.A != 0 {
							t.Fatal("project delete should turn red without a button background")
						}
					}
				}
			}
		}
	}
}

func TestChatRecentsSectionHasNoProjectActions(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "chat-recents-section", Kind: "projects-section", Title: "Recents", OnSelect: func() {}}
	view := chatProjectHistoryGroup(item, 240, woxcomponent.Theme{}, true, false, false, false, nil, nil, nil, nil).(woxwidget.Semantics)
	if view.Role != woxui.AccessibilityRoleGroup || !view.Expanded {
		t.Fatalf("unexpected section semantics: %+v", view)
	}
	stack := view.Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	actions := stack.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(actions.Children) != 0 {
		t.Fatal("recents must not expose project actions")
	}
}

// TestChatProjectRowHoverPaintsActions exercises hit testing and retained state together.
func TestChatProjectRowHoverPaintsActions(t *testing.T) {
	item := ChatCatalogItemProps{SelectID: "project-row", Kind: "project-group", Title: "Project", NewLabel: "New chat", EditLabel: "Edit project", OnSelect: func() {}, OnNew: func() {}, OnEdit: func() {}}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return ChatCatalogItem(item, 240, 38, woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}, PreviewText: woxui.Color{A: 255}})
	})
	host.AttachServices(terminalPreviewHostServices{})
	defer host.Dispose()
	paint := func() int {
		list := &woxui.DisplayList{}
		host.Frame(list, woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 100}, Scale: 1})
		return list.ImageDrawCount()
	}
	idle := paint()
	for _, x := range []float32{60, 195, 222} {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: x, Y: 16}})
		if images := paint(); images != idle+3 {
			t.Fatalf("pointer at %v: painted %v icons, want idle %v plus arrow and two actions", x, images, idle)
		}
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave})
	if images := paint(); images != idle {
		t.Fatalf("leaving row retained action icons: %v, want %v", images, idle)
	}
}

// TestChatRecentsHoverSurvivesScrollOverflow keeps hover while the section changes the scroll extent.
func TestChatRecentsHoverSurvivesScrollOverflow(t *testing.T) {
	collapsed := true
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		items := []ChatCatalogItemProps{
			{SelectID: "projects", Kind: "projects-section", Title: "Projects", OnSelect: func() {}},
			{SelectID: "recents", Kind: "projects-section", Title: "Recents", SpacingBefore: ChatHistorySectionGap, Collapsed: collapsed, OnSelect: func() { collapsed = !collapsed }},
		}
		if !collapsed {
			for i := 0; i < 12; i++ {
				items = append(items, ChatCatalogItemProps{SelectID: string(rune('a' + i)), Kind: "history", Title: "Conversation"})
			}
		}
		return chatHistoryCatalog(ChatCatalogProps{Key: "test", Width: 260, Height: 400, Items: items, ContentHeight: 46 + float32(len(items))*ChatHistoryRowHeight + ChatHistorySectionGap, OnScroll: func(float32) {}})
	})
	host.AttachServices(terminalPreviewHostServices{})
	defer host.Dispose()
	frame := func() woxui.AccessibilityNode {
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 260, Height: 400}, Scale: 1})
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "recents-group" {
				return node
			}
		}
		t.Fatal("Recents section missing")
		return woxui.AccessibilityNode{}
	}
	section := frame()
	point := woxui.Point{X: section.Bounds.X + 30, Y: section.Bounds.Y + section.Bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: point})
	if !frame().Hovered {
		t.Fatal("Recents should be hovered before clicking")
	}
	for i := 0; i < 2; i++ {
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		section = frame()
		if !section.Hovered || section.Expanded == collapsed {
			t.Fatalf("click %d: hovered=%v expanded=%v collapsed=%v", i+1, section.Hovered, section.Expanded, collapsed)
		}
	}
}

// TestChatProjectDialogFitsItsContentAndAlignsActions protects the modal's footer geometry.
func TestChatProjectDialogFitsItsContentAndAlignsActions(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1} {
		theme := woxcomponent.ControlTheme{DensityScale: scale}
		dialog := ChatProjectDialog(ChatProjectDialogProps{Width: 900, Height: 800, Theme: theme}).(woxwidget.Stateful).Widget.(woxcomponent.DialogProps)
		footer := dialog.Child.(woxwidget.Flex).Children[4].(woxwidget.Align)
		if dialog.Height >= theme.Scaled(300) || footer.Horizontal != 1 || footer.Height != theme.Scaled(32) {
			t.Fatalf("density %v: empty editor height %v, footer %+v", scale, dialog.Height, footer)
		}
	}
}

// TestChatProjectSelectorUsesModelChipLayout keeps both controls visually interchangeable.
func TestChatProjectSelectorUsesModelChipLayout(t *testing.T) {
	props := ChatInputProps{Key: "project", ProjectLabel: "A very long project name", ProjectIcon: &woxui.Image{}, OnProjects: func() {}}
	project := ChatProjectSelector(props, 140).(woxwidget.Stateful)
	model := ChatModelSelector(ChatInputProps{Key: "model", Model: props.ProjectLabel, ModelWidth: 140}).(woxwidget.Stateful)
	if project.Type != model.Type {
		t.Fatal("project and model chips no longer share their layout")
	}
	state := chatModelSelectorState{}
	semantics := state.Build(woxwidget.StateContext{}, project.Widget).(woxwidget.Semantics)
	gesture := semantics.Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	if gesture.OnHoverAt != nil {
		t.Fatal("project selector must not show a redundant tooltip")
	}
	chip := gesture.Child.(woxwidget.Container)
	aligned := chip.Child.(woxwidget.Align)
	row := aligned.Child.(woxwidget.Flex)
	if aligned.Height != 20 || aligned.Vertical != 0.5 || row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatal("selector content must share a vertical centerline")
	}
	labelSlot, ok := row.Children[2].(woxwidget.Expanded)
	if !ok || chip.Width != 140 {
		t.Fatal("long label must only consume the space between icon and chevron")
	}
	label := labelSlot.Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if label.AlignmentY != 0.5 || label.Height != 18 || label.LineHeight != 18 {
		t.Fatalf("project label slot = height %.0f line %.0f alignment %.1f; want an optically centered 18 slot", label.Height, label.LineHeight, label.AlignmentY)
	}
}
