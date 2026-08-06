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
	RecordFrameCounts(frameID uint64, nodes, commands, accessibilityNodes int)
}

type displayListDamageHostServices interface {
	DisplayListDamageCullingEnabled() bool
}

// Host reconciles, lays out, paints, and routes input for one retained widget tree.
type Host struct {
	window HostServices
	build  func(frame woxui.FrameInfo) Widget
	root   *node

	nextNodeID woxui.AccessibilityNodeID
	identities map[string]woxui.AccessibilityNodeID
	nodes      map[woxui.AccessibilityNodeID]*node

	hovered   woxui.AccessibilityNodeID
	pressed   woxui.AccessibilityNodeID
	pressedAt woxui.Point
	dragging  bool
	// selecting tracks the gesture node that started a drag-based selection, so subsequent
	// pointer-move events extend its selection until the pointer is released.
	selecting       woxui.AccessibilityNodeID
	panning         woxui.AccessibilityNodeID
	lastTapID       woxui.AccessibilityNodeID
	lastTapAt       time.Time
	lastTapPosition woxui.Point
	lastTapCount    int

	focused woxui.AccessibilityNodeID
	// focusVisible mirrors :focus-visible so pointer focus keeps keyboard behavior without painting a ring.
	focusVisible bool
	modalScopes  []woxui.AccessibilityNodeID
	scopeRestore map[woxui.AccessibilityNodeID]woxui.AccessibilityNodeID

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
	widget := h.build(frame)
	if widget == nil {
		h.elements.endFrame()
		h.recordFramePhase(frameID, woxui.FrameMetricBuildLayout, time.Since(buildLayoutStart))
		h.recordFrameCounts(frameID, 0, displayList.CommandCount(), 0)
		h.updateCaretBlink(false)
		h.animations.reset()
		return
	}

	oldNodes := h.nodes
	animation := h.animations.beginFrame()
	var debugFrame *repaintDebugFrame
	if h.repaintDebugMode != RepaintDebugOff {
		debugFrame = &repaintDebugFrame{
			mode: h.repaintDebugMode, now: time.Now(), repaintCount: h.elements.generation,
		}
	}
	caretVisible := h.caretVisibleForFrame()
	damageTracker := &frameDamageTracker{}
	root := widget.layout(context{window: h.window, animation: animation, damage: damageTracker, debug: debugFrame, elements: h.elements, element: h.elements.root}, constraints{width: frame.Size.Width, height: frame.Size.Height})
	diagnostics, removedDamage := h.elements.endFrame()
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
	h.animations.endFrame(animation, func() {
		if boundaryDamage.Width > 0 && boundaryDamage.Height > 0 {
			h.invalidateRect(boundaryDamage)
			return
		}
		h.invalidate()
	})
	identities := map[string]woxui.AccessibilityNodeID{}
	nodes := map[woxui.AccessibilityNodeID]*node{}
	h.assignIdentities(root, nil, "root", 0, h.identities, identities, nodes, &diagnostics, nil)
	h.root = root
	h.identities = identities
	h.nodes = nodes
	h.reconcileTransientState(oldNodes)
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
	h.root.draw(displayList, h.focused, focusRingTarget, caretVisible, false, false)
	debugFrame.draw(displayList)
	h.recordFramePhase(frameID, woxui.FrameMetricDrawRecord, time.Since(drawStart))

	accessibilityStart := time.Now()
	h.generation++
	tree, diagnostics := h.buildAccessibilityTree(diagnostics)
	h.publishSnapshot(tree, diagnostics)
	if err := h.window.UpdateAccessibility(tree, h.dispatchAccessibilityAction); err != nil {
		h.reportDiagnostic(fmt.Sprintf("publish accessibility tree: %v", err))
	}
	h.recordFramePhase(frameID, woxui.FrameMetricAccessibility, time.Since(accessibilityStart))
	h.recordFrameCounts(frameID, len(nodes), displayList.CommandCount(), len(tree.Nodes))
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

func (h *Host) recordFrameCounts(frameID uint64, nodes, commands, accessibilityNodes int) {
	if frameID == 0 {
		return
	}
	if services, ok := h.window.(frameMetricsHostServices); ok {
		services.RecordFrameCounts(frameID, nodes, commands, accessibilityNodes)
	}
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
	h.nodes = map[woxui.AccessibilityNodeID]*node{}
	h.identities = map[string]woxui.AccessibilityNodeID{}
}

func (h *Host) assignIdentities(current *node, parent *node, parentPath string, index int, previous, identities map[string]woxui.AccessibilityNodeID, nodes map[woxui.AccessibilityNodeID]*node, diagnostics *[]string, collectors []*boundaryCache) {
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
		*diagnostics = append(*diagnostics, cache.identityDiagnostics...)
		for entryIndex, entry := range cache.identityEntries {
			entryParent := entry.parent
			if entryIndex == 0 {
				entryParent = parent
			}
			entry.node.parent = entryParent
			entry.node.id = entry.id
			identities[entry.path] = entry.id
			nodes[entry.id] = entry.node
			for _, collector := range collectors {
				collector.identityEntries = append(collector.identityEntries, boundaryIdentityEntry{path: entry.path, node: entry.node, parent: entryParent, id: entry.id})
			}
		}
		return
	}
	diagnosticStart := len(*diagnostics)
	if cache != nil {
		cache.identityEntries = cache.identityEntries[:0]
		collectors = append(collectors, cache)
	}
	if id, ok := previous[path]; ok {
		current.id = id
	} else {
		h.nextNodeID++
		current.id = h.nextNodeID
	}
	identities[path] = current.id
	nodes[current.id] = current
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
		h.assignIdentities(child, current, childPath, childIndex, previous, identities, nodes, diagnostics, collectors)
	}
	if cache != nil {
		cache.identityRootPath = path
		cache.identityDiagnostics = append(cache.identityDiagnostics[:0], (*diagnostics)[diagnosticStart:]...)
		cache.identityValid = true
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

func (h *Host) reconcileTransientState(oldNodes map[woxui.AccessibilityNodeID]*node) {
	if h.hovered != 0 && h.nodes[h.hovered] == nil {
		if old := oldNodes[h.hovered]; old != nil && old.gesture != nil {
			if old.gesture.onHover != nil {
				old.gesture.onHover(false)
			}
			if old.gesture.onHoverAt != nil {
				old.gesture.onHoverAt(false, old.bounds)
			}
		}
		h.hovered = 0
		if h.window != nil {
			_ = h.window.SetPointerCursor(woxui.PointerCursorDefault)
		}
	}
	if h.pressed != 0 && h.nodes[h.pressed] == nil {
		h.pressed = 0
		h.dragging = false
	}
	// A widget rebuild can drop the node that started a drag selection; clear the state so a
	// later PointerUp does not dispatch a tap onto a stale selector.
	if h.selecting != 0 && h.nodes[h.selecting] == nil {
		h.selecting = 0
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
		if h.isFocusable(current) {
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
		start := current.bounds.Y - ancestor.bounds.Y + ancestor.scroll.offset
		end := start + current.bounds.Height
		if ancestor.scroll.horizontal {
			start = current.bounds.X - ancestor.bounds.X + ancestor.scroll.offset
			end = start + current.bounds.Width
		}
		if ancestor.scroll.ensureVisible(start, end) {
			return
		}
	}
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

// BoundsForKey returns the latest laid-out bounds for a retained widget key.
func (h *Host) BoundsForKey(key Key) (woxui.Rect, bool) {
	for _, current := range h.nodes {
		if current.key == key {
			return current.bounds, true
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
	_ = h.window.SetTextInputState(current.focus.textInput(current.bounds))
}

// Pointer dispatches hover, focus, tap, drag, and scroll by retained node identity.
func (h *Host) Pointer(event woxui.PointerEvent) {
	if h.root == nil {
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
	if event.Kind == woxui.PointerMove || event.Kind == woxui.PointerLeave {
		if event.Kind == woxui.PointerLeave {
			target = nil
		}
		targetID := nodeID(target)
		if targetID != h.hovered {
			h.setHovered(target)
		}
	}
	if event.Kind == woxui.PointerDown && event.Button == woxui.PointerButtonPrimary {
		if h.focusVisible {
			h.focusVisible = false
			h.invalidate()
		}
		h.resetCaretBlink()
		h.pressed = nodeID(target)
		h.pressedAt = event.Position
		h.dragging = false
		if target != nil && target.gesture != nil && target.gesture.onPressChange != nil {
			target.gesture.onPressChange(true)
			h.invalidate()
		}
		focused := h.nodes[h.focused]
		var pointerFocusTarget *node
		for focusTarget := target; focusTarget != nil; focusTarget = focusTarget.parent {
			if h.isFocusable(focusTarget) {
				pointerFocusTarget = focusTarget
				break
			}
		}
		if pointerFocusTarget != nil {
			h.setFocus(pointerFocusTarget.id)
		} else if focused != nil && focused.focus != nil && focused.focus.unfocusOnPointerOutside && (target == nil || !h.isDescendantOf(target, focused.id)) {
			h.setFocus(0)
		}
		// A selection gesture captures the press to begin a drag-based selection. Tap dispatch is
		// deferred to PointerUp; if the pointer moves meaningfully we keep the selection and skip tap.
		// Preserve the previous multi-click selection until the positioned double/triple handler replaces it.
		multiTap := target != nil && target.gesture != nil && h.continuesMultiTap(target.id, event.Position, time.Now()) &&
			((h.lastTapCount == 1 && target.gesture.onDoubleTapAt != nil) || (h.lastTapCount == 2 && target.gesture.onTripleTapAt != nil))
		if target != nil && target.gesture != nil && target.gesture.onSelectionStart != nil && !multiTap {
			h.selecting = h.pressed
			target.gesture.onSelectionStart(woxui.Point{X: event.Position.X - target.bounds.X, Y: event.Position.Y - target.bounds.Y})
			h.invalidate()
		} else if target != nil && target.gesture != nil && target.gesture.onPanStart != nil {
			h.panning = h.pressed
			target.gesture.onPanStart(woxui.Point{X: event.Position.X - target.bounds.X, Y: event.Position.Y - target.bounds.Y})
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
			selector.gesture.onSelectionExtend(woxui.Point{X: event.Position.X - selector.bounds.X, Y: event.Position.Y - selector.bounds.Y})
			h.invalidate()
		}
	}
	if event.Kind == woxui.PointerMove && h.panning != 0 {
		if panner := h.nodes[h.panning]; panner != nil && panner.gesture != nil && panner.gesture.onPanUpdate != nil {
			h.dragging = true
			panner.gesture.onPanUpdate(woxui.Point{X: event.Position.X - panner.bounds.X, Y: event.Position.Y - panner.bounds.Y})
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
			selectingMoved := h.dragging
			h.selecting = 0
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

func (h *Host) setHovered(target *node) {
	old := h.nodes[h.hovered]
	damage := woxui.Rect{}
	if old != nil && old.gesture != nil {
		damage = unionDamageRects(damage, old.bounds)
		if old.gesture.onHover != nil {
			old.gesture.onHover(false)
		}
		if old.gesture.onHoverAt != nil {
			old.gesture.onHoverAt(false, old.bounds)
		}
	}
	h.hovered = nodeID(target)
	cursor := woxui.PointerCursorDefault
	for current := target; current != nil; current = current.parent {
		if current.gesture != nil && current.gesture.cursor != woxui.PointerCursorDefault {
			cursor = current.gesture.cursor
			break
		}
	}
	if h.window != nil {
		_ = h.window.SetPointerCursor(cursor)
	}
	if target != nil && target.gesture != nil {
		damage = unionDamageRects(damage, target.bounds)
		if target.gesture.onHover != nil {
			target.gesture.onHover(true)
		}
		if target.gesture.onHoverAt != nil {
			target.gesture.onHoverAt(true, target.bounds)
		}
	}
	// Hover callbacks may change both visuals, so redraw only their combined bounds.
	if damage.Width > 0 && damage.Height > 0 {
		h.invalidateRect(damage)
	}
}

func (h *Host) activatePointerTarget(target *node, position woxui.Point) {
	if target == nil || target.gesture == nil {
		return
	}
	now := time.Now()
	localPosition := woxui.Point{X: position.X - target.bounds.X, Y: position.Y - target.bounds.Y}
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
			target.gesture.onTapBounds(target.bounds)
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

func (h *Host) buildAccessibilityTree(diagnostics []string) (woxui.AccessibilityTree, []string) {
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
	var visit func(current *node, semanticParent woxui.AccessibilityNodeID)
	visit = func(current *node, semanticParent woxui.AccessibilityNodeID) {
		if current == nil {
			return
		}
		cache := current.boundary
		if cache != nil && cache.hit && cache.a11yValid && cache.a11yRootID == current.id {
			cache.a11yReuses++
			rootIDs := make(map[woxui.AccessibilityNodeID]struct{}, len(cache.a11yRootIDs))
			for _, id := range cache.a11yRootIDs {
				rootIDs[id] = struct{}{}
			}
			deltaX := current.bounds.X - cache.a11yOrigin.X
			deltaY := current.bounds.Y - cache.a11yOrigin.Y
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
				ID:             current.id,
				ParentID:       semanticParent,
				AutomationID:   semantic.automationID,
				Role:           semantic.role,
				Label:          semantic.label,
				Description:    semantic.description,
				Value:          value,
				Bounds:         current.bounds,
				Actions:        actions,
				LiveRegion:     semantic.liveRegion,
				Enabled:        semantic.enabled,
				Focusable:      h.isFocusable(current),
				Focused:        current.id == h.focused,
				Selected:       semantic.selected,
				Checked:        semantic.checked,
				Expanded:       semantic.expanded,
				ReadOnly:       semantic.readOnly,
				Protected:      semantic.protected,
				NativeBoundary: semantic.nativeBoundary,
			}
			appendNode(nativeNode)
			nextParent = nativeNode.ID
		}
		for _, child := range current.children {
			visit(child, nextParent)
		}
		if cache != nil {
			cache.a11yOrigin = woxui.Point{X: current.bounds.X, Y: current.bounds.Y}
			cache.a11yRootID = current.id
			cache.a11yNodes = cloneAccessibilityNodes(nodes[cacheStart:])
			cache.a11yRootIDs = accessibilitySegmentRootIDs(cache.a11yNodes)
			cache.a11yValid = true
		}
	}
	visit(h.root, 0)
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
			current.gesture.onTapBounds(current.bounds)
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
