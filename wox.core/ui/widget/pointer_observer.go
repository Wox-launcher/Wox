package widget

import ui "wox/ui/runtime"

// observePointer shares hit geometry without changing capture, focus, selection,
// or raw input delivery. Ancestors observe only their visible hit-test branch,
// so clipping and modal/overlay occlusion follow normal pointer routing.
func (h *Host) observePointer(target *node, event ui.PointerEvent) {
	for current := target; current != nil; current = current.parent {
		if current.gesture == nil || current.gesture.onPointerObserve == nil {
			continue
		}
		local := event
		local.Position = current.localPoint(event.Position)
		current.gesture.onPointerObserve(local)
	}
}
