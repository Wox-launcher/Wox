package widget

import ui "wox/ui/runtime"

// pressObservation stores identity rather than callbacks so a release uses the
// current tree's props and never invokes a removed control. Gesture IDs also
// survive retained-path changes caused by overlays and cached subtree rebuilds.
type pressObservation struct {
	id        ui.AccessibilityNodeID
	gestureID string
}

// observePressNode captures the retained and explicit identities of one gesture.
func observePressNode(n *node) pressObservation {
	if n == nil {
		return pressObservation{}
	}
	observation := pressObservation{id: n.id}
	if n.gesture != nil {
		observation.gestureID = n.gesture.id
	}
	return observation
}

// node resolves a retained press against the live tree, including moved targets.
func (o pressObservation) node(h *Host) *node {
	if current := h.nodes[o.id]; current != nil {
		return current
	}
	return h.gestureNodeByID(o.gestureID)
}

// beginObservedPress runs before raw-pointer dispatch so embedded surfaces can
// consume the input while their containing card still reflects the press.
func (h *Host) beginObservedPress(target *node) {
	h.endObservedPress()
	h.pressTarget = observePressNode(target)
	for current := target; current != nil; current = current.parent {
		if current.gesture != nil && current.gesture.observeDescendants && current.gesture.onPressChange != nil {
			h.pressObservers = append(h.pressObservers, observePressNode(current))
		}
	}
	observers := append([]pressObservation(nil), h.pressObservers...)
	for _, observer := range observers {
		if current := observer.node(h); current != nil && current.gesture != nil && current.gesture.onPressChange != nil {
			current.gesture.onPressChange(true)
		}
	}
	if len(observers) > 0 {
		h.invalidate()
	}
}

// endObservedPress clears ownership before callbacks, allowing reentrant teardown.
func (h *Host) endObservedPress() {
	observers := h.pressObservers
	h.pressObservers = nil
	h.pressTarget = pressObservation{}
	for _, observer := range observers {
		if current := observer.node(h); current != nil && current.gesture != nil && current.gesture.onPressChange != nil {
			current.gesture.onPressChange(false)
		}
	}
	if len(observers) > 0 {
		h.invalidate()
	}
}

// reconcileObservedPress cancels surviving observers if the pressed target was
// removed or moved out of their subtree; newly added ancestors do not inherit it.
func (h *Host) reconcileObservedPress() {
	if len(h.pressObservers) == 0 {
		return
	}
	target := h.pressTarget.node(h)
	if target == nil {
		h.endObservedPress()
		return
	}
	previous := h.pressObservers
	h.pressObservers = nil
	for _, observer := range previous {
		current := observer.node(h)
		if current == nil || current.gesture == nil || current.gesture.onPressChange == nil {
			continue
		}
		if current.gesture.observeDescendants && h.isDescendantOf(target, current.id) {
			h.pressObservers = append(h.pressObservers, observePressNode(current))
		} else {
			current.gesture.onPressChange(false)
			h.invalidate()
		}
	}
}
