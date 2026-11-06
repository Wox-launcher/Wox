package woxview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestExitLifecycle removes the live control immediately while replaying its
// last paint above reflowed content, then drops every retained reference.
func TestExitLifecycle(t *testing.T) {
	for _, preset := range PresencePreset("").enumValues() {
		t.Run(preset, func(t *testing.T) {
			v := NewView()
			green := `{"type":"view","key":"green","props":{"label":"Green","onPress":{"$handler":"green"},"style":{"width":60,"height":40,"backgroundColor":"#00ff00"}}}`
			v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"column","children":[{"type":"view","key":"red","props":{"exit":%q,"label":"Red","onPress":{"$handler":"red"},"style":{"width":60,"height":40,"backgroundColor":"#ff0000"}}},%s]}}`, preset, green)))
			var events []recordedEvent
			host := layoutHost(v, &events)
			defer host.Dispose()
			now := time.Unix(1, 0)
			host.SetAnimationClock(func() time.Time { return now })
			statePixel(t, host, 1, 30, 20)
			old := automationBounds(t, host, "woxview/column0/k:red/view#press")
			if len(v.captures) != 1 {
				t.Fatal("missing live snapshot")
			}
			v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[`+green+`]}}`))
			if v.HasHandler("red") || len(v.exits) != 1 {
				t.Fatal("deletion did not revoke handler/start exit")
			}
			tap(host, old) // The previous frame's widget callbacks must already be revoked.
			if len(events) != 0 {
				t.Fatal("deleted node emitted before next frame")
			}
			if got := statePixel(t, host, 1, 30, 20); got.R != 255 {
				t.Fatalf("exit did not start from last paint: %+v", got)
			}
			if greenBounds := automationBounds(t, host, "woxview/column0/k:green/view#press"); greenBounds.Y != 0 {
				t.Fatal("exit consumed layout space")
			}
			tap(host, old)
			if len(events) != 1 || events[0].handler != "green" {
				t.Fatal("snapshot intercepted the live control")
			}
			now = now.Add(100 * time.Millisecond)
			for _, scale := range []float32{1, 1.25, 1.5, 2} {
				if got := statePixel(t, host, scale, 30, 20); got.R < 126 || got.R > 128 || got.G < 126 || got.G > 128 {
					t.Fatalf("midpoint at %g: %+v", scale, got)
				}
			}
			if preset == "slideUp" && statePixel(t, host, 1, 30, 37).R != 0 {
				t.Fatal("exit slideUp did not move upward")
			}
			if preset == "scale" && statePixel(t, host, 1, 0, 20).R != 0 {
				t.Fatal("exit scale did not shrink")
			}
			now = now.Add(100 * time.Millisecond)
			if got := statePixel(t, host, 2, 30, 20); got.G != 255 || got.R != 0 {
				t.Fatalf("exit did not finish: %+v", got)
			}
			if len(v.exits) != 0 || len(v.captures) != 0 {
				t.Fatal("finished exit retained paint")
			}
		})
	}
}

// TestExitRootAndReinsertion covers an empty document, independent keys,
// cancellation/reinsertion and repeated updates without restarting an exit.
func TestExitRootAndReinsertion(t *testing.T) {
	v := NewView()
	set := func(key, color string) {
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","key":%q,"props":{"exit":{"type":"fade","duration":400},"style":{"width":60,"height":40,"backgroundColor":%q}}}}`, key, color)))
	}
	set("box", "#ff0000")
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	statePixel(t, host, 1, 30, 20)
	v.SetTree(nil)
	if got := statePixel(t, host, 1, 30, 20); got.R != 255 {
		t.Fatal("nil tree discarded root snapshot")
	}
	now = now.Add(200 * time.Millisecond)
	if got := statePixel(t, host, 1, 30, 20); got.R < 126 || got.R > 128 {
		t.Fatal("explicit duration ignored")
	}
	v.SetTree(nil)
	if got := statePixel(t, host, 1, 30, 20); got.R < 126 || got.R > 128 {
		t.Fatal("repeated update restarted exit")
	}
	set("box", "#0000ff")
	if len(v.exits) != 0 {
		t.Fatal("reinsertion retained old ghost")
	}
	if got := statePixel(t, host, 1.5, 30, 20); got.B != 255 || got.R != 0 {
		t.Fatal("reinsertion changed new content")
	}
	v.SetTree(nil)
	if got := statePixel(t, host, 1, 30, 20); got.B != 255 {
		t.Fatal("new exit reused old clock")
	}
	now = now.Add(400 * time.Millisecond)
	if got := statePixel(t, host, 2, 30, 20); got.A != 0 {
		t.Fatal("empty document retained exit pixels")
	}
	if len(v.exits) != 0 {
		t.Fatal("root exit not released")
	}
	set("fresh", "#ff0000")
	statePixel(t, host, 1, 30, 20)
	v.SetTree(nil)
	v.CancelPending()
	if got := statePixel(t, host, 1, 30, 20); got.A != 0 || len(v.exits) != 0 {
		t.Fatal("document cancellation retained paint")
	}
}

// TestExitAncestorSelection freezes inherited group paint and avoids doubling
// a child when its exiting parent already contains the same drawing commands.
func TestExitAncestorSelection(t *testing.T) {
	for _, parentExit := range []string{`"fade"`, `null`} {
		v := NewView()
		v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"view","key":"parent","props":{"exit":%s,"style":{"opacity":0.5}},"children":[{"type":"view","key":"child","props":{"exit":"fade","style":{"width":60,"height":40,"backgroundColor":"#ff0000"}}}]}}`, parentExit)))
		var events []recordedEvent
		host := layoutHost(v, &events)
		t.Cleanup(host.Dispose)
		now := time.Unix(1, 0)
		host.SetAnimationClock(func() time.Time { return now })
		before := statePixel(t, host, 1, 30, 20)
		v.SetTree(nil)
		if len(v.exits) != 1 {
			t.Fatalf("parent=%s selected %d exits", parentExit, len(v.exits))
		}
		if got := statePixel(t, host, 1, 30, 20); got != before {
			t.Fatalf("ancestor paint changed: %+v want %+v", got, before)
		}
		now = now.Add(100 * time.Millisecond)
		if got := statePixel(t, host, 1.5, 30, 20); got.R < 63 || got.R > 64 {
			t.Fatal("ancestor opacity was lost or applied twice")
		}
	}
}

// TestExitInline captures wrapped fragments, including the current loop pose,
// without retaining selectable editor or span callbacks after deletion.
func TestExitInline(t *testing.T) {
	for _, selectable := range []bool{false, true} {
		for _, preset := range PresencePreset("").enumValues() {
			t.Run(fmt.Sprintf("%s/selectable=%v", preset, selectable), func(t *testing.T) {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":%v,"style":{"width":70}},"children":[{"type":"span","key":"run","props":{"exit":%q,"text":"abcdef abcdef","style":{"color":"#ff0000"},"animation":{"type":"spin","duration":1200},"onPress":{"$handler":"run"}}}]}}`, selectable, preset)))
				var events []recordedEvent
				host := layoutHost(v, &events)
				defer host.Dispose()
				now := time.Unix(1, 0)
				host.SetAnimationClock(func() time.Time { return now })
				animationFrameImage(t, host, 1.5)
				now = now.Add(100 * time.Millisecond)
				last := animationFrameImage(t, host, 1.5)
				v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text"}}`))
				if len(v.exits) != 1 || v.HasHandler("run") {
					t.Fatal("span removal did not start inert exit")
				}
				first := animationFrameImage(t, host, 1.5)
				if !bytes.Equal(first.Pix, last.Pix) {
					t.Fatal("span exit did not preserve its last rotated fragments")
				}
				now = now.Add(100 * time.Millisecond)
				mid := animationFrameImage(t, host, 1.5)
				if bytes.Equal(first.Pix, mid.Pix) {
					t.Fatal("span exit did not advance")
				}
				now = now.Add(100 * time.Millisecond)
				end := animationFrameImage(t, host, 1.5)
				for _, v := range end.Pix {
					if v != 0 {
						t.Fatal("finished span remained visible")
					}
				}
			})
		}
	}
}

// TestRemovedNodeCapability revokes old callbacks even when a live sibling uses
// the same handler ID, and never revives them when the original key returns.
func TestRemovedNodeCapability(t *testing.T) {
	v := NewView()
	set := func(both bool) {
		children := `{"type":"button","key":"other","props":{"label":"Other","onPress":{"$handler":"shared"}}}`
		if both {
			children += `,{"type":"button","key":"old","props":{"label":"Old","onPress":{"$handler":"shared"}}}`
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","children":[`+children+`]}}`))
	}
	set(true)
	id := "woxview/column0/k:old/button"
	owner := v.nodes[id]
	count := 0
	b := builder{view: v, nodeOwner: owner, eventOwner: owner, props: BuildProps{OnEvent: func(HandlerID, json.RawMessage) { count++ }}}
	emit := b.emit(&Handler{ID: "shared"})
	emit()
	set(false)
	if !v.HasHandler("shared") {
		t.Fatal("sibling handler missing")
	}
	emit()
	set(true)
	emit()
	if count != 1 {
		t.Fatal("old callback capability revived")
	}
	b.nodeOwner, b.eventOwner = v.nodes[id], v.nodes[id]
	fresh := b.emit(&Handler{ID: "shared"})
	fresh()
	v.CancelPending()
	fresh()
	b.emit(&Handler{ID: "shared"})() // A newly built callback uses the current epoch.
	if count != 3 {
		t.Fatal("fresh callback did not work")
	}
}

// TestExitCachedDamage compares retained partial frames with full repaint as
// snapshots move out of layout bounds, disappear and stop invalidating caches.
func TestExitCachedDamage(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		v := NewView()
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column","props":{"style":{"padding":20}},"children":[{"type":"view","key":"box","props":{"exit":"slideUp","style":{"width":60,"height":40,"backgroundColor":"#ff0000","shadow":"md"}}}]}}`))
		builds := 0
		onBuild := func() { builds++ }
		host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
			child := v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height})
			return widget.Boundary[cachedFormProps]{Key: "exit-cache", Props: cachedFormProps{child, v.Tree(), onBuild}, Build: func(p cachedFormProps) widget.Widget { p.onBuild(); return p.child }}
		})
		host.AttachServices(fakeServices{})
		t.Cleanup(host.Dispose)
		now := time.Unix(1, 0)
		host.SetAnimationClock(func() time.Time { return now })
		retained, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
		for step := range 6 {
			if step == 2 {
				v.SetTree(mustParse(t, `{"version":1,"root":{"type":"column"}}`))
			}
			if step >= 3 {
				now = now.Add(100 * time.Millisecond)
			}
			frame := ui.FrameInfo{Size: ui.Size{Width: 200, Height: 120}, Scale: scale}
			if step > 0 {
				frame.Damage = ui.Rect{X: 1, Y: 1, Width: 1, Height: 1}
			}
			list := &ui.DisplayList{}
			host.Frame(list, frame)
			list.TransformScene(scale, ui.Point{})
			if err := retained.Render(list); err != nil {
				t.Fatal(err)
			}
			frame.Damage = ui.Rect{}
			fullList := &ui.DisplayList{}
			host.Frame(fullList, frame)
			fullList.TransformScene(scale, ui.Point{})
			full, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
			if err := full.Render(fullList); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(retained.RGBA().Pix, full.RGBA().Pix) {
				t.Fatalf("scale=%g step=%d left stale exit paint", scale, step)
			}
			if step == 1 && (builds != 1 || len(v.captures) != 1) {
				t.Fatal("cached paint lost capture or rebuilt needlessly")
			}
		}
		before := builds
		now = now.Add(time.Second)
		animationFrameImage(t, host, scale)
		if builds != before || len(v.exits) != 0 {
			t.Fatal("finished exit kept cache invalidating")
		}
	}
}

// TestExitVisibilityCapture forgets unpainted and virtualized nodes rather than
// resurrecting a stale on-screen image when an offscreen row is later deleted.
func TestExitVisibilityCapture(t *testing.T) {
	v := NewView()
	set := func(first int) {
		rows := []string{}
		for i := first; i < 50; i++ {
			rows = append(rows, fmt.Sprintf(`{"type":"view","key":"%d","props":{"exit":"fade","label":"Row %d","onPress":{"$handler":"row"},"style":{"width":100,"height":40,"backgroundColor":"#ff0000"}}}`, i, i))
		}
		v.SetTree(mustParse(t, `{"version":1,"root":{"type":"lazy-list","key":"rows","props":{"itemHeight":40,"style":{"width":100,"height":80}},"children":[`+strings.Join(rows, ",")+`]}}`))
	}
	set(0)
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	now := time.Unix(1, 0)
	host.SetAnimationClock(func() time.Time { return now })
	animationFrameImage(t, host, 1)
	id := "woxview/k:rows/lazy-list/k:0/view"
	if v.captures[id] == nil {
		t.Fatal("visible row was not captured")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerScroll, Position: ui.Point{X: 10, Y: 10}, Scroll: ui.Point{Y: -1000}})
	animationFrameImage(t, host, 1.5)
	if hasAutomation(host, id+"#press") || v.captures[id] != nil {
		t.Fatal("offscreen row retained visible capture")
	}
	set(1)
	if len(v.exits) != 0 {
		t.Fatal("offscreen deletion replayed an old row")
	}
	// A live ancestor with zero opacity must likewise clear the old frame's paint.
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"view","props":{"style":{"opacity":0}},"children":[{"type":"view","key":"hidden","props":{"exit":"fade","style":{"width":40,"height":20,"backgroundColor":"#ff0000"}}}]}}`))
	v.clearPresence()
	animationFrameImage(t, host, 1)
	if len(v.captures) != 0 {
		t.Fatal("hidden child captured old pixels")
	}
	v.SetTree(nil)
	if len(v.exits) != 0 {
		t.Fatal("hidden child started a visible exit")
	}
}

// TestExitPrefixAndOwner releases old snapshots when the embedding namespace or
// document changes, without letting disposal revoke the replacement frame owner.
func TestExitPrefixAndOwner(t *testing.T) {
	v := NewView()
	tree := mustParse(t, `{"version":1,"root":{"type":"view","key":"box","props":{"exit":"fade","label":"Box","onPress":{"$handler":"box"},"style":{"width":60,"height":40,"backgroundColor":"#ff0000"}}}}`)
	v.SetTree(tree)
	prefix := "first"
	count := 0
	host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
		return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, IDPrefix: prefix, OnEvent: func(HandlerID, json.RawMessage) { count++ }})
	})
	host.AttachServices(fakeServices{})
	defer host.Dispose()
	animationFrameImage(t, host, 1)
	prefix = "second"
	animationFrameImage(t, host, 1)
	tap(host, automationBounds(t, host, "second/k:box/view#press"))
	if count != 1 || len(v.captures) != 1 {
		t.Fatal("old frame disposal revoked replacement owner")
	}
	v.SetTree(nil)
	animationFrameImage(t, host, 1)
	old := v
	v = NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"view"}}`))
	if got := statePixel(t, host, 1, 30, 20); got.A != 0 {
		t.Fatal("new View inherited old exit")
	}
	if len(old.exits) != 0 || len(old.captures) != 0 {
		t.Fatal("old document retained snapshots")
	}
}

// TestExitValidation keeps the shorthand/object contract strict and agrees with
// programmatic trees. Explicit zero opts out of capture and retained work.
func TestExitValidation(t *testing.T) {
	for _, raw := range []string{`"bounce"`, `""`, `{}`, `{"type":null}`, `{"type":"fade","duration":-1}`, `{"type":"fade","duration":60001}`, `{"type":"fade","duration":1.5}`, `{"type":"fade","extra":true}`, `false`, `[]`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"view","props":{"exit":` + raw + `}}}`)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	negative := -1
	if _, err := NewTree(&Node{Type: NodeView, Props: &BoxProps{Styled: Styled{CommonProps: CommonProps{Exit: &PresenceAnimation{Type: PresenceFade, Duration: &negative}}}}}); err == nil {
		t.Fatal("Go tree bypassed exit validation")
	}
	v := NewView()
	v.SetTree(mustParse(t, `{"version":1,"root":{"type":"view","props":{"exit":{"type":"fade","duration":0},"style":{"width":40,"height":20,"backgroundColor":"#ff0000"}}}}`))
	var events []recordedEvent
	host := layoutHost(v, &events)
	defer host.Dispose()
	animationFrameImage(t, host, 1)
	v.SetTree(nil)
	if got := statePixel(t, host, 1, 10, 10); got.A != 0 || len(v.exits) != 0 {
		t.Fatal("zero duration retained exit")
	}
}

// TestExitSelectedSpan retains the editor-owned selection background as well as
// glyphs, without applying span effects to native selection while the node lives.
func TestExitSelectedSpan(t *testing.T) {
	for _, opacity := range []float32{1, .5} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			t.Run(fmt.Sprintf("opacity=%g/scale=%g", opacity, scale), func(t *testing.T) {
				v := NewView()
				v.SetTree(mustParse(t, fmt.Sprintf(`{"version":1,"root":{"type":"text","props":{"selectable":true,"style":{"width":70,"opacity":%g}},"children":[{"type":"span","key":"run","props":{"exit":"fade","text":"abcdef abcdef","style":{"color":"#ff0000"}}}]}}`, opacity)))
				theme := component.ControlTheme{TextSelectionBackground: ui.Color{B: 255, A: 255}, TextSelectionText: ui.Color{R: 255, G: 255, B: 255, A: 255}}
				host := widget.NewHost(func(f ui.FrameInfo) widget.Widget {
					return v.Build(BuildProps{Width: f.Size.Width, Height: f.Size.Height, Theme: theme})
				})
				host.AttachServices(fakeServices{})
				defer host.Dispose()
				now := time.Unix(1, 0)
				host.SetAnimationClock(func() time.Time { return now })
				animationFrameImage(t, host, scale)
				if !host.FocusAutomationID("woxview/text0/selectable") {
					t.Fatal("missing editor")
				}
				if err := host.SelectAllWithin("woxview/text0/selectable"); err != nil {
					t.Fatal(err)
				}
				last := animationFrameImage(t, host, scale)
				v.SetTree(mustParse(t, `{"version":1,"root":{"type":"text","props":{"selectable":true,"style":{"width":70}}}}`))
				first := animationFrameImage(t, host, scale)
				if !bytes.Equal(last.Pix, first.Pix) {
					t.Fatal("exit lost selected span's last paint")
				}
				now = now.Add(100 * time.Millisecond)
				mid := animationFrameImage(t, host, scale)
				for i, value := range mid.Pix {
					if delta := int(value)*2 - int(last.Pix[i]); delta < -2 || delta > 2 {
						t.Fatalf("fragment overlap changed fade at byte %d: %d, original %d", i, value, last.Pix[i])
					}
				}
			})
		}
	}
}
