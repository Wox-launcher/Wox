package component

import (
	woxui "wox/ui/runtime"
)

const (
	tokenChipPadX     = float32(6)
	tokenChipHeight   = float32(18)
	tokenChipRadius   = float32(4)
	tokenChipFontSize = float32(11)
	tokenChipMinWidth = float32(24)
)

// MeasureTokenChip returns the inline advance reserved for a compact token chip.
func MeasureTokenChip(window textFieldMeasurer, label string) float32 {
	width := float32(len([]rune(label))) * 7
	if window != nil {
		if metrics, err := window.MeasureText(label, woxui.TextStyle{Size: tokenChipFontSize}); err == nil {
			width = metrics.Size.Width
		}
	}
	return max(tokenChipMinWidth, width+tokenChipPadX*2)
}

// PaintTokenChip draws a quiet pill that replaces a backing placeholder in the editor.
func PaintTokenChip(displayList *woxui.DisplayList, bounds woxui.Rect, label string, theme Theme) {
	if displayList == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	fill := theme.ResultSubtitle
	fill.A = uint8(float32(fill.A) * 0.16)
	height := min(tokenChipHeight, bounds.Height)
	chip := woxui.Rect{
		X: bounds.X, Y: bounds.Y + (bounds.Height-height)/2,
		Width: bounds.Width, Height: height,
	}
	displayList.FillRoundedRect(chip, tokenChipRadius, fill)
	displayList.DrawText(label, woxui.Rect{
		X: chip.X + tokenChipPadX, Y: chip.Y, Width: max(float32(0), chip.Width-tokenChipPadX*2), Height: chip.Height,
	}, woxui.TextStyle{Size: tokenChipFontSize}, theme.ResultTitle)
}

// NewTokenChipRun hides placeholder text and paints a compact chip in its place.
func NewTokenChipRun(start, end int, label string, window textFieldMeasurer, theme Theme) TextFieldRichRun {
	return TextFieldRichRun{
		Start: start, End: end, Advance: MeasureTokenChip(window, label), HideText: true,
		Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			PaintTokenChip(displayList, bounds, label, theme)
		},
	}
}
