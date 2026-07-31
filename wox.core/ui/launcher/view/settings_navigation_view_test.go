package view

import (
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

func TestSettingsSearchResultsShowFlutterLeadingIconLayout(t *testing.T) {
	icon := &woxui.Image{}
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 240, AvailableHeight: 200, Selected: 0, Theme: woxcomponent.Theme{},
		Results: []SettingsSearchResult{{Title: "Dictation", Subtitle: "Plugin · Dictation", Icon: icon}},
	}).(woxwidget.Container)
	scroll := panel.Child.(woxwidget.ScrollView)
	row := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	content := row.Child.(woxwidget.Flex)

	if content.Axis != woxwidget.Horizontal || content.Gap != 8 || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("search result content = %#v, want centered horizontal row with 8px gap", content)
	}
	leading := content.Children[0].(woxwidget.Align)
	image := leading.Child.(woxwidget.Image)
	if leading.Width != 24 || leading.Height != 38 || image.Source != icon || image.Width != 24 || image.Height != 24 {
		t.Fatalf("search result leading icon = %#v / %#v, want 24px Flutter icon slot", leading, image)
	}
	text := content.Children[1].(woxwidget.Align).Child.(woxwidget.Flex)
	if text.Axis != woxwidget.Vertical || len(text.Children) != 2 {
		t.Fatalf("search result text column = %#v, want title and subtitle", text)
	}
}

func TestSettingsSearchResultsHideIconInNarrowPanel(t *testing.T) {
	panel := SettingsSearchResults(SettingsSearchResultsProps{
		Width: 96, AvailableHeight: 200, Theme: woxcomponent.Theme{},
		Results: []SettingsSearchResult{{Title: "General", Subtitle: "Setting", Icon: &woxui.Image{}}},
	}).(woxwidget.Container)
	scroll := panel.Child.(woxwidget.ScrollView)
	row := scroll.Child.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)

	if content, ok := row.Child.(woxwidget.Flex); !ok || content.Axis != woxwidget.Vertical {
		t.Fatalf("narrow search result content = %T %#v, want icon-free text column", row.Child, row.Child)
	}
}
