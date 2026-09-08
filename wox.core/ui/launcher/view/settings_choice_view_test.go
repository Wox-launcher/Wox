package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFilteredSettingsChoicesInsertsGroupHeaders(t *testing.T) {
	choices := []SettingsChoice{
		{Value: "groq", Label: "groq", Group: "API"},
		{Value: "openai", Label: "openai", Group: "API"},
		{Value: "codex-cli", Label: "codex-cli", Group: "Installed CLI"},
	}

	visible := filteredSettingsChoices(choices, "")
	if len(visible) != 5 || !visible[0].header || visible[0].choice.Label != "API" {
		t.Fatalf("unfiltered list = %#v, want API header first", visible)
	}
	if visible[1].choice.Value != "groq" || visible[2].choice.Value != "openai" || visible[3].choice.Label != "Installed CLI" || !visible[3].header || visible[4].originalIndex != 2 {
		t.Fatalf("grouped list = %#v", visible)
	}

	visible = filteredSettingsChoices(choices, "codex")
	if len(visible) != 2 || !visible[0].header || visible[0].choice.Label != "Installed CLI" || visible[1].originalIndex != 2 {
		t.Fatalf("codex filter = %#v, want Installed CLI header and original index 2", visible)
	}

	visible = filteredSettingsChoices(choices, "api")
	if len(visible) != 3 || visible[1].choice.Value != "groq" || visible[2].choice.Value != "openai" {
		t.Fatalf("group-name filter = %#v, want both API providers", visible)
	}
}

func TestSettingsChoiceKeyboardSkipsGroupHeaders(t *testing.T) {
	choices := []SettingsChoice{
		{Value: "openai", Label: "openai", Group: "API"},
		{Value: "codex-cli", Label: "codex-cli", Group: "Installed CLI"},
	}
	visible := filteredSettingsChoices(choices, "")
	if got := settingsChoiceVisibleIndex(visible, "codex-cli"); got != 3 {
		t.Fatalf("visible index for codex-cli = %d, want 3", got)
	}
	if got := settingsChoiceNearestSelectable(visible, 0, 1); got != 1 {
		t.Fatalf("down from API header = %d, want first API option", got)
	}
	if got := settingsChoiceNearestSelectable(visible, 1, 1); got != 3 {
		t.Fatalf("down from openai = %d, want first CLI option", got)
	}
	single := filteredSettingsChoices([]SettingsChoice{{Value: "openai", Label: "openai", Group: "API"}}, "")
	if got := settingsChoiceNearestSelectable(single, 1, 1); got != 1 {
		t.Fatalf("wrap from the only option = %d, want the same option", got)
	}
	if settingsChoiceContentHeight(visible) != 2*settingsChoiceGroupHeight+2*settingsChoiceRowHeight {
		t.Fatalf("grouped content height = %.0f", settingsChoiceContentHeight(visible))
	}
}

func TestGroupedSettingsChoiceRendersSectionLabel(t *testing.T) {
	props := SettingsChoiceProps{
		ID: "providers", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		CurrentValue: "openai", Theme: woxcomponent.Theme{ActionHeader: woxui.Color{R: 160, A: 255}},
		Choices: []SettingsChoice{
			{Value: "openai", Label: "openai", Group: "API"},
			{Value: "codex-cli", Label: "codex-cli", Group: "Installed CLI"},
		},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)
	if state.selected != 1 {
		t.Fatalf("initial selected = %d, want first option after the API header", state.selected)
	}

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuContent := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	scroll := menuContent.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := scroll.Content.(woxwidget.Flex).Children
	if len(rows) != 4 {
		t.Fatalf("row count = %d, want two headers and two options", len(rows))
	}
	header := rows[0].(woxwidget.Semantics)
	if header.Role != woxui.AccessibilityRoleGroup || header.Label != "API" {
		t.Fatalf("first row = %#v, want a group header", header)
	}
	title := header.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Text)
	if title.Value != "API" || title.Style.Size != woxcomponent.SettingsSectionTitleFontSize || title.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("group label = %#v", title)
	}
}

func TestFilteredSettingsChoicesDoesNotSearchInternalJSONValue(t *testing.T) {
	choices := []SettingsChoice{
		{Value: `{"Name":"deepseek-v4-flash","Provider":"deepseek"}`, Label: "deepseek-v4-flash"},
		{Value: `{"Name":"deepseek-v4-pro","Provider":"deepseek"}`, Label: "deepseek-v4-pro"},
	}

	visible := filteredSettingsChoices(choices, "pro")
	if len(visible) != 1 || visible[0].choice.Label != "deepseek-v4-pro" {
		t.Fatalf("filtered choices = %#v, want only deepseek-v4-pro", visible)
	}
}

func TestFilterableSettingsChoiceCollapsesWhenNothingMatches(t *testing.T) {
	props := SettingsChoiceProps{
		ID: "models", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		Theme: woxcomponent.Theme{}, Choices: []SettingsChoice{{Value: "flash", Label: "deepseek-v4-flash"}},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)
	state.queryController.SetText("missing", false)

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuStack := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack)
	if menuStack.Height != settingsChoiceSearchHeight {
		t.Fatalf("empty filtered menu height = %.0f, want search-only height %.0f", menuStack.Height, settingsChoiceSearchHeight)
	}
	menuContent := menuStack.Children[0].Child.(woxwidget.Container)
	children := menuContent.Child.(woxwidget.Flex).Children
	if len(children) != 1 {
		t.Fatalf("empty filtered menu child count = %d, want only the search field", len(children))
	}
}

func TestFilterableSettingsChoiceShowsSearchIcon(t *testing.T) {
	icon := &woxui.Image{}
	props := SettingsChoiceProps{
		ID: "models", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		SearchIcon: icon, Theme: woxcomponent.Theme{}, Choices: []SettingsChoice{{Value: "flash", Label: "deepseek-v4-flash"}},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuStack := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack)
	menuContent := menuStack.Children[0].Child.(woxwidget.Container)
	searchLayer := menuContent.Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	searchRow := searchLayer.Children[1].Child.(woxwidget.Container)
	searchFlex := searchRow.Child.(woxwidget.Flex)
	if searchFlex.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("search row cross-axis alignment = %v, want center", searchFlex.CrossAxisAlignment)
	}
	searchChildren := searchFlex.Children
	if len(searchChildren) != 2 {
		t.Fatalf("search row child count = %d, want icon and text field", len(searchChildren))
	}
	searchField := searchChildren[1].(woxwidget.Stateful)
	searchFieldProps := searchField.Widget.(woxcomponent.TextFieldProps)
	if searchFieldProps.TextAlignmentY != 0.5 {
		t.Fatalf("search text vertical alignment = %.1f, want 0.5", searchFieldProps.TextAlignmentY)
	}
}

func TestSettingsChoiceTrailingUsesRowTextColor(t *testing.T) {
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	props := SettingsChoiceProps{
		ID: "glance", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		CurrentValue: "time", Theme: woxcomponent.Theme{ActionText: title, ResultSubtitle: woxui.Color{R: 255, A: 255}},
		Choices: []SettingsChoice{
			{Value: "battery", Label: "Battery", Trailing: "AC"},
			{Value: "time", Label: "Time", Trailing: "14:35"},
		},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)
	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuContent := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	scroll := menuContent.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := scroll.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	trailing := row.Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Text)
	if trailing.Value != "AC" || trailing.Color != title {
		t.Fatalf("choice trailing = %q %#v, want row ActionText so ResultSubtitle cannot restyle Glance values", trailing.Value, trailing.Color)
	}
}

func TestSettingsChoiceSelectedItemUsesThemeHighlight(t *testing.T) {
	highlight := woxui.Color{R: 54, G: 123, B: 220, A: 255}
	props := SettingsChoiceProps{
		ID: "start-page", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		CurrentValue: "recent", Theme: woxcomponent.Theme{SelectedBackground: highlight},
		Choices: []SettingsChoice{{Value: "recent", Label: "Recent"}, {Value: "blank", Label: "Blank"}},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuStack := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack)
	menuContent := menuStack.Children[0].Child.(woxwidget.Container)
	scrollProps := menuContent.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rowStack := scrollProps.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	row := rowStack.Children[0].Child.(woxwidget.Container)

	if row.Color != highlight {
		t.Fatalf("selected choice fill = %#v, want theme highlight %#v", row.Color, highlight)
	}
}

func TestSettingsChoiceHighlightsHoveredItemLikeLauncherResult(t *testing.T) {
	highlight := woxui.Color{R: 54, G: 123, B: 220, A: 200}
	props := SettingsChoiceProps{
		ID: "start-page", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		CurrentValue: "recent", Theme: woxcomponent.Theme{SelectedBackground: highlight},
		Choices: []SettingsChoice{{Value: "recent", Label: "Recent"}, {Value: "blank", Label: "Blank"}, {Value: "query", Label: "Query"}},
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)
	state.selected = 1
	state.hovered = 1

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuStack := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack)
	menuContent := menuStack.Children[0].Child.(woxwidget.Container)
	scrollProps := menuContent.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rowStack := scrollProps.Content.(woxwidget.Flex).Children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	row := rowStack.Children[0].Child.(woxwidget.Container)

	if row.Color.A != 50 || row.Color.R != highlight.R || row.Color.G != highlight.G || row.Color.B != highlight.B {
		t.Fatalf("highlighted choice fill = %#v, want launcher-style 25%% alpha of %#v", row.Color, highlight)
	}
}

func TestFilterableSettingsChoiceUsesSharedScrollbarAndRoundedEnds(t *testing.T) {
	choices := make([]SettingsChoice, 10)
	for index := range choices {
		choices[index] = SettingsChoice{Value: string(rune('a' + index)), Label: "Choice"}
	}
	props := SettingsChoiceProps{
		ID: "fonts", Width: 640, Height: 480, Anchor: woxui.Rect{X: 100, Y: 80, Width: 320, Height: 34}, Filterable: true,
		CurrentValue: choices[0].Value, Theme: woxcomponent.Theme{}, Choices: choices,
	}
	state := &settingsChoiceState{}
	state.InitState(woxwidget.StateContext{}, props)

	stack := state.Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	menuScope := stack.Children[1].Child.(woxwidget.FocusScope)
	menuStack := menuScope.Child.(woxwidget.Semantics).Child.(woxwidget.Stack)
	menuContent := menuStack.Children[0].Child.(woxwidget.Container)
	children := menuContent.Child.(woxwidget.Flex).Children
	searchLayer := children[0].(woxwidget.Stack)
	if _, ok := searchLayer.Children[0].Child.(woxwidget.Painter); !ok {
		t.Fatalf("search background type = %T, want rounded-end painter", searchLayer.Children[0].Child)
	}
	scrollbar := children[1].(woxwidget.Stateful)
	scrollProps := scrollbar.Widget.(woxcomponent.ScrollViewProps)
	if scrollProps.Controller != state.scrollController || scrollProps.ContentHeight != 0 {
		t.Fatalf("choice scrollbar = controller %p content hint %.0f, want measured shared controller", scrollProps.Controller, scrollProps.ContentHeight)
	}
	lastRow := scrollProps.Content.(woxwidget.Flex).Children[len(choices)-1].(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Stack)
	if _, ok := lastRow.Children[0].Child.(woxwidget.Painter); !ok {
		t.Fatalf("last row background type = %T, want rounded-end painter", lastRow.Children[0].Child)
	}
}

// TestSettingsChoiceGroupTooltip survives filtering and uses the icon bounds as its anchor.
func TestSettingsChoiceGroupTooltip(t *testing.T) {
	const explanation = "Use installed CLIs with their account quota."
	choices := []SettingsChoice{{Value: "codex-cli", Label: "Codex", Group: "Installed CLI", GroupTooltip: explanation}}
	visible := filteredSettingsChoices(choices, "codex")
	if len(visible) != 2 || visible[0].choice.GroupTooltip != explanation {
		t.Fatalf("filtered group lost tooltip: %+v", visible)
	}
	var gotInside bool
	var gotText string
	var gotAnchor woxui.Rect
	props := SettingsChoiceProps{InfoIcon: &woxui.Image{}, OnTooltip: func(inside bool, text string, anchor woxui.Rect) {
		gotInside, gotText, gotAnchor = inside, text, anchor
	}}
	header := settingsChoiceGroupHeader(320, visible[0].choice, 0, props).(woxwidget.Semantics)
	if header.Description != explanation {
		t.Fatal("group explanation is missing from accessibility semantics")
	}
	children := header.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
	if len(children) != 2 {
		t.Fatalf("group children = %d, want label and info icon", len(children))
	}
	hover := children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	anchor := woxui.Rect{X: 150, Y: 80, Width: 28, Height: 28}
	hover.OnHoverAt(true, anchor)
	if !gotInside || gotText != explanation || gotAnchor != anchor {
		t.Fatal("hover did not forward the group tooltip and icon bounds")
	}
	hover.OnHoverAt(false, anchor)
	if gotInside {
		t.Fatal("hover exit did not dismiss the tooltip")
	}
	visible[0].choice.GroupTooltip = ""
	header = settingsChoiceGroupHeader(320, visible[0].choice, 0, props).(woxwidget.Semantics)
	if len(header.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children) != 1 {
		t.Fatal("group without explanation should not show an icon")
	}
}
