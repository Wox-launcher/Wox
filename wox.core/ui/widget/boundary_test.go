package widget

import (
	"fmt"
	"strings"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

type boundaryTestProps struct {
	Value int
}

type incompleteBoundaryTestProps struct {
	Value int
	Label string
}

func (p incompleteBoundaryTestProps) Equal(other incompleteBoundaryTestProps) bool {
	return p.Value == other.Value
}

type boundaryAssertionRecorder struct {
	errors []string
	fatals []string
}

func (*boundaryAssertionRecorder) Helper() {}

func (r *boundaryAssertionRecorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *boundaryAssertionRecorder) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}

func (p boundaryTestProps) Equal(other boundaryTestProps) bool {
	return p == other
}

type boundaryOffset struct {
	left  float32
	child Widget
}

func (w boundaryOffset) layout(ctx context, available constraints) *node {
	child := w.child.layout(ctx, available)
	child.place(w.left, 0)
	return &node{bounds: woxui.Rect{Width: available.width, Height: available.height}, children: []*node{child}}
}

type boundaryChildConfig struct {
	context  *StateContext
	builds   *int
	disposes *int
}

type boundaryChildState struct{}

func (*boundaryChildState) InitState(context StateContext, widget any) {
	config := widget.(boundaryChildConfig)
	*config.context = context
}

func (*boundaryChildState) DidUpdateWidget(context StateContext, _ any, widget any) {
	config := widget.(boundaryChildConfig)
	*config.context = context
}

func (*boundaryChildState) Build(_ StateContext, widget any) Widget {
	config := widget.(boundaryChildConfig)
	*config.builds++
	return Container{Width: 20, Height: 20}
}

func (*boundaryChildState) Dispose() {}

type disposingBoundaryChildState struct {
	disposes *int
}

func (s *disposingBoundaryChildState) InitState(_ StateContext, widget any) {
	s.disposes = widget.(*int)
}

func (*disposingBoundaryChildState) DidUpdateWidget(StateContext, any, any) {}

func (*disposingBoundaryChildState) Build(StateContext, any) Widget {
	return Container{Width: 20, Height: 20}
}

func (s *disposingBoundaryChildState) Dispose() {
	*s.disposes++
}

type invalidatingBoundaryChildState struct {
	builds      *int
	invalidated bool
}

func (s *invalidatingBoundaryChildState) InitState(_ StateContext, widget any) {
	s.builds = widget.(*int)
}

func (*invalidatingBoundaryChildState) DidUpdateWidget(StateContext, any, any) {}

func (s *invalidatingBoundaryChildState) Build(context StateContext, _ any) Widget {
	*s.builds++
	if !s.invalidated {
		s.invalidated = true
		context.Invalidate()
	}
	return Container{Width: 20, Height: 20}
}

func (*invalidatingBoundaryChildState) Dispose() {}

func renderBoundaryTestFrame(host *Host, width float32) {
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 100}, PixelSize: woxui.PixelSize{Width: int(width), Height: 100}, Scale: 1})
}

func TestAssertEqualCoversAllFieldsFindsOmittedField(t *testing.T) {
	recorder := &boundaryAssertionRecorder{}
	AssertEqualCoversAllFields(recorder, incompleteBoundaryTestProps{})
	if len(recorder.fatals) != 0 || len(recorder.errors) != 1 || !strings.Contains(recorder.errors[0], "Label") {
		t.Fatalf("coverage assertion result = errors %v fatals %v", recorder.errors, recorder.fatals)
	}
	AssertEqualCoversAllFields(t, boundaryTestProps{})
}

func TestBoundaryCachesByPropsAndConstraints(t *testing.T) {
	props := boundaryTestProps{Value: 1}
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "test-boundary", Props: props, Build: func(boundaryTestProps) Widget {
			builds++
			return Container{Width: 20, Height: 20}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if builds != 1 {
		t.Fatalf("equal boundary build count = %d, want 1", builds)
	}
	props.Value = 2
	renderBoundaryTestFrame(host, 100)
	if builds != 2 {
		t.Fatalf("changed props build count = %d, want 2", builds)
	}
	renderBoundaryTestFrame(host, 80)
	if builds != 3 {
		t.Fatalf("changed constraints build count = %d, want 3", builds)
	}
}

func TestBoundaryInvalidateMarksRetainedAncestorsDirty(t *testing.T) {
	var childContext StateContext
	boundaryBuilds := 0
	childBuilds := 0
	childDisposes := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "dirty-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			boundaryBuilds++
			return Stateful{
				Key: "dirty-child", Type: (*boundaryChildState)(nil), Widget: boundaryChildConfig{context: &childContext, builds: &childBuilds, disposes: &childDisposes},
				CreateState: func() State { return &boundaryChildState{} },
			}
		}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if boundaryBuilds != 1 || childBuilds != 1 {
		t.Fatalf("cached builds = boundary %d child %d, want 1/1", boundaryBuilds, childBuilds)
	}
	childContext.Invalidate()
	if services.invalidations == 0 {
		t.Fatal("child invalidation did not schedule a frame")
	}
	renderBoundaryTestFrame(host, 100)
	if boundaryBuilds != 2 || childBuilds != 2 {
		t.Fatalf("dirty builds = boundary %d child %d, want 2/2", boundaryBuilds, childBuilds)
	}
}

func TestBoundaryCacheHitKeepsDescendantStateMounted(t *testing.T) {
	visible := true
	disposes := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		if !visible {
			return Container{Width: 20, Height: 20}
		}
		return Boundary[boundaryTestProps]{Key: "retained-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			return Stateful{Key: "retained-child", Type: (*disposingBoundaryChildState)(nil), Widget: &disposes, CreateState: func() State { return &disposingBoundaryChildState{} }}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if disposes != 0 {
		t.Fatalf("cached descendant dispose count = %d, want 0", disposes)
	}
	visible = false
	renderBoundaryTestFrame(host, 100)
	if disposes != 1 {
		t.Fatalf("removed descendant dispose count = %d, want 1", disposes)
	}
}

func TestBoundaryKeepsInvalidationRaisedDuringBuild(t *testing.T) {
	boundaryBuilds := 0
	childBuilds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "build-invalidation-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			boundaryBuilds++
			return Stateful{
				Key: "build-invalidation-child", Type: (*invalidatingBoundaryChildState)(nil), Widget: &childBuilds,
				CreateState: func() State { return &invalidatingBoundaryChildState{} },
			}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if boundaryBuilds != 2 || childBuilds != 2 {
		t.Fatalf("build-time invalidation counts = boundary %d child %d, want 2/2", boundaryBuilds, childBuilds)
	}
}

func TestBoundaryCaretDependencyInvalidatesCache(t *testing.T) {
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "caret-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return CaretPainter{Width: 20, Height: 20}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if builds != 1 {
		t.Fatalf("stable caret build count = %d, want 1", builds)
	}
	host.caretVisible = !host.caretVisible
	renderBoundaryTestFrame(host, 100)
	if builds != 2 {
		t.Fatalf("changed caret build count = %d, want 2", builds)
	}
}

func TestBoundaryNormalizesCachedSubtreeBeforeNewPlacement(t *testing.T) {
	left := float32(10)
	builds := 0
	services := &fakeHostServices{}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return boundaryOffset{left: left, child: Boundary[boundaryTestProps]{Key: "placed-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return Semantics{AutomationID: "placed-content", Role: woxui.AccessibilityRoleGroup, Label: "Placed", Child: Container{Width: 20, Height: 20}}
		}}}
	})
	host.AttachServices(services)

	renderBoundaryTestFrame(host, 100)
	if bounds := findAutomationNode(t, services.tree, "placed-content").Bounds; bounds.X != 10 {
		t.Fatalf("initial cached bounds = %+v, want x 10", bounds)
	}
	left = 30
	renderBoundaryTestFrame(host, 100)
	if builds != 1 {
		t.Fatalf("moved boundary build count = %d, want 1", builds)
	}
	if bounds := findAutomationNode(t, services.tree, "placed-content").Bounds; bounds.X != 30 {
		t.Fatalf("reused cached bounds = %+v, want x 30", bounds)
	}
}

func TestBoundaryAnimationAdvancesThenCachesFinalValue(t *testing.T) {
	target := 0
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "animation-boundary", Props: boundaryTestProps{Value: target}, Build: func(props boundaryTestProps) Widget {
			builds++
			return AnimatedFloat{Key: "boundary-animation", Target: float32(props.Value), Duration: 20 * time.Millisecond, Builder: func(value float32) Widget {
				return Container{Width: 20 + value, Height: 20}
			}}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()

	renderBoundaryTestFrame(host, 100)
	target = 1
	renderBoundaryTestFrame(host, 100)
	time.Sleep(3 * time.Millisecond)
	renderBoundaryTestFrame(host, 100)
	if builds != 3 {
		t.Fatalf("in-flight animation build count = %d, want 3", builds)
	}
	time.Sleep(25 * time.Millisecond)
	renderBoundaryTestFrame(host, 100)
	completedBuilds := builds
	renderBoundaryTestFrame(host, 100)
	if builds != completedBuilds {
		t.Fatalf("completed animation rebuilt: before %d after %d", completedBuilds, builds)
	}
}

func TestBoundaryLoopAnimationAdvancesAndCachesWhenPaused(t *testing.T) {
	paused := false
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		pausedValue := 0
		if paused {
			pausedValue = 1
		}
		return Boundary[boundaryTestProps]{Key: "loop-boundary", Props: boundaryTestProps{Value: pausedValue}, Build: func(boundaryTestProps) Widget {
			builds++
			return LoopAnimation{Key: "boundary-loop", Duration: 20 * time.Millisecond, Paused: paused, Builder: func(value float32) Widget {
				return Container{Width: 20 + value, Height: 20}
			}}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()

	renderBoundaryTestFrame(host, 100)
	time.Sleep(3 * time.Millisecond)
	renderBoundaryTestFrame(host, 100)
	if builds != 2 {
		t.Fatalf("running loop build count = %d, want 2", builds)
	}
	paused = true
	renderBoundaryTestFrame(host, 100)
	pausedBuilds := builds
	renderBoundaryTestFrame(host, 100)
	if builds != pausedBuilds {
		t.Fatalf("paused loop rebuilt: before %d after %d", pausedBuilds, builds)
	}
}

func TestBoundaryScrollControllerInvalidatesCache(t *testing.T) {
	controller := NewScrollController(0)
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "scroll-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return ScrollView{Key: "boundary-scroll", Width: 50, Height: 20, ContentHeight: 100, Controller: controller, Child: Container{Width: 50, Height: 100}}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if builds != 1 {
		t.Fatalf("stable scroll build count = %d, want 1", builds)
	}
	if !controller.JumpTo(10) {
		t.Fatal("scroll controller did not change offset")
	}
	renderBoundaryTestFrame(host, 100)
	if builds != 2 {
		t.Fatalf("scrolled boundary build count = %d, want 2", builds)
	}

	// Bypass retained-element invalidation to prove that the implicit offset key
	// independently protects a Boundary from reusing stale scroll geometry.
	controller.mu.Lock()
	controller.offset = 15
	controller.mu.Unlock()
	renderBoundaryTestFrame(host, 100)
	if builds != 3 {
		t.Fatalf("implicit scroll dependency build count = %d, want 3", builds)
	}
}

func TestBoundaryKillSwitchPreservesStructureButDisablesReuse(t *testing.T) {
	t.Setenv(DisableIncrementalEnvironment, "1")
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "disabled-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return Container{Width: 20, Height: 20}
		}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	renderBoundaryTestFrame(host, 100)
	if builds != 2 {
		t.Fatalf("disabled boundary build count = %d, want 2", builds)
	}
}

func TestBoundaryReportsMissingBuild(t *testing.T) {
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "missing-build", Props: boundaryTestProps{}}
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	diagnostics := host.Snapshot().Diagnostics
	if len(diagnostics) != 1 || diagnostics[0] != `boundary "missing-build" requires a build function` {
		t.Fatalf("missing build diagnostics = %v", diagnostics)
	}
}

func TestBoundaryReportsDuplicateCachedSubtreeReuse(t *testing.T) {
	tree := newElementTree(&Host{})
	tree.beginFrame()
	builds := 0
	state := &boundaryState[boundaryTestProps]{
		hasCache: true, props: boundaryTestProps{}, constraints: constraints{width: 100, height: 100},
		node: &node{bounds: woxui.Rect{Width: 20, Height: 20}}, reusedAt: tree.generation,
	}
	boundaryLayout[boundaryTestProps]{
		boundary: Boundary[boundaryTestProps]{Key: "duplicate-reuse", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return Container{Width: 20, Height: 20}
		}},
		state: state,
	}.layout(context{elements: tree}, constraints{width: 100, height: 100})

	if builds != 1 {
		t.Fatalf("duplicate reuse fallback build count = %d, want 1", builds)
	}
	if len(tree.diagnostics) != 1 || tree.diagnostics[0] != `boundary "duplicate-reuse" reused the same cached subtree more than once in frame 1` {
		t.Fatalf("duplicate reuse diagnostics = %v", tree.diagnostics)
	}
}

func TestBoundaryRainbowHighlightsOnlyRebuiltSubtrees(t *testing.T) {
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "rainbow-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			return Container{Width: 20, Height: 20}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	if err := host.SetRepaintDebugMode(RepaintDebugRainbow); err != nil {
		t.Fatal(err)
	}

	first := &woxui.DisplayList{}
	host.Frame(first, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
	second := &woxui.DisplayList{}
	host.Frame(second, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
	if first.CommandCount() != 1 || second.CommandCount() != 0 {
		t.Fatalf("rainbow command counts = first %d second %d, want 1/0", first.CommandCount(), second.CommandCount())
	}
}

func TestBoundaryVerifyReportsMutableExternalCapture(t *testing.T) {
	color := woxui.Color{R: 255, A: 255}
	builds := 0
	host := NewHost(func(woxui.FrameInfo) Widget {
		return Boundary[boundaryTestProps]{Key: "verify-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return Container{Width: 20, Height: 20, Color: color}
		}}
	})
	host.AttachServices(&fakeHostServices{})
	if err := host.SetRepaintDebugMode(RepaintDebugVerify); err != nil {
		t.Fatal(err)
	}

	renderBoundaryTestFrame(host, 100)
	color = woxui.Color{B: 255, A: 255}
	renderBoundaryTestFrame(host, 100)
	if builds != 3 {
		t.Fatalf("verify build count = %d, want shadow and fallback rebuild for a total of 3", builds)
	}
	diagnostics := host.Snapshot().Diagnostics
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], `boundary "verify-boundary" cache verification failed`) {
		t.Fatalf("verify diagnostics = %v", diagnostics)
	}
}

func TestHostRepaintDebugModeValidatesRuntimeSwitch(t *testing.T) {
	services := &fakeHostServices{}
	host := NewHost(func(woxui.FrameInfo) Widget { return Container{Width: 20, Height: 20} })
	host.AttachServices(services)
	if err := host.SetRepaintDebugMode("unknown"); err == nil {
		t.Fatal("unsupported repaint mode was accepted")
	}
	if err := host.SetRepaintDebugMode(RepaintDebugDamage); err != nil {
		t.Fatal(err)
	}
	if host.RepaintDebugMode() != RepaintDebugDamage || services.invalidations == 0 {
		t.Fatalf("runtime repaint mode = %q invalidations = %d", host.RepaintDebugMode(), services.invalidations)
	}
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
	if displayList.CommandCount() != 1 {
		t.Fatalf("damage overlay command count = %d, want 1", displayList.CommandCount())
	}
}

func TestBoundaryAccessibilityCacheTranslatesBoundsAndRefreshesFocus(t *testing.T) {
	left := float32(10)
	builds := 0
	services := &fakeHostServices{}
	host := NewHost(func(woxui.FrameInfo) Widget {
		return boundaryOffset{left: left, child: Boundary[boundaryTestProps]{Key: "a11y-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			builds++
			return Semantics{
				Key: "a11y-focus", AutomationID: "a11y-cached", Role: woxui.AccessibilityRoleButton, Label: "Cached",
				Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
				Child:   Focusable{Key: "a11y-focus", Child: Container{Width: 20, Height: 20}},
			}
		}}}
	})
	host.AttachServices(services)

	renderBoundaryTestFrame(host, 100)
	cache := host.root.children[0].boundary
	if cache == nil || !cache.a11yValid {
		t.Fatal("boundary did not retain its accessibility segment")
	}
	if !host.RequestFocus("a11y-focus") {
		t.Fatal("failed to focus cached boundary descendant")
	}
	left = 30
	renderBoundaryTestFrame(host, 100)
	node := findAutomationNode(t, services.tree, "a11y-cached")
	if builds != 1 || cache.identityReuses != 1 || cache.a11yReuses != 1 {
		t.Fatalf("boundary cache counts = builds %d identity %d a11y %d, want 1/1/1", builds, cache.identityReuses, cache.a11yReuses)
	}
	if node.Bounds.X != 30 || !node.Focused || !node.Focusable || !containsAction(node.Actions, woxui.AccessibilityActionFocus) {
		t.Fatalf("translated cached accessibility node = %+v", node)
	}
}

func TestBoundaryAccessibilityCacheRejectsChangedIdentityPath(t *testing.T) {
	wrapped := false
	host := NewHost(func(woxui.FrameInfo) Widget {
		boundary := Boundary[boundaryTestProps]{Key: "identity-boundary", Props: boundaryTestProps{}, Build: func(boundaryTestProps) Widget {
			return Semantics{AutomationID: "identity-child", Role: woxui.AccessibilityRoleGroup, Label: "Child", Child: Container{Width: 20, Height: 20}}
		}}
		root := Container{Width: 100, Height: 100, Child: boundary}
		if wrapped {
			return Semantics{Key: "new-parent", AutomationID: "identity-parent", Role: woxui.AccessibilityRoleGroup, Label: "Parent", Child: root}
		}
		return root
	})
	host.AttachServices(&fakeHostServices{})

	renderBoundaryTestFrame(host, 100)
	cache := host.root.children[0].boundary
	firstID := findAutomationNode(t, host.Snapshot().Tree, "identity-child").ID
	wrapped = true
	renderBoundaryTestFrame(host, 100)
	secondID := findAutomationNode(t, host.Snapshot().Tree, "identity-child").ID
	if firstID == secondID {
		t.Fatalf("changed identity path retained node ID %d", firstID)
	}
	if cache.identityReuses != 0 || cache.a11yReuses != 0 || cache.a11yRootID != host.root.children[0].id {
		t.Fatalf("identity-changed cache = identity reuses %d a11y reuses %d root %d current %d", cache.identityReuses, cache.a11yReuses, cache.a11yRootID, host.root.children[0].id)
	}
}
