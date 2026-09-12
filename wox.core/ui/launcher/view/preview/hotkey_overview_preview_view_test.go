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
	tag, ok := hotkeyOverviewCountTag("18 shortcuts", 1, hotkeyOverviewAccent).(woxwidget.Container)
	if !ok {
		t.Fatalf("count tag = %T, want container", hotkeyOverviewCountTag("18 shortcuts", 1, hotkeyOverviewAccent))
	}
	if tag.Width != 0 || tag.Radius != 5 || tag.Height != 24 {
		t.Fatalf("count tag size = %.0fx%.0f radius %.0f, want intrinsic width x24 radius 5", tag.Width, tag.Height, tag.Radius)
	}
	centered, ok := tag.Child.(woxwidget.Flex)
	if !ok || centered.Axis != woxwidget.Vertical || centered.MainAxisAlignment != woxwidget.MainAxisCenter {
		t.Fatalf("count tag child = %#v, want vertically centered label", tag.Child)
	}
}

func TestHotkeyOverviewHeaderKeepsCountTagUnclipped(t *testing.T) {
	header, ok := hotkeyOverviewHeader(HotkeyOverviewPreviewProps{
		Title: "查看所有 Wox 快捷键", Subtitle: "按生效场景查看当前可用的 Wox 快捷键。", Count: "{count} 个快捷键",
	}, 46, 1, woxui.Color{A: 255}, woxui.Color{A: 255}, hotkeyOverviewAccent, 21).(woxwidget.Flex)
	if !ok {
		t.Fatalf("header = %T, want horizontal flex", header)
	}
	title, ok := header.Children[1].(woxwidget.Expanded)
	if !ok {
		t.Fatalf("header title = %#v, want expanded text column", header.Children[1])
	}
	text, ok := title.Child.(woxwidget.Container)
	if !ok {
		t.Fatalf("header text = %#v, want expanded container", title.Child)
	}
	column, ok := text.Child.(woxwidget.Flex)
	if !ok || column.Axis != woxwidget.Vertical {
		t.Fatalf("header column = %#v, want vertical flex", text.Child)
	}
	titleRow, ok := column.Children[0].(woxwidget.Flex)
	if !ok || titleRow.Axis != woxwidget.Horizontal || titleRow.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("title row = %#v, want a centered horizontal flex", column.Children[0])
	}
	tag, ok := titleRow.Children[1].(woxwidget.Container)
	if !ok || tag.Width != 0 || tag.Height != 24 {
		t.Fatalf("count tag = %#v, want an unclipped intrinsic 24-high pill", titleRow.Children[1])
	}
}

func TestHotkeyOverviewDetailRowCentersShortcutAndSource(t *testing.T) {
	row, ok := hotkeyOverviewEntryRow(HotkeyOverviewPreviewEntry{
		RawShortcut: "shift+alt+l", Labels: []string{"Shift", "Alt", "L"}, Action: "Add to LinkedIn", Detail: "ld {wox:selected_text}", Source: "用户",
	}, 600, 50, 1, woxui.Color{A: 255}, woxui.Color{A: 255}).(woxwidget.Container)
	if !ok {
		t.Fatalf("row = %T, want padded container", row)
	}
	align, ok := row.Child.(woxwidget.Align)
	if !ok || align.Vertical != 0.5 || align.Height != 36 {
		t.Fatalf("row align = %#v, want a 36-high vertically centered slot", row.Child)
	}
	content, ok := align.Child.(woxwidget.Flex)
	if !ok || content.Axis != woxwidget.Horizontal || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("row content = %#v, want a cross-centered horizontal flex", align.Child)
	}
	if len(content.Children) != 3 {
		t.Fatalf("row columns = %d, want shortcut, action, and source", len(content.Children))
	}
}

func testHotkeyOverviewTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		PreviewText:            woxui.Color{R: 240, G: 240, B: 240, A: 255},
		PreviewPropertyContent: woxui.Color{R: 180, G: 180, B: 180, A: 255},
		PreviewSplit:           woxui.Color{R: 120, G: 120, B: 120, A: 255},
	}
}
