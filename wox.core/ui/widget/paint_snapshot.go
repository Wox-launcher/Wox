package widget

import ui "wox/ui/runtime"

// CapturePaint reports the child's last complete portable drawing, including
// overflow, without retaining its widget state. Use inside SnapshotScope. The
// callback runs during painting and must not rebuild or mutate the widget tree.
type CapturePaint struct {
	Child     Widget
	OnCapture func(*ui.PaintSnapshot)
}

// layout adds a paint-time observation without measuring or building twice.
func (w CapturePaint) layout(ctx context, available constraints) *node {
	result := &node{capturePaint: w.OnCapture}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	return available.constrainNode(result)
}

// SnapshotLayer is paint data only: it cannot retain focus, semantics, gestures,
// or callbacks from the captured widget. Opacity is explicit (1 is unchanged).
// A nil matrix means identity; otherwise the map uses the relative pivot.
type SnapshotLayer struct {
	Snapshot *ui.PaintSnapshot
	// Fragments follow Snapshot and share one opacity/transform around their union.
	Fragments        []*ui.PaintSnapshot
	Opacity          float32
	Matrix           *ui.AffineTransform
	OriginX, OriginY float32
}

// layout keeps a snapshot in scope coordinates, independently of current live
// layout constraints. Animation wrappers can sample each layer independently.
func (w SnapshotLayer) layout(_ context, _ constraints) *node {
	if w.Opacity <= 0 {
		return &node{}
	}
	snapshots := make([]*ui.PaintSnapshot, 0, 1+len(w.Fragments))
	if w.Snapshot != nil {
		snapshots = append(snapshots, w.Snapshot)
	}
	for _, fragment := range w.Fragments {
		if fragment != nil {
			snapshots = append(snapshots, fragment)
		}
	}
	if len(snapshots) == 0 {
		return &node{}
	}
	bounds := snapshots[0].Bounds()
	for _, snapshot := range snapshots[1:] {
		bounds = unionDamageRects(bounds, snapshot.Bounds())
	}
	alpha := min(float32(1), w.Opacity)
	paint := &node{bounds: bounds, opacity: &alpha, paint: func(list *ui.DisplayList, _ ui.Rect) { list.DrawSnapshots(snapshots) }}
	if w.Matrix != nil && w.Matrix.Finite() {
		x, y := w.OriginX*bounds.Width, w.OriginY*bounds.Height
		matrix := ui.TranslationTransform(-x, -y).Then(*w.Matrix).Then(ui.TranslationTransform(x, y))
		paint.transform = &matrix
	}
	return paint
}

// SnapshotScope anchors captures and inert foreground layers to the child's
// logical origin. Layers do not consume layout space or participate in input.
// Ancestor opacity/clip/transform outside this scope stays live at replay.
type SnapshotScope struct {
	Child Widget
	// Layers accepts SnapshotLayer widgets, optionally wrapped in animations.
	Layers []Widget
	// OnPaint runs before live paint even when Build was cached. It must not
	// rebuild the widget tree; owners use it to start fresh capture accounting.
	OnPaint func()
}

// layout measures live content once and reports snapshot overflow for damage
// and enclosing opacity bounds, including when live content shrinks to nothing.
func (w SnapshotScope) layout(ctx context, available constraints) *node {
	result := &node{snapshotScope: true, snapshotPaint: w.OnPaint}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	result = available.constrainNode(result)
	for _, layer := range w.Layers {
		if layer == nil {
			continue
		}
		paint := layer.layout(ctx, available)
		bounds := paint.drawingTransform(paint.bounds).Bounds(paint.opacityPaintBounds(ui.Point{}))
		result.children = append(result.children, paint)
		result.paintOutsets.Left = max(result.paintOutsets.Left, -bounds.X)
		result.paintOutsets.Top = max(result.paintOutsets.Top, -bounds.Y)
		result.paintOutsets.Right = max(result.paintOutsets.Right, bounds.X+bounds.Width-result.bounds.Width)
		result.paintOutsets.Bottom = max(result.paintOutsets.Bottom, bounds.Y+bounds.Height-result.bounds.Height)
	}
	return result
}
