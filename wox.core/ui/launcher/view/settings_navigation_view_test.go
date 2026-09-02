package view

import (
	"fmt"
	"runtime"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func settingsRailContent(rail woxwidget.Container) woxwidget.Flex {
	builder := rail.Child.(woxwidget.LayoutBuilder)
	return builder.Build(woxui.Size{
		Width: rail.Width - rail.Padding.Left - rail.Padding.Right, Height: rail.Height - rail.Padding.Top - rail.Padding.Bottom,
	}).(woxwidget.Flex)
}

func settingsSearchScroll(panel woxwidget.Container) woxcomponent.ScrollViewProps {
	builder := panel.Child.(woxwidget.LayoutBuilder)
	built := builder.Build(woxui.Size{
		Width: panel.Width - panel.Padding.Left - panel.Padding.Right, Height: panel.Height - panel.Padding.Top - panel.Padding.Bottom,
	}).(woxwidget.Stateful)
	return built.Widget.(woxcomponent.ScrollViewProps)
}

func TestSettingsRailMatchesFlutterSearchToNavigationGap(t *testing.T) {
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	content := settingsRailContent(rail)

	if content.Gap != 4 {
		t.Fatalf("settings rail search-to-navigation gap = %v, want Flutter's 4px navigation inset", content.Gap)
	}
}

func TestSettingsRailSelectedItemUsesThemeHighlight(t *testing.T) {
	highlight := woxui.Color{R: 80, G: 160, B: 145, A: 255}
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{SelectedBackground: highlight},
		Items: []SettingsNavItem{{ID: "themes.installed", Label: "Installed Themes", Selected: true}},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	navigation := settingsRailContent(rail).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := focusedControlGesture(props.Content.(woxwidget.Flex).Children[0]).Child.(woxwidget.Container)

	if row.Color != highlight {
		t.Fatalf("selected navigation fill = %#v, want theme highlight %#v", row.Color, highlight)
	}
}

func settingsRailItemLabel(item woxwidget.Widget) woxwidget.Text {
	child := item.(woxwidget.Semantics).Child
	if focusable, ok := child.(woxwidget.Focusable); ok {
		child = focusable.Child
		if stateful, ok := child.(woxwidget.Stateful); ok {
			child = stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget)
		}
	}
	row := child.(woxwidget.Gesture).Child.(woxwidget.Container)
	return row.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text)
}

func TestSettingsRailUsesRegularLabelWeight(t *testing.T) {
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{},
		Items: []SettingsNavItem{
			{ID: "network", Label: "Network"},
			{ID: "data", Label: "Data", Parent: true},
		},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	navigation := settingsRailContent(rail).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.Flex).Children

	destination := settingsRailItemLabel(rows[0])
	group := settingsRailItemLabel(rows[1])
	if destination.Style.Size != 13 || destination.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("destination label = %+v, want 13 regular", destination.Style)
	}
	if group.Style.Size != 13 || group.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("group header label = %+v, want the same 13 regular weight as destinations", group.Style)
	}
}

func TestSettingsSearchBoxUsesRailItemColor(t *testing.T) {
	toolbar := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	subtitle := woxui.Color{R: 255, A: 255}
	box := SettingsSearchBox(SettingsSearchBoxProps{
		Width: 232, Placeholder: "Search settings",
		Theme: woxcomponent.Theme{ToolbarText: toolbar, ResultSubtitle: subtitle, ResultTitle: woxui.Color{A: 255}},
	}).(woxwidget.Container)
	field := box.Child.(woxwidget.Container)
	wantBorder := toolbar
	wantBorder.A = 170
	if field.BorderColor != wantBorder {
		t.Fatalf("settings search border = %#v, want rail ToolbarText %#v", field.BorderColor, wantBorder)
	}
	input := field.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.Theme.ResultSubtitle != toolbar {
		t.Fatalf("settings search hint token = %#v, want ToolbarText so it matches unselected rail items", input.Theme.ResultSubtitle)
	}
}

func TestSettingsRailHoversDestinationsButNotGroupHeaders(t *testing.T) {
	text := woxui.Color{R: 180, G: 190, B: 200, A: 255}
	clicked := 0
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{ToolbarText: text},
		Items: []SettingsNavItem{
			{ID: "general", Label: "General", OnTap: func() { clicked++ }},
			{ID: "plugins", Label: "Plugins", Parent: true, OnTap: func() { clicked++ }},
		},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	navigation := settingsRailContent(rail).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.Flex).Children

	destination := rows[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Stateful)
	destinationGesture := destination.CreateState().Build(woxwidget.StateContext{}, destination.Widget).(woxwidget.Gesture)
	if destinationGesture.OnTap == nil || destinationGesture.OnHoverAt == nil {
		t.Fatal("settings destination should be clickable and hoverable")
	}
	destinationGesture.OnTap()

	group := rows[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	if group.OnTap != nil || group.OnHoverAt != nil {
		t.Fatal("settings group header should not be clickable or hoverable")
	}
	if clicked != 1 {
		t.Fatalf("settings destination taps = %d, want only the clickable item", clicked)
	}
}

func TestSettingsRailUsesSharedScrollWithoutScrollbar(t *testing.T) {
	items := make([]SettingsNavItem, 12)
	for index := range items {
		items[index] = SettingsNavItem{ID: fmt.Sprintf("item-%d", index), Label: "Setting"}
	}
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 300, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Items: items, Theme: woxcomponent.Theme{},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	navigation := settingsRailContent(rail).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)

	if !props.HideScrollbar {
		t.Fatal("settings rail should keep shared scrolling while hiding its scrollbar")
	}
}

func TestSettingsRailBackgroundMatchesWindowMaterial(t *testing.T) {
	theme := woxcomponent.Theme{
		Background:  woxui.Color{R: 24, G: 29, B: 38, A: 242},
		ToolbarText: woxui.Color{R: 166, G: 176, B: 190, A: 255},
	}
	overlay := settingsColorAlpha(theme.ToolbarText, 9)

	if got := settingsRailBackground(theme, false, true); got != overlay {
		t.Fatalf("non-linux rail = %#v, want toolbar overlay %#v", got, overlay)
	}
	if got := settingsRailBackground(theme, true, false); got != theme.Background {
		t.Fatalf("opaque linux rail = %#v, want page background %#v", got, theme.Background)
	}
	if got := settingsRailBackground(theme, true, true); got.A != 0 {
		t.Fatalf("translucent linux rail = %#v, want no extra fill", got)
	}
}

func TestSettingsRailLinuxUsesPageBackground(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific rail background behavior")
	}
	if woxui.HasNativeWindowMaterial() {
		t.Skip("translucent linux rails stay empty so they match the page wash")
	}
	background := woxui.Color{R: 248, G: 248, B: 248, A: 255}
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{Background: background},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)

	if rail.Color != background {
		t.Fatalf("linux settings rail background = %#v, want page background %#v", rail.Color, background)
	}
}

func TestSettingsRailLinuxTranslucentUsesNoFill(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific rail background behavior")
	}
	if !woxui.HasNativeWindowMaterial() {
		t.Skip("opaque linux rails match the page background")
	}
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50},
		Theme: woxcomponent.Theme{ToolbarText: woxui.Color{R: 166, G: 176, B: 190, A: 255}, Background: woxui.Color{R: 24, G: 29, B: 38, A: 242}},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)

	if rail.Color.A != 0 {
		t.Fatalf("translucent linux settings rail background = %#v, want no extra fill", rail.Color)
	}
}

func TestSettingsRailUsesToolbarOverlayWhenWindowIsTranslucent(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("linux rails do not use the windows toolbar overlay")
	}
	toolbarText := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	want := settingsColorAlpha(toolbarText, 9)
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50},
		Theme: woxcomponent.Theme{ToolbarText: toolbarText, Background: woxui.Color{R: 24, G: 29, B: 38, A: 242}},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)

	if rail.Color != want {
		t.Fatalf("translucent settings rail background = %#v, want toolbar overlay %#v", rail.Color, want)
	}
}

func TestSettingsSearchResultsShowFlutterLeadingIconLayout(t *testing.T) {
	icon := &woxui.Image{}
	border := woxui.Color{R: 96, G: 102, B: 110, A: 255}
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 240, AvailableHeight: 200, Selected: 0, Theme: woxcomponent.Theme{PreviewSplit: border},
		Results: []SettingsSearchResult{{Title: "Dictation", Subtitle: "Plugin · Dictation", Icon: icon}},
	}).(woxwidget.Container)
	if panel.Radius != 6 || panel.BorderColor != border || panel.BorderWidth != 1 {
		t.Fatalf("search panel geometry = radius %.0f border %#v/%.0f, want Flutter 6px radius with theme divider", panel.Radius, panel.BorderColor, panel.BorderWidth)
	}
	props := settingsSearchScroll(panel)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	alignment := row.Child.(woxwidget.Align)
	content := alignment.Child.(woxwidget.Flex)
	if row.Padding.Top != 0 || alignment.Height != 54 || alignment.Vertical != 0.5 {
		t.Fatalf("search result alignment = padding %#v slot %#v, want a full-height centered slot", row.Padding, alignment)
	}

	if content.Axis != woxwidget.Horizontal || content.Gap != 8 || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("search result content = %#v, want centered horizontal row with 8px gap", content)
	}
	leading := content.Children[0].(woxwidget.Align)
	image := leading.Child.(woxwidget.Image)
	if leading.Width != 24 || leading.Height != 38 || image.Source != icon || image.Width != 24 || image.Height != 24 {
		t.Fatalf("search result leading icon = %#v / %#v, want 24px Flutter icon slot", leading, image)
	}
	text := content.Children[1].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if text.Axis != woxwidget.Vertical || len(text.Children) != 2 {
		t.Fatalf("search result text column = %#v, want title and subtitle", text)
	}
}

func TestSettingsSearchResultsUseSelectedTextColors(t *testing.T) {
	titleColor := woxui.Color{R: 240, G: 245, B: 250, A: 255}
	subtitleColor := woxui.Color{R: 220, G: 235, B: 245, A: 255}
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 240, AvailableHeight: 200, Selected: 0,
		Theme:   woxcomponent.Theme{SelectedTitle: titleColor, SelectedSubtitle: subtitleColor},
		Results: []SettingsSearchResult{{Title: "AI", Subtitle: "Settings section"}},
	}).(woxwidget.Container)
	props := settingsSearchScroll(panel)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	text := row.Child.(woxwidget.Align).Child.(woxwidget.Flex)

	if title := text.Children[0].(woxwidget.Text); title.Color != titleColor {
		t.Fatalf("selected search title color = %#v, want %#v", title.Color, titleColor)
	}
	if subtitle := text.Children[1].(woxwidget.Text); subtitle.Color != subtitleColor {
		t.Fatalf("selected search subtitle color = %#v, want %#v", subtitle.Color, subtitleColor)
	}
}

func TestSettingsSearchResultsHideIconInNarrowPanel(t *testing.T) {
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 96, AvailableHeight: 200, Theme: woxcomponent.Theme{},
		Results: []SettingsSearchResult{{Title: "General", Subtitle: "Setting", Icon: &woxui.Image{}}},
	}).(woxwidget.Container)
	props := settingsSearchScroll(panel)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)

	if content, ok := row.Child.(woxwidget.Align).Child.(woxwidget.Flex); !ok || content.Axis != woxwidget.Vertical {
		t.Fatalf("narrow search result content = %T %#v, want icon-free text column", row.Child, row.Child)
	}
}

func TestSettingsSearchResultsUseSharedScrollbarWhenOverflowing(t *testing.T) {
	results := make([]SettingsSearchResult, 8)
	for index := range results {
		results[index] = SettingsSearchResult{Title: "Setting", Subtitle: "General"}
	}
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 240, AvailableHeight: 200, Selected: 7, Theme: woxcomponent.Theme{}, Results: results,
	}).(woxwidget.Container)
	props := settingsSearchScroll(panel)

	if props.Key != "settings-search-results" || props.ContentHeight != 0 || props.KeepVisible == nil {
		t.Fatalf("settings search scrollbar = key %q content hint %.0f keep visible %v, want measured shared overflow surface", props.Key, props.ContentHeight, props.KeepVisible)
	}
}
