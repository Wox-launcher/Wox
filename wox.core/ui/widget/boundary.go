package widget

import (
	"fmt"
	"os"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

// DisableIncrementalEnvironment forces every Boundary to rebuild while preserving retained element identity.
const DisableIncrementalEnvironment = "WOX_DISABLE_INCREMENTAL"

// BoundaryProps supplies value equality for one immutable Boundary input.
type BoundaryProps[T any] interface {
	Equal(T) bool
}

// Boundary retains one laid-out node subtree while its explicit and implicit inputs remain unchanged.
type Boundary[T BoundaryProps[T]] struct {
	Key   Key
	Label string
	Props T
	Build func(T) Widget
}

type animationDependencyKind uint8

const (
	animationDependencyFloat animationDependencyKind = iota
	animationDependencyLoop
)

type animationDependency struct {
	key   Key
	kind  animationDependencyKind
	value float32
}

type scrollDependency struct {
	controller *ScrollController
	offset     float32
}

type dynamicUse struct {
	animations []animationDependency
	scrolls    []scrollDependency
}

func (u *dynamicUse) merge(other dynamicUse) {
	if u == nil {
		return
	}
	u.animations = append(u.animations, other.animations...)
	u.scrolls = append(u.scrolls, other.scrolls...)
}

func (u dynamicUse) matches(ctx context) bool {
	for _, dependency := range u.animations {
		value, found := ctx.animation.observe(dependency)
		if !found || value != dependency.value {
			return false
		}
	}
	for _, dependency := range u.scrolls {
		if dependency.controller == nil || dependency.controller.Offset() != dependency.offset {
			return false
		}
	}
	return true
}

type boundaryState[T BoundaryProps[T]] struct {
	hasCache     bool
	props        T
	constraints  constraints
	node         *node
	dynamic      dynamicUse
	repaints     uint64
	repaintTimes []time.Time
	reusedAt     uint64
	cache        boundaryCache
}

type boundaryCache struct {
	hit                 bool
	node                *node
	identityValid       bool
	identityRootPath    string
	identityEntries     []boundaryIdentityEntry
	identityDiagnostics []string
	identityReuses      uint64
	// nestedOwners keeps descendant Boundary owners alive on a parent cache hit
	// so sweepIdentities does not walk every cached identity entry.
	nestedOwners  []*identityOwner
	a11yValid     bool
	a11yOrigin    woxui.Point
	a11yRootID    woxui.AccessibilityNodeID
	a11yNodes     []woxui.AccessibilityNode
	a11yRootIDs   []woxui.AccessibilityNodeID
	a11yReuses    uint64
	globalBounds  woxui.Rect
	identityOwner identityOwner
}

type identityOwner struct {
	seen uint64
}

type identityBinding struct {
	id    woxui.AccessibilityNodeID
	owner *identityOwner
	gen   uint64
}

type boundaryIdentityEntry struct {
	path   string
	node   *node
	parent *node
	id     woxui.AccessibilityNodeID
}

func (*boundaryState[T]) InitState(StateContext, any) {}

func (*boundaryState[T]) DidUpdateWidget(StateContext, any, any) {}

func (s *boundaryState[T]) Build(context StateContext, widget any) Widget {
	return boundaryLayout[T]{boundary: widget.(Boundary[T]), state: s, element: context.element, dirty: context.dirty}
}

func (*boundaryState[T]) Dispose() {}

type boundaryLayout[T BoundaryProps[T]] struct {
	boundary Boundary[T]
	state    *boundaryState[T]
	element  *stateElement
	dirty    bool
}

func (w Boundary[T]) layout(ctx context, available constraints) *node {
	return (Stateful{
		Key: w.Key, Type: (*boundaryState[T])(nil), Widget: w,
		CreateState: func() State { return &boundaryState[T]{} },
	}).layout(ctx, available)
}

// layout reuses the canonical local subtree only after every cache dependency is validated.
func (w boundaryLayout[T]) layout(ctx context, available constraints) *node {
	state := w.state
	if w.element != nil {
		w.element.boundary = &state.cache
	}
	oldBounds := state.cache.globalBounds
	if oldBounds == (woxui.Rect{}) && state.node != nil {
		oldBounds = state.node.bounds
	}
	cacheHit := !incrementalDisabled() && state.hasCache && state.node != nil && !w.dirty && state.constraints == available && w.boundary.Props.Equal(state.props) && state.dynamic.matches(ctx)
	if cacheHit && ctx.elements != nil && state.reusedAt == ctx.elements.generation {
		ctx.elements.diagnostics = append(ctx.elements.diagnostics, fmt.Sprintf("boundary %q reused the same cached subtree more than once in frame %d", boundaryLabel(w.boundary), ctx.elements.generation))
		cacheHit = false
	}
	if cacheHit {
		state.node.bounds.X = 0
		state.node.bounds.Y = 0
		if ctx.debug != nil && ctx.debug.mode == RepaintDebugVerify {
			if err := w.verifyCachedNode(ctx, available, state.node); err != nil {
				ctx.elements.diagnostics = append(ctx.elements.diagnostics, fmt.Sprintf("boundary %q cache verification failed: %v", boundaryLabel(w.boundary), err))
				cacheHit = false
			}
		}
	}
	if cacheHit {
		ctx.elements.markSubtreeSeen(w.element)
		state.cache.hit = true
		state.cache.node = state.node
		state.node.boundary = &state.cache
		state.reusedAt = ctx.elements.generation
		if ctx.work != nil {
			ctx.work.boundaryReuses++
		}
		if ctx.dynamic != nil {
			ctx.dynamic.merge(state.dynamic)
		}
		ctx.damage.add(oldBounds, state.node, false)
		return state.node
	}

	probe := dynamicUse{}
	buildContext := ctx
	buildContext.dynamic = &probe
	var result *node
	if w.boundary.Build == nil {
		if ctx.elements != nil {
			ctx.elements.diagnostics = append(ctx.elements.diagnostics, fmt.Sprintf("boundary %q requires a build function", boundaryLabel(w.boundary)))
		}
		result = &node{key: w.boundary.Key, kind: "boundary"}
	} else if child := w.boundary.Build(w.boundary.Props); child != nil {
		result = child.layout(buildContext, available)
	} else {
		result = &node{key: w.boundary.Key, kind: "boundary"}
	}
	if result.key == "" {
		result.key = w.boundary.Key
	}
	if result.kind == "" {
		result.kind = "boundary"
	}
	state.hasCache = true
	state.props = w.boundary.Props
	state.constraints = available
	state.node = result
	state.cache.hit = false
	state.cache.node = result
	state.cache.a11yValid = false
	result.boundary = &state.cache
	state.dynamic = probe
	state.repaints++
	if ctx.work != nil {
		ctx.work.boundaryBuilds++
	}
	if ctx.debug != nil && ctx.debug.mode == RepaintDebugCounts {
		cutoff := ctx.debug.now.Add(-time.Second)
		kept := state.repaintTimes[:0]
		for _, repaintAt := range state.repaintTimes {
			if !repaintAt.Before(cutoff) {
				kept = append(kept, repaintAt)
			}
		}
		state.repaintTimes = append(kept, ctx.debug.now)
		ctx.debug.repaints = append(ctx.debug.repaints, boundaryRepaint{node: result, repaintCount: state.repaints, recentCount: len(state.repaintTimes)})
	}
	state.reusedAt = 0
	ctx.damage.add(oldBounds, result, true)
	if ctx.dynamic != nil {
		ctx.dynamic.merge(probe)
	}
	return result
}

// verifyCachedNode shadow-builds a cache hit without retained elements and compares its paint stream.
func (w boundaryLayout[T]) verifyCachedNode(ctx context, available constraints, cached *node) error {
	if w.boundary.Build == nil {
		return nil
	}
	shadowContext := ctx
	shadowContext.elements = nil
	shadowContext.element = nil
	shadowContext.dynamic = nil
	shadowContext.debug = nil
	child := w.boundary.Build(w.boundary.Props)
	if child == nil {
		return fmt.Errorf("shadow build returned nil")
	}
	shadow := child.layout(shadowContext, available)
	if shadow == nil {
		return fmt.Errorf("shadow layout returned nil")
	}
	if shadow.key == "" {
		shadow.key = w.boundary.Key
	}
	if err := verifyNodeTopology(cached, shadow); err != nil {
		return err
	}
	focused := ^woxui.AccessibilityNodeID(0)
	cachedList := &woxui.DisplayList{}
	shadowList := &woxui.DisplayList{}
	cached.draw(cachedList, focused, focused, true, false, false, nil)
	shadow.draw(shadowList, focused, focused, true, false, false, nil)
	return cachedList.Compare(shadowList)
}

func incrementalDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DisableIncrementalEnvironment))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boundaryLabel[T BoundaryProps[T]](boundary Boundary[T]) string {
	if strings.TrimSpace(boundary.Label) != "" {
		return boundary.Label
	}
	return string(boundary.Key)
}
