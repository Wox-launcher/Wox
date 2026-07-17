package widget

import (
	"sync"

	woxui "wox/ui/runtime"
)

// ScrollController exposes optional imperative control while keeping scroll geometry inside the retained widget state.
type ScrollController struct {
	mu         sync.RWMutex
	offset     float32
	viewport   float32
	content    float32
	attachment *ScrollAttachment
}

// ScrollAttachment owns one retained ScrollView binding.
type ScrollAttachment struct {
	controller *ScrollController
	element    *stateElement
	onChanged  func(float32)
}

// NewScrollController creates a controller with an initial logical offset.
func NewScrollController(initialOffset float32) *ScrollController {
	return &ScrollController{offset: max(float32(0), initialOffset)}
}

// Offset reports the current clamped logical offset.
func (c *ScrollController) Offset() float32 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.offset
}

// MaxOffset reports the latest content extent beyond the viewport.
func (c *ScrollController) MaxOffset() float32 {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return max(float32(0), c.content-c.viewport)
}

// JumpTo clamps and publishes an absolute logical offset.
func (c *ScrollController) JumpTo(offset float32) bool {
	return c.setOffset(offset)
}

// ScrollBy applies a logical delta to the current offset.
func (c *ScrollController) ScrollBy(delta float32) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	offset := c.offset
	c.mu.RUnlock()
	return c.setOffset(offset + delta)
}

// EnsureVisible minimally scrolls so the supplied content interval is inside the viewport.
func (c *ScrollController) EnsureVisible(start, end float32) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	offset := c.offset
	viewport := c.viewport
	c.mu.RUnlock()
	if start < offset {
		return c.setOffset(start)
	}
	if end > offset+viewport {
		return c.setOffset(end - viewport)
	}
	return false
}

func (c *ScrollController) attach(context StateContext, onChanged func(float32)) *ScrollAttachment {
	if c == nil || !context.Mounted() {
		return nil
	}
	attachment := &ScrollAttachment{controller: c, element: context.element, onChanged: onChanged}
	c.mu.Lock()
	c.attachment = attachment
	c.mu.Unlock()
	return attachment
}

// Detach removes this retained binding without disturbing a controller that has moved elsewhere.
func (a *ScrollAttachment) Detach() {
	if a == nil || a.controller == nil {
		return
	}
	a.controller.mu.Lock()
	if a.controller.attachment == a {
		a.controller.attachment = nil
	}
	a.controller.mu.Unlock()
	a.controller = nil
	a.element = nil
	a.onChanged = nil
}

// setGeometry records measured extents and reclamps offsets after content changes.
func (c *ScrollController) setGeometry(viewport, content float32) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.viewport = max(float32(0), viewport)
	c.content = max(c.viewport, content)
	offset := min(max(float32(0), c.offset), max(float32(0), c.content-c.viewport))
	changed := offset != c.offset
	c.offset = offset
	element, onChanged := c.changeTargetLocked()
	c.mu.Unlock()
	if changed {
		publishScrollChange(element, onChanged, offset)
	}
}

// setOffset publishes a changed, geometry-clamped offset to the retained widget.
func (c *ScrollController) setOffset(offset float32) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	if c.content > 0 {
		offset = min(max(float32(0), offset), max(float32(0), c.content-c.viewport))
	} else {
		offset = max(float32(0), offset)
	}
	if offset == c.offset {
		c.mu.Unlock()
		return false
	}
	c.offset = offset
	element, onChanged := c.changeTargetLocked()
	c.mu.Unlock()
	publishScrollChange(element, onChanged, offset)
	return true
}

func (c *ScrollController) changeTargetLocked() (*stateElement, func(float32)) {
	if c.attachment == nil {
		return nil, nil
	}
	return c.attachment.element, c.attachment.onChanged
}

// publishScrollChange notifies the owner and schedules a new frame outside the controller lock.
func publishScrollChange(element *stateElement, onChanged func(float32), offset float32) {
	if element == nil || !element.mounted.Load() {
		return
	}
	if onChanged != nil {
		onChanged(offset)
	}
	StateContext{element: element}.Invalidate()
}

type scrollViewState struct {
	controller         *ScrollController
	internalController *ScrollController
	attachment         *ScrollAttachment
	keepVisiblePending bool
	hasGeometry        bool
	viewport           float32
	content            float32
}

// InitState creates the internal controller when the caller does not provide one.
func (s *scrollViewState) InitState(context StateContext, widget any) {
	props := widget.(ScrollView)
	s.updateBindings(context, props)
	s.keepVisiblePending = props.KeepVisible != nil
}

// DidUpdateWidget rebinds an externally replaced controller without resetting its offset.
func (s *scrollViewState) DidUpdateWidget(context StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(ScrollView)
	newProps := newWidget.(ScrollView)
	s.updateBindings(context, newProps)
	if !sameScrollRange(oldProps.KeepVisible, newProps.KeepVisible) {
		s.keepVisiblePending = newProps.KeepVisible != nil
	}
}

// Build connects pointer scrolling and measured geometry to the retained controller.
func (s *scrollViewState) Build(context StateContext, widget any) Widget {
	props := widget.(ScrollView)
	s.updateBindings(context, props)
	primitive := props
	primitive.Key = ""
	primitive.ID = ""
	primitive.Controller = nil
	primitive.KeepVisible = nil
	primitive.InitialOffset = 0
	primitive.OnOffsetChanged = nil
	primitive.Offset = s.controller.Offset()
	primitive.onGeometry = func(viewport, content float32) {
		geometryChanged := !s.hasGeometry || s.viewport != viewport || s.content != content
		s.hasGeometry = true
		s.viewport = viewport
		s.content = content
		s.controller.setGeometry(viewport, content)
		if props.KeepVisible != nil && (s.keepVisiblePending || geometryChanged) {
			s.keepVisiblePending = false
			s.controller.EnsureVisible(props.KeepVisible.Start, props.KeepVisible.End)
		}
	}
	id := props.ID
	if id == "" {
		id = string(props.Key)
	}
	return Gesture{ID: id, OnScroll: func(delta woxui.Point) {
		s.controller.ScrollBy(-delta.Y)
	}, Child: primitive}
}

// sameScrollRange compares declarative visibility targets by value across immutable rebuilds.
func sameScrollRange(left, right *ScrollRange) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Start == right.Start && left.End == right.End
}

// Dispose detaches the controller from the retained element.
func (s *scrollViewState) Dispose() {
	if s.attachment != nil {
		s.attachment.Detach()
		s.attachment = nil
	}
}

// updateBindings keeps one controller attached to the current retained ScrollView state.
func (s *scrollViewState) updateBindings(context StateContext, props ScrollView) {
	controller := props.Controller
	if controller == nil {
		if s.internalController == nil {
			s.internalController = NewScrollController(props.InitialOffset)
		}
		controller = s.internalController
	}
	if s.controller == controller && s.attachment != nil {
		controller.mu.Lock()
		s.attachment.onChanged = props.OnOffsetChanged
		controller.mu.Unlock()
		return
	}
	if s.attachment != nil {
		s.attachment.Detach()
	}
	s.controller = controller
	s.attachment = controller.attach(context, props.OnOffsetChanged)
	s.hasGeometry = false
	s.keepVisiblePending = props.KeepVisible != nil
}
