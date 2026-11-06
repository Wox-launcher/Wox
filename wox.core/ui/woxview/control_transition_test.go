package woxview

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestTransitionNativeControlReturn verifies the complete JSON/state/shared-paint
// path, including outer decoration and transparent defaults on compound controls.
func TestTransitionNativeControlReturn(t *testing.T) {
	for _, kind := range []string{"button", "icon-button", "text-field", "tag", "table"} {
		t.Run(kind, func(t *testing.T) {
			v := NewView()
			extra, point := `"label":"Test","onPress":{"$handler":"go"},`, ui.Point{X: 10, Y: 30}
			if kind == "icon-button" {
				extra += `"icon":"action.execute",`
			}
			if kind == "text-field" {
				extra = `"label":"Test",`
			}
			if kind == "tag" {
				extra, point = `"text":"Test",`, ui.Point{X: 4, Y: 10}
			}
			if kind == "table" {
				extra, point = `"columns":[{"key":"a","label":"A"}],"rows":[{"key":"r","cells":[""]}],`, ui.Point{X: 10, Y: 60}
			}
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"key":"control","props":{%s"transition":{"properties":["backgroundColor"],"duration":200,"curve":"linear"},"style":{"width":120,"height":80,"borderWidth":1,"borderColor":"#00000000"},"hoverStyle":{"backgroundColor":"#ff000080"}}}]}}`, kind, extra)))
			before, _ := json.Marshal(v.Tree().Root)
			theme := component.ControlTheme{InputBackground: ui.Color{G: 255, A: 255}, Surface: ui.Color{B: 255, A: 255}, Text: ui.Color{G: 255, A: 255}}
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) {}})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			now := time.Unix(1, 0)
			host.SetAnimationClock(func() time.Time { return now })
			pixel := func(scale float32) ui.Color { return statePixel(t, host, scale, point.X, point.Y) }
			initial := pixel(1)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: point})
			if got := pixel(1); got != initial {
				t.Fatalf("hover start changed %+v to %+v", initial, got)
			}
			now = now.Add(100 * time.Millisecond)
			mid := pixel(1.5)
			now = now.Add(100 * time.Millisecond)
			if got := pixel(2); got != (ui.Color{R: 128, A: 128}) {
				t.Fatalf("override was not painted once: %+v", got)
			}
			host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
			if got := pixel(2); got != (ui.Color{R: 128, A: 128}) {
				t.Fatalf("leave jumped or double-composited: %+v", got)
			}
			now = now.Add(100 * time.Millisecond)
			if got := pixel(1.5); got != mid {
				t.Fatalf("return midpoint %+v want %+v", got, mid)
			}
			now = now.Add(100 * time.Millisecond)
			if got := pixel(1); got != initial {
				t.Fatalf("default fill %+v want %+v", got, initial)
			}
			after, _ := json.Marshal(v.Tree().Root)
			if string(before) != string(after) {
				t.Fatal("native fill resolution mutated document")
			}
		})
	}
}

// TestTransitionNativeThemeAndDraft changes native targets during focused editing.
func TestTransitionNativeThemeAndDraft(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"field","props":{"transition":{"properties":["backgroundColor"],"duration":200,"curve":"linear"},"style":{"width":120,"height":80,"backgroundColor":"$surface"}}}]}}`))
	theme := component.ControlTheme{Surface: ui.Color{G: 255, A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 100, 60)
	if !host.FocusAutomationID("woxview/column0/k:field/text-field/field") {
		t.Fatal("field not focusable")
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "draft"})
	theme.Surface = ui.Color{B: 255, A: 255}
	if got := statePixel(t, host, 1, 100, 60); got.G != 255 {
		t.Fatalf("theme start jumped %+v", got)
	}
	now = now.Add(100 * time.Millisecond)
	if got := statePixel(t, host, 1.5, 100, 60); got != (ui.Color{G: 128, B: 128, A: 255}) {
		t.Fatalf("theme midpoint %+v", got)
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "!"})
	now = now.Add(100 * time.Millisecond)
	if got := statePixel(t, host, 2, 100, 60); got != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("theme target %+v", got)
	}
	if v.fieldValues["woxview/column0/k:field/text-field"] != "draft!" {
		t.Fatal("animation reset editor or focus")
	}
}
