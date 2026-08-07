package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestHotkeyOverviewFiltersEmptyEntriesAndNormalizedSearch(t *testing.T) {
	sections := hotkeyOverviewFilteredSections([]HotkeyOverviewPreviewSection{{
		Title: "Preview",
		Entries: []HotkeyOverviewPreviewEntry{
			{RawShortcut: "", Action: "Hidden"},
			{RawShortcut: "control+shift+f", Labels: []string{"Ctrl", "Shift", "F"}, Action: "Search in preview", Scope: "Preview", Source: "Built-in"},
			{RawShortcut: "g", Labels: []string{"G"}, Action: "Google", Scope: "Query Shortcuts", Source: "User"},
		},
	}}, "Ctrl Shift F")

	if len(sections) != 1 || len(sections[0].Entries) != 1 || sections[0].Entries[0].Action != "Search in preview" {
		t.Fatalf("filtered sections = %#v, want one normalized shortcut match", sections)
	}
}

func TestHotkeyOverviewViewUsesPreviewPaddingAndScrollableContent(t *testing.T) {
	view := HotkeyOverviewPreviewView(HotkeyOverviewPreviewProps{
		Width: 600, Height: 400, Title: "Hotkeys", Subtitle: "All shortcuts", Count: "{count} shortcuts", Empty: "Empty", Theme: testHotkeyOverviewTheme(),
		Sections: []HotkeyOverviewPreviewSection{{Title: "Global", Entries: []HotkeyOverviewPreviewEntry{{RawShortcut: "ctrl+j", Labels: []string{"Ctrl", "J"}, Action: "Open Wox", Source: "Settings"}}}},
	})

	container, ok := view.(woxwidget.Container)
	if !ok {
		t.Fatalf("view = %T, want preview container", view)
	}
	if container.Padding.Left != 18 || container.Padding.Top != 16 || container.Padding.Right != 16 || container.Padding.Bottom != 14 {
		t.Fatalf("preview padding = %#v, want Flutter hotkey overview padding", container.Padding)
	}
}

func TestHotkeyOverviewCountUsesTagRadius(t *testing.T) {
	tag, ok := hotkeyOverviewCountTag("18 shortcuts", 100, 1, hotkeyOverviewAccent).(woxwidget.Container)
	if !ok {
		t.Fatalf("count tag = %T, want container", hotkeyOverviewCountTag("18 shortcuts", 100, 1, hotkeyOverviewAccent))
	}
	if tag.Radius != 5 || tag.Height != 24 {
		t.Fatalf("count tag size = %.0fx%.0f radius %.0f, want 100x24 radius 5", tag.Width, tag.Height, tag.Radius)
	}
}

func testHotkeyOverviewTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		PreviewText:            woxui.Color{R: 240, G: 240, B: 240, A: 255},
		PreviewPropertyContent: woxui.Color{R: 180, G: 180, B: 180, A: 255},
		PreviewSplit:           woxui.Color{R: 120, G: 120, B: 120, A: 255},
	}
}
