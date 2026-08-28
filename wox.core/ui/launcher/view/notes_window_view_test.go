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
