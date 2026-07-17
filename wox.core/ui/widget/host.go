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

const caretBlinkInterval = 500 * time.Millisecond

// AutomationSnapshot is the immutable retained tree exposed to test drivers.
type AutomationSnapshot struct {
	Tree        woxui.AccessibilityTree
	Diagnostics []string
}

// HostServices is the minimal native surface required by the retained widget host.
type HostServices interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
	Invalidate() error
	SetTextInputState(state woxui.TextInputState) error
	UpdateAccessibility(tree woxui.AccessibilityTree, handler woxui.AccessibilityActionHandler) error
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
	lastTapID woxui.AccessibilityNodeID
	lastTapAt time.Time

	focused      woxui.AccessibilityNodeID
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
	animations           animationHost
	elements             *elementTree
	postFrame            []func()
	disposed             bool
}

// NewHost creates a retained host whose builder runs once per invalidated frame.
func NewHost(build func(frame woxui.FrameInfo) Widget) *Host {
	host := &Host{
		build:        build,
		identities:   map[string]woxui.AccessibilityNodeID{},
		nodes:        map[woxui.AccessibilityNodeID]*node{},
		scopeRestore: map[woxui.AccessibilityNodeID]woxui.AccessibilityNodeID{},
		change:       make(chan struct{}),
		reported:     map[string]bool{},
		caretVisible: true,
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

// Frame reconciles one widget description, publishes semantics, and paints it.
func (h *Host) Frame(displayList *woxui.DisplayList, frame woxui.FrameInfo) {
	if h.disposed || h.window == nil || h.build == nil {
		h.updateCaretBlink(false)
		h.animations.reset()
		return
	}
	h.elements.beginFrame()
	widget := h.build(frame)
	if widget == nil {
		h.elements.endFrame()
		h.updateCaretBlink(false)
		h.animations.reset()
		return
	}

	oldNodes := h.nodes
	animation := h.animations.beginFrame(h.window)
	root := widget.layout(context{window: h.window, caretVisible: h.caretVisibleForFrame(), animation: animation, elements: h.elements, element: h.elements.root}, constraints{width: frame.Size.Width, height: frame.Size.Height})
	h.animations.endFrame(animation)
	h.updateCaretBlink(nodeHasActiveCaret(root))
	identities := map[string]woxui.AccessibilityNodeID{}
	nodes := map[woxui.AccessibilityNodeID]*node{}
	diagnostics := h.elements.endFrame()
	h.assignIdentities(root, nil, "root", 0, h.identities, identities, nodes, &diagnostics)
	h.root = root
	h.identities = identities
	h.nodes = nodes
	h.reconcileTransientState(oldNodes)
	h.reconcileFocus()

	displayList.Clear(woxui.Color{})
	h.root.draw(displayList)
	h.generation++
	tree, diagnostics := h.buildAccessibilityTree(diagnostics)
	h.publishSnapshot(tree, diagnostics)
	if err := h.window.UpdateAccessibility(tree, h.dispatchAccessibilityAction); err != nil {
		h.reportDiagnostic(fmt.Sprintf("publish accessibility tree: %v", err))
	}
	h.syncTextInput()
	h.runPostFrameCallbacks()
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

func (h *Host) assignIdentities(current *node, parent *node, parentPath string, index int, previous, identities map[string]woxui.AccessibilityNodeID, nodes map[woxui.AccessibilityNodeID]*node, diagnostics *[]string) {
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
	if id, ok := previous[path]; ok {
		current.id = id
	} else {
		h.nextNodeID++
		current.id = h.nextNodeID
	}
	identities[path] = current.id
	nodes[current.id] = current

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
		h.assignIdentities(child, current, childPath, childIndex, previous, identities, nodes, diagnostics)
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
	}
	if h.pressed != 0 && h.nodes[h.pressed] == nil {
		h.pressed = 0
		h.dragging = false
	}
	if h.lastTapID != 0 && h.nodes[h.lastTapID] == nil {
		h.lastTapID = 0
		h.lastTapAt = time.Time{}
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
	h.setFocus(order[index].id)
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
	h.resetCaretBlink()
	h.syncTextInput()
	h.invalidate()
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
	tabTraversal := event.Down && event.Key == woxui.KeyTab && !event.Composing
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
		if target != nil {
			target.gesture.onScroll(event.Scroll)
			h.invalidate()
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
			h.setHovered(target)
		}
	}
	if event.Kind == woxui.PointerDown && event.Button == woxui.PointerButtonPrimary {
		h.resetCaretBlink()
		h.pressed = nodeID(target)
		h.pressedAt = event.Position
		h.dragging = false
		for focusTarget := target; focusTarget != nil; focusTarget = focusTarget.parent {
			if h.isFocusable(focusTarget) {
				h.setFocus(focusTarget.id)
				break
			}
		}
	}
	pressed := h.nodes[h.pressed]
	if event.Kind == woxui.PointerMove && pressed != nil && pressed.gesture != nil && pressed.gesture.onDragStart != nil && !h.dragging {
		deltaX := event.Position.X - h.pressedAt.X
		deltaY := event.Position.Y - h.pressedAt.Y
		if deltaX*deltaX+deltaY*deltaY >= 9 {
			h.pressed = 0
			h.dragging = true
			pressed.gesture.onDragStart()
		}
	}
	if event.Kind == woxui.PointerUp && event.Button == woxui.PointerButtonPrimary {
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
func nodeHasActiveCaret(current *node) bool {
	if current == nil {
		return false
	}
	if current.caret {
		return true
	}
	for _, child := range current.children {
		if nodeHasActiveCaret(child) {
			return true
		}
	}
	return false
}

func (h *Host) caretVisibleForFrame() bool {
	h.caretBlinkMu.Lock()
	defer h.caretBlinkMu.Unlock()
	return h.caretVisible
}

// updateCaretBlink starts or stops the one-shot blink cycle based on the current widget tree.
func (h *Host) updateCaretBlink(active bool) {
	h.caretBlinkMu.Lock()
	defer h.caretBlinkMu.Unlock()
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
		window := h.window
		h.caretBlinkMu.Unlock()
		if window != nil {
			_ = window.Invalidate()
		}
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
	window := h.window
	h.caretBlinkMu.Unlock()
	if wasHidden && window != nil {
		_ = window.Invalidate()
	}
}

func (h *Host) setHovered(target *node) {
	old := h.nodes[h.hovered]
	if old != nil && old.gesture != nil {
		if old.gesture.onHover != nil {
			old.gesture.onHover(false)
		}
		if old.gesture.onHoverAt != nil {
			old.gesture.onHoverAt(false, old.bounds)
		}
	}
	h.hovered = nodeID(target)
	if target != nil && target.gesture != nil {
		if target.gesture.onHover != nil {
			target.gesture.onHover(true)
		}
		if target.gesture.onHoverAt != nil {
			target.gesture.onHoverAt(true, target.bounds)
		}
	}
	h.invalidate()
}

func (h *Host) activatePointerTarget(target *node, position woxui.Point) {
	if target == nil || target.gesture == nil {
		return
	}
	now := time.Now()
	doubleTap := target.gesture.onDoubleTap != nil && target.id == h.lastTapID && now.Sub(h.lastTapAt) <= 200*time.Millisecond
	if doubleTap {
		target.gesture.onDoubleTap()
		h.lastTapID = 0
		h.lastTapAt = time.Time{}
	} else if target.gesture.onTap != nil {
		target.gesture.onTap()
		if target.gesture.onDoubleTap != nil {
			h.lastTapID = target.id
			h.lastTapAt = now
		}
	}
	if target.gesture.onTapAt != nil {
		target.gesture.onTapAt(woxui.Point{X: position.X - target.bounds.X, Y: position.Y - target.bounds.Y})
	}
	if target.gesture.onTapBounds != nil {
		target.gesture.onTapBounds(target.bounds)
	}
	h.invalidate()
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
	var visit func(current *node, semanticParent woxui.AccessibilityNodeID)
	visit = func(current *node, semanticParent woxui.AccessibilityNodeID) {
		if current == nil {
			return
		}
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
			if semanticParent != 0 {
				parentIndex := indexByID[semanticParent]
				nodes[parentIndex].Children = append(nodes[parentIndex].Children, nativeNode.ID)
			}
			nextParent = nativeNode.ID
		}
		for _, child := range current.children {
			visit(child, nextParent)
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
	if action == woxui.AccessibilityActionActivate && current.gesture != nil && current.gesture.onTap != nil {
		current.gesture.onTap()
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
	if h.window != nil {
		_ = h.window.Invalidate()
	}
}
