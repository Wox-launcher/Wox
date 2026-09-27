package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
	utilwindow "wox/util/window"
)

func TestDisplayArrangementKeepsNegativeOriginsInsideTheMap(t *testing.T) {
	selected := -1
	widget := DisplayArrangement(DisplayArrangementProps{
		Width: 640, Height: 360, PrimaryLabel: "Primary", TileIDPrefix: "show-display",
		Theme:    woxcomponent.ControlTheme{},
		OnSelect: func(index int) { selected = index },
		Tiles: []DisplayArrangementTile{
			{Index: 0, Bounds: utilwindow.WindowRect{X: -1920, Y: 0, Width: 1920, Height: 1080}},
			{Index: 1, IsPrimary: true, Bounds: utilwindow.WindowRect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		},
	})
	container, ok := widget.(woxwidget.Container)
	if !ok {
		t.Fatal("arrangement should be a container")
	}
	stack, ok := container.Child.(woxwidget.Stack)
	if !ok || len(stack.Children) != 2 {
		t.Fatalf("tiles = %#v", container.Child)
	}
	if stack.Children[0].Left < 0 || stack.Children[1].Left < 0 {
		t.Fatalf("tile positions = %.1f %.1f, want both inside the map", stack.Children[0].Left, stack.Children[1].Left)
	}
	if stack.Children[0].Left >= stack.Children[1].Left {
		t.Fatalf("left monitor position %.1f should stay left of %.1f", stack.Children[0].Left, stack.Children[1].Left)
	}
	gesture, ok := stack.Children[0].Child.(woxwidget.Gesture)
	if !ok || gesture.ID != "show-display-0" || gesture.OnTap == nil {
		t.Fatalf("first tile gesture = %#v", stack.Children[0].Child)
	}
	gesture.OnTap()
	if selected != 0 {
		t.Fatalf("selected = %d, want 0", selected)
	}
}

func TestWindowGroupArrangementKeepsDisplayGestures(t *testing.T) {
	widget := windowGroupDisplayArrangement(WindowGroupEditorProps{
		PrimaryDisplayLabel: "Primary", Theme: woxcomponent.ControlTheme{},
		DisplayTiles: []WindowGroupDisplayTileProps{
			{Index: 0, Bounds: utilwindow.WindowRect{Width: 1920, Height: 1080}, Slots: []WindowGroupSlotProps{{ID: "full", Cols: 1, Rows: 1, ColSpan: 1, RowSpan: 1}}},
			{Index: 1, Selected: true, Bounds: utilwindow.WindowRect{X: 1920, Width: 1920, Height: 1080}},
		},
	}, 640, 360)
	container := widget.(woxwidget.Container)
	stack := container.Child.(woxwidget.Stack)
	gesture, ok := stack.Children[0].Child.(woxwidget.Gesture)
	if !ok || gesture.ID != "window-group-display-0" {
		t.Fatalf("workspace display gesture = %#v", stack.Children[0].Child)
	}
}
