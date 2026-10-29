package view

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNotesFindBarFitsNarrowWindowAndScales(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		theme := woxcomponent.Theme{Controls: woxcomponent.ControlTheme{DensityScale: density}}
		root := NotesFindBar(NotesFindProps{Size: woxui.Size{Width: 460, Height: 320}, Theme: theme, Editor: woxwidget.NewTextEditingController(""), Focus: woxwidget.NewFocusNode(), Count: "0/0"}).(woxwidget.Stack)
		if len(root.Children) != 1 {
			t.Fatal("find should not add a modal scrim")
		}
		layer := root.Children[0]
		panel := layer.Child.(woxwidget.Container)
		if layer.Left < 0 || layer.Left+panel.Width > 460 || layer.Top != NotesToolbarHeight {
			t.Fatalf("invalid placement: %+v", layer)
		}
		row := panel.Child.(woxwidget.Flex)
		used := float32(len(row.Children)-1) * row.Gap
		for _, child := range row.Children {
			switch value := child.(type) {
			case woxwidget.Stateful:
				switch props := value.Widget.(type) {
				case woxcomponent.TextFieldProps:
					used += props.Width
					innerHeight := props.Height - props.Padding.Top - props.Padding.Bottom
					if props.Padding == (woxwidget.Insets{}) || props.LineHeight > innerHeight || props.LineHeight < props.Style.Size {
						t.Fatalf("find text clips at density %v: height=%v line=%v font=%v", density, innerHeight, props.LineHeight, props.Style.Size)
					}
					if props.TextAlignmentY != .5 {
						t.Fatal("find text must be vertically centered")
					}
				case woxcomponent.IconButtonProps:
					used += props.Width
				}
			case woxwidget.Container:
				used += value.Width
			}
		}
		if used > panel.Width-panel.Padding.Left-panel.Padding.Right {
			t.Fatalf("row overflows at density %v: %v", density, used)
		}
	}
}
