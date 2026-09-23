package widget

// defaultViewportSlotOverscan starts loading a picture shortly before it scrolls into view.
const defaultViewportSlotOverscan = 240

// viewportPass marks the outermost scroll that flushes visibility after nested axes agree.
type viewportPass struct{}

// viewportQuery is one slot's intersection with every ancestor scroll viewport.
type viewportQuery struct {
	overscan float32
	notify   func(bool)
	seen     bool
	visible  bool
	flushed  bool
}

func (q *viewportQuery) restrict(start, extent, offset, viewport float32) {
	overscan := q.overscan
	viewStart := offset - overscan
	viewEnd := offset + viewport + overscan
	visible := extent > 0 && start < viewEnd && start+extent > viewStart
	if !q.seen {
		q.visible = visible
		q.seen = true
		return
	}
	if !visible {
		q.visible = false
	}
}

// publishScrollViewport records whether each slot intersects this scroll axis.
// Call it before the content origin is shifted by the scroll offset.
func publishScrollViewport(root *node, offset, viewport float32, horizontal bool) {
	publishScrollViewportAt(root, 0, 0, offset, viewport, horizontal)
}

func publishScrollViewportAt(n *node, originX, originY, offset, viewport float32, horizontal bool) {
	if n == nil {
		return
	}
	x := originX + n.bounds.X
	y := originY + n.bounds.Y
	if n.viewport != nil {
		start, extent := y, n.bounds.Height
		if horizontal {
			start, extent = x, n.bounds.Width
		}
		n.viewport.restrict(start, extent, offset, viewport)
	}
	for _, child := range n.children {
		publishScrollViewportAt(child, x, y, offset, viewport, horizontal)
	}
}

// flushScrollViewport delivers the combined ancestor intersection once per slot.
func flushScrollViewport(root *node) {
	if root == nil {
		return
	}
	if root.viewport != nil && root.viewport.seen && !root.viewport.flushed {
		root.viewport.flushed = true
		if root.viewport.notify != nil {
			root.viewport.notify(root.viewport.visible)
		}
	}
	for _, child := range root.children {
		flushScrollViewport(child)
	}
}

// ViewportSlot builds its body only while the slot intersects an ancestor scroll viewport.
// A zero Overscan uses the default lookahead. A negative Overscan disables lookahead.
// The aspect returned by Build is width/height; return 0 to keep the previous aspect
// so a missing bitmap does not collapse the slot.
type ViewportSlot struct {
	Key      Key
	Overscan float32
	// Release runs when the slot leaves the viewport or is removed.
	Release func()
	Build   func(visible bool, aspect float32) (Widget, float32)
}

type viewportSlotState struct {
	visible   bool
	aspect    float32
	context   StateContext
	onRelease func()
}

func (s *viewportSlotState) InitState(StateContext, any) {}

func (s *viewportSlotState) DidUpdateWidget(StateContext, any, any) {}

func (s *viewportSlotState) Dispose() {
	s.releaseViewport()
}

func (s *viewportSlotState) Build(context StateContext, widget any) Widget {
	s.context = context
	props := widget.(ViewportSlot)
	s.onRelease = props.Release
	child, aspect := viewportSlotBuild(props, s.visible, s.aspect)
	if aspect > 0 {
		s.aspect = aspect
	}
	return viewportSlotBody{
		visible: s.visible, aspect: s.aspect, overscan: viewportSlotOverscan(props.Overscan),
		child: child, build: props.Build, notify: s.setVisible,
	}
}

func (s *viewportSlotState) setVisible(visible bool) {
	if s.visible == visible {
		return
	}
	s.visible = visible
	if !visible {
		s.releaseViewport()
	}
	s.context.Invalidate()
}

func (s *viewportSlotState) releaseViewport() {
	if s.onRelease != nil {
		s.onRelease()
	}
}

type viewportSlotBody struct {
	visible  bool
	aspect   float32
	overscan float32
	child    Widget
	build    func(bool, float32) (Widget, float32)
	notify   func(bool)
}

func (w viewportSlotBody) layout(ctx context, available constraints) *node {
	// No scroll ancestor means the slot is on screen. Rebuild so measurement
	// sees the loaded body instead of the deferred placeholder.
	if ctx.scroll == nil {
		child, _ := viewportSlotBuild(ViewportSlot{Build: w.build}, true, w.aspect)
		if child == nil {
			child = w.child
		}
		node := child.layout(ctx, available)
		if !w.visible && w.notify != nil {
			w.notify(true)
		}
		return node
	}
	if w.child == nil {
		return &node{}
	}
	node := w.child.layout(ctx, available)
	node.viewport = &viewportQuery{overscan: w.overscan, notify: w.notify}
	return node
}

func (w ViewportSlot) layout(ctx context, available constraints) *node {
	key := w.Key
	if key == "" {
		key = "viewport-slot"
	}
	return Stateful{
		Key: key, Type: (*viewportSlotState)(nil), Widget: w,
		CreateState: func() State { return &viewportSlotState{} },
	}.layout(ctx, available)
}

func viewportSlotBuild(props ViewportSlot, visible bool, aspect float32) (Widget, float32) {
	if props.Build == nil {
		return nil, 0
	}
	return props.Build(visible, aspect)
}

func viewportSlotOverscan(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value == 0 {
		return defaultViewportSlotOverscan
	}
	return value
}
