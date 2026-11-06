package woxview

import (
	"encoding/json"
	"fmt"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestStateControlFills checks the final composited pixels, including native hover
// feedback, explicit transparency, and controls nested inside decorated nodes.
func TestStateControlFills(t *testing.T) {
	for _, tc := range []struct {
		kind, props string
		x, y        float32
	}{
		{"switch", ``, 30, 12},
		{"checkbox", ``, 9, 9},
		{"slider", `"min":0,"max":100,`, 75, 12},
		{"hotkey", ``, 4, 4},
		{"code", `"code":"x",`, 3, 35},
		{"table", `"columns":[{"key":"c","label":""}],"rows":[{"key":"r","cells":[""]}],`, 30, 60},
		{"tag", `"text":"Tag",`, 3, 9},
		{"badge", `"text":"Tag",`, 3, 7},
		{"progress", `"value":0.25,`, 75, 12},
		{"loading", ``, 12, 12},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			v := NewView()
			background := "#ff000080"
			set := func() {
				width, height := 100, 24
				if tc.kind == "loading" {
					width = 24
				}
				if tc.kind == "table" {
					height = 80
				}
				if tc.kind == "code" {
					height = 100
				}
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"key":"control","props":{%s"style":{"width":%d,"height":%d},"hoverStyle":{"backgroundColor":%q,"color":"#00ff00","borderWidth":0,"borderRadius":0,"opacity":0.5}}}]}}`, tc.kind, tc.props, width, height, background)))
			}
			set()
			theme := component.ControlTheme{Text: ui.Color{B: 255, A: 255}, Border: ui.Color{B: 255, A: 255}, Surface: ui.Color{B: 255, A: 255}}
			host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			statePixel(t, host, 1, tc.x, tc.y)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: tc.x, Y: tc.y}})
			for i, scale := range []float32{1, 1.5, 2} {
				theme.DensityScale = []float32{0.9, 1, 1.1}[i]
				if got := statePixel(t, host, scale, tc.x, tc.y); got != (ui.Color{R: 64, A: 64}) {
					t.Fatalf("DPI %v density %v: fill %+v", scale, theme.DensityScale, got)
				}
			}
			background = "#00000000"
			set()
			if got := statePixel(t, host, 2, tc.x, tc.y); got != (ui.Color{}) {
				t.Fatalf("transparent override: %+v", got)
			}
		})
	}
}

// TestStateControlFocus survives insertion/removal of paint wrappers and the
// observers themselves. Keyboard activation must keep reaching the same control.
func TestStateControlFocus(t *testing.T) {
	for _, tc := range []struct{ kind, props, suffix string }{
		{"button", `"label":"Go","onPress":{"$handler":"act"},`, ""},
		{"switch", `"onChange":{"$handler":"act"},`, ""},
		{"checkbox", `"onChange":{"$handler":"act"},`, ""},
		{"slider", `"min":0,"max":100,"onChange":{"$handler":"act"},`, ""},
		{"hotkey", `"onChange":{"$handler":"act"},`, "/field"},
		{"list", `"options":[{"value":"a","label":"A"}],"onChange":{"$handler":"act"},`, "/option:YQ"},
		{"radio-group", `"options":[{"value":"a","label":"A"}],"onChange":{"$handler":"act"},`, "/option:YQ"},
		{"tabs", `"items":[{"value":"a","label":"A"}],"onChange":{"$handler":"act"},`, "/option:YQ"},
		{"multi-select", `"options":[{"value":"a","label":"A"}],"onChange":{"$handler":"act"},`, "/option:YQ"},
		{"tags", `"options":[{"value":"a","label":"A"}],"onChange":{"$handler":"act"},`, "/option:YQ"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			v := NewView()
			set := func(styles string) {
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"key":"control","props":{%s%s"style":{"width":150}}}]}}`, tc.kind, tc.props, styles)))
			}
			styles := `"focusStyle":{"borderColor":"#ff0000","opacity":0.5},"hoverStyle":{"backgroundColor":"#0000ff"},`
			set(styles)
			events := 0
			host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: func(HandlerID, json.RawMessage) { events++ }})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			id := "woxview/column0/k:control/" + tc.kind + tc.suffix
			statePixel(t, host, 1, 1, 1)
			if !host.FocusAutomationID(id) {
				t.Fatal("cannot focus", id)
			}
			before := automationBounds(t, host, id)
			for i, patch := range []string{styles, "", styles} {
				set(patch)
				host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: before.X + before.Width/2, Y: before.Y + before.Height/2}})
				for range 3 {
					statePixel(t, host, []float32{1, 1.5, 2}[i], 1, 1)
				}
				if host.FocusedKey() != widget.Key(id) || automationBounds(t, host, id) != before {
					t.Fatalf("lost focus or geometry: key=%q", host.FocusedKey())
				}
				host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
			}
			event := ui.KeyEvent{Key: ui.KeyEnter, Down: true}
			if tc.kind == "slider" {
				event.Key = ui.KeyArrowRight
			} else if tc.kind == "hotkey" {
				event.Key, event.Modifiers = "k", ui.KeyModifierControl
			}
			host.Key(event)
			if events != 1 {
				t.Fatalf("activation events=%d", events)
			}
		})
	}
}

// TestStateChoiceSurfaces keeps selection legible while unselected rows and
// nested tags expose a single translucent group fill through native hover.
func TestStateChoiceSurfaces(t *testing.T) {
	for _, kind := range []string{"list", "radio-group", "tabs", "multi-select", "tags"} {
		t.Run(kind, func(t *testing.T) {
			v := NewView()
			options, value := "options", `"a"`
			if kind == "tabs" {
				options = "items"
			}
			if kind == "tags" || kind == "multi-select" {
				value = `["a"]`
			}
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"key":"group","props":{"value":%s,%q:[{"value":"a","label":"Alpha"},{"value":"b","label":"Beta"}],"style":{"width":180},"hoverStyle":{"backgroundColor":"#ff000080","color":"#00ff00","opacity":0.5}}}]}}`, kind, value, options)))
			theme := component.ControlTheme{Text: ui.Color{A: 255}, SelectionBackground: ui.Color{B: 255, A: 255}}
			host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			id := "woxview/column0/k:group/" + kind
			for i, scale := range []float32{1, 1.5, 2} {
				theme.DensityScale = []float32{0.9, 1, 1.1}[i]
				statePixel(t, host, scale, 1, 1)
				b := automationBounds(t, host, id+"/option:Yg")
				x, y := b.X+b.Width-10, b.Y+b.Height-10
				host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: x, Y: y}})
				if got := statePixel(t, host, scale, x, y); got != (ui.Color{R: 64, A: 64}) {
					t.Fatalf("unselected DPI %v: %+v at %v,%v bounds %+v", scale, got, x, y, b)
				}
				a := automationBounds(t, host, id+"/option:YQ")
				if got := statePixel(t, host, scale, a.X+a.Width-10, a.Y+a.Height-10); got.B == 0 {
					t.Fatalf("lost selection surface: %+v", got)
				}
			}
		})
	}
}

// TestStatePaintKeepsSiblingLayout catches newly inserted paint containers
// accidentally activating width, height, or padding ignored by a native control.
func TestStatePaintKeepsSiblingLayout(t *testing.T) {
	for _, kind := range []string{"switch", "checkbox", "hotkey", "slider", "progress", "loading"} {
		for _, axis := range []string{"row", "column"} {
			for _, baseBorder := range []string{"", `,"borderLeftWidth":2`} {
				t.Run(kind+"/"+axis+"/"+baseBorder, func(t *testing.T) {
					v := NewView()
					v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":%q,"children":[{"type":%q,"props":{"style":{"width":60,"height":55,"padding":9%s},"hoverStyle":{"backgroundColor":"#f00","borderColor":"#fff","borderWidth":2,"opacity":0.5}}},{"type":"button","key":"after","props":{"label":"After"}}]}}`, axis, kind, baseBorder)))
					host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
						return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height})
					})
					host.AttachServices(fakeServices{})
					defer host.Dispose()
					statePixel(t, host, 1, 12, 12)
					id := "woxview/" + axis + "0/k:after/button"
					before := automationBounds(t, host, id)
					host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 12, Y: 12}})
					for _, scale := range []float32{1, 1.5, 2} {
						statePixel(t, host, scale, 12, 12)
						if after := automationBounds(t, host, id); after != before {
							t.Fatalf("state changed sibling bounds from %+v to %+v", before, after)
						}
					}
				})
			}
		}
	}
}
