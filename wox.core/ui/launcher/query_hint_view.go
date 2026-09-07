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
					if len(hints) >= 2 {
						// Paint each placeholder as its own chip so names that contain
						// spaces stay distinct. A single empty argument stays plain ghost text.
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
					} else {
						props.TextWidth += measure(props.CompletionSuffix)
					}
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
	return launcherview.LauncherQueryBoundary(props)
}
