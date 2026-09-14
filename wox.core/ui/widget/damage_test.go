package widget

import (
	"math"
	"testing"

	woxui "wox/ui/runtime"
)

// TestCaretHighlightsInsideWindowOutline covers a theme border painted after a floating editor.
func TestCaretHighlightsInsideWindowOutline(t *testing.T) {
	if !woxui.SupportsCaretPatch() {
		t.Skip("renderer does not support caret patches")
	}
	caret := woxui.Rect{X: 30, Y: 50, Width: 2, Height: 24}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Stack{Width: 200, Height: 120, Children: []StackChild{
			{Child: Container{Width: 200, Height: 120, Floating: true}},
			{Child: CaretPainter{Width: 200, Height: 120, Active: true, Paint: func(d *woxui.DisplayList, _ woxui.Rect, _, visible bool) {
				d.DrawCaret(caret, woxui.Color{A: 255}, visible)
			}}},
			{Child: Container{Width: 200, Height: 120, BorderWidth: 2, BorderColor: woxui.Color{A: 255}}},
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	if err := host.SetRepaintDebugMode(RepaintDebugRainbow); err != nil {
		t.Fatal(err)
	}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 120}, Scale: 1}
	var last woxui.DisplayList
	for phase := 0; phase < 4; phase++ {
		host.caretBlinkMu.Lock()
		host.caretVisible = phase%2 == 0
		host.caretBlinkMu.Unlock()
		last = woxui.DisplayList{}
		host.Frame(&last, frame)
		frame.Damage = caret
	}
	if last.CaretPatch() != caret {
		t.Fatalf("highlight damage=%+v, want caret %+v", last.CaretPatch(), caret)
	}
	if last.NativeDamage() != (woxui.Rect{}) {
		t.Fatal("debug overlay must still use a full native replay")
	}
}

func TestFrameDamageTrackerIncludesOldAndPlacedBoundaryBounds(t *testing.T) {
	current := &node{bounds: woxui.Rect{Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}, current, true)
	current.place(40, 40)

	got := tracker.resolve(woxui.Rect{X: 25, Y: 25, Width: 1, Height: 1})
	want := woxui.Rect{X: 20, Y: 20, Width: 30, Height: 30}
	if got != want {
		t.Fatalf("resolved damage = %+v, want %+v", got, want)
	}
}

func TestFrameDamageTrackerSeedsDamageFromBoundary(t *testing.T) {
	current := &node{bounds: woxui.Rect{X: 20, Y: 30, Width: 40, Height: 50}}
	tracker := &frameDamageTracker{}
	tracker.add(woxui.Rect{}, current, true)
	if got := tracker.resolve(woxui.Rect{}); got != current.bounds {
		t.Fatalf("boundary-seeded damage = %+v, want %+v", got, current.bounds)
	}
}

func TestFrameDamageTrackerSkipsStationaryCacheHit(t *testing.T) {
	current := &node{bounds: woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(current.bounds, current, false)
	base := woxui.Rect{X: 2, Y: 2, Width: 4, Height: 4}
	if got := tracker.resolve(base); got != base {
		t.Fatalf("stationary cache hit damage = %+v, want %+v", got, base)
	}
}

func TestFrameDamageTrackerIncludesMovedCacheHit(t *testing.T) {
	current := &node{bounds: woxui.Rect{Width: 10, Height: 10}}
	tracker := &frameDamageTracker{}
	tracker.add(woxui.Rect{X: 20, Y: 20, Width: 10, Height: 10}, current, false)
	current.place(40, 40)
	if got, want := tracker.resolve(woxui.Rect{X: 25, Y: 25, Width: 1, Height: 1}), (woxui.Rect{X: 20, Y: 20, Width: 30, Height: 30}); got != want {
		t.Fatalf("moved cache hit damage = %+v, want %+v", got, want)
	}
}

func TestCoverRenderedMaterialsWidensDamageThroughTouchedSurfaces(t *testing.T) {
	panel := woxui.Rect{X: 100, Y: 100, Width: 200, Height: 100}
	tooltip := woxui.Rect{X: 280, Y: 180, Width: 80, Height: 40}
	untouched := woxui.Rect{X: 600, Y: 600, Width: 50, Height: 50}
	materials := []woxui.Rect{panel, tooltip, untouched}

	// A tail refresh under the panel must repaint the whole panel, and covering the panel
	// reaches the tooltip stacked on its corner, but never a surface the damage misses.
	got := coverRenderedMaterials(woxui.Rect{X: 120, Y: 150, Width: 10, Height: 10}, materials, 0, 1)
	want := woxui.Rect{X: 100, Y: 100, Width: 260, Height: 120}
	if got != want {
		t.Fatalf("covered damage = %+v, want panel and tooltip %+v", got, want)
	}
	if base := (woxui.Rect{X: 10, Y: 10, Width: 5, Height: 5}); coverRenderedMaterials(base, materials, 0, 1) != base {
		t.Fatal("damage away from every surface was widened")
	}
	if base := (woxui.Rect{X: 90, Y: 90, Width: 400, Height: 400}); coverRenderedMaterials(base, materials, 0, 1) != base {
		t.Fatal("damage already containing the surfaces was widened")
	}
	if coverRenderedMaterials(woxui.Rect{}, materials, 0, 1) != (woxui.Rect{}) {
		t.Fatal("empty damage was widened")
	}
}

func TestCoverRenderedMaterialsKeepsAdjacentCardsLocal(t *testing.T) {
	panel := woxui.Rect{X: 400, Y: 120, Width: 320, Height: 400}
	toolbar := woxui.Rect{X: 0, Y: 540, Width: 750, Height: 40}
	caret := woxui.Rect{X: 410, Y: 480, Width: 80, Height: 30}
	const margin float32 = 36

	got := coverRenderedMaterials(caret, []woxui.Rect{panel, toolbar}, margin, 1)
	want := expandDamageRect(panel, margin)
	if got != want {
		t.Fatalf("adjacent-card caret damage = %+v, want the panel plus sample halo %+v", got, want)
	}
	if joined := unionDamageRects(panel, toolbar); damageRectContains(got, joined) {
		t.Fatalf("adjacent-card caret damage = %+v swallowed the result list between panel and toolbar %+v", got, joined)
	}
}

// Native invalidation rounds outward to physical pixels before returning logical damage.
func TestCoverRenderedMaterialsKeepsPixelRoundedPanelLocal(t *testing.T) {
	panel := woxui.Rect{X: 432.1, Y: 200.1, Width: 350, Height: 320}
	toolbar := woxui.Rect{X: 0, Y: 540, Width: 800, Height: 40}
	for _, scale := range []float32{1, 1.25, 1.5, 2.5} {
		left := float32(math.Floor(float64(panel.X*scale))) / scale
		top := float32(math.Floor(float64(panel.Y*scale))) / scale
		right := float32(math.Ceil(float64((panel.X+panel.Width)*scale))) / scale
		bottom := float32(math.Ceil(float64((panel.Y+panel.Height)*scale))) / scale
		damage := woxui.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
		got := coverRenderedMaterials(damage, []woxui.Rect{panel, toolbar}, 36, scale)
		want := expandDamageRect(panel, 36)
		if math.Abs(float64(got.X-want.X))+math.Abs(float64(got.Y-want.Y))+math.Abs(float64(got.Width-want.Width))+math.Abs(float64(got.Height-want.Height)) > 0.001 {
			t.Fatalf("scale %v: pixel-rounded panel damage = %+v, want %+v", scale, got, want)
		}
	}
}

func TestStateInvalidateUsesNearestBoundaryBounds(t *testing.T) {
	services := &fakeHostServices{}
	host := NewHost(nil)
	host.AttachServices(services)
	tree := newElementTree(host)
	boundaryElement := &stateElement{tree: tree, parent: tree.root, boundary: &boundaryCache{node: &node{bounds: woxui.Rect{X: 10, Y: 20, Width: 30, Height: 40}}}}
	boundaryElement.mounted.Store(true)
	child := &stateElement{tree: tree, parent: boundaryElement}
	child.mounted.Store(true)

	StateContext{element: child}.Invalidate()

	if services.invalidatedRect != boundaryElement.boundary.node.bounds {
		t.Fatalf("invalidated rect = %+v, want nearest boundary %+v", services.invalidatedRect, boundaryElement.boundary.node.bounds)
	}
	if !child.dirty.Load() || !boundaryElement.dirty.Load() {
		t.Fatal("state invalidation did not mark the retained ancestor chain dirty")
	}
}

// Uncached keyed state must repaint its old and new geometry without a full frame.
func TestStateInvalidateUsesKeyedPaintOwner(t *testing.T) {
	height := float32(40)
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Gesture{ID: "scroll", Child: Container{Width: 100, Height: height}}
	})
	defer host.Dispose()
	services := &fakeHostServices{}
	host.AttachServices(services)
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 600}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	element := &stateElement{tree: host.elements, parent: host.elements.root, key: "scroll-state", paintNode: host.root}
	element.mounted.Store(true)
	height = 60
	StateContext{element: element}.Invalidate()
	if host.fullDamage || services.invalidatedRect != (woxui.Rect{Width: 100, Height: 40}) || !element.dirty.Load() {
		t.Fatalf("state invalidation: full=%v rect=%+v dirty=%v", host.fullDamage, services.invalidatedRect, element.dirty.Load())
	}
	frame.Damage = services.invalidatedRect
	var list woxui.DisplayList
	host.Frame(&list, frame)
	if got := list.NativeDamage(); got != (woxui.Rect{Width: 104, Height: 64}) {
		t.Fatalf("resized state damage = %+v", got)
	}
}

func TestActiveCaretDamageUsesFocusedEditorBounds(t *testing.T) {
	backgroundCaret := &node{id: 2, bounds: woxui.Rect{X: 10, Y: 10, Width: 40, Height: 20}, caret: true, caretPaint: func(*woxui.DisplayList, woxui.Rect, bool, bool) {}}
	actionCaret := &node{id: 4, bounds: woxui.Rect{X: 60, Y: 60, Width: 30, Height: 15}, caret: true, caretPaint: func(*woxui.DisplayList, woxui.Rect, bool, bool) {}}
	root := &node{children: []*node{
		{id: 1, focus: &focusBehavior{}, children: []*node{backgroundCaret}},
		{id: 3, focus: &focusBehavior{}, children: []*node{actionCaret}},
	}}

	if got := activeCaretDamage(root, 3, false, false); got != actionCaret.bounds {
		t.Fatalf("active caret damage = %+v, want focused editor %+v", got, actionCaret.bounds)
	}
}

func TestHostConsumesPendingDamageOnlyWithNativeDamage(t *testing.T) {
	host := NewHost(nil)
	host.invalidateRect(woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10})
	if got := host.consumeFrameDamage(woxui.Rect{}, woxui.Size{Width: 100, Height: 100}); got != (woxui.Rect{}) {
		t.Fatalf("fallback platform damage = %+v, want full frame", got)
	}

	host.invalidateRect(woxui.Rect{X: 10, Y: 10, Width: 10, Height: 10})
	got := host.consumeFrameDamage(woxui.Rect{X: 30, Y: 30, Width: 10, Height: 10}, woxui.Size{Width: 100, Height: 100})
	want := woxui.Rect{X: 10, Y: 10, Width: 30, Height: 30}
	if got != want {
		t.Fatalf("combined platform damage = %+v, want %+v", got, want)
	}
}
