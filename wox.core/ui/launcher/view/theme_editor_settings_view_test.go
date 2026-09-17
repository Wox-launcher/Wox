package view

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestThemeEditorSharedChatSendStaysVisible covers the shared composer at narrow widths and multiple lines.
func TestThemeEditorSharedChatSendStaysVisible(t *testing.T) {
	for _, width := range []float32{500, 800} {
		for _, prompt := range []string{"Round the corners", "one\ntwo\nthree\nfour\nfive"} {
			paneWidth, paneHeight := ThemeEditorInspectorSize(width, 640, false)
			inputHeight := previewview.ChatComposerHeightForLines(0, previewview.ChatComposerVisibleLines(prompt, paneWidth-20, nil, nil))
			chatProps := previewview.ChatPreviewProps{Key: "theme-editor-ai", Width: paneWidth, Height: paneHeight - 40,
				Messages: previewview.ChatMessagesProps{Width: paneWidth - 20, Height: paneHeight - 40 - inputHeight - 14},
				Input:    previewview.ChatInputProps{Key: "theme-editor-ai", Width: paneWidth - 20, Height: inputHeight, Editing: woxui.TextEditingState{Text: prompt}, Model: "Model", ModelWidth: 120, ActionLabel: "Send"}}
			props := ThemeEditorSettingsProps{Width: width, Height: 640, AIExpanded: true,
				AIAssistant: previewview.ChatConversation(previewview.ChatConversationProps{ChatPreviewProps: chatProps})}
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return ThemeEditorSettingsView(props) })
			host.AttachServices(settingsWindowHostServices{})
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 640}, Scale: 1.5, PixelSize: woxui.PixelSize{Width: int(width * 1.5), Height: 960}})
			bounds, ok := host.BoundsForKey("chat-send-theme-editor-ai")
			if !ok || bounds.Y+bounds.Height > 640 || bounds.X+bounds.Width > width || (width >= 760 && bounds.X < width-340) {
				t.Fatalf("send outside pane: width=%v bounds=%+v", width, bounds)
			}
			if _, visible := host.BoundsForKey("theme-editor-search"); visible {
				t.Fatal("property search must be hidden while chatting")
			}
			modeBounds, modeVisible := host.BoundsForKey("theme-editor-mode-1")
			if !modeVisible {
				t.Fatal("AI mode switch must remain visible in chat")
			}
			props.AIExpanded = false
			props.AIAssistant = nil
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 640}, Scale: 1.5, PixelSize: woxui.PixelSize{Width: int(width * 1.5), Height: 960}})
			if _, visible := host.BoundsForKey("theme-editor-search"); visible {
				t.Fatal("property search must remain removed in editing mode")
			}
			if bounds, visible := host.BoundsForKey("theme-editor-mode-1"); !visible || bounds != modeBounds {
				t.Fatal("mode switch moved when returning to properties")
			}
			host.Dispose()
		}
	}
}

// TestThemeEditorFieldErrorPlacement keeps validation beside the value that needs correction.
func TestThemeEditorFieldErrorPlacement(t *testing.T) {
	row := themeEditorPropertyRow(ThemeEditorSettingsProps{}, ThemeEditorColorToken{Key: "AppBorderRadius", Numeric: true, Value: "bad", Error: "Enter a non-negative whole number."}, 324).(woxwidget.Container).Child.(woxwidget.Flex)
	if len(row.Children) != 3 || row.Children[2].(woxwidget.TextBlock).Value != "Enter a non-negative whole number." {
		t.Fatal("validation must follow the input controls")
	}
}

// TestThemeEditorStickyHeader checks the transition without replacing the retained scroll subtree.
func TestThemeEditorStickyHeader(t *testing.T) {
	props := ThemeEditorSettingsProps{Groups: []ThemeEditorColorGroup{
		{Label: "Window", Tokens: []ThemeEditorColorToken{{Key: "AppBackgroundColor"}}},
		{Label: "Query", Tokens: []ThemeEditorColorToken{{Key: "QueryBoxBackgroundColor"}}},
	}}
	state := themeEditorSettingsState{expanded: map[int]bool{1: true}}
	for _, offset := range []float32{0, 48, 49, 200} {
		state.scrollOffset = offset
		pane := state.inspector(woxwidget.StateContext{}, props, 340, 500).(woxwidget.Flex).Children[0].(woxwidget.Stack)
		if _, ok := pane.Children[0].Child.(woxwidget.Stateful); !ok {
			t.Fatal("scroll subtree changed identity")
		}
		if (len(pane.Children) == 2) != (offset > 48) {
			t.Fatalf("incorrect sticky header at offset %v", offset)
		}
		if len(pane.Children) == 2 && pane.Children[1].Child.(woxwidget.Container).Color.A != 255 {
			t.Fatal("sticky header must hide the scrolling labels beneath it")
		}
	}
	row := themeEditorPropertyRow(props, props.Groups[0].Tokens[0], 324).(woxwidget.Container).Child.(woxwidget.Flex)
	if len(row.Children) != 2 {
		t.Fatal("background must only show label and color controls")
	}
}

func TestThemeEditorColorSwatchOpensPicker(t *testing.T) {
	selected := ""
	props := ThemeEditorSettingsProps{OnEditToken: func(key string) { selected = key }}
	row := themeEditorPropertyRow(props, ThemeEditorColorToken{Key: "AppBackgroundColor"}, 324).(woxwidget.Container).Child.(woxwidget.Flex)
	controls := row.Children[1].(woxwidget.Flex)
	swatch := controls.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	swatch.OnTap()
	if selected != "AppBackgroundColor" {
		t.Fatal("swatch did not open its color picker")
	}
}

// TestThemeEditorInlineControls measures the mounted tree in logical units at multiple display scales.
func TestThemeEditorInlineControls(t *testing.T) {
	for _, width := range []float32{500, 800} {
		for _, scale := range []float32{1, 1.5, 2} {
			t.Run(fmt.Sprintf("%g/%g", width, scale), func(t *testing.T) {
				token := ThemeEditorColorToken{Key: "AppBorderWidth", Label: "Window border width", Numeric: true, Optional: true, Value: "2", Effective: "2"}
				props := ThemeEditorSettingsProps{Width: width, Height: 640, Title: "Theme", DefaultLabel: "Default", ResetLabel: "Reset", Groups: []ThemeEditorColorGroup{{Label: "Window", Tokens: []ThemeEditorColorToken{token}}}}
				props.OnChangeToken = func(key, value string) { props.Groups[0].Tokens[0].Value = value }
				host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return ThemeEditorSettingsView(props) })
				host.AttachServices(settingsWindowHostServices{})
				defer host.Dispose()
				frame := woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 640}, PixelSize: woxui.PixelSize{Width: int(width * scale), Height: int(640 * scale)}, Scale: scale}
				host.Frame(&woxui.DisplayList{}, frame)
				save, saveOK := host.BoundsForKey("theme-editor-save-as")
				search, searchOK := host.BoundsForKey("theme-editor-group-0")
				if !saveOK || !searchOK || save.X+save.Width != search.X+search.Width {
					t.Fatalf("save and group right edges differ: %+v, %+v", save, search)
				}
				bounds, ok := host.BoundsForKey("theme-editor-value-AppBorderWidth")
				if !ok || bounds.Width <= 0 || bounds.X < 0 || bounds.X+bounds.Width > width || bounds.Y+bounds.Height > 640 {
					t.Fatalf("field is outside inspector: %+v", bounds)
				}
				for _, edit := range []struct {
					id          string
					action      woxui.AccessibilityAction
					value, want string
				}{
					{"theme-editor-value-AppBorderWidth", woxui.AccessibilityActionSetValue, "0", "0"},
					{"theme-editor-step-AppBorderWidth+", woxui.AccessibilityActionActivate, "", "1"},
					{"theme-editor-reset-AppBorderWidth", woxui.AccessibilityActionActivate, "", ""},
				} {
					control, exists := host.BoundsForKey(woxwidget.Key(edit.id))
					if !exists {
						t.Fatalf("missing control %s", edit.id)
					}
					if control.Height != 32 || control.Y != bounds.Y {
						t.Fatalf("control %s is not aligned with the 32-unit input: %+v, input %+v", edit.id, control, bounds)
					}
					point := woxui.Point{X: control.X + control.Width/2, Y: control.Y + control.Height/2}
					host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
					host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
					if edit.action == woxui.AccessibilityActionSetValue {
						host.Key(woxui.KeyEvent{Key: woxui.Key("a"), Modifiers: woxui.KeyModifierControl | woxui.KeyModifierMeta, Down: true})
						host.TextInput(woxui.TextInputEvent{Text: edit.value})
					}
					if got := props.Groups[0].Tokens[0].Value; got != edit.want {
						t.Fatalf("%s produced %q, want %q", edit.id, got, edit.want)
					}
					host.Frame(&woxui.DisplayList{}, frame)
				}
			})
		}
	}
}

func TestThemeEditorSaveActions(t *testing.T) {
	for _, editable := range []bool{false, true} {
		row := themeEditorActions(ThemeEditorSettingsProps{CanOverwrite: editable}, 324, 32).(woxwidget.Flex)
		want := 2
		if editable {
			want = 3
		}
		if len(row.Children) != want {
			t.Fatalf("editable %v: got %d actions, want %d", editable, len(row.Children), want)
		}
	}
}

// TestThemeEditorPreviewLocatorBounds follows the same body and tag geometry as PreviewView.
func TestThemeEditorPreviewLocatorBounds(t *testing.T) {
	color := woxui.Color{R: 70, A: 90}
	for _, size := range [][2]float32{{702, 344}, {560, 360}} {
		for _, token := range []string{"PreviewBackgroundColor", "PreviewBorderColor", "PreviewTagFontColor", "PreviewTagBackgroundColor", "PreviewTagBorderColor", "PreviewPropertyTitleColor", "PreviewPropertyContentColor"} {
			t.Run(token, func(t *testing.T) {
				props := ThemeEditorSettingsProps{ActiveGroup: 3, FlashToken: token, DraftTheme: woxcomponent.Theme{PreviewTagFontColor: &color}}
				demo := themeEditorPreviewWindow(props, size[0], size[1]).(woxwidget.Clip).Child.(woxwidget.Stack)
				clip := demo.Children[len(demo.Children)-3].Child.(woxwidget.Clip)
				panel := clip.Child.(woxwidget.Stack)
				if panel.Width != clip.Width || panel.Height != clip.Height {
					t.Fatal("preview content exceeds its launcher viewport")
				}
				layout := previewview.ResolvePreviewLayout(panel.Width, panel.Height, true)
				shell := panel.Children[0].Child.(woxwidget.Container)
				slots := shell.Child.(woxwidget.Stack).Children
				if token == "PreviewPropertyTitleColor" || token == "PreviewPropertyContentColor" {
					if len(panel.Children) != 2 {
						t.Fatal("v2 property locator must not highlight footer tags")
					}
					body := slots[0].Child.(woxwidget.Container).Child.(woxwidget.Clip).Child.(woxwidget.Container).Child.(woxwidget.Flex)
					property := body.Children[2].(woxwidget.Flex)
					if property.Axis != woxwidget.Horizontal || property.Gap != 10 {
						t.Fatal("metadata must match the label/value row in file previews")
					}
					index := 0
					if token == "PreviewPropertyContentColor" {
						index = 1
					}
					if _, ok := property.Children[index].(woxwidget.Stack); !ok {
						t.Fatal("property locator missed its body label/value")
					}
					return
				}
				marker := panel.Children[2]
				overlay := marker.Child.(woxwidget.Stack)
				wantTop, wantHeight := shell.Padding.Top, layout.BodyHeight+2
				if token != "PreviewBackgroundColor" && token != "PreviewBorderColor" {
					wantTop += slots[1].Top
					wantHeight = slots[1].Child.(woxwidget.ScrollView).Height
				}
				if marker.Left != shell.Padding.Left || marker.Top != wantTop || overlay.Width != layout.InnerWidth || overlay.Height != wantHeight || marker.Top+overlay.Height > clip.Height {
					t.Fatal("locator does not match the visible preview surface/tag strip")
				}
			})
		}
	}
}

// TestThemeEditorInspectorLayout checks both responsive modes and vertical property overflow.
func TestThemeEditorInspectorLayout(t *testing.T) {
	for _, width := range []float32{500, 759, 760, 1000} {
		props := ThemeEditorSettingsProps{Width: width, Height: 640, Groups: []ThemeEditorColorGroup{{Label: "Window", Tokens: []ThemeEditorColorToken{{Key: "AppBorderWidth", Numeric: true, Value: "0"}}}}}
		state := themeEditorSettingsState{expanded: map[int]bool{0: true}}
		tree := state.Build(woxwidget.StateContext{}, props).(woxwidget.Flex)
		body := tree.Children[len(tree.Children)-1].(woxwidget.Flex)
		want := woxwidget.Vertical
		if width >= 760 {
			want = woxwidget.Horizontal
		}
		if body.Axis != want {
			t.Fatalf("width %v: axis %v, want %v", width, body.Axis, want)
		}
		inspector := body.Children[1].(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
		scroll := inspector.Children[0].(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		if scroll.Horizontal || scroll.Height <= 140 {
			t.Fatalf("inspector has no usable vertical viewport: %#v", scroll)
		}
		content := scroll.Content.(woxwidget.Flex)
		if content.Axis != woxwidget.Vertical || len(content.Children) != 3 {
			t.Fatal("expected header, property and divider")
		}
	}
}

// TestThemeEditorLinkedPadding preserves asymmetric drafts until linking is explicitly requested.
func TestThemeEditorLinkedPadding(t *testing.T) {
	for _, asymmetric := range []bool{false, true} {
		tokens := []ThemeEditorColorToken{}
		for _, side := range []string{"Left", "Top", "Right", "Bottom"} {
			value := "10"
			if asymmetric && side == "Top" {
				value = "20"
			}
			tokens = append(tokens, ThemeEditorColorToken{Key: "AppPadding" + side, Label: side, Numeric: true, Optional: true, Value: value, Effective: value})
		}
		var edit map[string]string
		props := ThemeEditorSettingsProps{Groups: []ThemeEditorColorGroup{{Tokens: tokens}}, OnChangeTokens: func(values map[string]string) { edit = values }}
		state := themeEditorSettingsState{expanded: map[int]bool{0: true}}
		inspector := state.inspector(woxwidget.StateContext{}, props, 340, 500).(woxwidget.Flex)
		rows := inspector.Children[0].(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children
		want := 4
		if asymmetric {
			want = 7
		}
		if len(rows) != want || edit != nil {
			t.Fatal("building padding controls changed values or hid asymmetric sides")
		}
		if !asymmetric {
			field := rows[2].(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Flex).Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
			field.OnChanged("0")
			if len(edit) != 4 {
				t.Fatal("linked edit did not update all four sides atomically")
			}
			for _, value := range edit {
				if value != "0" {
					t.Fatal("linked edit lost explicit zero")
				}
			}
		}
	}
}

func TestThemeEditorUsesCompleteLauncherDemo(t *testing.T) {
	preview := themeEditorPreviewWindow(ThemeEditorSettingsProps{
		DraftTheme:         woxcomponent.Theme{QueryText: woxui.Color{A: 255}},
		PreviewResultTitle: "Theme editor", QueryBoxLabel: "Query box", ResultsLabel: "Results",
		ToolbarCopyLabel: "Copy", ToolbarMoreLabel: "More Actions",
	}, 600, 320).(woxwidget.Clip)
	children := preview.Child.(woxwidget.Stack).Children
	query := children[2].Child.(woxwidget.Container)
	result := children[3].Child.(woxwidget.Container)
	toolbar := children[len(children)-2].Child.(woxwidget.Container)

	if query.Height != 55 || result.Height != 56 || toolbar.Height != 40 {
		t.Fatalf("shared launcher demo metrics = query %v, result %v, toolbar %v", query.Height, result.Height, toolbar.Height)
	}
	accessories := query.Child.(woxwidget.Flex).Children[1].(woxwidget.Flex)
	if len(accessories.Children) != 2 {
		t.Fatalf("theme demo query accessories = %d, want attention and glance", len(accessories.Children))
	}
	for _, accessory := range accessories.Children {
		aligned := accessory.(woxwidget.Container).Child.(woxwidget.Align)
		if aligned.Height != 30 || aligned.Vertical != 0.5 {
			t.Fatal("preview accessories must center their content in the same 30-unit slot")
		}
	}
	attentionRow := accessories.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	iconColor, _, _, _ := (woxcomponent.Theme{QueryText: woxui.Color{A: 255}}).AttentionBadgeColors(false)
	wantIcon := woxcomponent.NotificationGlyph(16, iconColor).(woxwidget.Image)
	if attentionRow.Children[0].(woxwidget.Image).Source != wantIcon.Source {
		t.Fatal("preview Attention icon differs from the launcher notification icon")
	}
	timeText := accessories.Children[1].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[1].(woxwidget.Text).Value
	if len(timeText) != 5 || timeText[2] != ':' {
		t.Fatalf("theme demo Glance = %q, want current HH:MM time", timeText)
	}
	firstRow := children[3].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	secondRow := children[4].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	thirdRow := children[5].Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(firstRow.Children) != 3 || len(secondRow.Children) != 2 || len(thirdRow.Children) != 3 {
		t.Fatalf("theme demo result tag slots = %d/%d/%d, want meaningful/none/meaningful diversity", len(firstRow.Children), len(secondRow.Children), len(thirdRow.Children))
	}
}

func TestThemeEditorMapsTokensToSemanticDemoHighlights(t *testing.T) {
	tests := map[string]woxcomponent.LauncherDemoHighlightTarget{
		"QueryBoxFontColor":               woxcomponent.LauncherDemoHighlightQueryText,
		"ResultItemSubTitleColor":         woxcomponent.LauncherDemoHighlightResultSubtitle,
		"ResultItemTailTextColor":         woxcomponent.LauncherDemoHighlightResultTail,
		"ResultItemActiveBackgroundColor": woxcomponent.LauncherDemoHighlightSelectedBackground,
		"ResultItemActiveTailTextColor":   woxcomponent.LauncherDemoHighlightSelectedTail,
		"ActionItemActiveFontColor":       woxcomponent.LauncherDemoHighlightActionSelectedText,
		"ToolbarFontColor":                woxcomponent.LauncherDemoHighlightToolbarText,
		"ToolbarHotkeyFontColor":          woxcomponent.LauncherDemoHighlightHotkey,
		"ToolbarHotkeyBackgroundColor":    woxcomponent.LauncherDemoHighlightHotkey,
		"ToolbarHotkeyBorderColor":        woxcomponent.LauncherDemoHighlightHotkey,
	}
	for token, want := range tests {
		if got := themeEditorDemoHighlightTarget(token); got != want {
			t.Fatalf("highlight target for %s = %v, want %v", token, got, want)
		}
	}
}

func TestThemeEditorPropertyLabelOmitsVisibleScope(t *testing.T) {
	for _, test := range []struct{ label, group, section, want string }{
		{"Content panel background", "Window", "Content panel", "Background"},
		{"Content panel · inset", "Window", "Content panel", "Inset"},
		{"Content panel · corner radius", "Window", "Content panel", "Corner radius"},
		{"内容面板背景", "窗口", "内容面板", "背景"},
		{"Glance hover background", "Query box", "Glance", "Hover background"},
		{"Action panel · border width", "Action panel", "Appearance", "Border width"},
		{"Selected keycap text", "Action panel", "Rows and selection", "Selected keycap text"},
	} {
		if got := themeEditorPropertyLabel(test.label, test.group, test.section); got != test.want {
			t.Errorf("%q: got %q, want %q", test.label, got, test.want)
		}
	}
}
