package woxview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestEnterLifecycle checks actual pixels and retained identity across producer
// updates, cancellation, completion, new keys and removal/remount.
func TestEnterLifecycle(t *testing.T) {
	for _, preset := range PresencePreset("").enumValues() {
		t.Run(preset, func(t *testing.T) {
			v := NewView()
			var events []recordedEvent
			host := layoutHost(v, &events)
			defer host.Dispose()
			now := time.Unix(1, 0)
			host.SetAnimationClock(func() time.Time { return now })
			set := func(key, enter string) {
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":%q,"props":{"label":"Enter","onPress":{"$handler":"press"},"enter":%s,"style":{"width":100,"height":80,"backgroundColor":"#ff0000","opacity":0.5}}}]}}`, key, enter)))
			}
			declaration := fmt.Sprintf(`{"type":%q}`, preset)
			set("box", declaration)
			before, _ := json.Marshal(v.Tree().Root)
			if got := statePixel(t, host, 1, 50, 40); got.A != 0 {
				t.Fatalf("initial: %+v", got)
			}
			bounds := automationBounds(t, host, "woxview/column0/k:box/view#press")
			tap(host, bounds)
			if len(events) != 1 {
				t.Fatal("entry removed input")
			}
			now = now.Add(100 * time.Millisecond)
			for _, scale := range []float32{1, 1.25, 1.5, 2} {
				if got := statePixel(t, host, scale, 50, 40); got.R < 63 || got.R > 64 || got.A != got.R {
					t.Fatalf("midpoint at %g: %+v", scale, got)
				}
				if got := automationBounds(t, host, "woxview/column0/k:box/view#press"); got != bounds {
					t.Fatalf("entry changed geometry: %+v", got)
				}
			}
			if preset == "slideUp" && statePixel(t, host, 1, 50, 2).A != 0 {
				t.Fatal("slideUp did not translate paint")
			}
			if preset == "scale" && statePixel(t, host, 1, 0, 40).A != 0 {
				t.Fatal("scale did not shrink paint")
			}
			set("box", fmt.Sprintf(`{"type":%q,"duration":1000}`, preset))
			if got := statePixel(t, host, 1.5, 50, 40); got.R < 63 || got.R > 64 {
				t.Fatalf("prop update restarted or retimed mount: %+v", got)
			}
			set("box", declaration)
			now = now.Add(100 * time.Millisecond)
			if got := statePixel(t, host, 2, 50, 40); got.R != 127 {
				t.Fatalf("completion: %+v", got)
			}
			statePixel(t, host, 2, 50, 40) // Retire the finished timeline.
			now = now.Add(time.Second)
			if got := statePixel(t, host, 1, 50, 40); got.R != 127 {
				t.Fatal("completed entry restarted")
			}
			after, _ := json.Marshal(v.Tree().Root)
			if !bytes.Equal(before, after) {
				t.Fatal("entry mutated source props")
			}
			set("new", declaration)
			if got := statePixel(t, host, 1, 50, 40); got.A != 0 {
				t.Fatal("new key inherited entry")
			}
			set("new", `null`)
			if got := statePixel(t, host, 1, 50, 40); got.R != 127 {
				t.Fatal("removing enter did not cancel")
			}
			set("new", declaration)
			if got := statePixel(t, host, 1, 50, 40); got.R != 127 {
				t.Fatal("adding enter after mount replayed it")
			}
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
			statePixel(t, host, 1, 50, 40)
			tap(host, bounds)
			if v.HasHandler("press") || len(events) != 1 {
				t.Fatal("removed node still responds")
			}
			set("new", declaration)
			if got := statePixel(t, host, 1, 50, 40); got.A != 0 {
				t.Fatal("remount inherited completed entry")
			}
		})
	}
}

// TestInlineEnter shares one mount across wrapped fragments without replacing
// the selectable editor, and does not replay after adding state-style observers.
func TestInlineEnter(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		for _, preset := range PresencePreset("").enumValues() {
			t.Run(fmt.Sprintf("%s/selectable=%v", preset, selectable), func(t *testing.T) {
				v := NewView()
				set := func(key, extra string) {
					v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":%v,"style":{"width":70}},"children":[{"type":"span","key":%q,"props":{"text":"abcdef abcdef","style":{"color":"#ff0000"},"enter":%q%s}}]}}`, selectable, key, preset, extra)))
				}
				set("run", "")
				var events []recordedEvent
				host := layoutHost(v, &events)
				defer host.Dispose()
				now := time.Unix(1, 0)
				host.SetAnimationClock(func() time.Time { return now })
				first := animationFrameImage(t, host, 1.5)
				bounds := automationBounds(t, host, "woxview/text0/k:run/span")
				for _, value := range first.Pix {
					if value != 0 {
						t.Fatal("span visible at initial pose")
					}
				}
				now = now.Add(100 * time.Millisecond)
				middle := animationFrameImage(t, host, 1.5)
				if bytes.Equal(first.Pix, middle.Pix) {
					t.Fatal("span never appeared")
				}
				set("run", `,"hoverStyle":{"color":"#00ff00"}`)
				if image := animationFrameImage(t, host, 1.5); !bytes.Equal(middle.Pix, image.Pix) {
					t.Fatal("observer update restarted entry")
				}
				now = now.Add(100 * time.Millisecond)
				last := animationFrameImage(t, host, 1.5)
				if bytes.Equal(middle.Pix, last.Pix) {
					t.Fatal("span did not finish")
				}
				if got := automationBounds(t, host, "woxview/text0/k:run/span"); got != bounds {
					t.Fatal("entry changed fragment geometry")
				}
				set("run", "")
				if image := animationFrameImage(t, host, 1.5); !bytes.Equal(last.Pix, image.Pix) {
					t.Fatal("removing observers replayed entry")
				}
				set("fresh", "")
				if image := animationFrameImage(t, host, 1.5); !bytes.Equal(first.Pix, image.Pix) {
					t.Fatal("new span key did not restart")
				}
			})
		}
	}
}

// TestEnterValidation exercises both spellings and malformed custom-decoder input.
func TestEnterValidation(t *testing.T) {
	for _, raw := range []string{`""`, `"bounce"`, `{}`, `{"type":null}`, `{"type":"fade","duration":-1}`, `{"type":"scale","duration":60001}`, `{"type":"fade","duration":1.5}`, `{"type":"fade","extra":true}`, `false`, `[]`, `1`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"enter":` + raw + `}}}`)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, preset := range PresencePreset("").enumValues() {
		for _, raw := range []string{fmt.Sprintf(`%q`, preset), fmt.Sprintf(`{"type":%q}`, preset), fmt.Sprintf(`{"type":%q,"duration":null}`, preset), fmt.Sprintf(`{"type":%q,"duration":0}`, preset), fmt.Sprintf(`{"type":%q,"duration":60000}`, preset), `null`} {
			tree := mustParse(t, `{"version":1,"root":{"type":"view","props":{"enter":`+raw+`}}}`)
			data, err := json.Marshal(tree)
			if err != nil {
				t.Fatal(err)
			}
			mustParse(t, string(data))
		}
	}
	negative := -1
	for _, animation := range []*PresenceAnimation{{Type: "invalid"}, {Type: PresenceFade, Duration: &negative}} {
		if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{CommonProps: CommonProps{Enter: animation}}}}); err == nil {
			t.Fatal("Go tree bypassed presence validation")
		}
	}
}

// TestEnterComposition verifies separate entry, transition and loop timelines
// under a cached boundary, including completion and a fresh View owner.
func TestEnterComposition(t *testing.T) {
	v := NewView()
	set := func(opacity float32, extra string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","props":{"enter":"fade","transition":{"properties":["opacity"],"duration":200,"curve":"linear"},"style":{"width":100,"height":80,"opacity":%g,"backgroundColor":"#ff0000"}%s}}}`, opacity, extra)))
	}
	set(1, `,"animation":{"type":"pulse","duration":400}`)
	builds := 0
	onBuild := func() { builds++ }
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height})
		return widget.Boundary[cachedFormProps]{Key: "enter-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 50, 40)
	set(.5, `,"animation":{"type":"pulse","duration":400}`)
	statePixel(t, host, 1, 50, 40)
	now = now.Add(100 * time.Millisecond)
	if got := statePixel(t, host, 1.5, 50, 40); got.R < 78 || got.R > 79 {
		t.Fatalf("entry chased another timeline: %+v", got)
	}
	previous := builds
	statePixel(t, host, 1.5, 50, 40)
	if builds != previous {
		t.Fatal("unchanged clock defeated boundary cache")
	}
	now = now.Add(100 * time.Millisecond)
	statePixel(t, host, 1.5, 50, 40)
	set(.5, "")
	statePixel(t, host, 1.5, 50, 40)
	previous = builds
	now = now.Add(time.Second)
	if got := statePixel(t, host, 1.5, 50, 40); got.R != 127 {
		t.Fatalf("completed entry left residual pose: %+v", got)
	}
	if builds != previous {
		t.Fatal("finished entry kept invalidating boundary")
	}
	v = NewView()
	set(1, "")
	if got := statePixel(t, host, 1.5, 50, 40); got.A != 0 {
		t.Fatal("new View inherited old mount")
	}
}

// TestEnterFieldAndDensity preserves the editor during a changing entry pose;
// the preset's logical travel scales once with interface density, never DPI.
func TestEnterFieldAndDensity(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text-field","key":"field","props":{"enter":"slideUp","value":"","style":{"width":180}}}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 10, 10)
	if !host.FocusAutomationID("woxview/k:field/text-field/field") {
		t.Fatal("entry field cannot focus")
	}
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "draft"})
	now = now.Add(100 * time.Millisecond)
	statePixel(t, host, 1.5, 10, 10)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "!"})
	now = now.Add(100 * time.Millisecond)
	statePixel(t, host, 2, 10, 10)
	host.TextInput(ui.TextInputEvent{Kind: ui.TextInputCommit, Text: "!"})
	if v.fieldValues["woxview/k:field/text-field"] != "draft!!" {
		t.Fatal("entry lost focus or editor state")
	}
	theme := component.ControlTheme{DensityScale: 1.5}
	style := Style{TranslateY: Offset{Token: SizeSpace2}}
	sample := presenceStyle(style, PresenceSlideUp, .5, theme)
	if sample.TranslateY.Value != style.TranslateY.resolve(theme)+theme.Scaled(6) {
		t.Fatal("travel scaled incorrectly")
	}
	zero := 0
	v = NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"view","props":{"enter":{"type":"fade","duration":0},"style":{"width":20,"height":20,"backgroundColor":"#ff0000"}}}}`))
	zeroHost := layoutHost(v, &events)
	defer zeroHost.Dispose()
	if got := statePixel(t, zeroHost, 1, 10, 10); got.R != 255 || (PresenceAnimation{Duration: &zero}).duration() != 0 {
		t.Fatal("zero duration still animates")
	}
}
