package pluginwindow

import (
	"encoding/json"
	"testing"
	"wox/plugin"
	ui "wox/ui/runtime"
	"wox/ui/widget"
	"wox/ui/woxview"
)

type dragTestServices struct{}

func (dragTestServices) MeasureText(text string, style ui.TextStyle) (ui.TextMetrics, error) {
	return ui.TextMetrics{Size: ui.Size{Width: float32(len([]rune(text))) * style.Size / 2, Height: style.Size}}, nil
}
func (dragTestServices) Invalidate() error                         { return nil }
func (dragTestServices) InvalidateRect(ui.Rect) error              { return nil }
func (dragTestServices) SetTextInputState(ui.TextInputState) error { return nil }
func (dragTestServices) SetPointerCursor(ui.PointerCursor) error   { return nil }
func (dragTestServices) UpdateAccessibility(ui.AccessibilityTree, ui.AccessibilityActionHandler) error {
	return nil
}

// TestContentDrag exercises real WoxView hit testing with and without the option.
func TestContentDrag(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, enabled := range []bool{false, true} {
			for _, movable := range []bool{false, true} {
				drags, clicks := 0, 0
				var hover []bool
				tree, err := woxview.Parse([]byte(`{"version":1,"root":{"type":"stack","props":{"onHoverChange":{"$handler":"hover"},"style":{"width":120,"height":76}},"children":[{"type":"text","props":{"text":"Timer","style":{"left":10,"top":30}}},{"type":"icon-button","props":{"icon":"control.close","label":"Close","onPress":{"$handler":"close"},"style":{"right":0,"top":0,"width":24,"height":24}}}]}}`))
				if err != nil {
					t.Fatal(err)
				}
				view := woxview.NewView()
				view.SetTree(tree)
				host := widget.NewHost(func(frame ui.FrameInfo) widget.Widget {
					body := view.Build(woxview.BuildProps{Width: 120, Height: 76, OnEvent: func(id woxview.HandlerID, args json.RawMessage) {
						if id == "close" {
							clicks++
						} else if id == "hover" {
							var values []bool
							if err := json.Unmarshal(args, &values); err != nil || len(values) != 1 {
								t.Fatalf("hover args: %s", args)
							}
							hover = append(hover, values[0])
						}
					}})
					return contentDrag(plugin.WindowOptions{Movable: &movable, DragContent: &enabled}, body, func() { drags++ })
				})
				host.AttachServices(dragTestServices{})
				host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 120, Height: 76}, PixelSize: ui.PixelSize{Width: int(120 * scale), Height: int(76 * scale)}, Scale: scale})
				pointer := func(kind ui.PointerEventKind, button ui.PointerButton, x, y float32) {
					host.Pointer(ui.PointerEvent{Kind: kind, Button: button, Position: ui.Point{X: x, Y: y}})
				}
				pointer(ui.PointerDown, ui.PointerButtonPrimary, 15, 35)
				pointer(ui.PointerMove, 0, 25, 35)
				pointer(ui.PointerUp, ui.PointerButtonPrimary, 25, 35)
				want := 0
				if enabled && movable {
					want = 1
				}
				if drags != want {
					t.Fatalf("scale=%v enabled=%v movable=%v: drags=%d", scale, enabled, movable, drags)
				}
				pointer(ui.PointerDown, ui.PointerButtonPrimary, 100, 12)
				pointer(ui.PointerUp, ui.PointerButtonPrimary, 100, 12)
				if clicks != 1 || drags != want {
					t.Fatalf("button: clicks=%d drags=%d", clicks, drags)
				}
				pointer(ui.PointerDown, ui.PointerButtonPrimary, 100, 12)
				pointer(ui.PointerMove, 0, 110, 12)
				pointer(ui.PointerUp, ui.PointerButtonPrimary, 110, 12)
				if drags != want || clicks != 2 {
					t.Fatalf("button drag: clicks=%d drags=%d", clicks, drags)
				}
				pointer(ui.PointerLeave, 0, 0, 0)
				if len(hover) != 2 || !hover[0] || hover[1] {
					t.Fatalf("hover transitions: %v", hover)
				}
				host.Dispose()
			}
		}
	}
}

// TestContentDragSurvivesUpdates covers the hover and countdown rebuilds between press and movement.
func TestContentDragSurvivesUpdates(t *testing.T) {
	view := woxview.NewView()
	setTree := func(time string, showClose bool) {
		close := ""
		if showClose {
			close = `,{"type":"icon-button","key":"close","props":{"icon":"control.close","label":"Close","onPress":{"$handler":"close"},"style":{"right":4,"top":4,"width":24,"height":24}}}`
		}
		data := `{"version":1,"root":{"type":"stack","props":{"onHoverChange":{"$handler":"hover"},"style":{"height":76}},"children":[{"type":"column","props":{"style":{"left":0,"right":0,"top":0,"bottom":0,"padding":8,"gap":5,"alignItems":"center","justifyContent":"center"}},"children":[{"type":"text","key":"time","props":{"text":"` + time + `","style":{"fontSize":22,"textAlign":"center"}}},{"type":"text","key":"note","props":{"text":"tea"}}]}` + close + `]}}`
		tree, err := woxview.Parse([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		view.SetTree(tree)
	}
	setTree("05:00", false)
	yes := true
	drags := 0
	host := widget.NewHost(func(frame ui.FrameInfo) widget.Widget {
		body := view.Build(woxview.BuildProps{Width: frame.Size.Width, Height: frame.Size.Height, OnEvent: func(woxview.HandlerID, json.RawMessage) {}})
		return contentDrag(plugin.WindowOptions{Movable: &yes, DragContent: &yes}, body, func() { drags++ })
	})
	host.AttachServices(dragTestServices{})
	defer host.Dispose()
	frame := func() { host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 128, Height: 76}, Scale: 1.5}) }
	frame()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerEnter, Position: ui.Point{X: 64, Y: 38}})
	setTree("05:00", true)
	frame()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 64, Y: 38}})
	setTree("04:59", true)
	frame()
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 74, Y: 38}})
	if drags != 1 {
		t.Fatalf("drag after hover and countdown updates = %d", drags)
	}
}
