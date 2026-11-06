package woxview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// animationFrameImage renders the same native-independent commands at a physical scale.
func animationFrameImage(t *testing.T, host *widget.Host, scale float32) *image.RGBA {
	t.Helper()
	list := &ui.DisplayList{}
	host.Frame(list, ui.FrameInfo{Size: ui.Size{Width: 200, Height: 120}, Scale: scale})
	list.TransformScene(scale, ui.Point{})
	r, err := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Render(list); err != nil {
		t.Fatal(err)
	}
	return r.RGBA()
}

// TestPulseIdentityAndComposition verifies default timing, authored opacity,
// mid-loop tree updates, fresh keys/periods and immediate removal.
func TestPulseIdentityAndComposition(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	set := func(key, animation string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":%q,"props":{"label":"Pulse","onPress":{"$handler":"pulse"},"animation":%s,"style":{"width":40,"height":20,"backgroundColor":"#ff0000","opacity":0.5}}}]}}`, key, animation)))
	}
	set("a", `{"type":"pulse"}`)
	if c := statePixel(t, host, 1, 10, 10); c.R != 127 || c.A != 127 {
		t.Fatalf("initial opacity %+v", c)
	}
	now = now.Add(600 * time.Millisecond)
	if c := statePixel(t, host, 1.5, 10, 10); c.R < 82 || c.R > 83 || c.A != c.R {
		t.Fatalf("pulse trough %+v", c)
	}
	set("a", `{"type":"pulse"}`)
	if c := statePixel(t, host, 2, 10, 10); c.R < 82 || c.R > 83 {
		t.Fatal("tree update restarted loop")
	}
	before, _ := json.Marshal(v.Tree().Root)
	set("b", `{"type":"pulse"}`)
	if c := statePixel(t, host, 1.25, 10, 10); c.R != 127 {
		t.Fatal("new key inherited phase")
	}
	now = now.Add(600 * time.Millisecond)
	statePixel(t, host, 1, 10, 10)
	set("b", `{"type":"pulse","duration":600}`)
	if c := statePixel(t, host, 1, 10, 10); c.R != 127 {
		t.Fatal("period change did not restart")
	}
	now = now.Add(300 * time.Millisecond)
	if c := statePixel(t, host, 1, 10, 10); c.R < 82 || c.R > 83 {
		t.Fatal("explicit period ignored")
	}
	set("b", `null`)
	if c := statePixel(t, host, 1, 10, 10); c.R != 127 {
		t.Fatal("removing animation did not restore style")
	}
	set("a", `{"type":"pulse"}`)
	statePixel(t, host, 1, 10, 10)
	after, _ := json.Marshal(v.Tree().Root)
	if string(before) != string(after) {
		t.Fatal("loop mutated authored props")
	}
	tap(host, automationBounds(t, host, "woxview/column0/k:a/view#press"))
	if len(events) != 1 {
		t.Fatal("pulse changed input routing")
	}
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
	statePixel(t, host, 1, 10, 10)
	if v.HasHandler("pulse") {
		t.Fatal("removed loop owns handler")
	}
	set("a", `{"type":"pulse"}`)
	if c := statePixel(t, host, 1, 10, 10); c.R != 127 {
		t.Fatal("remount reused removed loop")
	}
}

// TestSpinPaintAndInput rotates child pixels but leaves geometry and activation
// at their authored positions, including after changing the preset.
func TestSpinPaintAndInput(t *testing.T) {
	v := NewView()
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	set := func(kind string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"stack","children":[{"type":"row","key":"box","props":{"label":"Spin","onPress":{"$handler":"spin"},"animation":{"type":%q,"duration":1200},"style":{"left":30,"top":30,"width":40,"height":20}},"children":[{"type":"view","props":{"style":{"width":20,"height":20,"backgroundColor":"#ff0000"}}},{"type":"view","props":{"style":{"width":20,"height":20,"backgroundColor":"#00ff00"}}}]}]}}`, kind)))
	}
	set("spin")
	if c := statePixel(t, host, 1, 35, 40); c.R != 255 {
		t.Fatal("initial rotation changed")
	}
	initial := automationBounds(t, host, "woxview/stack0/k:box/row#press")
	now = now.Add(300 * time.Millisecond)
	for _, scale := range []float32{1.25, 1.5, 2} {
		if c := statePixel(t, host, scale, 50, 30); c.R != 255 {
			t.Fatalf("quarter rotation red %+v", c)
		}
		if c := statePixel(t, host, scale, 50, 50); c.G != 255 {
			t.Fatalf("quarter rotation green %+v", c)
		}
		if got := automationBounds(t, host, "woxview/stack0/k:box/row#press"); got != initial {
			t.Fatal("spin changed layout")
		}
	}
	tap(host, initial)
	if len(events) != 1 {
		t.Fatal("spin lost original hit target")
	}
	set("pulse")
	if c := statePixel(t, host, 1, 35, 40); c.R != 255 || c.A != 255 {
		t.Fatal("preset change did not reset phase/rotation")
	}
}

// TestShimmerAppearanceAndClipping checks light/dark contrast, seam-free looping,
// asymmetric margins, authored opacity and the two bounded immutable textures.
func TestShimmerAppearanceAndClipping(t *testing.T) {
	v := NewView()
	theme := component.ControlTheme{}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	set := func(background string, opacity float32) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":"shine","props":{"animation":{"type":"shimmer"},"style":{"width":100,"height":40,"margin":{"left":10,"right":20,"top":10},"backgroundColor":%q,"opacity":%g}}}]}}`, background, opacity)))
	}
	set("#ffffff", 1)
	if c := statePixel(t, host, 1, 60, 30); c.R != 255 {
		t.Fatal("shimmer starts inside box")
	}
	now = now.Add(600 * time.Millisecond)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		if c := statePixel(t, host, scale, 60, 30); c.R < 218 || c.R > 223 || c.A != 255 {
			t.Fatalf("light shimmer %+v", c)
		}
		if c := statePixel(t, host, scale, 20, 30); c.R != 255 {
			t.Fatal("shimmer band covers entire box")
		}
		if c := statePixel(t, host, scale, 5, 30); c.A != 0 {
			t.Fatal("shimmer painted margin")
		}
	}
	theme.Dark = true
	set("#000000", 1)
	if c := statePixel(t, host, 1.5, 60, 30); c.R < 32 || c.R > 37 || c.A != 255 {
		t.Fatalf("dark shimmer %+v", c)
	}
	set("#000000", 0)
	if c := statePixel(t, host, 2, 60, 30); c.A != 0 {
		t.Fatal("shimmer bypassed node opacity")
	}
	set("#000000", 1)
	now = now.Add(600 * time.Millisecond)
	if c := statePixel(t, host, 1, 60, 30); c.R != 0 {
		t.Fatal("loop seam visible")
	}
	a, b := shimmerTextures(), shimmerTextures()
	if a != b || a[0].Width != 256 || a[1].Height != 1 {
		t.Fatal("shimmer texture identity/storage changed")
	}
}

// TestInlineLoopPresets keeps one phase across wrapped fragments and both shared
// text render paths. Spin remains clipped by paragraph bounds and hit boxes stay fixed.
func TestInlineLoopPresets(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		for _, kind := range []string{"pulse", "spin", "shimmer"} {
			t.Run(fmt.Sprintf("%s/selectable=%v", kind, selectable), func(t *testing.T) {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":%v,"style":{"width":70}},"children":[{"type":"span","key":"run","props":{"text":"abcdef abcdef","style":{"color":"#ff0000"},"animation":{"type":%q}}}]}}`, selectable, kind)))
				var events []recordedEvent
				host := layoutHost(v, &events)
				defer host.Dispose()
				now := time.Unix(1, 0)
				host.SetAnimationClock(func() time.Time { return now })
				first := animationFrameImage(t, host, 1.5)
				bounds := automationBounds(t, host, "woxview/text0/k:run/span")
				elapsed := 600 * time.Millisecond
				if kind == "spin" {
					elapsed = 300 * time.Millisecond
				}
				now = now.Add(elapsed)
				mid := animationFrameImage(t, host, 1.5)
				if bytes.Equal(first.Pix, mid.Pix) {
					t.Fatal("inline loop produced no visible change")
				}
				if got := automationBounds(t, host, "woxview/text0/k:run/span"); got != bounds {
					t.Fatal("inline loop changed hit geometry")
				}
				now = now.Add(1200*time.Millisecond - elapsed)
				last := animationFrameImage(t, host, 1.5)
				if !bytes.Equal(first.Pix, last.Pix) {
					t.Fatal("inline loop failed to return to initial paint")
				}
			})
		}
	}
}

// TestAnimationValidation enforces the same strict preset/period contract in JSON and Go.
func TestAnimationValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"type":""}`, `{"type":"bounce"}`, `{"type":"pulse","duration":0}`, `{"type":"spin","duration":-1}`, `{"type":"shimmer","duration":60001}`, `{"type":"pulse","duration":1.5}`, `{"type":"pulse","keyframes":[]}`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"animation":` + raw + `}}}`)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, kind := range []string{"pulse", "spin", "shimmer"} {
		for _, duration := range []string{``, `,"duration":null`, `,"duration":1`, `,"duration":60000`} {
			mustParse(t, `{"version":1,"root":{"type":"view","props":{"animation":{"type":"`+kind+`"`+duration+`}}}}`)
		}
	}
	if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{CommonProps: CommonProps{Animation: &Animation{Type: "invalid"}}}}}); err == nil {
		t.Fatal("Go tree bypassed animation validation")
	}
}

// TestPulseWithTransitionAndBoundary keeps independent loop/target clocks while
// cached builds advance only when their recorded animation samples change.
func TestPulseWithTransitionAndBoundary(t *testing.T) {
	v := NewView()
	set := func(opacity float32, animation string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":"box","props":{"animation":%s,"transition":{"properties":["opacity"],"duration":200,"curve":"linear"},"style":{"width":40,"height":20,"backgroundColor":"#ff0000","opacity":%g}}}]}}`, animation, opacity)))
	}
	set(1, `{"type":"pulse"}`)
	builds := 0
	onBuild := func() { builds++ }
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		child := v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height})
		return widget.Boundary[cachedFormProps]{Key: "loop-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 10, 10)
	now = now.Add(300 * time.Millisecond)
	statePixel(t, host, 1, 10, 10)
	set(.5, `{"type":"pulse"}`)
	statePixel(t, host, 1, 10, 10)
	now = now.Add(100 * time.Millisecond)
	if c := statePixel(t, host, 1.5, 10, 10); c.R < 140 || c.R > 142 {
		t.Fatalf("transition chased pulse rather than composing: %+v", c)
	}
	previous := builds
	statePixel(t, host, 1.5, 10, 10)
	if builds != previous {
		t.Fatal("unchanged clock defeated boundary cache")
	}
	now = now.Add(200 * time.Millisecond)
	if c := statePixel(t, host, 1.5, 10, 10); c.R < 82 || c.R > 83 {
		t.Fatalf("composed trough %+v", c)
	}
	set(.5, `null`)
	statePixel(t, host, 1.5, 10, 10)
	previous = builds
	now = now.Add(time.Second)
	if c := statePixel(t, host, 1.5, 10, 10); c.R != 127 {
		t.Fatal("removed loop left residual opacity")
	}
	if builds != previous {
		t.Fatal("removed loop kept invalidating cache")
	}
}

// TestInlineShimmerPreservesSelection ensures decoration stays inside opacity
// and never repaints the native editor's selected range.
func TestInlineShimmerPreservesSelection(t *testing.T) {
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"selectable":true,"style":{"width":70}},"children":[{"type":"span","key":"run","props":{"text":"abcdef abcdef","animation":{"type":"shimmer"},"hoverStyle":{"opacity":0}}}]}}`))
	theme := component.ControlTheme{TextSelectionBackground: ui.Color{B: 255, A: 255}, TextSelectionText: ui.Color{R: 255, G: 255, B: 255, A: 255}, Dark: true}
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 25, 17)
	now = now.Add(600 * time.Millisecond)
	host.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Position: ui.Point{X: 25, Y: 17}})
	if c := statePixel(t, host, 1.5, 25, 17); c.A != 0 {
		t.Fatalf("shimmer bypassed inline opacity: %+v", c)
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerLeave})
	statePixel(t, host, 1.5, 25, 17)
	if !host.FocusAutomationID("woxview/text0/selectable") {
		t.Fatal("missing editor")
	}
	if err := host.SelectAllWithin("woxview/text0/selectable"); err != nil {
		t.Fatal(err)
	}
	if c := statePixel(t, host, 2, 25, 17); c != (ui.Color{B: 255, A: 255}) {
		t.Fatalf("shimmer tinted selection: %+v", c)
	}
}
