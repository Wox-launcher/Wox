package widget

import (
	stdcontext "context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	woxui "wox/ui/runtime"
)

const (
	caretBlinkInterval = 500 * time.Millisecond
	multiTapInterval   = 500 * time.Millisecond
	multiTapDistance   = float32(4)
)

// AutomationSnapshot is the immutable retained tree exposed to test drivers.
type AutomationSnapshot struct {
	Tree        woxui.AccessibilityTree
	Diagnostics []string
}

// HostServices is the minimal native surface required by the retained widget host.
type HostServices interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
	Invalidate() error
	InvalidateRect(rect woxui.Rect) error
	SetTextInputState(state woxui.TextInputState) error
	SetPointerCursor(cursor woxui.PointerCursor) error
	UpdateAccessibility(tree woxui.AccessibilityTree, handler woxui.AccessibilityActionHandler) error
}

type frameMetricsHostServices interface {
	RecordFramePhase(frameID uint64, phase woxui.FrameMetricPhase, duration time.Duration)
	RecordFrameCounts(frameID uint64, nodes, commands, accessibilityNodes int, logicalDamage woxui.Rect)
	RecordFrameWork(frameID uint64, work woxui.FrameWorkMetrics)
}

type displayListDamageHostServices interface {
	DisplayListDamageCullingEnabled() bool
}

// Host reconciles, lays out, paints, and routes input for one retained widget tree.
type Host struct {
	window HostServices
	build  func(frame woxui.FrameInfo) Widget
	root   *node

	nextNodeID    woxui.AccessibilityNodeID
	identities    map[string]woxui.AccessibilityNodeID
	identityMeta  map[string]identityBinding
	identityFrame uint64
	nodes         map[woxui.AccessibilityNodeID]*node

	hovered    woxui.AccessibilityNodeID
	rawHovered woxui.AccessibilityNodeID
	rawPressed woxui.AccessibilityNodeID
	pressed    woxui.AccessibilityNodeID
	pressedAt  woxui.Point
	dragging   bool
	// selecting tracks the gesture node that started a drag-based selection, so subsequent
	// pointer-move events extend its selection until the pointer is released.
	selecting          woxui.AccessibilityNodeID
	selectingGestureID string
	panning            woxui.AccessibilityNodeID
	lastTapID          woxui.AccessibilityNodeID
	lastTapAt          time.Time
	lastTapPosition    woxui.Point
	lastTapCount       int

	focused woxui.AccessibilityNodeID
	// focusVisible mirrors :focus-visible so pointer focus keeps keyboard behavior without painting a ring.
	focusVisible bool
	modalScopes  []woxui.AccessibilityNodeID
	scopeRestore map[woxui.AccessibilityNodeID]woxui.AccessibilityNodeID

	// overlay is painted above the build result in window coordinates (context menus, etc.).
	overlay       Widget
	overlayOwner  Key
	overlayToken  uint64
	lastFrameSize woxui.Size

	generation uint64
	snapshot   atomic.Value
	changeMu   sync.Mutex
	change     chan struct{}
	reported   map[string]bool

	caretBlinkMu         sync.Mutex
	caretBlinkTimer      *time.Timer
	caretBlinkActive     bool
	caretVisible         bool
	caretBlinkGeneration uint64
	windowFocused        bool
	animations           animationHost
	elements             *elementTree
	postFrame            []func()
	disposed             bool
	activeFrameMetricsID uint64
	repaintDebugMode     RepaintDebugMode
	damageMu             sync.Mutex
	pendingDamage        woxui.Rect
	fullDamage           bool
	caretDamage          woxui.Rect
}

// NewHost creates a retained host whose builder runs once per invalidated frame.
func NewHost(build func(frame woxui.FrameInfo) Widget) *Host {
	host := &Host{
		build:            build,
		identities:       map[string]woxui.AccessibilityNodeID{},
		identityMeta:     map[string]identityBinding{},
		nodes:            map[woxui.AccessibilityNodeID]*node{},
		scopeRestore:     map[woxui.AccessibilityNodeID]woxui.AccessibilityNodeID{},
		change:           make(chan struct{}),
		reported:         map[string]bool{},
		caretVisible:     true,
		windowFocused:    true,
		repaintDebugMode: repaintDebugModeFromEnvironment(),
	}
	host.snapshot.Store(AutomationSnapshot{})
	host.elements = newElementTree(host)
	return host
}

// Attach connects platform services used during layout, invalidation, and accessibility.
func (h *Host) Attach(window *woxui.Window) {
	h.window = window
}

// AttachServices connects a virtual or native host surface using the same widget execution path.
func (h *Host) AttachServices(services HostServices) {
	h.window = services
}

// SetRepaintDebugMode changes repaint diagnostics for subsequent frames.
func (h *Host) SetRepaintDebugMode(mode RepaintDebugMode) error {
	parsed, err := parseRepaintDebugMode(string(mode))
	if err != nil {
		return err
	}
	if h.repaintDebugMode == parsed {
		return nil
	}
	h.repaintDebugMode = parsed
	h.invalidate()
	return nil
}

// RepaintDebugMode returns the active repaint diagnostic mode.
func (h *Host) RepaintDebugMode() RepaintDebugMode {
	if h == nil {
		return RepaintDebugOff
	}
	return h.repaintDebugMode
}

// InvalidateBoundary schedules repaint for one retained Boundary's current bounds.
func (h *Host) InvalidateBoundary(key Key) bool {
	if h == nil || h.elements == nil {
		return false
	}
	rect, found := h.elements.boundaryBounds(key)
	if !found {
		return false
	}
	h.invalidateRect(rect)
	return true
}

// SetWindowFocused keeps the retained editor focus while suspending its caret and IME when the native window is inactive.
func (h *Host) SetWindowFocused(focused bool) {
	h.caretBlinkMu.Lock()
	if h.windowFocused == focused {
		h.caretBlinkMu.Unlock()
		return
	}
	h.windowFocused = focused
	window := h.window
	h.caretBlinkMu.Unlock()
	if !focused {
		h.updateCaretBlink(false)
		if window != nil {
			_ = window.SetTextInputState(woxui.TextInputState{})
		}
	}
	if window != nil {
		_ = window.Invalidate()
	}
}

// WindowFocused reports whether this host's native window currently owns key focus.
func (h *Host) WindowFocused() bool {
	if h == nil {
		return false
	}
	h.caretBlinkMu.Lock()
	defer h.caretBlinkMu.Unlock()
	return h.windowFocused
}

// Frame reconciles one widget description, publishes semantics, and paints it.
func (h *Host) Frame(displayList *woxui.DisplayList, frame woxui.FrameInfo) {
	if h.disposed || h.window == nil || h.build == nil {
		h.updateCaretBlink(false)
		h.animations.reset()
		return
	}
	frameID := displayList.FrameMetricsID()
	h.activeFrameMetricsID = frameID
	defer func() { h.activeFrameMetricsID = 0 }()
	buildLayoutStart := time.Now()
	damage := h.consumeFrameDamage(frame.Damage, frame.Size)
	// Preserve the zero-rectangle full-frame sentinel instead of narrowing it to rebuilt Boundary bounds.
	fullDamage := damage.Width <= 0 || damage.Height <= 0
	h.elements.beginFrame()
	h.caretBlinkMu.Lock()
	frame.WindowFocused = h.windowFocused
	h.caretBlinkMu.Unlock()
	base := h.build(frame)
	h.lastFrameSize = frame.Size
	if base == nil {
		h.elements.endFrame()
		h.recordFramePhase(frameID, woxui.FrameMetricBuildLayout, time.Since(buildLayoutStart))
		h.recordFrameCounts(frameID, 0, displayList.CommandCount(), 0, displayList.NativeDamage())
		h.recordFrameWork(frameID, frameWorkCounters{}.metrics(displayList.TextDrawCount(), displayList.ImageDrawCount()))
		h.updateCaretBlink(false)
		h.animations.reset()
		return
	}

	var focusedKey Key
	if old := h.nodes[h.focused]; old != nil {
		focusedKey = old.key
	}
	oldHovered := h.nodes[h.hovered]
	oldHoveredBounds := globalRect(oldHovered)
	animation := h.animations.beginFrame()
	var debugFrame *repaintDebugFrame
	if h.repaintDebugMode != RepaintDebugOff {
		debugFrame = &repaintDebugFrame{
			mode: h.repaintDebugMode, now: time.Now(), repaintCount: h.elements.generation,
		}
	}
	caretVisible := h.caretVisibleForFrame()
	damageTracker := &frameDamageTracker{}
	work := &frameWorkCounters{}
	layoutCtx := context{window: h.window, animation: animation, damage: damageTracker, debug: debugFrame, elements: h.elements, element: h.elements.root, work: work}
	layoutConstraints := constraints{width: frame.Size.Width, height: frame.Size.Height}
	// Layout the base tree once so overlay ownership can be validated against this frame's keys
	// before composing the already-built base node with a separately laid-out overlay.
	root := base.layout(layoutCtx, layoutConstraints)
	if h.overlay != nil && h.overlayOwner != "" && !nodeTreeHasKey(root, h.overlayOwner) {
		h.overlay = nil
		h.overlayOwner = ""
	}
	if overlay := h.overlay; overlay != nil {
		overlayNode := overlay.layout(layoutCtx, layoutConstraints)
		root = composeOverlayRoot(root, overlayNode, frame.Size)
	}
	diagnostics, removedDamage := h.elements.endFrame()
	prepareNodeTree(root)
	boundaryDamage := damageTracker.resolve(woxui.Rect{})
	if !fullDamage {
		damage = unionDamageRects(damage, boundaryDamage)
		damage = unionDamageRects(damage, removedDamage)
	}
	if damage.Width > 0 && damage.Height > 0 {
		damage = expandDamageRect(damage, 4)
	}
	damage = clipDamageRect(damage, frame.Size)
	if incrementalDisabled() {
		damage = woxui.Rect{}
	}
	logicalDamage := damage
	if h.repaintDebugMode != RepaintDebugOff {
		debugFrame.repaintRegion = damage
		if damage.Width <= 0 || damage.Height <= 0 {
			debugFrame.repaintRegion = woxui.Rect{Width: frame.Size.Width, Height: frame.Size.Height}
		}
		damage = woxui.Rect{}
	}
	displayList.SetNativeDamage(damage)
	if services, ok := h.window.(displayListDamageHostServices); ok && services.DisplayListDamageCullingEnabled() {
		displayList.SetDamage(damage)
	}
	var vsync animationFrameScheduler
	if scheduler, ok := h.window.(animationFrameScheduler); ok {
		vsync = scheduler
	}
	h.animations.endFrame(animation, func() {
		if boundaryDamage.Width > 0 && boundaryDamage.Height > 0 {
			h.invalidateRect(boundaryDamage)
			return
		}
		h.invalidate()
	}, vsync)
	work.layoutVisits = countLayoutVisits(root)
	h.identityFrame++
	h.assignIdentities(root, nil, "root", 0, h.identities, h.identities, h.nodes, &diagnostics, nil, work)
	h.sweepIdentities()
	h.root = root
	h.reconcileTransientState(oldHovered, oldHoveredBounds)
	h.reconcileOverlayOwner()
	// Remap focus by stable key before reconcileFocus so autofocus never wins and
	// pure ID remapping does not fire blur/focus or SetTextInputState.
	if focusedKey != "" {
		if current := h.nodes[h.focused]; current == nil || !h.isFocusable(current) || current.key != focusedKey {
			for _, node := range h.nodes {
				if node.key == focusedKey && h.isFocusable(node) {
					h.focused = node.id
					break
				}
			}
		}
	}
	h.reconcileFocus()
	h.updateCaretBlink(nodeHasActiveCaret(root, h.focused, false, false))
	h.setCaretDamage(activeCaretDamage(root, h.focused, false, false))
	h.recordFramePhase(frameID, woxui.FrameMetricBuildLayout, time.Since(buildLayoutStart))

	drawStart := time.Now()
	displayList.Clear(woxui.Color{})
	focusRingTarget := h.focused
	if !h.focusVisible {
		focusRingTarget = 0
	}
	h.root.draw(displayList, h.focused, focusRingTarget, caretVisible, false, false, work)
	debugFrame.draw(displayList)
	h.recordFramePhase(frameID, woxui.FrameMetricDrawRecord, time.Since(drawStart))

	accessibilityStart := time.Now()
	h.generation++
	tree, diagnostics := h.buildAccessibilityTree(diagnostics, work)
	h.publishSnapshot(tree, diagnostics)
	if err := h.window.UpdateAccessibility(tree, h.dispatchAccessibilityAction); err != nil {
		h.reportDiagnostic(fmt.Sprintf("publish accessibility tree: %v", err))
	}
	h.recordFramePhase(frameID, woxui.FrameMetricAccessibility, time.Since(accessibilityStart))
	h.recordFrameCounts(frameID, len(h.nodes), displayList.CommandCount(), len(tree.Nodes), logicalDamage)
	h.recordFrameWork(frameID, work.metrics(displayList.TextDrawCount(), displayList.ImageDrawCount()))
	h.syncTextInput()
	h.runPostFrameCallbacks()
}

// RecordSnapshotDuration records launcher-specific snapshot preparation inside the active Host frame.
func (h *Host) RecordSnapshotDuration(duration time.Duration) {
	if h == nil || h.activeFrameMetricsID == 0 {
		return
	}
	h.recordFramePhase(h.activeFrameMetricsID, woxui.FrameMetricSnapshot, duration)
}

func (h *Host) recordFramePhase(frameID uint64, phase woxui.FrameMetricPhase, duration time.Duration) {
	if frameID == 0 {
		return
	}
	if services, ok := h.window.(frameMetricsHostServices); ok {
		services.RecordFramePhase(frameID, phase, duration)
	}
}

func (h *Host) recordFrameCounts(frameID uint64, nodes, commands, accessibilityNodes int, logicalDamage woxui.Rect) {
	if frameID == 0 {
		return
	}
	if services, ok := h.window.(frameMetricsHostServices); ok {
		services.RecordFrameCounts(frameID, nodes, commands, accessibilityNodes, logicalDamage)
	}
}

func (h *Host) recordFrameWork(frameID uint64, work woxui.FrameWorkMetrics) {
	if frameID == 0 {
		return
	}
	if services, ok := h.window.(frameMetricsHostServices); ok {
		services.RecordFrameWork(frameID, work)
	}
}

// countLayoutVisits counts nodes whose layout actually ran. Cached Boundary
// subtrees are counted as one visit at the reused root.
func countLayoutVisits(current *node) int {
	if current == nil {
		return 0
	}
	if current.boundary != nil && current.boundary.hit {
		return 1
	}
	count := 1
	for _, child := range current.children {
		count += countLayoutVisits(child)
	}
	return count
}

// runPostFrameCallbacks executes retained lifecycle work after the current node tree is addressable.
func (h *Host) runPostFrameCallbacks() {
	callbacks := h.postFrame
	h.postFrame = nil
	for _, callback := range callbacks {
		callback()
	}
}

// Dispose releases retained widget state and frame-owned resources for this Host.
func (h *Host) Dispose() {
	if h == nil || h.disposed {
		return
	}
	h.disposed = true
	h.updateCaretBlink(false)
	h.animations.reset()
	if h.elements != nil {
		h.elements.dispose()
	}
	h.root = nil
	h.postFrame = nil
	h.overlay = nil
	h.overlayOwner = ""
	h.nodes = map[woxui.AccessibilityNodeID]*node{}
	h.identities = map[string]woxui.AccessibilityNodeID{}
	h.identityMeta = map[string]identityBinding{}
}

func (h *Host) assignIdentities(current *node, parent *node, parentPath string, index int, previous, identities map[string]woxui.AccessibilityNodeID, nodes map[woxui.AccessibilityNodeID]*node, diagnostics *[]string, collectors []*boundaryCache, work *frameWorkCounters) {
	if current == nil {
		return
	}
	current.parent = parent
	kind := current.kind
	if kind == "" {
		kind = nodeKind(current)
		current.kind = kind
	}
	segment := fmt.Sprintf("%s[%d]", kind, index)
	if current.key != "" {
		segment = fmt.Sprintf("%s{%s}", kind, current.key)
	}
	path := parentPath + "/" + segment
	cache := current.boundary
	if cache != nil && cache.hit && cache.identityValid && cache.identityRootPath == path {
		cache.identityReuses++
		markIdentityOwnersSeen(cache, h.identityFrame)
		*diagnostics = append(*diagnostics, cache.identityDiagnostics...)
		if len(cache.identityEntries) > 0 {
			entry := cache.identityEntries[0]
			entry.parent = parent
			entry.node.parent = parent
			cache.identityEntries[0] = entry
		}
		for _, collector := range collectors {
			collector.identityEntries = append(collector.identityEntries, cache.identityEntries...)
			appendNestedIdentityOwners(collector, cache)
		}
		return
	}
	if work != nil {
		work.identityVisits++
	}
	diagnosticStart := len(*diagnostics)
	var previousEntries []boundaryIdentityEntry
	if cache != nil {
		previousEntries = append([]boundaryIdentityEntry(nil), cache.identityEntries...)
		cache.identityEntries = cache.identityEntries[:0]
		cache.nestedOwners = cache.nestedOwners[:0]
		collectors = append(collectors, cache)
	}
	if id, ok := previous[path]; ok {
		current.id = id
	} else {
		h.nextNodeID++
		current.id = h.nextNodeID
	}
	owner := (*identityOwner)(nil)
	if cache != nil {
		owner = &cache.identityOwner
	} else if len(collectors) > 0 {
		owner = &collectors[len(collectors)-1].identityOwner
	}
	h.upsertIdentity(path, current.id, current, owner, work)
	entry := boundaryIdentityEntry{path: path, node: current, parent: parent, id: current.id}
	for _, collector := range collectors {
		collector.identityEntries = append(collector.identityEntries, entry)
	}

	siblingKeys := map[string]int{}
	for childIndex, child := range current.children {
		if child == nil {
			continue
		}
		childPath := path
		if child.key != "" {
			identity := string(child.key) + "|" + nodeKind(child)
			if first, exists := siblingKeys[identity]; exists {
				*diagnostics = append(*diagnostics, fmt.Sprintf("duplicate widget key %q under %s at children %d and %d", child.key, path, first, childIndex))
				childPath = fmt.Sprintf("%s/duplicate[%d]", path, childIndex)
			} else {
				siblingKeys[identity] = childIndex
			}
		}
		h.assignIdentities(child, current, childPath, childIndex, previous, identities, nodes, diagnostics, collectors, work)
	}
	if cache != nil {
		cache.identityRootPath = path
		cache.identityDiagnostics = append(cache.identityDiagnostics[:0], (*diagnostics)[diagnosticStart:]...)
		cache.identityValid = true
		cache.identityOwner.seen = h.identityFrame
		h.removeStaleIdentityEntries(previousEntries, cache.identityEntries)
		for _, collector := range collectors[:len(collectors)-1] {
			collector.nestedOwners = append(collector.nestedOwners, &cache.identityOwner)
		}
	}
}

// markIdentityOwnersSeen keeps a reused Boundary and its nested owners out of the identity sweep.
func markIdentityOwnersSeen(cache *boundaryCache, frame uint64) {
	if cache == nil {
		return
	}
	cache.identityOwner.seen = frame
	for _, owner := range cache.nestedOwners {
		if owner != nil {
			owner.seen = frame
		}
	}
}

// appendNestedIdentityOwners records a reused Boundary's owner tree on a parent that is rebuilding.
func appendNestedIdentityOwners(collector, cache *boundaryCache) {
	if collector == nil || cache == nil {
		return
	}
	collector.nestedOwners = append(collector.nestedOwners, &cache.identityOwner)
	collector.nestedOwners = append(collector.nestedOwners, cache.nestedOwners...)
}

func (h *Host) upsertIdentity(path string, id woxui.AccessibilityNodeID, current *node, owner *identityOwner, work *frameWorkCounters) {
	if h.identities[path] != id {
		if work != nil {
			work.identityUpserts++
		}
	}
	h.identities[path] = id
	h.nodes[id] = current
	h.identityMeta[path] = identityBinding{id: id, owner: owner, gen: h.identityFrame}
}

func (h *Host) removeStaleIdentityEntries(previous, current []boundaryIdentityEntry) {
	live := make(map[string]struct{}, len(current))
	for _, entry := range current {
		live[entry.path] = struct{}{}
	}
	for _, entry := range previous {
		if _, ok := live[entry.path]; ok {
			continue
		}
		if h.identities[entry.path] == entry.id {
			delete(h.identities, entry.path)
		}
		if h.nodes[entry.id] == entry.node {
			delete(h.nodes, entry.id)
		}
		delete(h.identityMeta, entry.path)
	}
}

func (h *Host) sweepIdentities() {
	for path, binding := range h.identityMeta {
		if binding.owner != nil && binding.owner.seen == h.identityFrame {
			continue
		}
		if binding.gen == h.identityFrame {
			continue
		}
		if h.identities[path] == binding.id {
			delete(h.identities, path)
		}
		if node := h.nodes[binding.id]; node != nil && node.id == binding.id {
			delete(h.nodes, binding.id)
		}
		delete(h.identityMeta, path)
	}
}

func nodeKind(current *node) string {
	switch {
	case current.semantic != nil:
		return "semantics"
	case current.focus != nil:
		return "focusable"
	case current.gesture != nil:
		return "gesture"
	case current.paint != nil:
		return "paint"
	default:
		return "layout"
	}
}

func (h *Host) reconcileTransientState(oldHovered *node, oldHoveredBounds woxui.Rect) {
	if h.hovered != 0 && h.nodes[h.hovered] == nil {
		if oldHovered != nil && oldHovered.gesture != nil {
			if oldHovered.gesture.onHover != nil {
				oldHovered.gesture.onHover(false)
			}
			if oldHovered.gesture.onHoverAt != nil {
				oldHovered.gesture.onHoverAt(false, oldHoveredBounds)
			}
		}
		h.hovered = 0
		if h.window != nil {
			_ = h.window.SetPointerCursor(woxui.PointerCursorDefault)
		}
	}
	// Form rebuilds may move a focused editor under a different retained path. Preserve pointer
	// capture by remapping the active selection through its stable gesture ID.
	if h.selecting != 0 && h.nodes[h.selecting] == nil {
		if replacement := h.gestureNodeByID(h.selectingGestureID); replacement != nil {
			h.selecting = replacement.id
			if h.pressed != 0 && h.nodes[h.pressed] == nil {
				h.pressed = replacement.id
			}
		} else {
			h.selecting = 0
			h.selectingGestureID = ""
			h.dragging = false
		}
	}
	if h.pressed != 0 && h.nodes[h.pressed] == nil {
		h.pressed = 0
		h.dragging = false
	}
	if h.lastTapID != 0 && h.nodes[h.lastTapID] == nil {
		h.lastTapID = 0
		h.lastTapAt = time.Time{}
		h.lastTapPosition = woxui.Point{}
		h.lastTapCount = 0
	}
}

func (h *Host) reconcileFocus() {
	oldScopes := append([]woxui.AccessibilityNodeID(nil), h.modalScopes...)
	h.modalScopes = h.collectModalScopes()
	common := 0
	for common < len(oldScopes) && common < len(h.modalScopes) && oldScopes[common] == h.modalScopes[common] {
		common++
	}
	for index := common; index < len(h.modalScopes); index++ {
		h.scopeRestore[h.modalScopes[index]] = h.focused
	}

	if current := h.nodes[h.focused]; h.focused != 0 && !h.isFocusable(current) {
		h.setFocus(0)
	}
	activeScope := h.activeModalScope()
	if h.focused != 0 && activeScope != 0 && !h.isDescendantOf(h.nodes[h.focused], activeScope) {
		h.setFocus(0)
	}
	if h.focused == 0 {
		if target := h.firstFocusable(activeScope, true); target != nil {
			h.setFocus(target.id)
		} else if activeScope != 0 {
			h.setFocusNode(h.firstFocusable(activeScope, false))
		}
	}
	if len(oldScopes) > len(h.modalScopes) {
		for index := len(oldScopes) - 1; index >= common; index-- {
			restore := h.scopeRestore[oldScopes[index]]
			delete(h.scopeRestore, oldScopes[index])
			if h.focused == 0 && h.isFocusable(h.nodes[restore]) {
				h.setFocus(restore)
				break
			}
		}
	}
}

func (h *Host) collectModalScopes() []woxui.AccessibilityNodeID {
	result := []woxui.AccessibilityNodeID{}
	var visit func(current *node)
	visit = func(current *node) {
		if current == nil {
			return
		}
		if current.scope != nil && current.scope.modal {
			result = append(result, current.id)
		}
		for _, child := range current.children {
			visit(child)
		}
	}
	visit(h.root)
	return result
}

func (h *Host) activeModalScope() woxui.AccessibilityNodeID {
	if len(h.modalScopes) == 0 {
		return 0
	}
	return h.modalScopes[len(h.modalScopes)-1]
}

func (h *Host) isDescendantOf(current *node, ancestorID woxui.AccessibilityNodeID) bool {
	for current != nil {
		if current.id == ancestorID {
			return true
		}
		current = current.parent
	}
	return false
}

func (h *Host) isFocusable(current *node) bool {
	return current != nil && current.focus != nil && !current.focus.disabled && (current.semantic == nil || !current.semantic.hidden)
}

func (h *Host) firstFocusable(scopeID woxui.AccessibilityNodeID, autofocusOnly bool) *node {
	var found *node
	var visit func(current *node)
	visit = func(current *node) {
		if current == nil || found != nil {
			return
		}
		if h.isFocusable(current) && (!autofocusOnly || current.focus.autofocus) {
			found = current
			return
		}
		for _, child := range current.children {
			visit(child)
		}
	}
	if scopeID != 0 {
		visit(h.nodes[scopeID])
	} else {
		visit(h.root)
	}
	return found
}

func (h *Host) focusOrder() []*node {
	result := []*node{}
	scope := h.activeModalScope()
	var visit func(current *node)
	visit = func(current *node) {
		if current == nil {
			return
		}
		if h.isFocusable(current) && !current.focus.skipTraversal {
			result = append(result, current)
		}
		for _, child := range current.children {
			visit(child)
		}
	}
	if scope != 0 {
		visit(h.nodes[scope])
	} else {
		visit(h.root)
	}
	return result
}

func (h *Host) moveFocus(reverse bool) bool {
	order := h.focusOrder()
	if len(order) == 0 {
		return false
	}
	index := -1
	for currentIndex, current := range order {
		if current.id == h.focused {
			index = currentIndex
			break
		}
	}
	if reverse {
		index--
		if index < 0 {
			index = len(order) - 1
		}
	} else {
		index = (index + 1) % len(order)
	}
	target := order[index].id
	if target == h.focused {
		if !h.focusVisible {
			h.focusVisible = true
			h.invalidate()
		}
		return true
	}
	h.focusVisible = true
	h.setFocus(target)
	return true
}

func (h *Host) setFocusNode(current *node) {
	if current == nil {
		return
	}
	h.setFocus(current.id)
}

func (h *Host) setFocus(id woxui.AccessibilityNodeID) {
	if id != 0 && !h.isFocusable(h.nodes[id]) {
		return
	}
	if activeScope := h.activeModalScope(); id != 0 && activeScope != 0 && !h.isDescendantOf(h.nodes[id], activeScope) {
		return
	}
	if h.focused == id {
		return
	}
	old := h.nodes[h.focused]
	h.focused = id
	if old != nil && old.focus != nil && old.focus.onFocusChange != nil {
		old.focus.onFocusChange(false)
	}
	current := h.nodes[h.focused]
	if current != nil && current.focus != nil && current.focus.onFocusChange != nil {
		current.focus.onFocusChange(true)
	}
	h.ensureFocusedVisible()
	h.resetCaretBlink()
	h.syncTextInput()
	h.invalidate()
}

// ensureFocusedVisible minimally scrolls the nearest clipped ancestor that hides the focused node.
func (h *Host) ensureFocusedVisible() {
	current := h.nodes[h.focused]
	if current == nil {
		return
	}
	for ancestor := current.parent; ancestor != nil; ancestor = ancestor.parent {
		if ancestor.scroll == nil {
			continue
		}
		offsetX, offsetY := offsetInAncestor(current, ancestor)
		start := offsetY + ancestor.scroll.offset
		end := start + current.bounds.Height
		if ancestor.scroll.horizontal {
			start = offsetX + ancestor.scroll.offset
			end = start + current.bounds.Width
		}
		if ancestor.scroll.ensureVisible(start, end) {
			return
		}
	}
}

// SetOverlay places a window-coordinate widget above the retained build tree.
// ownerKey identifies the widget that owns the overlay; when that key leaves the
// tree the overlay is cleared automatically so stale menus cannot outlive their field.
func (h *Host) SetOverlay(ownerKey Key, widget Widget) uint64 {
	if h == nil {
		return 0
	}
	h.overlayOwner = ownerKey
	h.overlayToken++
	h.overlay = widget
	h.invalidate()
	return h.overlayToken
}

// ClearOverlay removes the Host overlay layer when ownerKey and token still match.
// Pass token 0 to clear any overlay owned by ownerKey regardless of token.
func (h *Host) ClearOverlay(ownerKey Key, token uint64) {
	if h == nil {
		return
	}
	if h.overlay == nil {
		return
	}
	if ownerKey != "" && h.overlayOwner != ownerKey {
		return
	}
	if token != 0 && h.overlayToken != token {
		return
	}
	h.overlay = nil
	h.overlayOwner = ""
	h.invalidate()
}

// HasOverlay reports whether a window-level overlay is currently set.
func (h *Host) HasOverlay() bool {
	return h != nil && h.overlay != nil
}

// OverlayOwner returns the stable key that currently owns the Host overlay, if any.
func (h *Host) OverlayOwner() Key {
	if h == nil || h.overlay == nil {
		return ""
	}
	return h.overlayOwner
}

func (h *Host) reconcileOverlayOwner() {
	if h == nil || h.overlay == nil || h.overlayOwner == "" {
		return
	}
	for _, node := range h.nodes {
		if node.key == h.overlayOwner {
			return
		}
	}
	h.overlay = nil
	h.overlayOwner = ""
	// Root was built without this overlay already when ownership is checked pre-compose;
	// still invalidate so any dependent UI settles on the cleared state.
	h.invalidate()
}

func nodeTreeHasKey(root *node, key Key) bool {
	if root == nil || key == "" {
		return false
	}
	if root.key == key {
		return true
	}
	for _, child := range root.children {
		if nodeTreeHasKey(child, key) {
			return true
		}
	}
	return false
}

// composeOverlayRoot stacks an already-laid-out base tree under a separately laid-out overlay
// without invoking base.layout a second time.
func composeOverlayRoot(base, overlay *node, size woxui.Size) *node {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}
	return &node{
		kind:     "overlay-stack",
		bounds:   woxui.Rect{Width: size.Width, Height: size.Height},
		children: []*node{base, overlay},
	}
}

// FrameSize returns the most recent Frame logical size.
func (h *Host) FrameSize() woxui.Size {
	if h == nil {
		return woxui.Size{}
	}
	return h.lastFrameSize
}

// RequestFocus focuses the retained element with the matching widget key.
func (h *Host) RequestFocus(key Key) bool {
	for _, current := range h.nodes {
		if current.key == key && h.isFocusable(current) {
			h.setFocus(current.id)
			return true
		}
	}
	return false
}

// ClearFocus releases the retained focus node and its native text input state.
func (h *Host) ClearFocus() {
	h.setFocus(0)
}

func (h *Host) clearFocusForKey(key Key) {
	current := h.nodes[h.focused]
	if current != nil && current.key == key {
		h.setFocus(0)
	}
}

func (h *Host) isFocusedKey(key Key) bool {
	current := h.nodes[h.focused]
	return current != nil && current.key == key
}

// HasFocus reports whether the Host's single focused element has the given stable key.
func (h *Host) HasFocus(key Key) bool {
	return h != nil && h.isFocusedKey(key)
}

// FocusedKey reports the stable key of the Host's current logical focus owner.
func (h *Host) FocusedKey() Key {
	if h == nil {
		return ""
	}
	current := h.nodes[h.focused]
	if current == nil {
		return ""
	}
	return current.key
}

// BoundsForKey returns the latest laid-out bounds for a retained widget key.
func (h *Host) BoundsForKey(key Key) (woxui.Rect, bool) {
	for _, current := range h.nodes {
		if current.key == key {
			return globalRect(current), true
		}
	}
	return woxui.Rect{}, false
}

// FocusAutomationID focuses the accessible element with a stable automation identifier.
func (h *Host) FocusAutomationID(automationID string) bool {
	for _, current := range h.nodes {
		if current.semantic != nil && current.semantic.automationID == automationID && h.isFocusable(current) {
			h.setFocus(current.id)
			return true
		}
	}
	return false
}

// PerformAutomationAction invokes an accessibility action through the native UI thread.
func (h *Host) PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error {
	automationID = strings.TrimSpace(automationID)
	if automationID == "" {
		return fmt.Errorf("automation id is required")
	}
	var targetID woxui.AccessibilityNodeID
	for _, current := range h.Snapshot().Tree.Nodes {
		if current.AutomationID == automationID {
			targetID = current.ID
			break
		}
	}
	if targetID == 0 {
		return fmt.Errorf("automation element %q was not found", automationID)
	}
	return h.dispatchAccessibilityAction(targetID, action, value)
}

// Key routes one semantic key event through capture, target, and bubble phases.
func (h *Host) Key(event woxui.KeyEvent) bool {
	if event.Down {
		h.resetCaretBlink()
	}
	// The active modal gets first refusal so a nested dialog cannot let Escape reach its parent.
	if scope := h.nodes[h.activeModalScope()]; scope != nil && scope.scope != nil && scope.scope.onKey != nil && scope.scope.onKey(event) {
		return true
	}
	tabTraversal := event.Down && event.Key == woxui.KeyTab && !event.Composing && event.Modifiers & ^woxui.KeyModifierShift == 0
	target := h.nodes[h.focused]
	if target == nil {
		if tabTraversal {
			return h.moveFocus(event.Modifiers&woxui.KeyModifierShift != 0)
		}
		return false
	}
	path := []*node{}
	for current := target; current != nil; current = current.parent {
		path = append(path, current)
	}
	for index := len(path) - 1; index >= 0; index-- {
		if path[index].focus != nil && path[index].focus.onKeyCapture != nil && path[index].focus.onKeyCapture(event) {
			return true
		}
	}
	for _, current := range path {
		if current.focus != nil && current.focus.onKey != nil && current.focus.onKey(event) {
			return true
		}
	}
	// Focusable semantic controls inherit the same activation path used by accessibility and pointer input.
	if event.Down && !event.Composing && event.Modifiers == 0 && (event.Key == woxui.KeyEnter || event.Key == woxui.KeySpace) && target.semantic != nil && containsAction(target.semantic.actions, woxui.AccessibilityActionActivate) {
		if err := h.performAccessibilityAction(target.id, woxui.AccessibilityActionActivate, ""); err == nil {
			return true
		}
	}
	if tabTraversal {
		return h.moveFocus(event.Modifiers&woxui.KeyModifierShift != 0)
	}
	return false
}

// TextInput routes IME composition and commits only to the focused element.
func (h *Host) TextInput(event woxui.TextInputEvent) bool {
	h.resetCaretBlink()
	current := h.nodes[h.focused]
	return current != nil && current.focus != nil && current.focus.onTextInput != nil && current.focus.onTextInput(event)
}

func (h *Host) syncTextInput() {
	if h.window == nil || h.focused == 0 {
		return
	}
	current := h.nodes[h.focused]
	if current == nil || current.focus == nil {
		return
	}
	if current.focus.textInput == nil {
		_ = h.window.SetTextInputState(woxui.TextInputState{})
		return
	}
	_ = h.window.SetTextInputState(current.focus.textInput(globalRect(current)))
}

// Pointer dispatches hover, focus, tap, drag, and scroll by retained node identity.
func (h *Host) Pointer(event woxui.PointerEvent) {
	if h.root == nil {
		return
	}
	if h.dispatchRawPointer(event) {
		return
	}
	if event.Kind == woxui.PointerScroll {
		target := h.root.hitTestScroll(event.Position)
		for current := target; current != nil; current = current.parent {
			if current.gesture == nil {
				continue
			}
			if current.gesture.onScrollHandled != nil {
				if !current.gesture.onScrollHandled(event.Scroll) {
					continue
				}
				h.invalidate()
				return
			}
			if current.gesture.onScroll != nil {
				current.gesture.onScroll(event.Scroll)
				h.invalidate()
				return
			}
		}
		return
	}
	target := h.root.hitTest(event.Position)
	if event.Kind == woxui.PointerMove || event.Kind == woxui.PointerEnter || event.Kind == woxui.PointerLeave {
		if event.Kind == woxui.PointerLeave {
			target = nil
		}
		targetID := nodeID(target)
		if targetID != h.hovered {
			h.setHovered(target, event.Position)
		} else {
			h.updatePointerCursor(target, event.Position)
		}
	}
	if event.Kind == woxui.PointerDown && event.Button == woxui.PointerButtonPrimary {
		h.updatePointerFocus(target)
		h.pressed = nodeID(target)
		h.pressedAt = event.Position
		h.dragging = false
		h.selecting = 0
		h.selectingGestureID = ""
		if target != nil && target.gesture != nil && target.gesture.onPressChange != nil {
			target.gesture.onPressChange(true)
			h.invalidate()
		}
		// A selection gesture captures the press to begin a drag-based selection. Tap dispatch is
		// deferred to PointerUp; if the pointer moves meaningfully we keep the selection and skip tap.
		// Preserve the previous multi-click selection until the positioned double/triple handler replaces it.
		multiTap := target != nil && target.gesture != nil && h.continuesMultiTap(target.id, event.Position, time.Now()) &&
			((h.lastTapCount == 1 && target.gesture.onDoubleTapAt != nil) || (h.lastTapCount == 2 && target.gesture.onTripleTapAt != nil))
		if target != nil && target.gesture != nil && target.gesture.onSelectionStart != nil && !multiTap {
			h.selecting = h.pressed
			h.selectingGestureID = target.gesture.id
			target.gesture.onSelectionStart(target.localPoint(event.Position), event.Modifiers)
			h.invalidate()
		} else if target != nil && target.gesture != nil && target.gesture.onPanStart != nil {
			h.panning = h.pressed
			target.gesture.onPanStart(target.localPoint(event.Position))
			h.invalidate()
		}
	}
	if event.Kind == woxui.PointerDown && event.Button == woxui.PointerButtonSecondary {
		if target != nil && target.gesture != nil && target.gesture.onSecondaryTapDown != nil {
			target.gesture.onSecondaryTapDown(event.Position)
			h.invalidate()
		}
	}
	pressed := h.nodes[h.pressed]
	if event.Kind == woxui.PointerMove && pressed != nil && pressed.gesture != nil && pressed.gesture.onDragStart != nil && !h.dragging && h.selecting == 0 {
		deltaX := event.Position.X - h.pressedAt.X
		deltaY := event.Position.Y - h.pressedAt.Y
		if deltaX*deltaX+deltaY*deltaY >= 9 {
			if pressed.gesture.onPressChange != nil {
				pressed.gesture.onPressChange(false)
				h.invalidate()
			}
			h.pressed = 0
			h.dragging = true
			pressed.gesture.onDragStart()
		}
	}
	// Extend an active drag selection by mapping the current position into the selecting node's local coords.
	if event.Kind == woxui.PointerMove && h.selecting != 0 {
		if selector := h.nodes[h.selecting]; selector != nil && selector.gesture != nil && selector.gesture.onSelectionExtend != nil {
			deltaX := event.Position.X - h.pressedAt.X
			deltaY := event.Position.Y - h.pressedAt.Y
			// Any movement counts as a drag so a click without movement still collapses to a caret via tap.
			if !h.dragging && deltaX*deltaX+deltaY*deltaY >= 1 {
				h.dragging = true
			}
			selector.gesture.onSelectionExtend(selector.localPoint(event.Position))
			h.invalidate()
		}
	}
	if event.Kind == woxui.PointerMove && h.panning != 0 {
		if panner := h.nodes[h.panning]; panner != nil && panner.gesture != nil && panner.gesture.onPanUpdate != nil {
			h.dragging = true
			panner.gesture.onPanUpdate(panner.localPoint(event.Position))
			h.invalidate()
		}
	}
	if event.Kind == woxui.PointerUp && event.Button == woxui.PointerButtonPrimary {
		if pressed != nil && pressed.gesture != nil && pressed.gesture.onPressChange != nil {
			pressed.gesture.onPressChange(false)
			h.invalidate()
		}
		if h.panning != 0 {
			panner := h.nodes[h.panning]
			h.panning = 0
			h.dragging = false
			h.pressed = 0
			if panner != nil && panner.gesture != nil && panner.gesture.onPanEnd != nil {
				panner.gesture.onPanEnd()
			}
			h.invalidate()
			return
		}
		// Finalize a drag selection: if movement occurred keep the selection and skip tap dispatch;
		// otherwise fall through so a plain click still triggers tap (e.g. place caret).
		if h.selecting != 0 {
			selector := h.nodes[h.selecting]
			deltaX := event.Position.X - h.pressedAt.X
			deltaY := event.Position.Y - h.pressedAt.Y
			// Native backends may coalesce pointer moves while a complex form is rebuilding. Always
			// apply the release position so the final drag range is not mistaken for a plain click.
			if selector != nil && selector.gesture != nil && selector.gesture.onSelectionExtend != nil && deltaX*deltaX+deltaY*deltaY >= 1 {
				selector.gesture.onSelectionExtend(selector.localPoint(event.Position))
				h.dragging = true
			}
			selectingMoved := h.dragging
			if selector != nil && selector.gesture != nil && selector.gesture.onSelectionEnd != nil {
				selector.gesture.onSelectionEnd()
			}
			h.selecting = 0
			h.selectingGestureID = ""
			h.dragging = false
			if selectingMoved {
				h.pressed = 0
				h.invalidate()
				return
			}
		}
		if h.dragging {
			h.dragging = false
			h.pressed = 0
			return
		}
		if target != nil && target.id == h.pressed {
			h.activatePointerTarget(target, event.Position)
		}
		h.pressed = 0
	}
}

// updatePointerFocus keeps Host focus ownership aligned even when a raw native surface consumes the press.
func (h *Host) updatePointerFocus(target *node) {
	if h.focusVisible {
		h.focusVisible = false
		h.invalidate()
	}
	h.resetCaretBlink()
	focused := h.nodes[h.focused]
	for current := target; current != nil; current = current.parent {
		if h.isFocusable(current) {
			h.setFocus(current.id)
			return
		}
	}
	if focused != nil && focused.focus != nil && focused.focus.unfocusOnPointerOutside && (target == nil || !h.isDescendantOf(target, focused.id)) {
		h.setFocus(0)
	}
}

func (h *Host) dispatchRawPointer(event woxui.PointerEvent) bool {
	var target *node
	if h.rawPressed != 0 && (event.Kind == woxui.PointerMove || event.Kind == woxui.PointerUp) {
		target = h.nodes[h.rawPressed]
	} else if event.Kind != woxui.PointerLeave {
		target = h.root.hitTest(event.Position)
	}
	if event.Kind == woxui.PointerMove || event.Kind == woxui.PointerEnter || event.Kind == woxui.PointerLeave {
		targetID := nodeID(target)
		if h.rawHovered != 0 && h.rawHovered != targetID {
			if previous := h.nodes[h.rawHovered]; previous != nil && previous.gesture != nil && previous.gesture.onPointer != nil {
				leave := event
				leave.Kind = woxui.PointerLeave
				leave.Position = previous.localPoint(event.Position)
				previous.gesture.onPointer(leave)
			}
		}
		h.rawHovered = targetID
	}
	if target == nil || target.gesture == nil || target.gesture.onPointer == nil {
		return false
	}
	local := event
	local.Position = target.localPoint(event.Position)
	if !target.gesture.onPointer(local) {
		return false
	}
	if event.Kind == woxui.PointerDown {
		// Native surfaces take platform focus for every mouse-button press, so Host ownership must follow too.
		h.updatePointerFocus(target)
		h.rawPressed = target.id
	} else if event.Kind == woxui.PointerUp {
		h.rawPressed = 0
	}
	return true
}

func (h *Host) gestureNodeByID(id string) *node {
	if id == "" {
		return nil
	}
	for _, current := range h.nodes {
		if current != nil && current.gesture != nil && current.gesture.id == id {
			return current
		}
	}
	return nil
}

// nodeHasActiveCaret reports whether the current retained tree contains an active editor caret.
func nodeHasActiveCaret(current *node, focused woxui.AccessibilityNodeID, focusWithin, focusableWithin bool) bool {
	if current == nil {
		return false
	}
	if current.focus != nil {
		focusWithin = current.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || current.id == focused
	}
	if current.caretPaint != nil {
		caretActive := current.caret
		if focusableWithin {
			caretActive = focusWithin
		}
		if caretActive {
			return true
		}
	}
	for _, child := range current.children {
		if nodeHasActiveCaret(child, focused, focusWithin, focusableWithin) {
			return true
		}
	}
	return false
}

func (h *Host) caretVisibleForFrame() bool {
	h.caretBlinkMu.Lock()
	defer h.caretBlinkMu.Unlock()
	return h.windowFocused && h.caretVisible
}

// updateCaretBlink starts or stops the one-shot blink cycle based on the current widget tree.
func (h *Host) updateCaretBlink(active bool) {
	h.caretBlinkMu.Lock()
	defer h.caretBlinkMu.Unlock()
	active = active && h.windowFocused
	if h.caretBlinkActive != active {
		h.caretBlinkGeneration++
		if h.caretBlinkTimer != nil {
			h.caretBlinkTimer.Stop()
			h.caretBlinkTimer = nil
		}
		h.caretBlinkActive = active
		h.caretVisible = true
	}
	if active && h.caretBlinkTimer == nil {
		h.scheduleCaretBlinkLocked()
	}
}

// scheduleCaretBlinkLocked schedules one phase change; the resulting frame schedules the next one.
func (h *Host) scheduleCaretBlinkLocked() {
	generation := h.caretBlinkGeneration
	h.caretBlinkTimer = time.AfterFunc(caretBlinkInterval, func() {
		h.caretBlinkMu.Lock()
		if !h.caretBlinkActive || h.caretBlinkGeneration != generation {
			h.caretBlinkMu.Unlock()
			return
		}
		h.caretVisible = !h.caretVisible
		h.caretBlinkTimer = nil
		h.caretBlinkMu.Unlock()
		h.invalidateCaret()
	})
}

// resetCaretBlink makes the caret visible immediately after editing or caret movement.
func (h *Host) resetCaretBlink() {
	h.caretBlinkMu.Lock()
	if !h.caretBlinkActive {
		h.caretBlinkMu.Unlock()
		return
	}
	wasHidden := !h.caretVisible
	h.caretVisible = true
	h.caretBlinkGeneration++
	if h.caretBlinkTimer != nil {
		h.caretBlinkTimer.Stop()
		h.caretBlinkTimer = nil
	}
	h.scheduleCaretBlinkLocked()
	h.caretBlinkMu.Unlock()
	if wasHidden {
		h.invalidateCaret()
	}
}

func (h *Host) setHovered(target *node, position woxui.Point) {
	old := h.nodes[h.hovered]
	damage := woxui.Rect{}
	if old != nil && old.gesture != nil {
		damage = unionDamageRects(damage, globalRect(old))
		if old.gesture.onHover != nil {
			old.gesture.onHover(false)
		}
		if old.gesture.onHoverAt != nil {
			old.gesture.onHoverAt(false, globalRect(old))
		}
	}
	h.hovered = nodeID(target)
	h.updatePointerCursor(target, position)
	if target != nil && target.gesture != nil {
		damage = unionDamageRects(damage, globalRect(target))
		if target.gesture.onHover != nil {
			target.gesture.onHover(true)
		}
		if target.gesture.onHoverAt != nil {
			target.gesture.onHoverAt(true, globalRect(target))
		}
	}
	// Hover callbacks may change both visuals, so redraw only their combined bounds.
	if damage.Width > 0 && damage.Height > 0 {
		h.invalidateRect(damage)
	}
}

// updatePointerCursor resolves position-sensitive cursors before static ancestor fallbacks.
func (h *Host) updatePointerCursor(target *node, position woxui.Point) {
	cursor := woxui.PointerCursorDefault
	for current := target; current != nil; current = current.parent {
		if current.gesture != nil && current.gesture.cursorAt != nil {
			cursor = current.gesture.cursorAt(current.localPoint(position))
			break
		}
		if current.gesture != nil && current.gesture.cursor != woxui.PointerCursorDefault {
			cursor = current.gesture.cursor
			break
		}
	}
	if h.window != nil {
		_ = h.window.SetPointerCursor(cursor)
	}
}

func (h *Host) activatePointerTarget(target *node, position woxui.Point) {
	if target == nil || target.gesture == nil {
		return
	}
	now := time.Now()
	localPosition := target.localPoint(position)
	hasDoubleTap := target.gesture.onDoubleTap != nil || target.gesture.onDoubleTapAt != nil
	hasTripleTap := target.gesture.onTripleTapAt != nil
	if h.continuesMultiTap(target.id, position, now) {
		h.lastTapCount++
	} else {
		h.lastTapCount = 1
	}
	h.lastTapID = target.id
	h.lastTapAt = now
	h.lastTapPosition = position

	if h.lastTapCount == 3 && hasTripleTap {
		target.gesture.onTripleTapAt(localPosition)
		h.lastTapID = 0
		h.lastTapAt = time.Time{}
		h.lastTapPosition = woxui.Point{}
		h.lastTapCount = 0
	} else if h.lastTapCount == 2 && hasDoubleTap {
		if target.gesture.onDoubleTap != nil {
			target.gesture.onDoubleTap()
		}
		if target.gesture.onDoubleTapAt != nil {
			target.gesture.onDoubleTapAt(localPosition)
		}
		if !hasTripleTap {
			h.lastTapID = 0
			h.lastTapAt = time.Time{}
			h.lastTapPosition = woxui.Point{}
			h.lastTapCount = 0
		}
	} else {
		if target.gesture.onTap != nil {
			target.gesture.onTap()
		}
		if target.gesture.onTapAt != nil {
			target.gesture.onTapAt(localPosition)
		}
		if target.gesture.onTapBounds != nil {
			target.gesture.onTapBounds(globalRect(target))
		}
	}
	h.invalidate()
}

// continuesMultiTap applies the shared time and movement thresholds for consecutive clicks.
func (h *Host) continuesMultiTap(target woxui.AccessibilityNodeID, position woxui.Point, now time.Time) bool {
	if target != h.lastTapID || h.lastTapAt.IsZero() || now.Sub(h.lastTapAt) > multiTapInterval {
		return false
	}
	deltaX := position.X - h.lastTapPosition.X
	deltaY := position.Y - h.lastTapPosition.Y
	return deltaX*deltaX+deltaY*deltaY <= multiTapDistance*multiTapDistance
}

func nodeID(current *node) woxui.AccessibilityNodeID {
	if current == nil {
		return 0
	}
	return current.id
}

func (h *Host) buildAccessibilityTree(diagnostics []string, work *frameWorkCounters) (woxui.AccessibilityTree, []string) {
	nodes := []woxui.AccessibilityNode{}
	indexByID := map[woxui.AccessibilityNodeID]int{}
	automationIDs := map[string]woxui.AccessibilityNodeID{}
	appendNode := func(nativeNode woxui.AccessibilityNode) {
		if nativeNode.AutomationID != "" {
			if previous, exists := automationIDs[nativeNode.AutomationID]; exists {
				diagnostics = append(diagnostics, fmt.Sprintf("duplicate automation id %q on nodes %d and %d", nativeNode.AutomationID, previous, nativeNode.ID))
			} else {
				automationIDs[nativeNode.AutomationID] = nativeNode.ID
			}
		}
		if (nativeNode.Focusable || len(nativeNode.Actions) > 0) && (nativeNode.Role == "" || strings.TrimSpace(nativeNode.Label) == "" || nativeNode.AutomationID == "") {
			diagnostics = append(diagnostics, fmt.Sprintf("interactive node %d requires role, label, and automation id", nativeNode.ID))
		}
		indexByID[nativeNode.ID] = len(nodes)
		nodes = append(nodes, nativeNode)
		if nativeNode.ParentID != 0 {
			if parentIndex, found := indexByID[nativeNode.ParentID]; found {
				nodes[parentIndex].Children = append(nodes[parentIndex].Children, nativeNode.ID)
			}
		}
	}
	var visit func(current *node, origin woxui.Point, semanticParent woxui.AccessibilityNodeID)
	visit = func(current *node, origin woxui.Point, semanticParent woxui.AccessibilityNodeID) {
		if current == nil {
			return
		}
		bounds := offsetRect(current.bounds, origin)
		childOrigin := woxui.Point{X: bounds.X, Y: bounds.Y}
		cache := current.boundary
		if cache != nil && cache.hit && cache.a11yValid && cache.a11yRootID == current.id {
			cache.a11yReuses++
			rootIDs := make(map[woxui.AccessibilityNodeID]struct{}, len(cache.a11yRootIDs))
			for _, id := range cache.a11yRootIDs {
				rootIDs[id] = struct{}{}
			}
			deltaX := bounds.X - cache.a11yOrigin.X
			deltaY := bounds.Y - cache.a11yOrigin.Y
			for _, cachedNode := range cache.a11yNodes {
				nativeNode := cachedNode
				nativeNode.Children = nil
				nativeNode.Actions = append([]woxui.AccessibilityAction(nil), cachedNode.Actions...)
				if _, root := rootIDs[nativeNode.ID]; root {
					nativeNode.ParentID = semanticParent
				}
				nativeNode.Bounds.X += deltaX
				nativeNode.Bounds.Y += deltaY
				currentNode := h.nodes[nativeNode.ID]
				nativeNode.Focusable = currentNode != nil && h.isFocusable(currentNode)
				nativeNode.Focused = nativeNode.ID == h.focused
				nativeNode.Actions = accessibilityActionsForFocusability(nativeNode.Actions, nativeNode.Focusable)
				appendNode(nativeNode)
			}
			return
		}
		if work != nil {
			work.a11yVisits++
		}
		cacheStart := len(nodes)
		nextParent := semanticParent
		if current.semantic != nil && !current.semantic.hidden {
			semantic := current.semantic
			value := semantic.value
			if semantic.protected {
				value = ""
			}
			actions := append([]woxui.AccessibilityAction(nil), semantic.actions...)
			if h.isFocusable(current) && !containsAction(actions, woxui.AccessibilityActionFocus) {
				actions = append(actions, woxui.AccessibilityActionFocus)
			}
			nativeNode := woxui.AccessibilityNode{
				ID:               current.id,
				ParentID:         semanticParent,
				AutomationID:     semantic.automationID,
				Role:             semantic.role,
				Label:            semantic.label,
				Description:      semantic.description,
				Value:            value,
				Bounds:           bounds,
				Actions:          actions,
				LiveRegion:       semantic.liveRegion,
				Enabled:          semantic.enabled,
				Focusable:        h.isFocusable(current),
				Focused:          current.id == h.focused,
				Selected:         semantic.selected,
				Checked:          semantic.checked,
				Expanded:         semantic.expanded,
				ReadOnly:         semantic.readOnly,
				Protected:        semantic.protected,
				NativeBoundary:   semantic.nativeBoundary,
				HasTextSelection: semantic.hasTextSelection,
				SelectionStart:   semantic.selectionStart,
				SelectionEnd:     semantic.selectionEnd,
			}
			appendNode(nativeNode)
			nextParent = nativeNode.ID
		}
		for _, child := range current.children {
			visit(child, childOrigin, nextParent)
		}
		if cache != nil {
			cache.a11yOrigin = woxui.Point{X: bounds.X, Y: bounds.Y}
			cache.a11yRootID = current.id
			cache.a11yNodes = cloneAccessibilityNodes(nodes[cacheStart:])
			cache.a11yRootIDs = accessibilitySegmentRootIDs(cache.a11yNodes)
			cache.a11yValid = true
			if work != nil {
				// Replay hits stay at 0. This counts rebuilt cache writes, not unused tree diffs.
				work.a11yUpserts += len(cache.a11yNodes)
			}
		}
	}
	visit(h.root, woxui.Point{}, 0)
	roots := []woxui.AccessibilityNodeID{}
	for _, current := range nodes {
		if current.ParentID == 0 {
			roots = append(roots, current.ID)
		}
	}
	for _, diagnostic := range diagnostics {
		h.reportDiagnostic(diagnostic)
	}
	return woxui.AccessibilityTree{Generation: h.generation, RootIDs: roots, Nodes: nodes}, diagnostics
}

func accessibilityActionsForFocusability(actions []woxui.AccessibilityAction, focusable bool) []woxui.AccessibilityAction {
	result := actions[:0]
	for _, action := range actions {
		if action != woxui.AccessibilityActionFocus {
			result = append(result, action)
		}
	}
	if focusable {
		result = append(result, woxui.AccessibilityActionFocus)
	}
	return result
}

func cloneAccessibilityNodes(nodes []woxui.AccessibilityNode) []woxui.AccessibilityNode {
	clone := append([]woxui.AccessibilityNode(nil), nodes...)
	for index := range clone {
		clone[index].Children = append([]woxui.AccessibilityNodeID(nil), nodes[index].Children...)
		clone[index].Actions = append([]woxui.AccessibilityAction(nil), nodes[index].Actions...)
	}
	return clone
}

func accessibilitySegmentRootIDs(nodes []woxui.AccessibilityNode) []woxui.AccessibilityNodeID {
	ids := make(map[woxui.AccessibilityNodeID]struct{}, len(nodes))
	for _, node := range nodes {
		ids[node.ID] = struct{}{}
	}
	roots := make([]woxui.AccessibilityNodeID, 0, 1)
	for _, node := range nodes {
		if _, internal := ids[node.ParentID]; !internal {
			roots = append(roots, node.ID)
		}
	}
	return roots
}

func containsAction(actions []woxui.AccessibilityAction, expected woxui.AccessibilityAction) bool {
	for _, action := range actions {
		if action == expected {
			return true
		}
	}
	return false
}

func (h *Host) dispatchAccessibilityAction(nodeID woxui.AccessibilityNodeID, action woxui.AccessibilityAction, value string) error {
	var actionErr error
	if err := woxui.Call(func() {
		actionErr = h.performAccessibilityAction(nodeID, action, value)
	}); err != nil {
		return err
	}
	return actionErr
}

func (h *Host) performAccessibilityAction(nodeID woxui.AccessibilityNodeID, action woxui.AccessibilityAction, value string) error {
	current := h.nodes[nodeID]
	if current == nil || current.semantic == nil || current.semantic.hidden {
		return fmt.Errorf("accessibility node %d is unavailable", nodeID)
	}
	if action == woxui.AccessibilityActionFocus {
		if !h.isFocusable(current) {
			return fmt.Errorf("accessibility node %d is not focusable", nodeID)
		}
		h.setFocus(nodeID)
		return nil
	}
	if current.semantic.onAction != nil {
		if err := current.semantic.onAction(action, value); err != nil {
			return err
		}
		h.invalidate()
		return nil
	}
	if action == woxui.AccessibilityActionActivate && current.gesture != nil && (current.gesture.onTap != nil || current.gesture.onTapBounds != nil) {
		if current.gesture.onTap != nil {
			current.gesture.onTap()
		}
		if current.gesture.onTapBounds != nil {
			current.gesture.onTapBounds(globalRect(current))
		}
		h.invalidate()
		return nil
	}
	return fmt.Errorf("accessibility action %q is not supported by node %d", action, nodeID)
}

func (h *Host) publishSnapshot(tree woxui.AccessibilityTree, diagnostics []string) {
	snapshot := AutomationSnapshot{Tree: cloneTree(tree), Diagnostics: append([]string(nil), diagnostics...)}
	h.snapshot.Store(snapshot)
	h.changeMu.Lock()
	close(h.change)
	h.change = make(chan struct{})
	h.changeMu.Unlock()
}

// Snapshot returns a detached semantics snapshot for assertions and automation.
func (h *Host) Snapshot() AutomationSnapshot {
	value := h.snapshot.Load().(AutomationSnapshot)
	return AutomationSnapshot{Tree: cloneTree(value.Tree), Diagnostics: append([]string(nil), value.Diagnostics...)}
}

// WaitForChange blocks until a newer frame is published or the context ends.
func (h *Host) WaitForChange(ctx stdcontext.Context, afterGeneration uint64) (AutomationSnapshot, error) {
	for {
		current := h.Snapshot()
		if current.Tree.Generation > afterGeneration {
			return current, nil
		}
		h.changeMu.Lock()
		current = h.Snapshot()
		if current.Tree.Generation > afterGeneration {
			h.changeMu.Unlock()
			return current, nil
		}
		change := h.change
		h.changeMu.Unlock()
		select {
		case <-ctx.Done():
			return AutomationSnapshot{}, ctx.Err()
		case <-change:
		}
	}
}

func cloneTree(tree woxui.AccessibilityTree) woxui.AccessibilityTree {
	clone := tree
	clone.RootIDs = append([]woxui.AccessibilityNodeID(nil), tree.RootIDs...)
	clone.Nodes = append([]woxui.AccessibilityNode(nil), tree.Nodes...)
	for index := range clone.Nodes {
		clone.Nodes[index].Children = append([]woxui.AccessibilityNodeID(nil), tree.Nodes[index].Children...)
		clone.Nodes[index].Actions = append([]woxui.AccessibilityAction(nil), tree.Nodes[index].Actions...)
	}
	return clone
}

func (h *Host) reportDiagnostic(message string) {
	if message == "" || h.reported[message] {
		return
	}
	h.reported[message] = true
	log.Printf("widget diagnostic: %s", message)
}

func (h *Host) invalidate() {
	h.damageMu.Lock()
	h.fullDamage = true
	h.pendingDamage = woxui.Rect{}
	h.damageMu.Unlock()
	if h.window != nil {
		_ = h.window.Invalidate()
	}
}

// invalidateRect accumulates a logical redraw region until the native surface requests a frame.
func (h *Host) invalidateRect(rect woxui.Rect) {
	if rect.Width <= 0 || rect.Height <= 0 {
		h.invalidate()
		return
	}
	h.damageMu.Lock()
	if !h.fullDamage {
		h.pendingDamage = unionDamageRects(h.pendingDamage, rect)
	}
	h.damageMu.Unlock()
	if h.window != nil {
		_ = h.window.InvalidateRect(rect)
	}
}

// consumeFrameDamage combines platform damage with retained invalidations and resets the pending region.
func (h *Host) consumeFrameDamage(native woxui.Rect, size woxui.Size) woxui.Rect {
	h.damageMu.Lock()
	pending := h.pendingDamage
	full := h.fullDamage
	h.pendingDamage = woxui.Rect{}
	h.fullDamage = false
	h.damageMu.Unlock()
	// A zero native region is the portable contract for a complete frame. This also keeps
	// platforms without persistent back buffers correct while their damage path is disabled.
	if native.Width <= 0 || native.Height <= 0 || full {
		return woxui.Rect{}
	}
	return clipDamageRect(unionDamageRects(native, pending), size)
}

func (h *Host) setCaretDamage(rect woxui.Rect) {
	h.damageMu.Lock()
	h.caretDamage = rect
	h.damageMu.Unlock()
}

func (h *Host) invalidateCaret() {
	h.damageMu.Lock()
	damage := h.caretDamage
	h.damageMu.Unlock()
	if damage.Width > 0 && damage.Height > 0 {
		h.invalidateRect(damage)
		return
	}
	h.invalidate()
}
