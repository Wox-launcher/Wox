package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeEditorTokensUseHorizontalScroll(t *testing.T) {
	tokens := make([]ThemeEditorColorToken, 5)
	for index := range tokens {
		tokens[index] = ThemeEditorColorToken{Key: string(rune('a' + index))}
	}
	props, content := themeEditorScroll(t, themeEditorTokens(ThemeEditorSettingsProps{
		ActiveGroup: 0,
		Groups:      []ThemeEditorColorGroup{{Tokens: tokens}},
	}, 500, 58))

	if !props.Horizontal || props.AlwaysShowScrollbar {
		t.Fatalf("theme token scroll = %#v, want a hover-revealed horizontal strip", props)
	}
	if props.ContentWidth <= props.Width {
		t.Fatalf("theme token content width = %v, want greater than viewport %v", props.ContentWidth, props.Width)
	}
	card := content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if card.Height != 44 {
		t.Fatalf("theme token height = %v, want Flutter height 44", card.Height)
	}
	label := card.Child.(woxwidget.Flex).Children[0].(woxwidget.Clip).Child.(woxwidget.Align)
	if label.Vertical != 0.5 {
		t.Fatalf("theme token label vertical alignment = %v, want centered", label.Vertical)
	}
}

func themeEditorScroll(t *testing.T, view woxwidget.Widget) (woxcomponent.ScrollViewProps, woxwidget.Widget) {
	t.Helper()
	switch typed := view.(type) {
	case woxwidget.Stateful:
		props := typed.Widget.(woxcomponent.ScrollViewProps)
		return props, props.Content
	case woxwidget.Gesture:
		scroll := typed.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
		return woxcomponent.ScrollViewProps{Width: scroll.Width, ContentWidth: scroll.ContentWidth, Horizontal: scroll.Horizontal}, scroll.Child
	default:
		t.Fatalf("theme editor scroll = %T, want WoxScrollView", view)
		return woxcomponent.ScrollViewProps{}, nil
	}
}

func TestThemeEditorGroupUsesMeasuredFlutterWidth(t *testing.T) {
	_, content := themeEditorScroll(t, themeEditorGroupSelector(ThemeEditorSettingsProps{
		ActiveGroup: 0,
		Groups:      []ThemeEditorColorGroup{{Label: "操作面板", LabelWidth: 48}},
	}, 500, 40))
	semantic := content.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	stateful := semantic.Child.(woxwidget.Stateful)
	chip := (&themeEditorGroupChipState{}).Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture).Child.(woxwidget.Container)
	if chip.Width != 72 {
		t.Fatalf("theme group width = %v, want measured label width plus Flutter padding 72", chip.Width)
	}
	if !semantic.Selected {
		t.Fatal("active theme group is not exposed as selected")
	}
}

func TestThemeEditorGroupAddsHoverSurface(t *testing.T) {
	_, content := themeEditorScroll(t, themeEditorGroupSelector(ThemeEditorSettingsProps{
		Groups: []ThemeEditorColorGroup{{Label: "Window"}, {Label: "Query box"}},
	}, 500, 40))
	semantic := content.(woxwidget.Flex).Children[1].(woxwidget.Semantics)
	stateful := semantic.Child.(woxwidget.Stateful)
	normal := (&themeEditorGroupChipState{}).Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture)
	hovered := (&themeEditorGroupChipState{hovered: true}).Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Gesture)

	if normal.OnHoverAt == nil {
		t.Fatal("theme editor group does not retain hover input")
	}
	if normal.Child.(woxwidget.Container).Color.A != 0 || hovered.Child.(woxwidget.Container).Color.A == 0 {
		t.Fatalf("theme editor group hover colors = %#v/%#v, want transparent/visible", normal.Child.(woxwidget.Container).Color, hovered.Child.(woxwidget.Container).Color)
	}
}

func TestThemeEditorGroupSelectorScrollsNarrowViewport(t *testing.T) {
	groups := make([]ThemeEditorColorGroup, 5)
	for index := range groups {
		groups[index] = ThemeEditorColorGroup{Label: "Group", LabelWidth: 54}
	}
	props, _ := themeEditorScroll(t, themeEditorGroupSelector(ThemeEditorSettingsProps{Groups: groups}, 300, 40))
	if !props.Horizontal || props.AlwaysShowScrollbar || props.ContentWidth <= props.Width {
		t.Fatalf("theme group selector = %#v, want hover-revealed horizontal overflow", props)
	}
}

func TestThemeEditorControlPaneReservesActionButtonWidth(t *testing.T) {
	pane := themeEditorControlPane(ThemeEditorSettingsProps{
		DiscardLabel: "Discard", OverwriteLabel: "Overwrite", SaveAsLabel: "Save as",
		DiscardIcon: &woxui.Image{}, OverwriteIcon: &woxui.Image{}, SaveAsIcon: &woxui.Image{},
	}, 716, themeEditorControlPaneHeight).(woxwidget.Stack)
	actions := pane.Children[2].Child.(woxwidget.Flex)
	button := focusedControlGesture(actions.Children[1]).Child.(woxwidget.Container)
	if button.Width != 100 || button.Padding.Left != 10 || button.Padding.Right != 10 {
		t.Fatalf("narrow theme action button width/padding = %.0f/%v, want 100/10px horizontal", button.Width, button.Padding)
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
	accessory := query.Child.(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	timeText := accessory.Children[1].(woxwidget.Text).Value
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
	}
	for token, want := range tests {
		if got := themeEditorDemoHighlightTarget(token); got != want {
			t.Fatalf("highlight target for %s = %v, want %v", token, got, want)
		}
	}
}
