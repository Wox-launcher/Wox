package woxview

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// statePixel renders complete frames so assertions cover the actual widget paint path.
func statePixel(t *testing.T, host *widget.Host, scale, x, y float32) ui.Color {
	t.Helper()
	list := &ui.DisplayList{}
	host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 120}, PixelSize: ui.PixelSize{Width: int(200 * scale), Height: int(120 * scale)}, Scale: scale})
	list.TransformScene(scale, ui.Point{})
	r, err := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Render(list); err != nil {
		t.Fatal(err)
	}
	c := r.RGBA().RGBAAt(int(x*scale), int(y*scale))
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// TestStateStyleCachedBoundary invalidates only from local state, with stable document props.
func TestStateStyleCachedBoundary(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","props":{"style":{"width":100,"height":60,"backgroundColor":"#000"},"hoverStyle":{"backgroundColor":"#f00"}}}]}}`))
	builds := 0
	onBuild := func() { builds++ }
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height})
		return widget.Boundary[cachedFormProps]{Key: "state-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 50, 40)
	before := builds
	statePixel(t, host, 1, 50, 40)
	if builds != before {
		t.Fatal("fixture did not cache")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 50, Y: 40}})
	if got := statePixel(t, host, 1.5, 50, 40); got != (ui.Color{R: 255, A: 255}) {
		t.Fatalf("cached hover remained stale: %+v", got)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	if got := statePixel(t, host, 2, 50, 40); got != (ui.Color{A: 255}) {
		t.Fatalf("cached leave remained stale: %+v", got)
	}
}

// TestStateStyleThemeAndAlpha checks live token resolution and avoids double-composited control fills.
func TestStateStyleThemeAndAlpha(t *testing.T) {
	for _, kind := range []string{"button", "text-field", "number-field", "icon-button", "select"} {
		t.Run(kind, func(t *testing.T) {
			v := NewView()
			extra := `"label":"Control"`
			if kind == "icon-button" {
				extra += `,"icon":"action.execute"`
			}
			if kind == "select" {
				extra += `,"options":[{"value":"a","label":"A"}]`
			}
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":%q,"props":{%s,"style":{"width":100,"height":60},"hoverStyle":{"backgroundColor":{"light":"#ff000080","dark":"$surface"},"borderColor":"#fff","borderWidth":"$space1","borderRadius":0,"opacity":0.5}}}]}}`, kind, extra)))
			theme := component.ControlTheme{Surface: ui.Color{B: 255, A: 128}}
			host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, Theme: theme})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			y := float32(45)
			if kind == "select" {
				y = 26
			}
			statePixel(t, host, 1, 10, y)
			host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 10, Y: y}})
			for i, density := range []float32{0.9, 1, 1.1} {
				theme.DensityScale = density
				theme.Dark = i != 0
				scale := []float32{1, 1.5, 2}[i]
				got := statePixel(t, host, scale, 10, y)
				want := ui.Color{R: 64, A: 64}
				if theme.Dark {
					want = ui.Color{B: 64, A: 64}
				}
				if got != want {
					t.Fatalf("density %v alpha: %+v want %+v", density, got, want)
				}
			}
		})
	}
}

// TestStateStyleCardPrecedence retains input and immutable props through local state changes.
func TestStateStyleCardPrecedence(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","key":"card","props":{"style":{"width":100,"height":70,"backgroundColor":"#111"},"hoverStyle":{"backgroundColor":"#0f0"},"pressedStyle":{"backgroundColor":"#00f"},"focusStyle":{"backgroundColor":"#ff0"}},"children":[{"type":"text-field","key":"input","props":{"label":"Draft","style":{"width":80,"height":32}}}]}]}}`))
	before, _ := json.Marshal(v.Tree().Root)
	events := 0
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: func(HandlerID, json.RawMessage) { events++ }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2} {
		host.ClearFocus()
		host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
		if got := statePixel(t, host, scale, 90, 60); got != (ui.Color{R: 17, G: 17, B: 17, A: 255}) {
			t.Fatalf("base: %+v", got)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 90, Y: 60}})
		if got := statePixel(t, host, scale, 90, 60); got != (ui.Color{G: 255, A: 255}) {
			t.Fatalf("hover: %+v", got)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 90, Y: 60}})
		if got := statePixel(t, host, scale, 90, 60); got != (ui.Color{B: 255, A: 255}) {
			t.Fatalf("press: %+v", got)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 90, Y: 60}})
		if got := statePixel(t, host, scale, 90, 60); got != (ui.Color{G: 255, A: 255}) {
			t.Fatalf("release: %+v", got)
		}
		if !host.FocusAutomationID("woxview/column0/k:card/view/k:input/text-field/field") {
			t.Fatal("lost field")
		}
		if got := statePixel(t, host, scale, 90, 60); got != (ui.Color{R: 255, G: 255, A: 255}) {
			t.Fatalf("focus precedence: %+v", got)
		}
		host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "x"})
	}
	after, _ := json.Marshal(v.Tree().Root)
	if string(before) != string(after) || events != 0 || v.fieldValues["woxview/column0/k:card/view/k:input/text-field"] != "xxx" {
		t.Fatalf("state mutated document, emitted RPC, or lost edits: %v", v.fieldValues)
	}
}

// TestStateStyleButtonPaintAndDisabled tests shared chrome, explicit zero, and disabled precedence.
func TestStateStyleButtonPaintAndDisabled(t *testing.T) {
	v := NewView()
	set := func(disabled bool) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"button","key":"action","props":{"label":"Action","disabled":%v,"onPress":{"$handler":"go"},"style":{"width":100,"height":50,"borderRadius":8},"hoverStyle":{"backgroundColor":"#f00","borderRadius":0,"borderWidth":0},"pressedStyle":{"backgroundColor":"#0f0"},"disabledStyle":{"backgroundColor":"#00f","color":"#fff"}}}]}}`, disabled)))
	}
	set(false)
	taps := 0
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height, OnEvent: func(HandlerID, json.RawMessage) { taps++ }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 1, 1)
	before := automationBounds(t, host, "woxview/column0/k:action/button")
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 5, Y: 5}})
	if got := statePixel(t, host, 1.5, 1, 1); got != (ui.Color{R: 255, A: 255}) {
		t.Fatalf("square hover override: %+v", got)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 5, Y: 5}})
	if got := statePixel(t, host, 2, 1, 1); got != (ui.Color{G: 255, A: 255}) {
		t.Fatalf("press override: %+v", got)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 5, Y: 5}})
	set(true)
	if got := statePixel(t, host, 2, 10, 40); got != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("disabled override: %+v", got)
	}
	tap(host, before)
	statePixel(t, host, 2, 10, 40)
	if taps != 1 || automationBounds(t, host, "woxview/column0/k:action/button") != before {
		t.Fatalf("disabled activation or layout changed: taps=%d", taps)
	}
}

// TestStateStyleFormDisabledAndRemoval protects edited values when paint contracts are replaced.
func TestStateStyleFormDisabledAndRemoval(t *testing.T) {
	v := NewView()
	set := func(disabled bool, patch string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"form","props":{"disabled":%v},"children":[{"type":"text-field","key":"input","props":{"label":"Input","style":{"width":100,"height":40},%s}}]}}`, disabled, patch)))
	}
	styles := `"hoverStyle":{"backgroundColor":"#f00"},"focusStyle":{"backgroundColor":"#0f0"},"disabledStyle":{"backgroundColor":"#00f"}`
	set(false, styles)
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 90, 30)
	field := "woxview/form0/k:input/text-field"
	host.FocusAutomationID(field + "/field")
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "draft"})
	if got := statePixel(t, host, 1.5, 90, 30); got != (ui.Color{G: 255, A: 255}) {
		t.Fatalf("field focus: %+v", got)
	}
	set(true, styles)
	if got := statePixel(t, host, 2, 90, 30); got != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("form disabled: %+v", got)
	}
	set(false, `"value":""`)
	statePixel(t, host, 1, 90, 30)
	set(false, styles)
	statePixel(t, host, 1.5, 90, 30)
	if v.fieldValues[field] != "draft" {
		t.Fatalf("style replacement reset field: %v", v.fieldValues)
	}
}

// TestStateStyleRejectsLayout accepts only paint fields and checks programmatic trees too.
func TestStateStyleRejectsLayout(t *testing.T) {
	for _, state := range []string{"hoverStyle", "pressedStyle", "focusStyle", "disabledStyle"} {
		for _, patch := range []string{`{"width":10}`, `{"padding":2}`, `{"fontSize":12}`, `{"gap":4}`, `{"floating":true}`, `{"opacity":2}`, `{"borderWidth":-1}`, `{"borderColor":"bad"}`, `{"color":{"light":"#fff"}}`, `true`} {
			if _, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{%q:%s}}}`, state, patch))); err == nil {
				t.Fatalf("accepted %s=%s", state, patch)
			}
		}
		mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{%q:{"color":{"light":"#123","dark":"$text"},"opacity":0,"borderWidth":0,"borderLeftWidth":"$space1","borderRadius":"$radiusMd"}}}}`, state))
	}
	nan := float32(math.NaN())
	p := &BoxProps{Styled: Styled{InteractionStyles: InteractionStyles{HoverStyle: &StateStyle{Opacity: &nan}}}}
	if _, err := NewTree(&Node{Type: NodeView, Props: p}); err == nil || !strings.Contains(err.Error(), "hoverStyle") {
		t.Fatalf("programmatic invalid opacity: %v", err)
	}
}

// TestStateStyleObserverRemoval forgets focus lost while the visual observer was absent.
func TestStateStyleObserverRemoval(t *testing.T) {
	v := NewView()
	set := func(patch string) {
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"text-field","key":"input","props":{"label":"Input","style":{"width":100,"height":40,"backgroundColor":"#111"}`+patch+`}}]}}`))
	}
	patch := `,"focusStyle":{"backgroundColor":"#0f0"}`
	set(patch)
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 90, 30)
	host.FocusAutomationID("woxview/column0/k:input/text-field/field")
	if got := statePixel(t, host, 1, 90, 30); got != (ui.Color{G: 255, A: 255}) {
		t.Fatalf("focus missing: %+v", got)
	}
	set("")
	statePixel(t, host, 1, 90, 30)
	host.ClearFocus()
	set(patch)
	if got := statePixel(t, host, 1.5, 90, 30); got != (ui.Color{R: 17, G: 17, B: 17, A: 255}) {
		t.Fatalf("removed observer retained focus: %+v", got)
	}
}
