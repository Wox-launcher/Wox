package widget

import (
	"fmt"
	"math"
	"slices"
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

// PaintStateless lets immediate-mode surfaces reuse layout primitives without retaining an input or focus tree.
func PaintStateless(window HostServices, widget Widget, displayList *woxui.DisplayList, bounds woxui.Rect) {
	root := widget.layout(context{window: window}, constraints{width: bounds.Width, height: bounds.Height})
	root.place(bounds.X, bounds.Y)
	root.draw(displayList, 0, 0, false, false, false, nil)
}

type textMeasurer interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
}

type frameWorkCounters struct {
	layoutVisits    int
	identityVisits  int
	paintVisits     int
	a11yVisits      int
	boundaryBuilds  int
	boundaryReuses  int
	identityUpserts int
	a11yUpserts     int
}

func (w frameWorkCounters) metrics(textDraws, imageDraws int) woxui.FrameWorkMetrics {
	return woxui.FrameWorkMetrics{
		LayoutVisits:    w.layoutVisits,
		IdentityVisits:  w.identityVisits,
		PaintVisits:     w.paintVisits,
		A11yVisits:      w.a11yVisits,
		BoundaryBuilds:  w.boundaryBuilds,
		BoundaryReuses:  w.boundaryReuses,
		IdentityUpserts: w.identityUpserts,
		A11yUpserts:     w.a11yUpserts,
		TextDraws:       textDraws,
		ImageDraws:      imageDraws,
	}
}

type context struct {
	window    textMeasurer
	animation animationFrame
	dynamic   *dynamicUse
	damage    *frameDamageTracker
	debug     *repaintDebugFrame
	elements  *elementTree
	element   *stateElement
	work      *frameWorkCounters
	scroll    *scrollLayoutEnv
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
	// width and height are maximum extents; minWidth and minHeight make an axis tight when equal to its maximum.
	minWidth  float32
	width     float32
	minHeight float32
	height    float32
}

func (c constraints) constrainWidth(width float32) float32 {
	return min(max(width, c.minWidth), c.width)
}

func (c constraints) constrainHeight(height float32) float32 {
	return min(max(height, c.minHeight), c.height)
}

func (c constraints) loose() constraints {
	c.minWidth = 0
	c.minHeight = 0
	return c
}

func (c constraints) tightWidth(width float32) constraints {
	width = c.constrainWidth(width)
	c.minWidth = width
	c.width = width
	return c
}

func (c constraints) tightHeight(height float32) constraints {
	height = c.constrainHeight(height)
	c.minHeight = height
	c.height = height
	return c
}

func (c constraints) constrainNode(result *node) *node {
	if result == nil {
		return &node{bounds: woxui.Rect{Width: c.minWidth, Height: c.minHeight}}
	}
	result.bounds.Width = c.constrainWidth(result.bounds.Width)
	result.bounds.Height = c.constrainHeight(result.bounds.Height)
	return result
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
}

func (n *node) hitTest(point woxui.Point) *node {
	return n.hitTestAt(point, woxui.Point{})
}

// hitTestAt hit-tests against window-space bounds accumulated from origin.
func (n *node) hitTestAt(point woxui.Point, origin woxui.Point) *node {
	bounds := offsetRect(n.bounds, origin)
	if !containsPoint(bounds, point) {
		return nil
	}
	childOrigin := woxui.Point{X: bounds.X, Y: bounds.Y}
	for index := len(n.children) - 1; index >= 0; index-- {
		if hit := n.children[index].hitTestAt(point, childOrigin); hit != nil {
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
	return n.hitTestScrollAt(point, woxui.Point{})
}

// hitTestScrollAt finds the deepest scrollable gesture using accumulated origin.
func (n *node) hitTestScrollAt(point woxui.Point, origin woxui.Point) *node {
	bounds := offsetRect(n.bounds, origin)
	if !containsPoint(bounds, point) {
		return nil
	}
	childOrigin := woxui.Point{X: bounds.X, Y: bounds.Y}
	for index := len(n.children) - 1; index >= 0; index-- {
		if hit := n.children[index].hitTestScrollAt(point, childOrigin); hit != nil {
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
	Width             float32
	Height            float32
	Padding           Insets
	Color             woxui.Color
	BorderColor       woxui.Color
	BorderWidth       float32
	LeftBorderColor   woxui.Color
	LeftBorderWidth   float32
	RightBorderColor  woxui.Color
	RightBorderWidth  float32
	BottomBorderColor woxui.Color
	BottomBorderWidth float32
	Radius            float32
	Child             Widget
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
		width = available.constrainWidth(w.Width)
	}
	height := available.height
	if w.Height > 0 {
		height = available.constrainHeight(w.Height)
	}
	width = available.constrainWidth(width)
	height = available.constrainHeight(height)
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
		maxWidth = max(available.minWidth, min(maxWidth, w.MaxWidth))
	}
	maxHeight := available.height
	if w.MaxHeight > 0 {
		maxHeight = max(available.minHeight, min(maxHeight, w.MaxHeight))
	}
	minWidth := min(max(available.minWidth, w.MinWidth), maxWidth)
	minHeight := min(max(available.minHeight, w.MinHeight), maxHeight)
	if w.FillWidth && maxWidth < math.MaxFloat32 {
		minWidth = maxWidth
	}
	if w.FillHeight && maxHeight < math.MaxFloat32 {
		minHeight = maxHeight
	}
	childConstraints := constraints{minWidth: minWidth, width: maxWidth, minHeight: minHeight, height: maxHeight}
	if w.Child == nil {
		return &node{bounds: woxui.Rect{Width: minWidth, Height: minHeight}}
	}
	return childConstraints.constrainNode(w.Child.layout(ctx, childConstraints))
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
		contentWidth = available.constrainWidth(w.Width)
	}
	contentHeight := available.height
	if w.Height > 0 {
		contentHeight = available.constrainHeight(w.Height)
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
	width = available.constrainWidth(width)
	height = available.constrainHeight(height)
	result := &node{bounds: woxui.Rect{Width: width, Height: height}}
	if w.Color.A != 0 || (w.BorderColor.A != 0 && w.BorderWidth > 0) || containerHasEdgeBorder(w) {
		result.paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if w.Color.A != 0 {
				displayList.FillRoundedRect(bounds, w.Radius, w.Color)
			}
			if w.BorderColor.A != 0 && w.BorderWidth > 0 {
				displayList.StrokeRoundedRect(bounds, w.Radius, w.BorderWidth, w.BorderColor)
			}
			paintContainerEdgeBorders(displayList, bounds, w)
		}
	}
	if child != nil {
		result.children = []*node{child}
	}
	return result
}

func containerHasEdgeBorder(w Container) bool {
	return (w.LeftBorderColor.A != 0 && w.LeftBorderWidth > 0) ||
		(w.RightBorderColor.A != 0 && w.RightBorderWidth > 0) ||
		(w.BottomBorderColor.A != 0 && w.BottomBorderWidth > 0)
}

// paintContainerEdgeBorders draws per-side strokes inside the box. Horizontal
// edges own shared corners so adjacent 1px sides do not darken the same pixel.
func paintContainerEdgeBorders(displayList *woxui.DisplayList, bounds woxui.Rect, w Container) {
	bottom := float32(0)
	if w.BottomBorderColor.A != 0 && w.BottomBorderWidth > 0 {
		bottom = min(w.BottomBorderWidth, bounds.Height)
		displayList.FillRect(woxui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height - bottom, Width: bounds.Width, Height: bottom}, w.BottomBorderColor)
	}
	innerHeight := max(float32(0), bounds.Height-bottom)
	if innerHeight <= 0 {
		return
	}
	if w.LeftBorderColor.A != 0 && w.LeftBorderWidth > 0 {
		displayList.FillRect(woxui.Rect{X: bounds.X, Y: bounds.Y, Width: min(w.LeftBorderWidth, bounds.Width), Height: innerHeight}, w.LeftBorderColor)
	}
	if w.RightBorderColor.A != 0 && w.RightBorderWidth > 0 {
		width := min(w.RightBorderWidth, bounds.Width)
		displayList.FillRect(woxui.Rect{X: bounds.X + bounds.Width - width, Y: bounds.Y, Width: width, Height: innerHeight}, w.RightBorderColor)
	}
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

// Flexible gives its child a weighted maximum share of the remaining Flex main-axis extent without forcing it to fill that share.
type Flexible struct {
	Flex  float32
	Child Widget
}

func (w Flexible) layout(ctx context, available constraints) *node {
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
	Key           Key
	ID            string
	Width         float32
	Height        float32
	MaxHeight     float32
	ContentWidth  float32
	ContentHeight float32
	Horizontal    bool
	// MapVerticalWheel lets a standalone horizontal strip consume an ordinary
	// mouse wheel. Nested strips must leave this unset so vertical wheels still
	// reach the outer list instead of sliding the inner content sideways.
	MapVerticalWheel  bool
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
	width := available.constrainWidth(min(w.Width, available.width))
	height := available.constrainHeight(min(w.Height, available.height))
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
		width = available.constrainWidth(w.Width)
	}
	height := available.height
	if w.Height > 0 {
		height = available.constrainHeight(w.Height)
	} else if w.MaxHeight > 0 {
		height = available.constrainHeight(w.MaxHeight)
	}
	if w.Horizontal {
		contentWidth := max(width, w.ContentWidth)
		scroll := &scrollLayoutEnv{offset: max(float32(0), w.Offset), viewport: width}
		var child *node
		if w.Child != nil {
			childCtx := ctx
			childCtx.scroll = scroll
			child = w.Child.layout(childCtx, constraints{width: math.MaxFloat32, height: height})
			contentWidth = max(contentWidth, child.bounds.Width)
		}
		offset := min(max(float32(0), scroll.offset), max(float32(0), contentWidth-width))
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
	scroll := &scrollLayoutEnv{offset: max(float32(0), w.Offset), viewport: height}
	var child *node
	if w.Child != nil {
		childCtx := ctx
		childCtx.scroll = scroll
		child = w.Child.layout(childCtx, constraints{width: width, height: math.MaxFloat32})
		// Flex children can legitimately exceed a caller's estimated extent. The measured height must remain scrollable.
		contentHeight = max(contentHeight, child.bounds.Height)
	}
	if w.Height <= 0 && w.MaxHeight > 0 && child != nil {
		height = max(float32(1), min(height, child.bounds.Height))
		contentHeight = max(height, max(w.ContentHeight, child.bounds.Height))
	}
	offset := min(max(float32(0), scroll.offset), max(float32(0), contentHeight-height))
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
	x, y, ok := findNodeOffset(root, key, 0, 0)
	if !ok {
		return nil
	}
	target := findNodeByKey(root, key)
	if target == nil {
		return nil
	}
	if horizontal {
		return &ScrollRange{Start: x, End: x + target.bounds.Width}
	}
	return &ScrollRange{Start: y, End: y + target.bounds.Height}
}

// findNodeOffset returns the keyed descendant's offset relative to root's parent origin.
func findNodeOffset(root *node, key Key, originX, originY float32) (float32, float32, bool) {
	if root == nil {
		return 0, 0, false
	}
	x := originX + root.bounds.X
	y := originY + root.bounds.Y
	if root.key == key {
		return x, y, true
	}
	for _, child := range root.children {
		if childX, childY, found := findNodeOffset(child, key, x, y); found {
			return childX, childY, true
		}
	}
	return 0, 0, false
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
	} else {
		width = available.constrainWidth(width)
	}
	height := w.Height
	if height <= 0 {
		height = available.height
	} else {
		height = available.constrainHeight(height)
	}
	width = available.constrainWidth(width)
	height = available.constrainHeight(height)
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
		childConstraints := constraints{width: childWidth, height: childHeight}
		if positioned.StretchWidth {
			childConstraints = childConstraints.tightWidth(childWidth)
		}
		if positioned.StretchHeight {
			childConstraints = childConstraints.tightHeight(childHeight)
		}
		child := childConstraints.constrainNode(positioned.Child.layout(ctx, childConstraints))
		x := positioned.Left
		y := positioned.Top
		if !positioned.StretchWidth && positioned.AnchorRight {
			x = max(float32(0), width-positioned.Right-child.bounds.Width)
		}
		if !positioned.StretchHeight && positioned.AnchorBottom {
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
			switch child.(type) {
			case Expanded, Flexible:
				enhanced = true
			}
			if enhanced {
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
		flexed bool
		tight  bool
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
			child.flexed = true
			child.tight = true
		}
		if flexible, ok := childWidget.(Flexible); ok {
			child.widget = flexible.Child
			child.flex = flexible.Flex
			child.flexed = true
		}
		if child.flexed {
			if child.flex <= 0 {
				child.flex = 1
			}
		}
		if child.widget != nil {
			if child.flexed {
				totalFlex += child.flex
			}
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
		childAvailable := available.loose()
		if w.CrossAxisAlignment == CrossAxisStretch {
			if w.Axis == Horizontal && available.height < math.MaxFloat32 {
				childAvailable = childAvailable.tightHeight(available.height)
			} else if w.Axis == Vertical && available.width < math.MaxFloat32 {
				childAvailable = childAvailable.tightWidth(available.width)
			}
		}
		children[index].node = childAvailable.constrainNode(children[index].widget.layout(ctx, childAvailable))
		fixedExtent += flexMainExtent(children[index].node, w.Axis)
	}
	remaining := max(float32(0), mainAvailable-fixedExtent-totalGap)
	for index := range children {
		if children[index].flex <= 0 || !mainBounded {
			continue
		}
		share := remaining * children[index].flex / totalFlex
		childAvailable := available.loose()
		if w.Axis == Horizontal {
			childAvailable.width = share
			if children[index].tight {
				childAvailable = childAvailable.tightWidth(share)
			}
		} else {
			childAvailable.height = share
			if children[index].tight {
				childAvailable = childAvailable.tightHeight(share)
			}
		}
		if w.CrossAxisAlignment == CrossAxisStretch {
			if w.Axis == Horizontal && available.height < math.MaxFloat32 {
				childAvailable = childAvailable.tightHeight(available.height)
			} else if w.Axis == Vertical && available.width < math.MaxFloat32 {
				childAvailable = childAvailable.tightWidth(available.width)
			}
		}
		children[index].node = childAvailable.constrainNode(children[index].widget.layout(ctx, childAvailable))
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
	if mainBounded && totalFlex > 0 {
		mainExtent = mainAvailable
	}
	freeExtent := max(float32(0), mainAvailable-contentExtent)
	startOffset := float32(0)
	gap := w.Gap
	switch {
	case !mainBounded:
	case w.MainAxisAlignment == MainAxisCenter:
		// The aligned children occupy the allocated extent; shrink-wrapping here leaves them outside pointer hit-testing bounds.
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
	recordLayoutOverflow(ctx, "flex", w.Axis, contentExtent, mainAvailable)
	return available.constrainNode(result)
}

// layoutSequential preserves the original shrink-wrapped path for Flex trees that do not allocate free space.
func (w Flex) layoutSequential(ctx context, available constraints) *node {
	result := &node{}
	var cursor float32
	for _, childWidget := range w.Children {
		if childWidget == nil {
			continue
		}
		childAvailable := available.loose()
		child := childAvailable.constrainNode(childWidget.layout(ctx, childAvailable))
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
	mainAvailable := available.width
	contentExtent := result.bounds.Width
	if w.Axis == Vertical {
		mainAvailable = available.height
		contentExtent = result.bounds.Height
	}
	recordLayoutOverflow(ctx, "flex", w.Axis, contentExtent, mainAvailable)
	return available.constrainNode(result)
}

// overflowLocationDepth keeps an overflow location identifying but short. The
// innermost ancestors name the control; the outer shell adds no information.
const overflowLocationDepth = 6

// recordLayoutOverflow exposes clipped content while repaint diagnostics are enabled.
func recordLayoutOverflow(ctx context, layout string, axis Axis, contentExtent, availableExtent float32) {
	if availableExtent >= math.MaxFloat32 || contentExtent <= availableExtent+0.5 || ctx.debug == nil || ctx.elements == nil {
		return
	}
	axisLabel := "horizontal"
	if axis == Vertical {
		axisLabel = "vertical"
	}
	// Both extents are reported because the overflow amount alone hides the scale it
	// came from. A window collapsed to zero during a hide or resize transition and a
	// genuinely oversized child produce the same delta but need opposite fixes.
	ctx.elements.diagnostics = append(ctx.elements.diagnostics, fmt.Sprintf("%s %s overflowed by %.1f logical pixels (content %.1f, available %.1f) in %s", axisLabel, layout, contentExtent-availableExtent, contentExtent, availableExtent, overflowLocation(ctx.element)))
}

// overflowLocation names the widget subtree that produced an overflow. Without it
// a diagnostic reports only the layout kind, which cannot be traced to a control.
func overflowLocation(element *stateElement) string {
	segments := make([]string, 0, overflowLocationDepth)
	for current := element; current != nil && len(segments) < overflowLocationDepth; current = current.parent {
		if current.widgetType == nil {
			continue
		}
		segment := current.widgetType.Name()
		if segment == "" {
			segment = current.widgetType.String()
		}
		if current.key != "" {
			segment += "{" + string(current.key) + "}"
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		// The synthetic tree root carries no widget type, so a layout composed above
		// every Stateful widget collects no segments. Naming it separates a top-level
		// window layout from a detached tree, which used to report the same nothing.
		if element != nil {
			return "window root"
		}
		return "unknown widget"
	}
	slices.Reverse(segments)
	return strings.Join(segments, "/")
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
		width = available.constrainWidth(w.Width)
	}
	width = available.constrainWidth(width)
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
	usedColumns := min(columns, len(w.Children))
	gridWidth := float32(usedColumns) * cellWidth
	if usedColumns > 1 {
		gridWidth += float32(usedColumns-1) * w.ColumnGap
	}
	recordLayoutOverflow(ctx, "grid", Horizontal, gridWidth, available.width)
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
			cellAvailable := available.loose().tightWidth(cellWidth)
			if w.CellHeight > 0 {
				cellAvailable.height = min(w.CellHeight, available.height)
			}
			child := cellAvailable.constrainNode(w.Children[index].layout(ctx, cellAvailable))
			rowHeight = max(rowHeight, child.bounds.Height)
			row = append(row, child)
		}
		for column, child := range row {
			if child == nil {
				continue
			}
			if w.CrossAxisAlignment == CrossAxisStretch {
				// The tallest cell is only known after measuring the row. Re-layout would build stateful cells twice,
				// so equal-height rows stretch the measured cell surface while cell width remains a true tight constraint.
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
	recordLayoutOverflow(ctx, "grid", Vertical, result.bounds.Height, available.height)
	return available.constrainNode(result)
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
		childAvailable := available.loose()
		child := childAvailable.constrainNode(childWidget.layout(ctx, childAvailable))
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
	recordLayoutOverflow(ctx, "wrap", Vertical, result.bounds.Height, available.height)
	return available.constrainNode(result)
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
	// AlignmentY optically places each measured line box inside the authored
	// slot. 0 keeps the native top edge; 0.5 centers CJK metrics that are
	// taller than the slot by clipping extra ascent and descent equally.
	AlignmentY float32
	// ShrinkWrap keeps short text at its measured width while preserving Width as the truncation limit.
	ShrinkWrap bool
	Layout     *TextBlockLayout
}

// TextBlockLayout is the portable line layout used by TextBlock and scroll containers.
type TextBlockLayout struct {
	Lines              []string
	Size               woxui.Size
	LineHeight         float32
	ConstraintWidth    float32
	HasConstraintWidth bool
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
	if w.Source != nil && w.Source.IsAnimated() {
		// FrameAnimation owns the timeline so a result-icon Boundary only
		// rebuilds when the visible GIF frame actually changes.
		return (FrameAnimation{
			Key:    Key(fmt.Sprintf("image-gif-%d", w.Source.ID())),
			Delays: w.Source.FrameDelays(),
			Builder: func(index int) Widget {
				frame := w
				frame.Source = w.Source.Frame(index)
				return frame
			},
		}).layout(ctx, available)
	}
	width := available.constrainWidth(min(w.Width, available.width))
	height := available.constrainHeight(min(w.Height, available.height))
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			imageBounds := fittedImageBounds(w.Source, bounds, w.Fit)
			if w.Radius > 0 {
				// Corner radius belongs to the widget box. Cover's overflowing
				// destination would round the hidden overflow corners and leave
				// the visible window corners filled with wallpaper.
				displayList.DrawRotatedRoundedImage(w.Source, bounds, 0, w.Radius)
				return
			}
			if w.Fit == ImageFitCover {
				displayList.PushClipRect(bounds)
				defer displayList.PopClipRect()
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
	width := available.constrainWidth(metrics.Size.Width)
	height := available.constrainHeight(metrics.Size.Height)
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
		width = available.constrainWidth(w.Width)
	}
	if w.ShrinkWrap {
		metrics, _ := ctx.window.MeasureText(w.Value, w.Style)
		width = available.constrainWidth(min(width, metrics.Size.Width))
	}
	width = available.constrainWidth(width)
	heightLimit := available.height
	if w.Height > 0 {
		heightLimit = available.constrainHeight(w.Height)
	}
	metrics, _ := ctx.window.MeasureText("Mg", w.Style)
	lineHeight := w.LineHeight
	if lineHeight <= 0 {
		lineHeight = max(metrics.Size.Height, w.Style.Size*1.35)
	}
	maxLines := w.MaxLines
	// Scroll content is measured with unbounded height (MaxFloat32). Converting
	// that sentinel to a line count overflows int and collapses wrapping to one
	// ellipsized line, so only finite boxes may reduce MaxLines.
	if heightLimit > 0 && heightLimit < math.MaxFloat32 {
		visibleLines := max(1, int(heightLimit/lineHeight))
		if maxLines <= 0 || visibleLines < maxLines {
			maxLines = visibleLines
		}
	}
	textLayout := TextBlockLayout{}
	if w.Layout != nil {
		textLayout = *w.Layout
		if textLayout.HasConstraintWidth && float32(math.Abs(float64(textLayout.ConstraintWidth-width))) > 0.5 {
			if ctx.debug != nil && ctx.elements != nil {
				ctx.elements.diagnostics = append(ctx.elements.diagnostics, fmt.Sprintf("text layout measured at width %.1f was reflowed for width %.1f", textLayout.ConstraintWidth, width))
			}
			textLayout = layoutTextBlock(ctx.window, w.Value, w.Style, width, maxLines, lineHeight)
		}
	} else {
		textLayout = layoutTextBlock(ctx.window, w.Value, w.Style, width, maxLines, lineHeight)
	}
	height := min(heightLimit, textLayout.Size.Height)
	if w.Height > 0 {
		height = heightLimit
	}
	height = available.constrainHeight(height)
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
				remaining := bounds.Y + bounds.Height - y
				if remaining <= 0 {
					break
				}
				// CJK fonts can report a taller line box than a single-line slot.
				// Clip the draw rect instead of skipping the only visible line.
				lineBounds := woxui.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: min(lineHeight, remaining)}
				if w.Centered {
					metrics, _ := window.MeasureText(line, w.Style)
					lineWidth := min(metrics.Size.Width, bounds.Width)
					lineBounds = woxui.Rect{X: bounds.X + (bounds.Width-lineWidth)/2, Y: y, Width: lineWidth, Height: lineBounds.Height}
				}
				drawBounds := lineBounds
				if w.AlignmentY > 0 && line != "" {
					if metrics, err := window.MeasureText(line, w.Style); err == nil && metrics.Size.Height > 0 {
						drawBounds = alignedTextLineBounds(lineBounds, metrics.Size.Height, w.AlignmentY)
					}
				}
				if drawBounds.Y < lineBounds.Y || drawBounds.Y+drawBounds.Height > lineBounds.Y+lineBounds.Height {
					displayList.PushClipRect(lineBounds)
					displayList.DrawText(line, drawBounds, w.Style, w.Color)
					displayList.PopClipRect()
					continue
				}
				displayList.DrawText(line, drawBounds, w.Style, w.Color)
			}
		},
	}
}

// alignedTextLineBounds places a measured line box inside an authored slot.
func alignedTextLineBounds(bounds woxui.Rect, measuredHeight, alignment float32) woxui.Rect {
	alignment = min(max(float32(0), alignment), float32(1))
	if measuredHeight <= 0 || alignment == 0 {
		return bounds
	}
	bounds.Y += (bounds.Height - measuredHeight) * alignment
	bounds.Height = measuredHeight
	return bounds
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
	return TextBlockLayout{Lines: lines, Size: woxui.Size{Width: width, Height: float32(len(lines)) * lineHeight}, LineHeight: lineHeight, ConstraintWidth: width, HasConstraintWidth: true}
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
	cursor             woxui.PointerCursor
	cursorAt           func(woxui.Point) woxui.PointerCursor
	id                 string
	onHover            func(bool)
	onHoverAt          func(bool, woxui.Rect)
	coverHover         bool
	onPressChange      func(bool)
	onTap              func()
	onSecondaryTapDown func(position woxui.Point)
	onDoubleTap        func()
	onDoubleTapAt      func(woxui.Point)
	onTripleTapAt      func(woxui.Point)
	onTapAt            func(woxui.Point)
	onTapBounds        func(woxui.Rect)
	onDragStart        func()
	onPanStart         func(woxui.Point)
	onPanUpdate        func(woxui.Point)
	onPanEnd           func()
	onScroll           func(woxui.Point)
	onPointer          func(woxui.PointerEvent) bool
	// onScrollHandled reports whether this gesture consumed the delta so an
	// ancestor scroll view can continue at nested-scroll boundaries.
	onScrollHandled func(woxui.Point) bool
	// onSelectionStart begins a drag-based text selection anchored at the given local point.
	onSelectionStart func(position woxui.Point, modifiers woxui.KeyModifiers)
	// onSelectionExtend updates the selection focus to the given local point while dragging.
	onSelectionExtend func(position woxui.Point)
	// onSelectionEnd reports that an active drag selection finished (pointer up/cancel).
	onSelectionEnd func()
}

// Gesture adds pointer behavior without changing its child's layout or paint.
type Gesture struct {
	ID        string
	Cursor    woxui.PointerCursor
	CursorAt  func(position woxui.Point) woxui.PointerCursor
	Child     Widget
	OnHover   func(bool)
	OnHoverAt func(inside bool, bounds woxui.Rect)
	// CoverHover also reports OnHover while a descendant owns hit-testing, so a
	// scroll surface can reveal its thumb when the pointer is over child cards.
	CoverHover bool
	// OnPressChange reports primary-button press and release without changing tap activation.
	OnPressChange func(pressed bool)
	OnTap         func()
	// OnSecondaryTapDown reports a secondary-button press with window/client coordinates.
	OnSecondaryTapDown func(position woxui.Point)
	OnDoubleTap        func()
	OnDoubleTapAt      func(position woxui.Point)
	OnTripleTapAt      func(position woxui.Point)
	OnTapAt            func(position woxui.Point)
	OnTapBounds        func(bounds woxui.Rect)
	OnDragStart        func()
	OnPanStart         func(position woxui.Point)
	OnPanUpdate        func(position woxui.Point)
	OnPanEnd           func()
	OnScroll           func(delta woxui.Point)
	// OnPointer handles the complete pointer stream in local coordinates.
	OnPointer func(event woxui.PointerEvent) bool
	// OnScrollHandled returns false to pass an unconsumed delta to the nearest
	// ancestor scroll gesture.
	OnScrollHandled func(delta woxui.Point) bool
	// OnSelectionStart begins a drag-based selection (e.g. text drag-select) anchored at the local point.
	// When set, pointer-down on this gesture starts a selection drag instead of a tap, so OnTap/OnTapAt
	// are skipped until the pointer is released without significant movement.
	// Modifiers let callers implement Shift+click selection extension.
	OnSelectionStart func(position woxui.Point, modifiers woxui.KeyModifiers)
	// OnSelectionExtend updates the active selection drag to the given local point.
	OnSelectionExtend func(position woxui.Point)
	// OnSelectionEnd reports that an active drag selection finished (pointer up/cancel).
	OnSelectionEnd func()
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
		id: w.ID, cursor: w.Cursor, cursorAt: w.CursorAt, onHover: w.OnHover, onHoverAt: w.OnHoverAt, coverHover: w.CoverHover, onPressChange: w.OnPressChange, onTap: w.OnTap, onSecondaryTapDown: w.OnSecondaryTapDown, onDoubleTap: w.OnDoubleTap,
		onDoubleTapAt: w.OnDoubleTapAt, onTripleTapAt: w.OnTripleTapAt, onTapAt: w.OnTapAt,
		onTapBounds: w.OnTapBounds, onDragStart: w.OnDragStart, onPanStart: w.OnPanStart, onPanUpdate: w.OnPanUpdate, onPanEnd: w.OnPanEnd,
		onScroll: w.OnScroll, onScrollHandled: w.OnScrollHandled, onPointer: w.OnPointer,
		onSelectionStart: w.OnSelectionStart, onSelectionExtend: w.OnSelectionExtend, onSelectionEnd: w.OnSelectionEnd,
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
	return &node{bounds: woxui.Rect{Width: available.constrainWidth(w.Width), Height: available.constrainHeight(w.Height)}, paint: w.Paint}
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
		bounds:     woxui.Rect{Width: available.constrainWidth(w.Width), Height: available.constrainHeight(w.Height)},
		caret:      w.Active,
		caretPaint: paint,
	}
}
