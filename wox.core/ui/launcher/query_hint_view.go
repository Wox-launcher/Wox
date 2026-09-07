package launcher

import (
	"strings"
	"wox/common"
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// Keep neighboring ghost chips far enough apart that their 3-unit paint pads do not merge.
const queryHintChipGap = float32(12)

// queryHintView decorates the ordinary editor without introducing separate
// hit targets, padding, or text layout. Selection and IME retain one coordinate space.
func (a *App) queryHintView(snapshot viewSnapshot, width, height, lineHeight float32) woxwidget.Widget {
	snapshot.completionHint = nil
	props := a.queryViewProps(snapshot, width, height, lineHeight)
	measure := func(text string) float32 {
		metrics, _ := a.window.MeasureText(text, props.Style)
		return metrics.Size.Width
	}
	valueSlots := 0
	for _, element := range snapshot.hint.Elements {
		if element.Kind == common.QueryElementArgument || element.Kind == common.QueryElementBlock {
			valueSlots++
		}
	}
	// Composition temporarily changes display offsets; resume marks after commit.
	if props.State.Composition == "" {
		offset := 0
		text := []rune(props.State.Text)
		for index, element := range snapshot.hint.Elements {
			start, end := offset, offset+len([]rune(element.Content()))
			offset = end
			if element.Kind == common.QueryElementText {
				continue
			}
			start, end = min(start, len(text)), min(end, len(text))
			if start == end {
				if props.CompletionSuffix == "" && strings.Trim(string(text[end:]), " ") == "" {
					// Empty later arguments still contribute separators to the editor text.
					// Anchor ghost text at this argument, before those trailing separators.
					props.CompletionOffset = -measure(string(text[end:]))
					hints := []string{}
					for _, e := range snapshot.hint.Elements {
						if e.Kind == common.QueryElementArgument && e.Value == "" {
							hints = append(hints, string(e.Placeholder))
						}
					}
					props.CompletionSuffix = strings.Join(hints, " ")
					if snapshot.queryHintCandidate {
						props.CompletionSuffix = " " + props.CompletionSuffix
					} else if index > 1 && snapshot.hint.Elements[index-1].Kind == common.QueryElementText && snapshot.hint.Elements[index-1].Text == "" {
						// Deferred separators still separate the next ghost hint visually.
						props.CompletionSuffix = " " + props.CompletionSuffix
					}
					// Every empty argument is a hole: the same quiet chip, even
					// when only one variable remains. Plain ghost text is reserved
					// for completion suffixes, not query slots.
					x := float32(0)
					if strings.HasPrefix(props.CompletionSuffix, " ") {
						x = measure(" ")
					}
					for i, hint := range hints {
						if i > 0 {
							x += queryHintChipGap
						}
						chipWidth := measure(hint)
						props.CompletionChips = append(props.CompletionChips, launcherview.LauncherQueryCompletionChip{Text: hint, X: x, Width: chipWidth})
						x += chipWidth
					}
					props.TextWidth += x
				}
				continue
			}
			if element.Kind != common.QueryElementBlock && (element.Kind != common.QueryElementArgument || valueSlots < 2) {
				continue
			}
			lineStart := 0
			for lineIndex, line := range props.Lines {
				lineEnd := lineStart + len([]rune(line.Text))
				left, right := max(start, lineStart), min(end, lineEnd)
				if left < right {
					runes := []rune(line.Text)
					x := measure(string(runes[:left-lineStart]))
					rightX := measure(string(runes[:right-lineStart]))
					props.Marks = append(props.Marks, launcherview.LauncherQueryMark{Line: lineIndex, X: x, Width: rightX - x,
						Active: props.State.Selection.Collapsed() && props.State.Selection.Focus >= start && props.State.Selection.Focus <= end})
				}
				lineStart = lineEnd + 1
			}
		}
	}
	a.applyQueryTabHint(&props, snapshot, measure)
	return launcherview.LauncherQueryBoundary(props)
}

// applyQueryTabHint attaches one keycap after the next successful Tab target.
func (a *App) applyQueryTabHint(props *launcherview.LauncherQueryProps, snapshot viewSnapshot, measure func(string) float32) {
	if props == nil || !props.Focused || props.State.Composition != "" {
		return
	}
	size, gap := a.queryTabHintChrome(snapshot.densityMetrics)
	if size <= 0 {
		return
	}
	label := formatHotkeyLabels("tab")[0]
	if snapshot.queryHintCandidate {
		if props.CompletionSuffix == "" {
			return
		}
		x, line := queryTabHintAfterGhost(*props, measure)
		attachQueryTabHint(props, launcherview.LauncherQueryTabHint{
			Visible: true, Label: label, X: x + gap, Width: size, Height: size, Line: line,
		})
		return
	}
	if snapshot.hint == nil {
		return
	}
	active := queryHintResolveActive(snapshot.hint, snapshot.queryHintActive, props.State.Selection)
	next, ok := queryHintForwardTabTarget(snapshot.hint, active)
	if !ok {
		return
	}
	x, line := queryTabHintAfterElement(*props, snapshot.hint, next, measure)
	attachQueryTabHint(props, launcherview.LauncherQueryTabHint{
		Visible: true, Label: label, X: x + gap, Width: size, Height: size, Line: line,
	})
}

func (a *App) queryTabHintChrome(density launcherDensityMetrics) (size, gap float32) {
	return launcherview.LauncherQueryTabHintChrome(density.normalized().scale)
}

func attachQueryTabHint(props *launcherview.LauncherQueryProps, hint launcherview.LauncherQueryTabHint) {
	props.TabHint = hint
	if end := hint.X + hint.Width; end > props.TextWidth {
		props.TextWidth = end
	}
}

func queryTabHintAfterGhost(props launcherview.LauncherQueryProps, measure func(string) float32) (x float32, line int) {
	if len(props.Lines) == 0 {
		return 0, 0
	}
	line = len(props.Lines) - 1
	x = props.Lines[line].TextWidth + props.CompletionOffset
	if n := len(props.CompletionChips); n > 0 {
		chip := props.CompletionChips[n-1]
		return x + chip.X + chip.Width, line
	}
	return x + measure(props.CompletionSuffix), line
}

func queryTabHintAfterElement(props launcherview.LauncherQueryProps, hint *common.QueryHint, index int, measure func(string) float32) (x float32, line int) {
	if hint == nil || index < 0 || index >= len(hint.Elements) {
		return queryTabHintAfterGhost(props, measure)
	}
	element := hint.Elements[index]
	if element.Kind == common.QueryElementArgument && element.Value == "" {
		if len(props.Lines) > 0 {
			line = len(props.Lines) - 1
			x = props.Lines[line].TextWidth + props.CompletionOffset
		}
		if slot := queryHintEmptyArgumentIndex(hint, index); slot >= 0 && slot < len(props.CompletionChips) {
			chip := props.CompletionChips[slot]
			return x + chip.X + chip.Width, line
		}
		if props.CompletionSuffix != "" {
			return x + measure(props.CompletionSuffix), line
		}
	}
	_, end := queryElementRange(hint, index)
	return queryTextOffsetPoint(props, end, measure)
}

func queryHintEmptyArgumentIndex(hint *common.QueryHint, index int) int {
	slot := 0
	for i, element := range hint.Elements {
		if element.Kind != common.QueryElementArgument || element.Value != "" {
			continue
		}
		if i == index {
			return slot
		}
		slot++
	}
	return -1
}

func queryTextOffsetPoint(props launcherview.LauncherQueryProps, offset int, measure func(string) float32) (x float32, line int) {
	remaining := offset
	for index, queryLine := range props.Lines {
		runes := []rune(queryLine.Text)
		if remaining <= len(runes) {
			return measure(string(runes[:remaining])), index
		}
		remaining -= len(runes) + 1
		line = index
		x = queryLine.TextWidth
	}
	return x, line
}
