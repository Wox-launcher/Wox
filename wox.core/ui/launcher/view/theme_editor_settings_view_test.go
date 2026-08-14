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
	view := themeEditorTokens(ThemeEditorSettingsProps{
		ActiveGroup: 0,
		Groups:      []ThemeEditorColorGroup{{Tokens: tokens}},
	}, 500, 58)

	scroll, ok := view.(woxwidget.ScrollView)
	if !ok || !scroll.Horizontal {
		t.Fatalf("theme token view = %T, want horizontal ScrollView", view)
	}
	if scroll.ContentWidth <= scroll.Width {
		t.Fatalf("theme token content width = %v, want greater than viewport %v", scroll.ContentWidth, scroll.Width)
	}
	card := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if card.Height != 44 {
		t.Fatalf("theme token height = %v, want Flutter height 44", card.Height)
	}
	label := card.Child.(woxwidget.Flex).Children[0].(woxwidget.Clip).Child.(woxwidget.Align)
	if label.Vertical != 0.5 {
		t.Fatalf("theme token label vertical alignment = %v, want centered", label.Vertical)
	}
}

func TestThemeEditorGroupUsesMeasuredFlutterWidth(t *testing.T) {
	selector := themeEditorGroupSelector(ThemeEditorSettingsProps{
		ActiveGroup: 0,
		Groups:      []ThemeEditorColorGroup{{Label: "操作面板", LabelWidth: 48}},
	}, 500, 40).(woxwidget.ScrollView)
	semantic := selector.Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics)
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
	selector := themeEditorGroupSelector(ThemeEditorSettingsProps{
		Groups: []ThemeEditorColorGroup{{Label: "Window"}, {Label: "Query box"}},
	}, 500, 40).(woxwidget.ScrollView)
	semantic := selector.Child.(woxwidget.Flex).Children[1].(woxwidget.Semantics)
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
	selector := themeEditorGroupSelector(ThemeEditorSettingsProps{Groups: groups}, 300, 40).(woxwidget.ScrollView)
	if !selector.Horizontal || selector.ContentWidth <= selector.Width {
		t.Fatalf("theme group selector = %#v, want horizontal overflow", selector)
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

func TestThemeEditorTailFlashDoesNotWrapResultRows(t *testing.T) {
	props := ThemeEditorSettingsProps{
		FlashToken:          "ResultItemTailTextColor",
		PreviewTailP1Width:  28,
		PreviewTail4msWidth: 34,
		DraftTheme: woxcomponent.Theme{
			ResultTitle:    woxui.Color{A: 255},
			ResultSubtitle: woxui.Color{A: 255},
		},
	}
	rows := themeEditorPreviewResultRows(props, 600, 240).(woxwidget.Flex)
	for index, row := range rows.Children {
		if _, wrapsRow := row.(woxwidget.Stack); wrapsRow {
			t.Fatalf("tail flash wrapped result row %d", index)
		}
	}

	inactiveRow := rows.Children[1].(woxwidget.Container)
	content := inactiveRow.Child.(woxwidget.Flex)
	tail := content.Children[2].(woxwidget.Align)
	if tail.Vertical != 0.5 {
		t.Fatalf("inactive tail vertical alignment = %v, want centered like launcher result rows", tail.Vertical)
	}
	if _, highlightsTail := tail.Child.(woxwidget.Stack); !highlightsTail {
		t.Fatalf("inactive tail content = %T, want localized flash Stack", tail.Child)
	}
	row := inactiveRow.Child.(woxwidget.Flex)
	innerWidth := float32(600) - props.PreviewGeometry.ResultItemPadding.Left - props.PreviewGeometry.ResultItemPadding.Right
	usedWidth := row.Children[0].(woxwidget.Align).Width + row.Children[1].(woxwidget.Align).Width + tail.Width + row.Gap*2
	if usedWidth > innerWidth {
		t.Fatalf("result row children width = %v, exceeds inner width %v", usedWidth, innerWidth)
	}
}

func TestThemeEditorPreviewChromeMatchesLauncherLayout(t *testing.T) {
	props := ThemeEditorSettingsProps{
		DraftTheme: woxcomponent.Theme{PreviewSplit: woxui.Color{R: 1, A: 255}},
	}
	query := themeEditorPreviewQuery(props, 600, 55).(woxwidget.Container)
	queryRow := query.Child.(woxwidget.Flex)
	queryText := queryRow.Children[0].(woxwidget.Container)
	glance := queryRow.Children[1].(woxwidget.Align)
	if queryText.Width+glance.Width != 586 || glance.Horizontal != 1 {
		t.Fatalf("query content width/alignment = %v + %v, %v; want 586 and right aligned", queryText.Width, glance.Width, glance.Horizontal)
	}

	actionPanel := themeEditorPreviewWithActionPanel(props, 600, 240).(woxwidget.Stack)
	panel := actionPanel.Children[1].Child.(woxwidget.Container)
	panelColumn := panel.Child.(woxwidget.Flex)
	divider := panelColumn.Children[1].(woxwidget.Container)
	line := divider.Child.(woxwidget.Container)
	if divider.Height != ActionDividerHeight || line.Height != 1 || line.Color != props.DraftTheme.PreviewSplit {
		t.Fatalf("action divider = %#v, want launcher divider", divider)
	}

	preview := themeEditorPreviewWithTextPanel(props, 600, 240).(woxwidget.Flex)
	previewPanel := preview.Children[1].(woxwidget.Stack)
	previewShell := previewPanel.Children[0].Child.(woxwidget.Container)
	previewStack := previewShell.Child.(woxwidget.Stack)
	surface := previewStack.Children[0].Child.(woxwidget.Container)
	tagLayer := previewStack.Children[1]
	if tagLayer.Top <= surface.Height {
		t.Fatalf("preview tags top = %v, want below content surface height %v", tagLayer.Top, surface.Height)
	}
	tagScroll := tagLayer.Child.(woxwidget.ScrollView)
	tags := tagScroll.Child.(woxwidget.Flex)
	if len(tags.Children) != 4 {
		t.Fatalf("preview tag count = %d, want Flutter metadata sample count 4", len(tags.Children))
	}
	if !tagScroll.Horizontal || tagScroll.ContentWidth < tagScroll.Width {
		t.Fatalf("preview tag strip = %#v, want Flutter horizontal overflow", tagScroll)
	}
	firstTag := tags.Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if firstTag.Value != "2026-05-26 10:47:08" {
		t.Fatalf("first preview tag = %q, want Flutter metadata sample", firstTag.Value)
	}
	previewDivider := previewPanel.Children[1].Child.(woxwidget.Container)
	if previewDivider.Width != 1 || previewDivider.Height != 240 {
		t.Fatalf("preview divider = %#v, want Flutter left-only divider", previewDivider)
	}
}

func TestThemeEditorToolbarKeepsFlutterActionAndKeySpacing(t *testing.T) {
	toolbar := themeEditorPreviewToolbar(ThemeEditorSettingsProps{
		DraftTheme: woxcomponent.Theme{ToolbarText: woxui.Color{A: 255}},
	}, 600, 40).(woxwidget.Stack)
	body := toolbar.Children[0].Child.(woxwidget.Container)
	if body.BorderWidth != 0 || toolbar.Children[1].Child.(woxwidget.Container).Height != 1 {
		t.Fatalf("toolbar border = %v, want only Flutter top divider", body.BorderWidth)
	}
	right := body.Child.(woxwidget.Flex)
	if right.Gap != 16 {
		t.Fatalf("toolbar action gap = %v, want Flutter spacing 16", right.Gap)
	}
	moreAction := right.Children[1].(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	keycaps := moreAction.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(keycaps.Children) != 2 || keycaps.Gap != 4 {
		t.Fatalf("more-action keycaps = %d gap %v, want separate Cmd/J keys with Flutter spacing 4", len(keycaps.Children), keycaps.Gap)
	}
}
