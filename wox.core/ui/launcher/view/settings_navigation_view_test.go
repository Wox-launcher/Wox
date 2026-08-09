package view

import (
	"fmt"
	"runtime"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsRailMatchesFlutterSearchToNavigationGap(t *testing.T) {
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	content := rail.Child.(woxwidget.Flex)

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
	navigation := rail.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if row.Color != highlight {
		t.Fatalf("selected navigation fill = %#v, want theme highlight %#v", row.Color, highlight)
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
	navigation := rail.Child.(woxwidget.Flex).Children[1].(woxwidget.Stack)
	props := navigation.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)

	if !props.HideScrollbar {
		t.Fatal("settings rail should keep shared scrolling while hiding its scrollbar")
	}
}

func TestSettingsRailLinuxUsesPageBackground(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-specific rail background behavior")
	}
	background := woxui.Color{R: 248, G: 248, B: 248, A: 255}
	rail := SettingsRail(SettingsRailProps{
		Width: 260, Height: 600, SearchBox: woxwidget.Container{Width: 232, Height: 50}, Theme: woxcomponent.Theme{Background: background},
	}).(woxwidget.Stack).Children[0].Child.(woxwidget.Container)

	if rail.Color != background {
		t.Fatalf("linux settings rail background = %#v, want page background %#v", rail.Color, background)
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
	props := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)

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
	props := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	text := row.Child.(woxwidget.Flex)

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
	props := panel.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)

	if content, ok := row.Child.(woxwidget.Flex); !ok || content.Axis != woxwidget.Vertical {
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
	scrollbar := panel.Child.(woxwidget.Stateful)
	props := scrollbar.Widget.(woxcomponent.ScrollViewProps)

	if props.Key != "settings-search-results" || props.ContentHeight != 0 || props.KeepVisible == nil {
		t.Fatalf("settings search scrollbar = key %q content hint %.0f keep visible %v, want measured shared overflow surface", props.Key, props.ContentHeight, props.KeepVisible)
	}
}
