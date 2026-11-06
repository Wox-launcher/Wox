package woxview

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestTransitionDocumentFrames exercises all seven channels through JSON, layout,
// paint and input, including display changes and retargeting halfway through.
func TestTransitionDocumentFrames(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	set := func(key, style string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":%q,"props":{"label":"Box","onPress":{"$handler":"box"},"transition":{"properties":["opacity","backgroundColor","width","height","translateX","translateY","scale"],"duration":200,"curve":"linear"},"style":%s}}]}}`, key, style)))
	}
	set("box", `{"width":40,"height":20,"backgroundColor":"#ff0000"}`)
	if got := statePixel(t, host, 1, 20, 10); got.R != 255 {
		t.Fatalf("initial paint: %+v", got)
	}
	set("box", `{"width":80,"height":40,"backgroundColor":"#0000ff80","opacity":0.5,"translateX":20,"translateY":20,"scale":2}`)
	before, _ := json.Marshal(v.Tree().Root)
	statePixel(t, host, 1.25, 20, 10)
	now = now.Add(100 * time.Millisecond)
	got := statePixel(t, host, 1.5, 30, 15)
	if got.R < 70 || got.R > 73 || got.B < 70 || got.B > 73 || got.A < 142 || got.A > 145 {
		t.Fatalf("midpoint RGBA: %+v", got)
	}
	bounds := automationBounds(t, host, "woxview/column0/k:box/view#press")
	if bounds.Width != 60 || bounds.Height != 30 {
		t.Fatalf("midpoint layout: %+v", bounds)
	}
	if edge := statePixel(t, host, 2, 80, 20); edge.A == 0 {
		t.Fatal("scale/translation did not move paint beyond layout")
	}
	tap(host, bounds)
	if len(events) != 1 {
		t.Fatal("animation lost handler")
	}
	after, _ := json.Marshal(v.Tree().Root)
	if string(before) != string(after) {
		t.Fatal("animation mutated source tree")
	}
	set("box", `{"width":20,"height":20,"backgroundColor":"#00ff00"}`)
	statePixel(t, host, 1, 10, 10)
	if b := automationBounds(t, host, "woxview/column0/k:box/view#press"); b.Width != 60 {
		t.Fatalf("retarget jumped: %+v", b)
	}
	now = now.Add(100 * time.Millisecond)
	statePixel(t, host, 1, 10, 10)
	if b := automationBounds(t, host, "woxview/column0/k:box/view#press"); b.Width != 40 {
		t.Fatalf("retarget midpoint: %+v", b)
	}
	set("new", `{"width":90,"height":20,"backgroundColor":"#00ff00"}`)
	statePixel(t, host, 1, 10, 10)
	if b := automationBounds(t, host, "woxview/column0/k:new/view#press"); b.Width != 90 {
		t.Fatalf("new key reused animation: %+v", b)
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
	if v.HasHandler("box") {
		t.Fatal("deleted node retained handler")
	}
	statePixel(t, host, 1, 10, 10)
	tap(host, bounds)
	if len(events) != 1 {
		t.Fatal("deleted node still interactive")
	}
	set("box", `{"width":100,"height":20,"backgroundColor":"#00ff00"}`)
	statePixel(t, host, 1, 10, 10)
	if b := automationBounds(t, host, "woxview/column0/k:box/view#press"); b.Width != 100 {
		t.Fatal("remount reused removed timeline")
	}
}

// TestTransitionInlineFeedback shares one timeline across wrapped span fragments.
func TestTransitionInlineFeedback(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":%v,"style":{"width":70}},"children":[{"type":"span","key":"run","props":{"text":"abcdef abcdef","transition":{"properties":["opacity","backgroundColor"],"duration":200,"curve":"linear"},"hoverStyle":{"backgroundColor":"#ff0000","opacity":0.5}}}]}}`, selectable)))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		now := time.Unix(1, 0)
		host.SetAnimationClock(func() time.Time { return now })
		statePixel(t, host, 1, 4, 4)
		bounds := automationBounds(t, host, "woxview/text0/k:run/span")
		host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 4, Y: 4}})
		statePixel(t, host, 1, 4, 4)
		now = now.Add(100 * time.Millisecond)
		for _, y := range []float32{bounds.Height/2 - 4, bounds.Height - 4} {
			got := statePixel(t, host, 1.5, 4, y)
			if got.R < 46 || got.R > 49 || got.A < 94 || got.A > 97 {
				t.Fatalf("inline midpoint selectable=%v y=%g: %+v", selectable, y, got)
			}
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
		statePixel(t, host, 1.5, 4, bounds.Height-4)
		now = now.Add(200 * time.Millisecond)
		if got := statePixel(t, host, 2, 4, bounds.Height-4); got != (ui.Color{}) {
			t.Fatalf("leave did not finish: %+v", got)
		}
	}
}

// TestTransitionValidation rejects unusable channels rather than silently ignoring them.
func TestTransitionValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"properties":[]}`, `{"properties":null}`, `{"properties":["rotate"]}`, `{"properties":["opacity","opacity"]}`, `{"properties":["opacity"],"duration":-1}`, `{"properties":["opacity"],"duration":60001}`, `{"properties":["opacity"],"curve":"spring"}`, `{"properties":["opacity"],"extra":1}`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"transition":` + raw + `}}}`)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	_, err := Parse([]byte(`{"version":1,"root":{"type":"text","children":[{"type":"span","props":{"text":"x","transition":{"properties":["width"]}}}]}}`))
	if err == nil || !strings.Contains(err.Error(), "span transition") {
		t.Fatalf("span validation: %v", err)
	}
	for _, raw := range []string{`null`, `{"properties":["opacity"]}`, `{"properties":["opacity"],"duration":0}`, `{"properties":["opacity"],"duration":60000,"curve":"easeOutBack"}`} {
		mustParse(t, `{"version":1,"root":{"type":"view","props":{"transition":`+raw+`}}}`)
	}
}

// TestTransitionCurvesAndCancellation verifies public timing and explicit zero,
// including natural-size boundaries and a newly-created view in the same Host.
func TestTransitionCurvesAndCancellation(t *testing.T) {
	for curve, midpoint := range map[string]float32{"linear": 45, "easeInOut": 26.25, "": 26.25, "easeOutBack": 101.74097} {
		t.Run(curve, func(t *testing.T) {
			v := NewView()
			theme := component.ControlTheme{DensityScale: 1}
			host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
				return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme, OnEvent: func(HandlerID, json.RawMessage) {}})
			})
			host.AttachServices(fakeServices{})
			defer host.Dispose()
			now := time.Unix(1, 0)
			host.SetAnimationClock(func() time.Time { return now })
			set := func(width string, duration string) {
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":"box","props":{"onPress":{"$handler":"box"},"transition":{"properties":["width"],"curve":%q%s},"style":{"width":%s,"height":20}}}]}}`, curve, duration, width)))
			}
			width := func() float32 {
				renderLayout(host, 200, 120, 1.5)
				return automationBounds(t, host, "woxview/column0/k:box/view#press").Width
			}
			set("20", "")
			if width() != 20 {
				t.Fatal("initial target animated")
			}
			set("120", "")
			width()
			now = now.Add(50 * time.Millisecond)
			if got := width(); math.Abs(float64(got-midpoint)) > .001 {
				t.Fatalf("curve midpoint %g want %g", got, midpoint)
			}
			set("80", `,"duration":0`)
			if width() != 80 {
				t.Fatal("zero duration did not snap")
			}
			set("60", "")
			if width() != 60 {
				t.Fatal("cancelled channel retained")
			}
			set("0", "")
			width()
			set(`"$space4"`, "")
			if width() != 16 {
				t.Fatal("natural-size change did not reset timeline")
			}
			theme.DensityScale = 1.1
			width()
			now = now.Add(200 * time.Millisecond)
			if got := width(); math.Abs(float64(got-theme.Scaled(16))) > .001 {
				t.Fatalf("density target %g", got)
			}
			v = NewView()
			set("100", "")
			if width() != 100 {
				t.Fatal("new document owner inherited prior timeline")
			}
		})
	}
}

// TestTransitionCachedHover follows local state both into and out of hover while
// preserving the parent Boundary cache and the producer's authored base color.
func TestTransitionCachedHover(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[{"type":"view","props":{"transition":{"properties":["backgroundColor","opacity"],"duration":200,"curve":"linear"},"style":{"width":100,"height":60,"backgroundColor":"#0000ff"},"hoverStyle":{"backgroundColor":"#ff0000","opacity":0.5}}}]}}`))
	builds := 0
	onBuild := func() { builds++ }
	host := widget.NewHost(func(info ui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: info.Size.Width, Height: info.Size.Height})
		return widget.Boundary[cachedFormProps]{Key: "transition-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 50, 40)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 50, Y: 40}})
	if got := statePixel(t, host, 1, 50, 40); got.B != 255 {
		t.Fatalf("hover start jumped: %+v", got)
	}
	now = now.Add(100 * time.Millisecond)
	if got := statePixel(t, host, 1.5, 50, 40); got.R < 95 || got.R > 96 || got.B < 95 || got.B > 96 {
		t.Fatalf("hover midpoint: %+v", got)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	statePixel(t, host, 1.5, 50, 40)
	now = now.Add(200 * time.Millisecond)
	if got := statePixel(t, host, 2, 50, 40); got != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("hover return: %+v", got)
	}
	settled := builds
	statePixel(t, host, 2, 50, 40)
	if builds != settled {
		t.Fatal("settled transition invalidated parent cache")
	}
}
