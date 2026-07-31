package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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
	searchRow := menuContent.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
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
