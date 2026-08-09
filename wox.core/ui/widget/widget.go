package widget

import (
	"math"
	"strings"
	"unicode"

	woxui "wox/ui/runtime"
)

// Widget produces one laid-out render node for the current constraints.
type Widget interface {
	layout(context, constraints) *node
}

// MeasureStateless returns the natural size of a widget tree that does not contain retained State.
func MeasureStateless(window HostServices, widget Widget, width float32) woxui.Size {
	if widget == nil {
		return woxui.Size{}
	}
	node := widget.layout(context{window: window}, constraints{width: width, height: math.MaxFloat32})
	return woxui.Size{Width: node.bounds.Width, Height: node.bounds.Height}
}

type textMeasurer interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
}

type context struct {
	window    textMeasurer
	animation animationFrame
	dynamic   *dynamicUse
	damage    *frameDamageTracker
	debug     *repaintDebugFrame
	elements  *elementTree
	element   *stateElement
}

func (c context) withElement(element *stateElement) context {
	c.element = element
	return c
}

func (c context) useScroll(controller *ScrollController, offset float32) {
	if c.dynamic != nil && controller != nil {
		c.dynamic.scrolls = append(c.dynamic.scrolls, scrollDependency{controller: controller, offset: offset})
	}
}

type constraints struct {
	width  float32
	height float32
}

type node struct {
	id         woxui.AccessibilityNodeID
	key        Key
	kind       string
	parent     *node
	bounds     woxui.Rect
	paint      func(*woxui.DisplayList, woxui.Rect)
	gesture    *gesture
	focus      *focusBehavior
	scope      *focusScopeBehavior
	semantic   *semanticBehavior
	scroll     *scrollBehavior
	caret      bool
	caretPaint func(*woxui.DisplayList, woxui.Rect, bool, bool)
	clip       bool
	children   []*node
	boundary   *boundaryCache
}

func (n *node) place(x, y float32) {
	n.bounds.X += x
	n.bounds.Y += y
	for _, child := range n.children {
		child.place(x, y)
	}
}

func (n *node) draw(displayList *woxui.DisplayList, focused, focusRingTarget woxui.AccessibilityNodeID, caretVisible, focusWithin, focusableWithin bool) {
	if n.focus != nil {
		focusWithin = n.id == focused
		focusableWithin = true
	} else {
		focusWithin = focusWithin || n.id == focused
	}
	if n.paint != nil {
		n.paint(displayList, n.bounds)
	}
	if n.caretPaint != nil {
		caretFocused := n.caret
		if focusableWithin {
			// Reconciliation runs after retained widgets build, so the Host focus is the
			// authoritative caret state for this frame rather than the captured FocusNode value.
			caretFocused = focusWithin
		}
		n.caretPaint(displayList, n.bounds, caretFocused, caretVisible)
	}
	if n.clip {
		displayList.PushClipRect(n.bounds)
	}
	for _, child := range n.children {
		child.draw(displayList, focused, focusRingTarget, caretVisible, focusWithin, focusableWithin)
	}
	if n.id == focusRingTarget && n.focus != nil && n.focus.focusRingColor.A != 0 {
		outsets := n.focus.focusRingOutsets
		bounds := woxui.Rect{
			X: n.bounds.X - outsets.Left, Y: n.bounds.Y - outsets.Top,
			Width: n.bounds.Width + outsets.Left + outsets.Right, Height: n.bounds.Height + outsets.Top + outsets.Bottom,
		}
		displayList.StrokeRoundedRect(bounds, n.focus.focusRingRadius, 2, n.focus.focusRingColor)
	}
	if n.clip {
		displayList.PopClipRect()
	}
}

func (n *node) hitTest(point woxui.Point) *node {
	if point.X < n.bounds.X || point.Y < n.bounds.Y || point.X >= n.bounds.X+n.bounds.Width || point.Y >= n.bounds.Y+n.bounds.Height {
		return nil
	}
	for index := len(n.children) - 1; index >= 0; index-- {
		if hit := n.children[index].hitTest(point); hit != nil {
			return hit
		}
	}
	if n.gesture != nil {
		return n
	}
	// Modal scopes are opaque pointer surfaces. Returning the scope for an otherwise
	// empty area prevents a backdrop behind the dialog from receiving the click.
	if n.scope != nil && n.scope.modal {
		return n
	}
	return nil
}

func (n *node) hitTestScroll(point woxui.Point) *node {
	if point.X < n.bounds.X || point.Y < n.bounds.Y || point.X >= n.bounds.X+n.bounds.Width || point.Y >= n.bounds.Y+n.bounds.Height {
		return nil
	}
	for index := len(n.children) - 1; index >= 0; index-- {
		if hit := n.children[index].hitTestScroll(point); hit != nil {
			return hit
		}
	}
	if n.gesture != nil && (n.gesture.onScroll != nil || n.gesture.onScrollHandled != nil || n.gesture.onPointer != nil) {
		return n
	}
	return nil
}

// Insets describes logical padding around a child.
type Insets struct {
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}

// UniformInsets creates equal padding on all sides.
func UniformInsets(value float32) Insets {
	return Insets{Left: value, Top: value, Right: value, Bottom: value}
}

// Container paints an optional background and positions one child.
type Container struct {
	Width       float32
	Height      float32
	Padding     Insets
	Color       woxui.Color
	BorderColor woxui.Color
	BorderWidth float32
	Radius      float32
	Child       Widget
}

// Align positions one child inside a fixed box using normalized axis factors.
type Align struct {
	Width      float32
	Height     float32
	Horizontal float32
	Vertical   float32
	Child      Widget
}

// Constrained applies optional size bounds and fill behavior to one child.
type Constrained struct {
	MinWidth   float32
	MaxWidth   float32
	MinHeight  float32
	MaxHeight  float32
	FillWidth  bool
	FillHeight bool
	Child      Widget
}

// LayoutBuilder builds a child from the immutable size available during layout.
// Build should only use that size and immutable values captured from parent props.
type LayoutBuilder struct {
	Build func(woxui.Size) Widget
}

func (w Align) layout(ctx context, available constraints) *node {
	width := available.width
	if w.Width > 0 {
		width = min(w.Width, available.width)
	}
	height := available.height
	if w.Height > 0 {
		height = min(w.Height, available.height)
	}
	result := &node{bounds: woxui.Rect{Width: width, Height: height}}
	if w.Child == nil {
		return result
	}
	child := w.Child.layout(ctx, constraints{width: width, height: height})
	horizontal := min(max(float32(0), w.Horizontal), float32(1))
	vertical := min(max(float32(0), w.Vertical), float32(1))
	child.place(max(float32(0), width-child.bounds.Width)*horizontal, max(float32(0), height-child.bounds.Height)*vertical)
	result.children = []*node{child}
	return result
}

func (w Constrained) layout(ctx context, available constraints) *node {
	maxWidth := available.width
	if w.MaxWidth > 0 {
		maxWidth = min(maxWidth, w.MaxWidth)
	}
	maxHeight := available.height
	if w.MaxHeight > 0 {
		maxHeight = min(maxHeight, w.MaxHeight)
	}
	minWidth := min(max(float32(0), w.MinWidth), maxWidth)
	minHeight := min(max(float32(0), w.MinHeight), maxHeight)
	if w.Child == nil {
		width := minWidth
		height := minHeight
		if w.FillWidth && maxWidth < math.MaxFloat32 {
			width = maxWidth
		}
		if w.FillHeight && maxHeight < math.MaxFloat32 {
			height = maxHeight
		}
		return &node{bounds: woxui.Rect{Width: width, Height: height}}
	}
	child := w.Child.layout(ctx, constraints{width: maxWidth, height: maxHeight})
	width := min(max(child.bounds.Width, minWidth), maxWidth)
	height := min(max(child.bounds.Height, minHeight), maxHeight)
	if w.FillWidth && maxWidth < math.MaxFloat32 {
		width = maxWidth
	}
	if w.FillHeight && maxHeight < math.MaxFloat32 {
		height = maxHeight
	}
	child.bounds.Width = width
	child.bounds.Height = height
	return child
}

func (w LayoutBuilder) layout(ctx context, available constraints) *node {
	if w.Build == nil {
		return &node{}
	}
	child := w.Build(woxui.Size{Width: available.width, Height: available.height})
	if child == nil {
		return &node{}
	}
	return child.layout(ctx, available)
}

func (w Container) layout(ctx context, available constraints) *node {
	contentWidth := available.width
	if w.Width > 0 {
		contentWidth = w.Width
	}
	contentHeight := available.height
	if w.Height > 0 {
		contentHeight = w.Height
	}
	contentWidth = max(0, contentWidth-w.Padding.Left-w.Padding.Right)
	contentHeight = max(0, contentHeight-w.Padding.Top-w.Padding.Bottom)
	var child *node
	if w.Child != nil {
		child = w.Child.layout(ctx, constraints{width: contentWidth, height: contentHeight})
		child.place(w.Padding.Left, w.Padding.Top)
	}
	width := w.Width
	if width <= 0 {
		width = w.Padding.Left + w.Padding.Right
		if child != nil {
			width += child.bounds.Width
		}
	}
	height := w.Height
	if height <= 0 {
		height = w.Padding.Top + w.Padding.Bottom
		if child != nil {
			height += child.bounds.Height
		}
	}
	result := &node{bounds: woxui.Rect{Width: width, Height: height}}
	if w.Color.A != 0 || (w.BorderColor.A != 0 && w.BorderWidth > 0) {
		result.paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if w.Color.A != 0 {
				displayList.FillRoundedRect(bounds, w.Radius, w.Color)
			}
			if w.BorderColor.A != 0 && w.BorderWidth > 0 {
				displayList.StrokeRoundedRect(bounds, w.Radius, w.BorderWidth, w.BorderColor)
			}
		}
	}
	if child != nil {
		result.children = []*node{child}
	}
	return result
}

// Axis names the main direction of a Flex widget.
type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

// CrossAxisAlignment positions Flex children perpendicular to its main axis.
type CrossAxisAlignment uint8

const (
	CrossAxisStart CrossAxisAlignment = iota
	CrossAxisCenter
	CrossAxisEnd
	CrossAxisStretch
)

// MainAxisAlignment positions Flex children when the main axis has unused space.
type MainAxisAlignment uint8

const (
	MainAxisStart MainAxisAlignment = iota
	MainAxisCenter
	MainAxisEnd
	MainAxisSpaceBetween
)

// Flex lays children out sequentially with a fixed gap.
type Flex struct {
	Axis               Axis
	Gap                float32
	MainAxisAlignment  MainAxisAlignment
	CrossAxisAlignment CrossAxisAlignment
	Children           []Widget
}

// Expanded gives its child a weighted share of the remaining Flex main-axis extent.
type Expanded struct {
	Flex  float32
	Child Widget
}

func (w Expanded) layout(ctx context, available constraints) *node {
	if w.Child == nil {
		return &node{}
	}
	return w.Child.layout(ctx, available)
}

// StackChild positions one child by insets; stretch fills between opposite insets and takes precedence over anchoring.
type StackChild struct {
	Left          float32
	Top           float32
	Right         float32
	Bottom        float32
	AnchorRight   bool
	AnchorBottom  bool
	StretchWidth  bool
	StretchHeight bool
	Child         Widget
}

// Stack overlays children in declaration order; later children receive pointer events first.
type Stack struct {
	Width    float32
	Height   float32
	Children []StackChild
}

// ScrollRange identifies a content interval that a retained ScrollView should keep visible.
type ScrollRange struct {
	Start float32
	End   float32
}

// ScrollView clips a larger child and optionally retains its own offset when Key is set.
type ScrollView struct {
	Key               Key
	ID                string
	Width             float32
	Height            float32
	MaxHeight         float32
	ContentWidth      float32
	ContentHeight     float32
	Horizontal        bool
	Offset            float32
	InitialOffset     float32
	Controller        *ScrollController
	KeepVisible       *ScrollRange
	KeepVisibleKey    Key
	OnOffsetChanged   func(float32)
	OnGeometryChanged func(viewport, content float32)
	Child             Widget
	onGeometry        func(viewport, content float32, measuredKeepVisible *ScrollRange)
	onEnsureVisible   func(start, end float32) bool
	dynamicController *ScrollController
}

// Clip confines a child to a fixed logical rectangle without applying scrolling.
type Clip struct {
	Width  float32
	Height float32
	Child  Widget
}

func (w Clip) layout(ctx context, available constraints) *node {
	width := min(w.Width, available.width)
	height := min(w.Height, available.height)
	result := &node{bounds: woxui.Rect{Width: width, Height: height}, clip: true}
	if w.Child != nil {
		result.children = []*node{w.Child.layout(ctx, constraints{width: width, height: height})}
	}
	return result
}

func (w ScrollView) layout(ctx context, available constraints) *node {
	if w.Key != "" {
		return (Stateful{
			Key: w.Key, Type: (*scrollViewState)(nil), Widget: w,
			CreateState: func() State { return &scrollViewState{} },
		}).layout(ctx, available)
	}
	ctx.useScroll(w.dynamicController, w.Offset)
	width := available.width
	if w.Width > 0 {
		width = min(w.Width, available.width)
	}
	height := available.height
	if w.Height > 0 {
		height = min(w.Height, available.height)
	} else if w.MaxHeight > 0 {
		height = min(w.MaxHeight, available.height)
	}
	if w.Horizontal {
		contentWidth := max(width, w.ContentWidth)
		var child *node
		if w.Child != nil {
			child = w.Child.layout(ctx, constraints{width: contentWidth, height: height})
			contentWidth = max(contentWidth, child.bounds.Width)
		}
		offset := min(max(float32(0), w.Offset), max(float32(0), contentWidth-width))
		if w.onGeometry != nil {
			w.onGeometry(width, contentWidth, scrollChildRange(child, w.KeepVisibleKey, true))
		} else if w.OnGeometryChanged != nil {
			w.OnGeometryChanged(width, contentWidth)
		}
		result := &node{bounds: woxui.Rect{Width: width, Height: height}, clip: true}
		if w.onEnsureVisible != nil {
			result.scroll = &scrollBehavior{horizontal: true, offset: offset, ensureVisible: w.onEnsureVisible}
		}
		if child != nil {
			child.place(-offset, 0)
			result.children = []*node{child}
		}
		return result
	}
	contentHeight := max(height, w.ContentHeight)
	var child *node
	if w.Child != nil {
		child = w.Child.layout(ctx, constraints{width: width, height: contentHeight})
		// Flex children can legitimately exceed a caller's estimated extent. The measured height must remain scrollable.
		contentHeight = max(contentHeight, child.bounds.Height)
	}
	if w.Height <= 0 && w.MaxHeight > 0 && child != nil {
		height = max(float32(1), min(height, child.bounds.Height))
		contentHeight = max(height, max(w.ContentHeight, child.bounds.Height))
	}
	offset := min(max(float32(0), w.Offset), max(float32(0), contentHeight-height))
	if w.onGeometry != nil {
		w.onGeometry(height, contentHeight, scrollChildRange(child, w.KeepVisibleKey, false))
	} else if w.OnGeometryChanged != nil {
		w.OnGeometryChanged(height, contentHeight)
	}
	result := &node{bounds: woxui.Rect{Width: width, Height: height}, clip: true}
	if w.onEnsureVisible != nil {
		result.scroll = &scrollBehavior{offset: offset, ensureVisible: w.onEnsureVisible}
	}
	if child != nil {
		child.place(0, -offset)
		result.children = []*node{child}
	}
	return result
}

// scrollChildRange resolves a keyed descendant after layout so scrolling follows measured geometry.
func scrollChildRange(root *node, key Key, horizontal bool) *ScrollRange {
	if root == nil || key == "" {
		return nil
	}
	target := findNodeByKey(root, key)
	if target == nil {
		return nil
	}
	if horizontal {
		return &ScrollRange{Start: target.bounds.X, End: target.bounds.X + target.bounds.Width}
	}
	return &ScrollRange{Start: target.bounds.Y, End: target.bounds.Y + target.bounds.Height}
}

func findNodeByKey(root *node, key Key) *node {
	if root == nil {
		return nil
	}
	if root.key == key {
		return root
	}
	for _, child := range root.children {
		if target := findNodeByKey(child, key); target != nil {
			return target
		}
	}
	return nil
}

func (w Stack) layout(ctx context, available constraints) *node {
	width := w.Width
	if width <= 0 {
		width = available.width
	}
	height := w.Height
	if height <= 0 {
		height = available.height
	}
	result := &node{bounds: woxui.Rect{Width: width, Height: height}}
	for _, positioned := range w.Children {
		if positioned.Child == nil {
			continue
		}
		childWidth := max(float32(0), width-positioned.Left)
		if positioned.StretchWidth {
			childWidth = max(float32(0), childWidth-positioned.Right)
		}
		childHeight := max(float32(0), height-positioned.Top)
		if positioned.StretchHeight {
			childHeight = max(float32(0), childHeight-positioned.Bottom)
		}
		child := positioned.Child.layout(ctx, constraints{width: childWidth, height: childHeight})
		x := positioned.Left
		y := positioned.Top
		if positioned.StretchWidth {
			child.bounds.Width = childWidth
		} else if positioned.AnchorRight {
			x = max(float32(0), width-positioned.Right-child.bounds.Width)
		}
		if positioned.StretchHeight {
			child.bounds.Height = childHeight
		} else if positioned.AnchorBottom {
			y = max(float32(0), height-positioned.Bottom-child.bounds.Height)
		}
		child.place(x, y)
		result.children = append(result.children, child)
	}
	return result
}

func (w Flex) layout(ctx context, available constraints) *node {
	enhanced := w.MainAxisAlignment != MainAxisStart || w.CrossAxisAlignment == CrossAxisStretch
	if !enhanced {
		for _, child := range w.Children {
			if _, ok := child.(Expanded); ok {
				enhanced = true
				break
			}
		}
	}
	if !enhanced {
		return w.layoutSequential(ctx, available)
	}

	type flexChild struct {
		widget Widget
		flex   float32
		node   *node
	}

	children := make([]flexChild, 0, len(w.Children))
	totalFlex := float32(0)
	for _, childWidget := range w.Children {
		if childWidget == nil {
			continue
		}
		child := flexChild{widget: childWidget}
		if expanded, ok := childWidget.(Expanded); ok {
			child.widget = expanded.Child
			child.flex = expanded.Flex
			if child.flex <= 0 {
				child.flex = 1
			}
			totalFlex += child.flex
		}
		if child.widget != nil {
			children = append(children, child)
		}
	}

	result := &node{}
	if len(children) == 0 {
		return result
	}
	mainAvailable := available.width
	if w.Axis == Vertical {
		mainAvailable = available.height
	}
	mainBounded := mainAvailable < math.MaxFloat32
	totalGap := w.Gap * float32(len(children)-1)
	fixedExtent := float32(0)
	for index := range children {
		if children[index].flex > 0 && mainBounded {
			continue
		}
		children[index].node = children[index].widget.layout(ctx, available)
		fixedExtent += flexMainExtent(children[index].node, w.Axis)
	}
	remaining := max(float32(0), mainAvailable-fixedExtent-totalGap)
	for index := range children {
		if children[index].flex <= 0 || !mainBounded {
			continue
		}
		share := remaining * children[index].flex / totalFlex
		childAvailable := available
		if w.Axis == Horizontal {
			childAvailable.width = share
		} else {
			childAvailable.height = share
		}
		child := children[index].widget.layout(ctx, childAvailable)
		// Expanded is a tight main-axis slot even when its child naturally shrink-wraps.
		if w.Axis == Horizontal {
			child.bounds.Width = share
		} else {
			child.bounds.Height = share
		}
		children[index].node = child
	}

	contentExtent := totalGap
	crossExtent := float32(0)
	for _, child := range children {
		contentExtent += flexMainExtent(child.node, w.Axis)
		crossExtent = max(crossExtent, flexCrossExtent(child.node, w.Axis))
	}
	if w.CrossAxisAlignment == CrossAxisStretch {
		if w.Axis == Horizontal {
			if available.height < math.MaxFloat32 {
				crossExtent = available.height
			}
		} else {
			if available.width < math.MaxFloat32 {
				crossExtent = available.width
			}
		}
	}

	mainExtent := contentExtent
	freeExtent := max(float32(0), mainAvailable-contentExtent)
	startOffset := float32(0)
	gap := w.Gap
	switch {
	case !mainBounded:
	case w.MainAxisAlignment == MainAxisCenter:
		mainExtent = mainAvailable
		startOffset = freeExtent / 2
	case w.MainAxisAlignment == MainAxisEnd:
		mainExtent = mainAvailable
		startOffset = freeExtent
	case w.MainAxisAlignment == MainAxisSpaceBetween:
		mainExtent = mainAvailable
		if len(children) > 1 {
			gap += freeExtent / float32(len(children)-1)
		}
	}

	cursor := startOffset
	for _, child := range children {
		if w.CrossAxisAlignment == CrossAxisStretch {
			if w.Axis == Horizontal {
				child.node.bounds.Height = crossExtent
			} else {
				child.node.bounds.Width = crossExtent
			}
		}
		crossX, crossY := flexChildOffset(child.node, w.Axis, crossExtent, w.CrossAxisAlignment)
		child.node.place(crossX, crossY)
		if w.Axis == Horizontal {
			child.node.place(cursor, 0)
		} else {
			child.node.place(0, cursor)
		}
		cursor += flexMainExtent(child.node, w.Axis) + gap
		result.children = append(result.children, child.node)
	}
	if w.Axis == Horizontal {
		result.bounds = woxui.Rect{Width: mainExtent, Height: crossExtent}
	} else {
		result.bounds = woxui.Rect{Width: crossExtent, Height: mainExtent}
	}
	return result
}

// layoutSequential preserves the original shrink-wrapped path for Flex trees that do not allocate free space.
func (w Flex) layoutSequential(ctx context, available constraints) *node {
	result := &node{}
	var cursor float32
	for _, childWidget := range w.Children {
		if childWidget == nil {
			continue
		}
		child := childWidget.layout(ctx, available)
		if w.Axis == Horizontal {
			child.place(cursor, 0)
			cursor += child.bounds.Width + w.Gap
			result.bounds.Width = cursor - w.Gap
			result.bounds.Height = max(result.bounds.Height, child.bounds.Height)
		} else {
			child.place(0, cursor)
			cursor += child.bounds.Height + w.Gap
			result.bounds.Height = cursor - w.Gap
			result.bounds.Width = max(result.bounds.Width, child.bounds.Width)
		}
		result.children = append(result.children, child)
	}
	crossAxisFactor := float32(0)
	switch w.CrossAxisAlignment {
	case CrossAxisCenter:
		crossAxisFactor = 0.5
	case CrossAxisEnd:
		crossAxisFactor = 1
	}
	if crossAxisFactor > 0 {
		for _, child := range result.children {
			if w.Axis == Horizontal {
				child.place(0, max(float32(0), result.bounds.Height-child.bounds.Height)*crossAxisFactor)
			} else {
				child.place(max(float32(0), result.bounds.Width-child.bounds.Width)*crossAxisFactor, 0)
			}
		}
	}
	return result
}

func flexMainExtent(child *node, axis Axis) float32 {
	if axis == Horizontal {
		return child.bounds.Width
	}
	return child.bounds.Height
}

func flexCrossExtent(child *node, axis Axis) float32 {
	if axis == Horizontal {
		return child.bounds.Height
	}
	return child.bounds.Width
}

func flexChildOffset(child *node, axis Axis, crossExtent float32, alignment CrossAxisAlignment) (float32, float32) {
	crossAxisFactor := float32(0)
	switch alignment {
	case CrossAxisCenter:
		crossAxisFactor = 0.5
	case CrossAxisEnd:
		crossAxisFactor = 1
	}
	if axis == Horizontal {
		return 0, max(float32(0), crossExtent-child.bounds.Height) * crossAxisFactor
	}
	return max(float32(0), crossExtent-child.bounds.Width) * crossAxisFactor, 0
}

// Wrap lays children horizontally and starts a new run when width is exhausted.
type Wrap struct {
	Gap      float32
	RunGap   float32
	Children []Widget
}

// Grid lays children into equal-width columns and rows measured from their tallest cells.
type Grid struct {
	Width              float32
	Columns            int
	MinColumnWidth     float32
	MaxColumns         int
	CellWidth          float32
	CellHeight         float32
	ColumnGap          float32
	RowGap             float32
	CrossAxisAlignment CrossAxisAlignment
	Children           []Widget
}

func (w Grid) layout(ctx context, available constraints) *node {
	width := available.width
	if w.Width > 0 {
		width = min(w.Width, available.width)
	}
	columns := w.Columns
	if columns <= 0 {
		columnWidth := w.MinColumnWidth
		if columnWidth <= 0 {
			columnWidth = w.CellWidth
		}
		if columnWidth > 0 && width < math.MaxFloat32 {
			columns = max(1, int((width+w.ColumnGap)/(columnWidth+w.ColumnGap)))
		} else {
			columns = 1
		}
	}
	if w.MaxColumns > 0 {
		columns = min(columns, w.MaxColumns)
	}
	columns = max(1, columns)
	cellWidth := w.CellWidth
	if cellWidth <= 0 {
		cellWidth = max(float32(0), (width-float32(columns-1)*w.ColumnGap)/float32(columns))
	}
	result := &node{bounds: woxui.Rect{Width: width}}
	for rowStart, y := 0, float32(0); rowStart < len(w.Children); rowStart += columns {
		rowEnd := min(rowStart+columns, len(w.Children))
		row := make([]*node, 0, rowEnd-rowStart)
		rowHeight := w.CellHeight
		for index := rowStart; index < rowEnd; index++ {
			if w.Children[index] == nil {
				row = append(row, nil)
				continue
			}
			cellAvailable := constraints{width: cellWidth, height: available.height}
			if w.CellHeight > 0 {
				cellAvailable.height = min(w.CellHeight, available.height)
			}
			child := w.Children[index].layout(ctx, cellAvailable)
			child.bounds.Width = cellWidth
			rowHeight = max(rowHeight, child.bounds.Height)
			row = append(row, child)
		}
		for column, child := range row {
			if child == nil {
				continue
			}
			if w.CrossAxisAlignment == CrossAxisStretch {
				child.bounds.Height = rowHeight
			}
			_, offsetY := flexChildOffset(child, Horizontal, rowHeight, w.CrossAxisAlignment)
			child.place(float32(column)*(cellWidth+w.ColumnGap), y+offsetY)
			result.children = append(result.children, child)
		}
		y += rowHeight
		if rowEnd < len(w.Children) {
			y += w.RowGap
		}
		result.bounds.Height = y
	}
	return result
}

func (w Wrap) layout(ctx context, available constraints) *node {
	result := &node{}
	var x float32
	var y float32
	var runHeight float32
	for _, childWidget := range w.Children {
		if childWidget == nil {
			continue
		}
		child := childWidget.layout(ctx, available)
		if x > 0 && x+child.bounds.Width > available.width {
			x = 0
			y += runHeight + w.RunGap
			runHeight = 0
		}
		child.place(x, y)
		x += child.bounds.Width + w.Gap
		runHeight = max(runHeight, child.bounds.Height)
		result.bounds.Width = max(result.bounds.Width, x-w.Gap)
		result.children = append(result.children, child)
	}
	result.bounds.Height = y + runHeight
	return result
}

// Text paints one measured line using the platform UI font.
type Text struct {
	Value     string
	Style     woxui.TextStyle
	Color     woxui.Color
	Underline bool
}

// TextBlock wraps and clips text in Go so every renderer receives the same shaped line boxes.
type TextBlock struct {
	Value      string
	Style      woxui.TextStyle
	Color      woxui.Color
	Width      float32
	Height     float32
	LineHeight float32
	MaxLines   int
	Centered   bool
	// ShrinkWrap keeps short text at its measured width while preserving Width as the truncation limit.
	ShrinkWrap bool
	Layout     *TextBlockLayout
}

// TextBlockLayout is the portable line layout used by TextBlock and scroll containers.
type TextBlockLayout struct {
	Lines      []string
	Size       woxui.Size
	LineHeight float32
}

// ImageFit controls how an image preserves its aspect ratio inside its bounds.
type ImageFit uint8

const (
	ImageFitFill ImageFit = iota
	ImageFitContain
	ImageFitCover
)

// Image paints a raster resource into a fixed logical rectangle.
type Image struct {
	Source *woxui.Image
	Width  float32
	Height float32
	Fit    ImageFit
	Radius float32
}

func (w Image) layout(ctx context, available constraints) *node {
	_ = ctx
	width := min(w.Width, available.width)
	height := min(w.Height, available.height)
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if w.Fit == ImageFitCover {
				displayList.PushClipRect(bounds)
				defer displayList.PopClipRect()
			}
			imageBounds := fittedImageBounds(w.Source, bounds, w.Fit)
			if w.Radius > 0 {
				displayList.DrawRotatedRoundedImage(w.Source, imageBounds, 0, w.Radius)
				return
			}
			displayList.DrawImage(w.Source, imageBounds)
		},
	}
}

// fittedImageBounds applies contain or cover sizing while keeping the image centered.
func fittedImageBounds(source *woxui.Image, bounds woxui.Rect, fit ImageFit) woxui.Rect {
	if source == nil || source.Width <= 0 || source.Height <= 0 || fit == ImageFitFill {
		return bounds
	}
	scale := min(bounds.Width/float32(source.Width), bounds.Height/float32(source.Height))
	if fit == ImageFitCover {
		scale = max(bounds.Width/float32(source.Width), bounds.Height/float32(source.Height))
	}
	width := float32(source.Width) * scale
	height := float32(source.Height) * scale
	return woxui.Rect{X: bounds.X + (bounds.Width-width)/2, Y: bounds.Y + (bounds.Height-height)/2, Width: width, Height: height}
}

func (w Text) layout(ctx context, available constraints) *node {
	metrics, _ := ctx.window.MeasureText(w.Value, w.Style)
	width := min(metrics.Size.Width, available.width)
	height := min(metrics.Size.Height, available.height)
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.DrawText(w.Value, bounds, w.Style, w.Color)
			if w.Underline && bounds.Width > 0 && bounds.Height > 0 {
				displayList.FillRect(woxui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - 1, Width: bounds.Width, Height: 1}, w.Color)
			}
		},
	}
}

func (w TextBlock) layout(ctx context, available constraints) *node {
	width := available.width
	if w.Width > 0 {
		width = min(width, w.Width)
	}
	if w.ShrinkWrap {
		metrics, _ := ctx.window.MeasureText(w.Value, w.Style)
		width = min(width, metrics.Size.Width)
	}
	heightLimit := available.height
	if w.Height > 0 {
		heightLimit = min(heightLimit, w.Height)
	}
	metrics, _ := ctx.window.MeasureText("Mg", w.Style)
	lineHeight := w.LineHeight
	if lineHeight <= 0 {
		lineHeight = max(metrics.Size.Height, w.Style.Size*1.35)
	}
	maxLines := w.MaxLines
	if heightLimit > 0 {
		visibleLines := max(1, int(heightLimit/lineHeight))
		if maxLines <= 0 || visibleLines < maxLines {
			maxLines = visibleLines
		}
	}
	textLayout := TextBlockLayout{}
	if w.Layout != nil {
		textLayout = *w.Layout
	} else {
		textLayout = layoutTextBlock(ctx.window, w.Value, w.Style, width, maxLines, lineHeight)
	}
	height := min(heightLimit, textLayout.Size.Height)
	if w.Height > 0 {
		height = heightLimit
	}
	window := ctx.window
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			start := 0
			end := len(textLayout.Lines)
			if clip, ok := displayList.ClipRect(); ok {
				if clip.Y > bounds.Y {
					start = max(0, min(end, int((clip.Y-bounds.Y)/lineHeight)))
				}
				clipBottom := clip.Y + clip.Height
				if clipBottom < bounds.Y+bounds.Height {
					end = max(start, min(end, int((clipBottom-bounds.Y)/lineHeight)+1))
				}
			}
			for index := start; index < end; index++ {
				line := textLayout.Lines[index]
				y := bounds.Y + float32(index)*lineHeight
				if y+lineHeight > bounds.Y+bounds.Height+0.5 {
					break
				}
				lineBounds := woxui.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: lineHeight}
				if w.Centered {
					metrics, _ := window.MeasureText(line, w.Style)
					lineWidth := min(metrics.Size.Width, bounds.Width)
					lineBounds = woxui.Rect{X: bounds.X + (bounds.Width-lineWidth)/2, Y: y, Width: lineWidth, Height: lineHeight}
				}
				displayList.DrawText(line, lineBounds, w.Style, w.Color)
			}
		},
	}
}

// LayoutTextBlock wraps text with the same platform font metrics used during rendering.
func LayoutTextBlock(window *woxui.Window, value string, style woxui.TextStyle, width float32, maxLines int, lineHeight float32) TextBlockLayout {
	return layoutTextBlock(window, value, style, width, maxLines, lineHeight)
}

func layoutTextBlock(window textMeasurer, value string, style woxui.TextStyle, width float32, maxLines int, lineHeight float32) TextBlockLayout {
	if lineHeight <= 0 {
		metrics, _ := window.MeasureText("Mg", style)
		lineHeight = max(metrics.Size.Height, style.Size*1.35)
	}
	lines := wrapTextLines(window, value, style, width, maxLines)
	return TextBlockLayout{Lines: lines, Size: woxui.Size{Width: width, Height: float32(len(lines)) * lineHeight}, LineHeight: lineHeight}
}

func wrapTextLines(window textMeasurer, value string, style woxui.TextStyle, width float32, maxLines int) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	paragraphs := strings.Split(value, "\n")
	lines := make([]string, 0, len(paragraphs))
	truncated := false
	for paragraphIndex, paragraph := range paragraphs {
		remaining := []rune(paragraph)
		if len(remaining) == 0 {
			lines = append(lines, "")
		}
		for len(remaining) > 0 {
			if maxLines > 0 && len(lines) >= maxLines {
				truncated = true
				break
			}
			fit := fittingRunePrefix(window, remaining, style, width)
			if fit >= len(remaining) {
				lines = append(lines, string(remaining))
				remaining = nil
				continue
			}
			breakAt := fit
			for index := fit - 1; index > 0; index-- {
				if unicode.IsSpace(remaining[index]) {
					hasCJK := false
					for _, candidate := range remaining[index+1 : fit] {
						if unicode.In(candidate, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
							hasCJK = true
							break
						}
					}
					// Mixed CJK text can break between characters; backing up to a distant Latin-space boundary wastes most of the line.
					if !hasCJK {
						breakAt = index
					}
					break
				}
			}
			line := strings.TrimRightFunc(string(remaining[:breakAt]), unicode.IsSpace)
			if line == "" {
				line = string(remaining[:fit])
				breakAt = fit
			}
			lines = append(lines, line)
			remaining = remaining[breakAt:]
			for len(remaining) > 0 && unicode.IsSpace(remaining[0]) {
				remaining = remaining[1:]
			}
		}
		if truncated {
			break
		}
		if maxLines > 0 && len(lines) >= maxLines && paragraphIndex < len(paragraphs)-1 {
			truncated = true
			break
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	if truncated {
		last := []rune(strings.TrimRightFunc(lines[len(lines)-1], unicode.IsSpace))
		for len(last) > 0 {
			metrics, _ := window.MeasureText(string(last)+"…", style)
			if metrics.Size.Width <= width {
				break
			}
			last = last[:len(last)-1]
		}
		lines[len(lines)-1] = string(last) + "…"
	}
	return lines
}

func fittingRunePrefix(window textMeasurer, runes []rune, style woxui.TextStyle, width float32) int {
	if len(runes) == 0 {
		return 0
	}
	if width <= 0 {
		return 1
	}
	low, high := 1, len(runes)
	for low < high {
		mid := low + (high-low+1)/2
		metrics, _ := window.MeasureText(string(runes[:mid]), style)
		if metrics.Size.Width <= width {
			low = mid
		} else {
			high = mid - 1
		}
	}
	return max(1, low)
}

type gesture struct {
	cursor        woxui.PointerCursor
	id            string
	onHover       func(bool)
	onHoverAt     func(bool, woxui.Rect)
	onPressChange func(bool)
	onTap         func()
	onDoubleTap   func()
	onDoubleTapAt func(woxui.Point)
	onTripleTapAt func(woxui.Point)
	onTapAt       func(woxui.Point)
	onTapBounds   func(woxui.Rect)
	onDragStart   func()
	onPanStart    func(woxui.Point)
	onPanUpdate   func(woxui.Point)
	onPanEnd      func()
	onScroll      func(woxui.Point)
	onPointer     func(woxui.PointerEvent) bool
	// onScrollHandled reports whether this gesture consumed the delta so an
	// ancestor scroll view can continue at nested-scroll boundaries.
	onScrollHandled func(woxui.Point) bool
	// onSelectionStart begins a drag-based text selection anchored at the given local point.
	onSelectionStart func(woxui.Point)
	// onSelectionExtend updates the selection focus to the given local point while dragging.
	onSelectionExtend func(woxui.Point)
}

// Gesture adds pointer behavior without changing its child's layout or paint.
type Gesture struct {
	ID        string
	Cursor    woxui.PointerCursor
	Child     Widget
	OnHover   func(bool)
	OnHoverAt func(inside bool, bounds woxui.Rect)
	// OnPressChange reports primary-button press and release without changing tap activation.
	OnPressChange func(pressed bool)
	OnTap         func()
	OnDoubleTap   func()
	OnDoubleTapAt func(position woxui.Point)
	OnTripleTapAt func(position woxui.Point)
	OnTapAt       func(position woxui.Point)
	OnTapBounds   func(bounds woxui.Rect)
	OnDragStart   func()
	OnPanStart    func(position woxui.Point)
	OnPanUpdate   func(position woxui.Point)
	OnPanEnd      func()
	OnScroll      func(delta woxui.Point)
	// OnPointer handles the complete pointer stream in local coordinates.
	OnPointer func(event woxui.PointerEvent) bool
	// OnScrollHandled returns false to pass an unconsumed delta to the nearest
	// ancestor scroll gesture.
	OnScrollHandled func(delta woxui.Point) bool
	// OnSelectionStart begins a drag-based selection (e.g. text drag-select) anchored at the local point.
	// When set, pointer-down on this gesture starts a selection drag instead of a tap, so OnTap/OnTapAt
	// are skipped until the pointer is released without significant movement.
	OnSelectionStart func(position woxui.Point)
	// OnSelectionExtend updates the active selection drag to the given local point.
	OnSelectionExtend func(position woxui.Point)
}

func (w Gesture) layout(ctx context, available constraints) *node {
	child := w.Child.layout(ctx, available)
	target := child
	if child.gesture != nil {
		target = &node{
			bounds:   woxui.Rect{Width: child.bounds.Width, Height: child.bounds.Height},
			children: []*node{child},
		}
	}
	if w.ID != "" {
		target.key = Key(w.ID)
	}
	target.kind = "gesture"
	target.gesture = &gesture{
		id: w.ID, cursor: w.Cursor, onHover: w.OnHover, onHoverAt: w.OnHoverAt, onPressChange: w.OnPressChange, onTap: w.OnTap, onDoubleTap: w.OnDoubleTap,
		onDoubleTapAt: w.OnDoubleTapAt, onTripleTapAt: w.OnTripleTapAt, onTapAt: w.OnTapAt,
		onTapBounds: w.OnTapBounds, onDragStart: w.OnDragStart, onPanStart: w.OnPanStart, onPanUpdate: w.OnPanUpdate, onPanEnd: w.OnPanEnd,
		onScroll: w.OnScroll, onScrollHandled: w.OnScrollHandled, onPointer: w.OnPointer,
		onSelectionStart: w.OnSelectionStart, onSelectionExtend: w.OnSelectionExtend,
	}
	return target
}

// Painter is the escape hatch for small visuals not worth a dedicated widget.
type Painter struct {
	Width  float32
	Height float32
	Paint  func(displayList *woxui.DisplayList, bounds woxui.Rect)
}

func (w Painter) layout(ctx context, available constraints) *node {
	_ = ctx
	return &node{bounds: woxui.Rect{Width: min(w.Width, available.width), Height: min(w.Height, available.height)}, paint: w.Paint}
}

// CaretPainter paints editor content with the host-managed caret blink phase.
type CaretPainter struct {
	Width  float32
	Height float32
	Active bool
	Paint  func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool)
}

func (w CaretPainter) layout(ctx context, available constraints) *node {
	var paint func(*woxui.DisplayList, woxui.Rect, bool, bool)
	if w.Paint != nil {
		paint = func(displayList *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
			w.Paint(displayList, bounds, focused, caretVisible && focused)
		}
	}
	return &node{
		bounds:     woxui.Rect{Width: min(w.Width, available.width), Height: min(w.Height, available.height)},
		caret:      w.Active,
		caretPaint: paint,
	}
}
