package widget

import (
	"fmt"
	"math"

	woxui "wox/ui/runtime"
)

const (
	// DefaultLazyListOverscan keeps two extra items on each side of the viewport.
	DefaultLazyListOverscan = 2
	// DefaultLazyGridOverscan keeps one extra row on each side of the viewport.
	DefaultLazyGridOverscan = 1
	// NoLazyOverscan disables extra items when a test or caller wants an exact window.
	NoLazyOverscan = -1
)

// LazyList builds only the visible slice of a known-extent vertical list.
// Place it as the child of ScrollView so the parent supplies offset and viewport.
type LazyList struct {
	Key          Key
	Width        float32
	ItemCount    int
	ItemExtent   float32
	ItemExtentAt func(int) float32
	// ExtentRevision is a caller-owned cache key for variable-height items.
	// A non-zero value is trusted; 0 falls back to a full prefix compare.
	ExtentRevision uint64
	ItemBuilder    func(int) Widget
	ItemKey        func(int) Key
	Overscan       int
	Offset         float32
	Viewport       float32
	StickToEnd     bool
}

// LazyGrid builds only the visible rows of a known-extent grid.
type LazyGrid struct {
	Key          Key
	Width        float32
	Columns      int
	ItemCount    int
	ItemExtent   float32
	ItemExtentAt func(int) float32
	// ExtentRevision is a caller-owned cache key for variable-height rows.
	// A non-zero value is trusted; 0 falls back to a full prefix compare.
	ExtentRevision uint64
	ItemBuilder    func(int) Widget
	ItemKey        func(int) Key
	Overscan       int
	Offset         float32
	Viewport       float32
}

type lazyExtentIndex struct {
	revision uint64
	count    int
	fixed    float32
	prefix   []float32
}

type lazyListState struct {
	index       lazyExtentIndex
	lastContent float32
	stuckToEnd  bool
}

type lazyGridState struct {
	index       lazyExtentIndex
	lastContent float32
}

type scrollLayoutEnv struct {
	offset   float32
	viewport float32
}

func (w LazyList) layout(ctx context, available constraints) *node {
	if w.ItemExtentAt != nil || w.StickToEnd {
		key := w.Key
		if key == "" {
			key = "lazy-list"
		}
		return (Stateful{
			Key: key, Type: (*lazyListState)(nil), Widget: w,
			CreateState: func() State { return &lazyListState{} },
		}).layout(ctx, available)
	}
	return w.layoutItems(ctx, available, nil)
}

func (s *lazyListState) InitState(_ StateContext, _ any) {}

func (s *lazyListState) DidUpdateWidget(_ StateContext, _, _ any) {}

func (s *lazyListState) Dispose() {}

func (s *lazyListState) Build(_ StateContext, widget any) Widget {
	list := widget.(LazyList)
	return lazyListContent{list: list, state: s}
}

type lazyListContent struct {
	list  LazyList
	state *lazyListState
}

func (w lazyListContent) layout(ctx context, available constraints) *node {
	return w.list.layoutItems(ctx, available, w.state)
}

func (w LazyGrid) layout(ctx context, available constraints) *node {
	if w.ItemExtentAt != nil {
		key := w.Key
		if key == "" {
			key = "lazy-grid"
		}
		return (Stateful{
			Key: key, Type: (*lazyGridState)(nil), Widget: w,
			CreateState: func() State { return &lazyGridState{} },
		}).layout(ctx, available)
	}
	return w.layoutItems(ctx, available, nil)
}

func (s *lazyGridState) InitState(_ StateContext, _ any) {}

func (s *lazyGridState) DidUpdateWidget(_ StateContext, _, _ any) {}

func (s *lazyGridState) Dispose() {}

func (s *lazyGridState) Build(_ StateContext, widget any) Widget {
	grid := widget.(LazyGrid)
	return lazyGridContent{grid: grid, state: s}
}

type lazyGridContent struct {
	grid  LazyGrid
	state *lazyGridState
}

func (w lazyGridContent) layout(ctx context, available constraints) *node {
	return w.grid.layoutItems(ctx, available, w.state)
}

func (w LazyList) layoutItems(ctx context, available constraints, state *lazyListState) *node {
	width := available.constrainWidth(w.Width)
	if w.Width <= 0 {
		width = available.width
	}
	offset, viewport := resolveLazyScroll(ctx, w.Offset, w.Viewport, available.height)
	overscan := resolveLazyOverscan(w.Overscan, DefaultLazyListOverscan)
	index := lazyIndexFor(stateIndex(state), w.ItemCount, w.ExtentRevision, w.ItemExtent, w.ItemExtentAt)
	if state != nil {
		state.index = index
	}
	content := index.total()
	if state != nil && w.StickToEnd {
		if state.stuckToEnd || (state.lastContent > 0 && offset+viewport >= state.lastContent-1) {
			offset = max(float32(0), content-viewport)
			state.stuckToEnd = content > viewport
		}
		state.lastContent = content
		if ctx.scroll != nil {
			ctx.scroll.offset = offset
		}
	}
	start, end := index.visibleRange(offset, viewport, overscan)
	children := make([]Widget, 0, end-start+2)
	if start > 0 {
		children = append(children, Painter{Width: width, Height: index.offsetOf(start)})
	}
	for item := start; item < end; item++ {
		children = append(children, keyedLazyItem(w.ItemKey, item, w.ItemBuilder))
	}
	if end < w.ItemCount {
		children = append(children, Painter{Width: width, Height: content - index.offsetOf(end)})
	}
	return Semantics{
		Role:  woxui.AccessibilityRoleList,
		Value: fmt.Sprintf("%d-%d/%d", start, max(start, end-1), w.ItemCount),
		Child: Flex{Axis: Vertical, Children: children},
	}.layout(ctx, constraints{width: width, height: math.MaxFloat32}.tightWidth(width))
}

func (w LazyGrid) layoutItems(ctx context, available constraints, state *lazyGridState) *node {
	width := available.constrainWidth(w.Width)
	if w.Width <= 0 {
		width = available.width
	}
	columns := max(1, w.Columns)
	offset, viewport := resolveLazyScroll(ctx, w.Offset, w.Viewport, available.height)
	overscan := resolveLazyOverscan(w.Overscan, DefaultLazyGridOverscan)
	rowCount := (w.ItemCount + columns - 1) / columns
	index := lazyIndexFor(gridStateIndex(state), rowCount, w.ExtentRevision, w.ItemExtent, w.ItemExtentAt)
	if state != nil {
		state.index = index
	}
	content := index.total()
	startRow, endRow := index.visibleRange(offset, viewport, overscan)
	children := make([]Widget, 0, endRow-startRow+2)
	if startRow > 0 {
		children = append(children, Painter{Width: width, Height: index.offsetOf(startRow)})
	}
	for row := startRow; row < endRow; row++ {
		cells := make([]Widget, 0, columns)
		for column := 0; column < columns; column++ {
			item := row*columns + column
			if item >= w.ItemCount {
				cells = append(cells, Painter{Width: width / float32(columns), Height: index.extentOf(row)})
				continue
			}
			cells = append(cells, keyedLazyItem(w.ItemKey, item, w.ItemBuilder))
		}
		children = append(children, Flex{Axis: Horizontal, Children: cells})
	}
	if endRow < rowCount {
		children = append(children, Painter{Width: width, Height: content - index.offsetOf(endRow)})
	}
	startItem := startRow * columns
	endItem := min(w.ItemCount, endRow*columns)
	if endItem > 0 {
		endItem--
	}
	return Semantics{
		Role:  woxui.AccessibilityRoleList,
		Value: fmt.Sprintf("%d-%d/%d", startItem, endItem, w.ItemCount),
		Child: Flex{Axis: Vertical, Children: children},
	}.layout(ctx, constraints{width: width, height: math.MaxFloat32}.tightWidth(width))
}

func keyedLazyItem(itemKey func(int) Key, index int, builder func(int) Widget) Widget {
	var child Widget
	if builder != nil {
		child = builder(index)
	}
	if child == nil {
		child = Painter{}
	}
	key := Key(fmt.Sprintf("lazy-item-%d", index))
	if itemKey != nil {
		if explicit := itemKey(index); explicit != "" {
			key = explicit
		}
	}
	return Keyed{Key: key, Child: child}
}

// resolveLazyOverscan keeps 0 as the product default and treats a negative value as none.
func resolveLazyOverscan(overscan, fallback int) int {
	if overscan == 0 {
		return fallback
	}
	if overscan < 0 {
		return 0
	}
	return overscan
}

func resolveLazyScroll(ctx context, offset, viewport, availableHeight float32) (float32, float32) {
	if ctx.scroll != nil {
		if viewport <= 0 {
			viewport = ctx.scroll.viewport
		}
		offset = ctx.scroll.offset
	}
	if availableHeight > 0 && availableHeight < math.MaxFloat32 {
		if viewport <= 0 || viewport > availableHeight {
			viewport = availableHeight
		}
	}
	return max(float32(0), offset), max(float32(0), viewport)
}

func stateIndex(state *lazyListState) *lazyExtentIndex {
	if state == nil {
		return nil
	}
	return &state.index
}

func gridStateIndex(state *lazyGridState) *lazyExtentIndex {
	if state == nil {
		return nil
	}
	return &state.index
}

func lazyIndexFor(existing *lazyExtentIndex, count int, revision uint64, fixed float32, extentAt func(int) float32) lazyExtentIndex {
	if extentAt == nil {
		return lazyExtentIndex{revision: revision, count: count, fixed: max(float32(0), fixed)}
	}
	if existing != nil && existing.revision == revision && existing.count == count && len(existing.prefix) == count+1 {
		if revision != 0 || lazyPrefixMatches(*existing, fixed, extentAt) {
			return *existing
		}
	}
	index := lazyExtentIndex{revision: revision, count: count, prefix: make([]float32, count+1)}
	for item := 0; item < count; item++ {
		index.prefix[item+1] = index.prefix[item] + max(float32(0), extentAt(item))
	}
	return index
}

// lazyPrefixMatches is the revision==0 fallback that detects stale variable-height caches.
func lazyPrefixMatches(index lazyExtentIndex, fixed float32, extentAt func(int) float32) bool {
	for item := 0; item < index.count; item++ {
		extent := fixed
		if extentAt != nil {
			extent = extentAt(item)
		}
		if index.extentOf(item) != max(float32(0), extent) {
			return false
		}
	}
	return true
}

func (idx lazyExtentIndex) total() float32 {
	if idx.fixed > 0 {
		return float32(idx.count) * idx.fixed
	}
	if len(idx.prefix) == 0 {
		return 0
	}
	return idx.prefix[len(idx.prefix)-1]
}

func (idx lazyExtentIndex) offsetOf(item int) float32 {
	if item <= 0 {
		return 0
	}
	if idx.fixed > 0 {
		if item >= idx.count {
			return float32(idx.count) * idx.fixed
		}
		return float32(item) * idx.fixed
	}
	if len(idx.prefix) == 0 {
		return 0
	}
	if item >= len(idx.prefix) {
		return idx.prefix[len(idx.prefix)-1]
	}
	return idx.prefix[item]
}

func (idx lazyExtentIndex) extentOf(item int) float32 {
	if idx.fixed > 0 {
		if item < 0 || item >= idx.count {
			return 0
		}
		return idx.fixed
	}
	if item < 0 || item+1 >= len(idx.prefix) {
		return 0
	}
	return idx.prefix[item+1] - idx.prefix[item]
}

// visibleRange returns a half-open item interval covering the viewport plus overscan.
func (idx lazyExtentIndex) visibleRange(offset, viewport float32, overscan int) (int, int) {
	if idx.count <= 0 {
		return 0, 0
	}
	if viewport <= 0 {
		return 0, 0
	}
	start := idx.itemAtOffset(offset)
	end := idx.itemAtOffset(offset + viewport)
	if end < idx.count && idx.offsetOf(end) < offset+viewport {
		end++
	}
	start = max(0, start-overscan)
	end = min(idx.count, end+overscan)
	if end < start {
		end = start
	}
	return start, end
}

func (idx lazyExtentIndex) itemAtOffset(offset float32) int {
	if idx.count <= 0 || offset <= 0 {
		return 0
	}
	if offset >= idx.total() {
		return max(0, idx.count-1)
	}
	if idx.fixed > 0 {
		return min(idx.count-1, int(offset/idx.fixed))
	}
	low, high := 0, idx.count
	for low < high {
		mid := (low + high) / 2
		if idx.prefix[mid+1] <= offset {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low
}
