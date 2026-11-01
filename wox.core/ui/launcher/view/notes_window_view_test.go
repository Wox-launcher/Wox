package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNotesWindowPaintsThemeBackground(t *testing.T) {
	background := woxui.Color{R: 22, G: 22, B: 26, A: 133}
	root := NotesWindow(NotesWindowProps{
		Width: 400, Height: 300, Label: "Notes",
		Toolbar: woxwidget.Container{Width: 400, Height: NotesToolbarHeight},
		Editor:  woxwidget.Container{Width: 400, Height: 260},
		Theme:   woxcomponent.Theme{Background: background},
	}).(woxwidget.Semantics)
	body := root.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	if body.Color != background {
		t.Fatalf("notes window fill = %#v, want theme.Background %#v", body.Color, background)
	}
}

func TestNotesFormatBarKeepsStatsCenteredAcrossModes(t *testing.T) {
	var positions [2]woxui.Rect
	for i, markdown := range []bool{false, true} {
		props := NotesFormatBarProps{
			Width: 600, StatsWidth: 80, Stats: "379 characters", Markdown: markdown,
			Actions: []NotesFormatAction{{ID: "bold", Label: "Bold"}},
		}
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return NotesFormatBar(props) })
		host.AttachServices(actionSearchHostServices{})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 600, Height: NotesFormatBarHeight}, Scale: 1})
		found := false
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "notes.format.stats" {
				positions[i] = node.Bounds
				found = true
			}
		}
		host.Dispose()
		if !found {
			t.Fatalf("markdown %v: stats missing", markdown)
		}
	}
	if positions[0].Y != positions[1].Y || positions[1].Y <= 0 {
		t.Fatalf("stats positions = %+v, want the same centered position in both modes", positions)
	}
}
