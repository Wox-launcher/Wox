package launcher

import (
	"context"
	"math"
	"runtime"
	"strconv"
	"time"

	woxui "wox/ui/runtime"
	"wox/util"
)

const (
	quickSelectHoldDelay = 300 * time.Millisecond
	quickSelectMaxNumber = 9
)

// quickSelectViewport remembers the last painted result viewport so number keys
// resolve against the same visible rows the badges were drawn on.
type quickSelectViewport struct {
	grid       bool
	offset     float32
	height     float32
	topPadding float32
	rowHeight  float32
	gap        float32
	columns    int
	cellHeight float32
}

// onQuickSelectKey holds Alt (Windows/Linux) or Command (macOS) to number visible results.
func (a *App) onQuickSelectKey(event woxui.KeyEvent) bool {
	if event.Composing || a.quickSelectBlocked() {
		a.stopQuickSelectLocked()
		return false
	}
	if event.Down && !event.Repeat && a.quickSelectMode {
		if number := quickSelectDigit(event.Key); number > 0 {
			if a.activateQuickSelectNumber(number) {
				return true
			}
		}
	}
	if event.Repeat && isQuickSelectModifierKey(event) {
		return true
	}
	if event.Down && !event.Repeat && isQuickSelectModifierKeyOnly(event) {
		a.startQuickSelectTimerLocked()
		return true
	}
	if isQuickSelectModifierKey(event) || a.quickSelectKeyPressed || a.quickSelectMode {
		a.stopQuickSelectLocked()
		return isQuickSelectModifierKey(event)
	}
	return false
}

// quickSelectBlocked keeps overlays and empty result sets from entering hold-to-number mode.
func (a *App) quickSelectBlocked() bool {
	if a.actionPanel || a.form != nil || a.requirementForm != nil || a.launcherTableEditor != nil {
		return true
	}
	return !hasQuickSelectResult(a.results)
}

// startQuickSelectTimerLocked begins Flutter's 300ms hold before showing numbers.
func (a *App) startQuickSelectTimerLocked() {
	if a.quickSelectMode || a.quickSelectBlocked() {
		return
	}
	a.quickSelectKeyPressed = true
	if a.quickSelectTimer != nil {
		a.quickSelectTimer.Stop()
	}
	a.quickSelectTimer = time.AfterFunc(quickSelectHoldDelay, func() {
		if err := a.runOnUI("activate quick select", func() {
			if !a.quickSelectKeyPressed || a.quickSelectBlocked() {
				return
			}
			a.activateQuickSelectModeLocked()
		}); err != nil {
			util.GetLogger().Warn(quickSelectLogContext(a), "dispatch quick select: "+err.Error())
		}
	})
}

// stopQuickSelectLocked cancels a pending hold and hides result numbers.
func (a *App) stopQuickSelectLocked() {
	a.quickSelectKeyPressed = false
	if a.quickSelectTimer != nil {
		a.quickSelectTimer.Stop()
		a.quickSelectTimer = nil
	}
	if !a.quickSelectMode {
		return
	}
	a.quickSelectMode = false
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// activateQuickSelectModeLocked shows 1-9 on the visible non-group results.
func (a *App) activateQuickSelectModeLocked() {
	a.quickSelectTimer = nil
	if a.quickSelectMode || a.quickSelectBlocked() {
		return
	}
	a.quickSelectMode = true
	util.GetLogger().Debug(quickSelectLogContext(a), "Quick select: activating mode")
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// activateQuickSelectNumber runs the default action of the numbered visible result.
func (a *App) activateQuickSelectNumber(number int) bool {
	index := quickSelectResultIndex(a.results, a.quickSelectVisibleLocked(), number)
	if index < 0 {
		return false
	}
	util.GetLogger().Debug(quickSelectLogContext(a), "Quick select: selecting item "+strconv.Itoa(number)+" at index "+strconv.Itoa(index))
	a.stopQuickSelectLocked()
	if a.window != nil {
		a.selectResult(index)
	} else {
		a.selected = index
	}
	a.activateResult(index)
	return true
}

// rememberQuickSelectViewport stores the geometry used to number the current result surface.
func (a *App) rememberQuickSelectViewport(viewport quickSelectViewport) {
	a.quickSelectViewport = viewport
}

// quickSelectVisibleLocked marks the rows that should receive 1-9 for the current viewport.
func (a *App) quickSelectVisibleLocked() []bool {
	viewport := a.quickSelectViewport
	if viewport.height <= 0 {
		return allQuickSelectVisible(len(a.results))
	}
	if viewport.grid {
		return quickSelectVisibleGridResults(a.results, viewport.columns, viewport.cellHeight, viewport.offset, viewport.height)
	}
	return quickSelectVisibleListResults(len(a.results), viewport.offset, viewport.height, viewport.topPadding, viewport.rowHeight, viewport.gap)
}

func quickSelectLogContext(a *App) context.Context {
	if a != nil && a.lifecycleCtx != nil {
		return a.lifecycleCtx
	}
	return context.Background()
}

func isQuickSelectModifierKey(event woxui.KeyEvent) bool {
	if runtime.GOOS == "darwin" {
		return event.Key == woxui.KeyMeta
	}
	return event.Key == woxui.KeyAlt
}

func isQuickSelectModifierKeyOnly(event woxui.KeyEvent) bool {
	if !isQuickSelectModifierKey(event) {
		return false
	}
	extra := event.Modifiers
	if runtime.GOOS == "darwin" {
		extra &^= woxui.KeyModifierMeta
	} else {
		extra &^= woxui.KeyModifierAlt
	}
	return extra == 0
}

func quickSelectDigit(key woxui.Key) int {
	if len(key) != 1 || key[0] < '1' || key[0] > '9' {
		return 0
	}
	return int(key[0] - '0')
}

func hasQuickSelectResult(results []queryResult) bool {
	for _, result := range results {
		if !result.IsGroup {
			return true
		}
	}
	return false
}

func allQuickSelectVisible(count int) []bool {
	visible := make([]bool, count)
	for index := range visible {
		visible[index] = true
	}
	return visible
}

// quickSelectVisibleListResults marks rows that intersect the result viewport without overscan.
func quickSelectVisibleListResults(count int, offset, viewport, topPadding, rowHeight, gap float32) []bool {
	visible := make([]bool, count)
	if count <= 0 || rowHeight <= 0 || viewport <= 0 {
		return visible
	}
	stride := rowHeight + gap
	start := int(math.Floor(float64((offset - topPadding) / stride)))
	end := int(math.Ceil(float64((offset + viewport - topPadding) / stride)))
	if start < 0 {
		start = 0
	}
	if end > count {
		end = count
	}
	if start > end {
		start = end
	}
	for index := start; index < end; index++ {
		visible[index] = true
	}
	return visible
}

// quickSelectVisibleGridResults marks cells whose row intersects the viewport without overscan.
func quickSelectVisibleGridResults(results []queryResult, columns int, rowHeight, offset, viewport float32) []bool {
	visible := make([]bool, len(results))
	if columns <= 0 || rowHeight <= 0 || viewport <= 0 {
		return visible
	}
	bottom := offset + viewport
	y := float32(0)
	for index := 0; index < len(results); {
		if results[index].IsGroup {
			y += gridGroupHeaderHeight
			index++
			continue
		}
		rowStart := index
		for index < len(results) && !results[index].IsGroup && index-rowStart < columns {
			index++
		}
		if y+rowHeight > offset && y < bottom {
			for itemIndex := rowStart; itemIndex < index; itemIndex++ {
				visible[itemIndex] = true
			}
		}
		y += rowHeight
	}
	return visible
}

func quickSelectNumberFor(results []queryResult, visible []bool, index int) string {
	if index < 0 || index >= len(results) || results[index].IsGroup {
		return ""
	}
	number := 0
	limit := len(results)
	if len(visible) < limit {
		limit = len(visible)
	}
	for current := 0; current < limit; current++ {
		if !visible[current] || results[current].IsGroup {
			continue
		}
		number++
		if number > quickSelectMaxNumber {
			return ""
		}
		if current == index {
			return strconv.Itoa(number)
		}
	}
	return ""
}

func quickSelectResultIndex(results []queryResult, visible []bool, number int) int {
	if number < 1 || number > quickSelectMaxNumber {
		return -1
	}
	current := 0
	limit := len(results)
	if len(visible) < limit {
		limit = len(visible)
	}
	for index := 0; index < limit; index++ {
		if !visible[index] || results[index].IsGroup {
			continue
		}
		current++
		if current == number {
			return index
		}
	}
	return -1
}
