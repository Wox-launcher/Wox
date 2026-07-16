package widget

import (
	"strings"
	"time"
	"unicode"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

// Widget produces one laid-out render node for the current constraints.
type Widget interface {
	layout(context, constraints) *node
}

type context struct {
	window *woxui.Window
}

type constraints struct {
	width  float32
	height float32
}

type node struct {
	bounds   woxui.Rect
	paint    func(*woxui.DisplayList, woxui.Rect)
	gesture  *gesture
	clip     bool
	children []*node
}

func (n *node) place(x, y float32) {
	n.bounds.X += x
	n.bounds.Y += y
	for _, child := range n.children {
		child.place(x, y)
	}
}

func (n *node) draw(displayList *woxui.DisplayList) {
	if n.paint != nil {
		n.paint(displayList, n.bounds)
	}
	if n.clip {
		displayList.PushClipRect(n.bounds)
	}
	for _, child := range n.children {
		child.draw(displayList)
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
	if n.gesture != nil && n.gesture.onScroll != nil {
		return n
	}
	return nil
}

// Host rebuilds, lays out, paints, and dispatches pointer events for one widget tree.
type Host struct {
	window    *woxui.Window
	build     func(frame woxui.FrameInfo) Widget
	root      *node
	hovered   *node
	pressed   *node
	pressedAt woxui.Point
	dragging  bool
	lastTapID string
	lastTapAt time.Time
}

// NewHost creates a host whose builder runs once per invalidated frame.
func NewHost(build func(frame woxui.FrameInfo) Widget) *Host {
	return &Host{build: build}
}

// Attach connects platform services used during layout and invalidation.
func (h *Host) Attach(window *woxui.Window) {
	h.window = window
}

// Frame lays out and paints the current tree.
func (h *Host) Frame(displayList *woxui.DisplayList, frame woxui.FrameInfo) {
	if h.window == nil || h.build == nil {
		return
	}
	root := h.build(frame)
	if root == nil {
		return
	}
	displayList.Clear(woxui.Color{})
	h.root = root.layout(context{window: h.window}, constraints{width: frame.Size.Width, height: frame.Size.Height})
	h.root.draw(displayList)
}

// Pointer dispatches hover, tap, and scroll to the deepest gesture node.
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
		sameTarget := target == h.hovered || (target != nil && h.hovered != nil && target.gesture.id != "" && target.gesture.id == h.hovered.gesture.id)
		if sameTarget {
			h.hovered = target
		} else {
			if h.hovered != nil && h.hovered.gesture.onHover != nil {
				h.hovered.gesture.onHover(false)
			}
			if h.hovered != nil && h.hovered.gesture.onHoverAt != nil {
				h.hovered.gesture.onHoverAt(false, h.hovered.bounds)
			}
			h.hovered = target
			if h.hovered != nil && h.hovered.gesture.onHover != nil {
				h.hovered.gesture.onHover(true)
			}
			if h.hovered != nil && h.hovered.gesture.onHoverAt != nil {
				h.hovered.gesture.onHoverAt(true, h.hovered.bounds)
			}
			h.invalidate()
		}
	}
	if event.Kind == woxui.PointerDown && event.Button == woxui.PointerButtonPrimary {
		h.pressed = target
		h.pressedAt = event.Position
		h.dragging = false
	}
	if event.Kind == woxui.PointerMove && h.pressed != nil && h.pressed.gesture.onDragStart != nil && !h.dragging {
		deltaX := event.Position.X - h.pressedAt.X
		deltaY := event.Position.Y - h.pressedAt.Y
		if deltaX*deltaX+deltaY*deltaY >= 9 {
			dragTarget := h.pressed
			h.pressed = nil
			h.dragging = true
			dragTarget.gesture.onDragStart()
		}
	}
	if event.Kind == woxui.PointerUp && event.Button == woxui.PointerButtonPrimary {
		if h.dragging {
			h.dragging = false
			h.pressed = nil
			return
		}
		if target != nil && target == h.pressed {
			now := time.Now()
			doubleTap := target.gesture.onDoubleTap != nil && target.gesture.id != "" && target.gesture.id == h.lastTapID && now.Sub(h.lastTapAt) <= 200*time.Millisecond
			if doubleTap {
				target.gesture.onDoubleTap()
				h.lastTapID = ""
				h.lastTapAt = time.Time{}
			} else if target.gesture.onTap != nil {
				target.gesture.onTap()
				if target.gesture.onDoubleTap != nil && target.gesture.id != "" {
					h.lastTapID = target.gesture.id
					h.lastTapAt = now
				}
			}
			if target.gesture.onTapAt != nil {
				target.gesture.onTapAt(woxui.Point{X: event.Position.X - target.bounds.X, Y: event.Position.Y - target.bounds.Y})
			}
			h.invalidate()
		}
		h.pressed = nil
	}
}

func (h *Host) invalidate() {
	if h.window != nil {
		_ = h.window.Invalidate()
	}
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
	Width   float32
	Height  float32
	Padding Insets
	Color   woxui.Color
	Radius  float32
	Child   Widget
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
	if w.Color.A != 0 {
		result.paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRoundedRect(bounds, w.Radius, w.Color)
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

// Flex lays children out sequentially with a fixed gap.
type Flex struct {
	Axis     Axis
	Gap      float32
	Children []Widget
}

// StackChild positions one child relative to its stack's top-left corner.
type StackChild struct {
	Left  float32
	Top   float32
	Child Widget
}

// Stack overlays children in declaration order; later children receive pointer events first.
type Stack struct {
	Width    float32
	Height   float32
	Children []StackChild
}

// ScrollView clips a larger child to a fixed viewport and translates it by Offset.
type ScrollView struct {
	Width         float32
	Height        float32
	ContentHeight float32
	Offset        float32
	Child         Widget
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
	width := available.width
	if w.Width > 0 {
		width = min(w.Width, available.width)
	}
	height := available.height
	if w.Height > 0 {
		height = min(w.Height, available.height)
	}
	contentHeight := max(height, w.ContentHeight)
	offset := min(max(float32(0), w.Offset), max(float32(0), contentHeight-height))
	result := &node{bounds: woxui.Rect{Width: width, Height: height}, clip: true}
	if w.Child != nil {
		child := w.Child.layout(ctx, constraints{width: width, height: contentHeight})
		child.place(0, -offset)
		result.children = []*node{child}
	}
	return result
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
		child := positioned.Child.layout(ctx, constraints{width: max(0, width-positioned.Left), height: max(0, height-positioned.Top)})
		child.place(positioned.Left, positioned.Top)
		result.children = append(result.children, child)
	}
	return result
}

func (w Flex) layout(ctx context, available constraints) *node {
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
	return result
}

// Wrap lays children horizontally and starts a new run when width is exhausted.
type Wrap struct {
	Gap      float32
	RunGap   float32
	Children []Widget
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
	Value string
	Style woxui.TextStyle
	Color woxui.Color
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
	Layout     *TextBlockLayout
}

// TextBlockLayout is the portable line layout used by TextBlock and scroll containers.
type TextBlockLayout struct {
	Lines      []string
	Size       woxui.Size
	LineHeight float32
}

// Image paints a raster resource into a fixed logical rectangle.
type Image struct {
	Source *woxui.Image
	Width  float32
	Height float32
}

func (w Image) layout(ctx context, available constraints) *node {
	_ = ctx
	width := min(w.Width, available.width)
	height := min(w.Height, available.height)
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.DrawImage(w.Source, bounds)
		},
	}
}

func (w Text) layout(ctx context, available constraints) *node {
	metrics, _ := ctx.window.MeasureText(w.Value, w.Style)
	width := min(metrics.Size.Width, available.width)
	height := min(metrics.Size.Height, available.height)
	return &node{
		bounds: woxui.Rect{Width: width, Height: height},
		paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.DrawText(w.Value, bounds, w.Style, w.Color)
		},
	}
}

func (w TextBlock) layout(ctx context, available constraints) *node {
	width := available.width
	if w.Width > 0 {
		width = min(width, w.Width)
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
		textLayout = LayoutTextBlock(ctx.window, w.Value, w.Style, width, maxLines, lineHeight)
	}
	height := min(heightLimit, textLayout.Size.Height)
	if w.Height > 0 {
		height = heightLimit
	}
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
				displayList.DrawText(line, woxui.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: lineHeight}, w.Style, w.Color)
			}
		},
	}
}

// LayoutTextBlock wraps text with the same platform font metrics used during rendering.
func LayoutTextBlock(window *woxui.Window, value string, style woxui.TextStyle, width float32, maxLines int, lineHeight float32) TextBlockLayout {
	if lineHeight <= 0 {
		metrics, _ := window.MeasureText("Mg", style)
		lineHeight = max(metrics.Size.Height, style.Size*1.35)
	}
	lines := wrapTextLines(window, value, style, width, maxLines)
	return TextBlockLayout{Lines: lines, Size: woxui.Size{Width: width, Height: float32(len(lines)) * lineHeight}, LineHeight: lineHeight}
}

func wrapTextLines(window *woxui.Window, value string, style woxui.TextStyle, width float32, maxLines int) []string {
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
					breakAt = index
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

func fittingRunePrefix(window *woxui.Window, runes []rune, style woxui.TextStyle, width float32) int {
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
	id          string
	onHover     func(bool)
	onHoverAt   func(bool, woxui.Rect)
	onTap       func()
	onDoubleTap func()
	onTapAt     func(woxui.Point)
	onDragStart func()
	onScroll    func(woxui.Point)
}

// Gesture adds pointer behavior without changing its child's layout or paint.
type Gesture struct {
	ID          string
	Child       Widget
	OnHover     func(bool)
	OnHoverAt   func(inside bool, bounds woxui.Rect)
	OnTap       func()
	OnDoubleTap func()
	OnTapAt     func(position woxui.Point)
	OnDragStart func()
	OnScroll    func(delta woxui.Point)
}

func (w Gesture) layout(ctx context, available constraints) *node {
	child := w.Child.layout(ctx, available)
	return &node{
		bounds:   woxui.Rect{Width: child.bounds.Width, Height: child.bounds.Height},
		gesture:  &gesture{id: w.ID, onHover: w.OnHover, onHoverAt: w.OnHoverAt, onTap: w.OnTap, onDoubleTap: w.OnDoubleTap, onTapAt: w.OnTapAt, onDragStart: w.OnDragStart, onScroll: w.OnScroll},
		children: []*node{child},
	}
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
