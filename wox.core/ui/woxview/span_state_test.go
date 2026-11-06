package woxview

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestSpanStatePaint applies one state to every wrapped fragment without changing
// paragraph layout or mutating the producer tree, including selectable paragraphs.
func TestSpanStatePaint(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		for i, density := range []float32{0.9, 1, 1.1} {
			t.Run(fmt.Sprintf("selectable=%v/density=%g", selectable, density), func(t *testing.T) {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"text","props":{"selectable":%v,"style":{"width":70}},"children":[{"type":"span","key":"run","props":{"text":"abcdef abcdef","hoverStyle":{"color":"#ffff00","backgroundColor":{"light":"#00ff00","dark":"#0000ff"},"borderColor":"#fff","borderWidth":2,"opacity":0.5},"pressedStyle":{"backgroundColor":"#ff0000"}}}]},{"type":"button","key":"after","props":{"label":"After"}}]}}`, selectable)))
				before, _ := json.Marshal(v.Tree().Root)
				events := 0
				theme := component.ControlTheme{DensityScale: density}
				host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
					return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) { events++ }})
				})
				host.AttachServices(fakeServices{})
				defer host.Dispose()
				scale := []float32{1, 1.5, 2}[i]
				statePixel(t, host, scale, 4, 4)
				sibling := automationBounds(t, host, "woxview/column0/k:after/button")
				bounds := automationBounds(t, host, "woxview/column0/text0/k:run/span")
				line := bounds.Height / 2
				host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 4, Y: 4}})
				for _, y := range []float32{line - 4, 2*line - 4} {
					if got := statePixel(t, host, scale, 4, y); got != (ui.Color{G: 127, A: 127}) {
						t.Fatalf("hover at y=%g: %+v bounds %+v", y, got, bounds)
					}
				}
				if got := statePixel(t, host, scale, 10, 0); got != (ui.Color{R: 127, G: 127, B: 127, A: 127}) {
					t.Fatalf("border: %+v", got)
				}
				if got := statePixel(t, host, scale, 3, line/2); got != (ui.Color{R: 127, G: 127, A: 127}) {
					t.Fatalf("glyph and background did not share group opacity: %+v", got)
				}
				host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 4, Y: 4}})
				if got := statePixel(t, host, scale, 4, 2*line-4); got != (ui.Color{R: 127, A: 127}) {
					t.Fatalf("shared press: %+v", got)
				}
				host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 4, Y: 4}})
				theme.Dark = true
				if got := statePixel(t, host, scale, 4, 2*line-4); got != (ui.Color{B: 127, A: 127}) {
					t.Fatalf("live theme: %+v", got)
				}
				host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
				if got := statePixel(t, host, scale, 4, 2*line-4); got != (ui.Color{}) {
					t.Fatalf("leave: %+v", got)
				}
				after, _ := json.Marshal(v.Tree().Root)
				if string(before) != string(after) || events != 0 || sibling != automationBounds(t, host, "woxview/column0/k:after/button") {
					t.Fatal("state changed producer data, events or layout")
				}
			})
		}
	}
}

// TestSpanStateSelection keeps the native selection visible above opaque state
// backgrounds, and retains selected text while the producer removes the styles.
func TestSpanStateSelection(t *testing.T) {
	clipboard := &codeClipboard{}
	component.SetClipboardProvider(clipboard)
	defer component.SetClipboardProvider(nil)
	v := NewView()
	set := func(patch string) {
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"selectable":true},"children":[{"type":"span","key":"run","props":{"text":"hello 世界",`+patch+`"style":{"color":"#fff"}}}]}}`))
	}
	set(`"hoverStyle":{"backgroundColor":"#f00","opacity":0.5},`)
	theme := component.ControlTheme{TextSelectionBackground: ui.Color{B: 255, A: 255}, TextSelectionText: ui.Color{A: 255}}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	statePixel(t, host, 1, 7, 17)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 7, Y: 17}})
	statePixel(t, host, 1, 7, 17)
	if !host.FocusAutomationID("woxview/text0/selectable") {
		t.Fatal("editor focus missing")
	}
	if err := host.SelectAllWithin("woxview/text0/selectable"); err != nil {
		t.Fatal(err)
	}
	if got := statePixel(t, host, 1.5, 7, 17); got != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("state paint obscured selection: %+v", got)
	}
	set("")
	statePixel(t, host, 2, 7, 17)
	modifier := ui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = ui.KeyModifierMeta
	}
	host.Key(ui.KeyEvent{Key: "c", Modifiers: modifier, Down: true})
	if clipboard.text != "hello 世界" {
		t.Fatalf("selection lost after style removal: %q", clipboard.text)
	}
}

// TestSpanStateFocusAndDisabled follows keyed links across producer reorder and
// suppresses active styles when the embedding surface cannot deliver the action.
func TestSpanStateFocusAndDisabled(t *testing.T) {
	v := NewView()
	set := func(reverse bool) {
		runs := []string{`{"type":"span","key":"prefix","props":{"text":"prefix "}}`, `{"type":"span","key":"link","props":{"text":"link","onPress":{"$handler":"go"},"focusStyle":{"backgroundColor":"#00f"},"disabledStyle":{"backgroundColor":"#ff0"}}}`}
		if reverse {
			runs[0], runs[1] = runs[1], runs[0]
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","children":[`+strings.Join(runs, ",")+`]}}`))
	}
	set(false)
	events := 0
	var emit func(HandlerID, json.RawMessage) = func(HandlerID, json.RawMessage) { events++ }
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, OnEvent: emit})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	id := "woxview/text0/k:link/span"
	statePixel(t, host, 1, 1, 1)
	if !host.FocusAutomationID(id) {
		t.Fatal("link not focusable")
	}
	for i, reverse := range []bool{false, true} {
		set(reverse)
		statePixel(t, host, 1, 1, 1)
		b := automationBounds(t, host, id)
		if got := statePixel(t, host, []float32{1.5, 2}[i], b.X+3, b.Y+3); got != (ui.Color{B: 255, A: 255}) || host.FocusedKey() != widget.Key(id) {
			t.Fatalf("focus after reorder: %+v key %q", got, host.FocusedKey())
		}
	}
	host.Key(ui.KeyEvent{Key: ui.KeyEnter, Down: true})
	if events != 1 {
		t.Fatal("keyboard activation missing or state emitted events")
	}
	emit = nil
	if got := statePixel(t, host, 2, 3, 3); got != (ui.Color{R: 255, G: 255, A: 255}) || host.FocusAutomationID(id) {
		t.Fatalf("disabled link state: %+v", got)
	}
}

// TestSpanStateValidation accepts the shared paint contract and rejects layout
// changes rather than silently ignoring them on inline nodes.
func TestSpanStateValidation(t *testing.T) {
	for kind, spec := range catalog {
		if _, ok := spec.newProps().(interface{ interactionStyles() InteractionStyles }); !ok {
			t.Fatalf("node %s has no state-style contract", kind)
		}
	}
	for _, name := range []string{"hoverStyle", "pressedStyle", "focusStyle", "disabledStyle"} {
		for _, bad := range []string{`{"width":2}`, `{"padding":2}`, `{"opacity":2}`, `{"color":"bad"}`, `{"borderRadius":-1}`} {
			_, err := Parse([]byte(fmt.Sprintf(`{"version":1,"root":{"type":"text","children":[{"type":"span","props":{"text":"x",%q:%s}}]}}`, name, bad)))
			if err == nil {
				t.Fatalf("accepted %s=%s", name, bad)
			}
		}
		mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","children":[{"type":"span","props":{"text":"x",%q:{"color":{"light":"#000","dark":"$text"},"borderWidth":"$space1","borderRadius":0,"opacity":0}}}]}}`, name))
	}
}

// TestSpanStateCachedTransparency drives a cached paragraph through both explicit
// transparent glyph paint and removal of that paint without replacing the tree.
func TestSpanStateCachedTransparency(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":%v},"children":[{"type":"span","props":{"text":"xx","hoverStyle":{"color":"#00000000"}}}]}}`, selectable)))
		builds := 0
		onBuild := func() { builds++ }
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			child := v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: component.ControlTheme{Text: ui.Color{R: 255, A: 255}}})
			return widget.Boundary[cachedFormProps]{Key: "cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
		})
		host.AttachServices(fakeServices{})
		statePixel(t, host, 1, 3, 7)
		count := builds
		if got := statePixel(t, host, 1, 3, 7); got != (ui.Color{R: 255, A: 255}) || builds != count {
			t.Fatalf("initial glyph or boundary cache: %+v", got)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 3, Y: 7}})
		if got := statePixel(t, host, 1.5, 3, 7); got != (ui.Color{}) {
			t.Fatalf("transparent glyph selectable=%v: %+v", selectable, got)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
		if got := statePixel(t, host, 2, 3, 7); got != (ui.Color{R: 255, A: 255}) {
			t.Fatalf("restored glyph: %+v", got)
		}
		host.Dispose()
	}
}
