package widget

import (
	"strings"
	"unicode"

	woxui "wox/ui/runtime"
)

// Widget produces one laid-out render node for the current constraints.
type Widget interface {
	layout(context, constraints) *node
}

type textMeasurer interface {
	MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error)
}

type context struct {
	window textMeasurer
}

type constraints struct {
	width  float32
	height float32
}

type node struct {
	id       woxui.AccessibilityNodeID
	key      Key
	kind     string
	parent   *node
	bounds   woxui.Rect
	paint    func(*woxui.DisplayList, woxui.Rect)
	gesture  *gesture
	focus    *focusBehavior
	scope    *focusScopeBehavior
	semantic *semanticBehavior
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
		textLayout = layoutTextBlock(ctx.window, w.Value, w.Style, width, maxLines, lineHeight)
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
	target.gesture = &gesture{id: w.ID, onHover: w.OnHover, onHoverAt: w.OnHoverAt, onTap: w.OnTap, onDoubleTap: w.OnDoubleTap, onTapAt: w.OnTapAt, onDragStart: w.OnDragStart, onScroll: w.OnScroll}
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
