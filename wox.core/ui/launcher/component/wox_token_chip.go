package component

import (
	woxui "wox/ui/runtime"
)

const (
	tokenChipPadX        = float32(6)
	tokenChipHeight      = float32(18)
	tokenChipRadius      = float32(4)
	tokenChipFontSize    = float32(11)
	tokenChipMinWidth    = float32(24)
	tokenChipCloseCircle = float32(12)
	tokenChipCloseIcon   = float32(8)
	tokenChipCloseGap    = float32(4)
	tokenChipCloseSlot   = tokenChipCloseCircle + tokenChipCloseGap
	tokenChipEditSlot    = tokenChipCloseCircle + tokenChipCloseGap
	tokenChipCloseHitMin = float32(0.85)
	tokenChipDismissMs   = 180
)

type tokenChipAction uint8

const (
	tokenChipActionNone tokenChipAction = iota
	tokenChipActionBody
	tokenChipActionEdit
	tokenChipActionClose
)

func (action tokenChipAction) isButton() bool {
	return action == tokenChipActionEdit || action == tokenChipActionClose
}

func tokenChipHoverExtra(run TextFieldRichRun) float32 {
	extra := tokenChipCloseSlot
	if run.ChipEditable {
		extra += tokenChipEditSlot
	}
	return extra
}

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
	paintTokenChip(displayList, bounds, label, theme, 0, false)
}

func paintTokenChip(displayList *woxui.DisplayList, bounds woxui.Rect, label string, theme Theme, progress float32, editable bool) {
	if displayList == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	fill := theme.ResultSubtitle
	fill.A = uint8(float32(fill.A) * (0.16 + 0.08*progress))
	height := min(tokenChipHeight, bounds.Height)
	chip := woxui.Rect{
		X: bounds.X, Y: bounds.Y + (bounds.Height-height)/2,
		Width: bounds.Width, Height: height,
	}
	displayList.FillRoundedRect(chip, tokenChipRadius, fill)
	extra := tokenChipCloseSlot
	if editable {
		extra += tokenChipEditSlot
	}
	labelWidth := max(float32(0), chip.Width-tokenChipPadX*2-extra*progress)
	displayList.DrawText(label, woxui.Rect{
		X: chip.X + tokenChipPadX, Y: chip.Y, Width: labelWidth, Height: chip.Height,
	}, woxui.TextStyle{Size: tokenChipFontSize}, theme.ResultTitle)
	if progress > 0 {
		if editable {
			paintTokenChipEdit(displayList, chip, theme, progress)
		}
		paintTokenChipClose(displayList, chip, theme, progress)
	}
}

// tokenChipCloseColor uses the theme danger color, with the shared window-close red as fallback.
func tokenChipCloseColor(theme Theme) woxui.Color {
	if theme.ErrorText.A > 0 {
		return theme.ErrorText
	}
	return woxui.Color{R: 232, G: 17, B: 35, A: 255}
}

// paintTokenChipEdit draws the quiet circular edit control to the left of close.
func paintTokenChipEdit(displayList *woxui.DisplayList, chip woxui.Rect, theme Theme, progress float32) {
	circle := theme.ResultTitle
	circle.A = uint8(float32(theme.ResultTitle.A) * 0.16)
	paintTokenChipAction(displayList, chip, circle, theme.ResultTitle, "control.edit", progress, tokenChipCloseSlot)
}

// paintTokenChipClose draws the red circular close control, fading and scaling with hover progress.
func paintTokenChipClose(displayList *woxui.DisplayList, chip woxui.Rect, theme Theme, progress float32) {
	paintTokenChipAction(displayList, chip, tokenChipCloseColor(theme), woxui.Color{R: 255, G: 255, B: 255, A: 255}, "control.close", progress, 0)
}

// paintTokenChipAction draws one circular chip action, offset from the trailing edge.
func paintTokenChipAction(displayList *woxui.DisplayList, chip woxui.Rect, circle, iconColor woxui.Color, iconName string, progress, trailingOffset float32) {
	circle.A = uint8(float32(circle.A) * progress)
	iconColor.A = uint8(float32(iconColor.A) * progress)
	size := tokenChipCloseCircle * (0.7 + 0.3*progress)
	x := chip.X + chip.Width - tokenChipPadX - trailingOffset - tokenChipCloseCircle + (tokenChipCloseCircle-size)/2
	y := chip.Y + (chip.Height-size)/2
	displayList.FillRoundedRect(woxui.Rect{X: x, Y: y, Width: size, Height: size}, size/2, circle)
	icon := svgIconImage(iconName, tokenChipCloseIcon, iconColor)
	if icon == nil {
		return
	}
	inset := (size - tokenChipCloseIcon*progress) / 2
	displayList.DrawImage(icon, woxui.Rect{
		X: x + inset, Y: y + inset, Width: max(float32(0), size-inset*2), Height: max(float32(0), size-inset*2),
	})
}

// NewTokenChipRun hides placeholder text and paints a compact chip in its place.
func NewTokenChipRun(start, end int, label string, window textFieldMeasurer, theme Theme) TextFieldRichRun {
	return TextFieldRichRun{
		Start: start, End: end, Advance: MeasureTokenChip(window, label), HideText: true, ChipLabel: label,
		Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintTokenChip(displayList, bounds, label, theme, 0, false)
		},
	}
}

// WithDismissible marks a token chip so hover can reveal a trailing close affordance.
func (run TextFieldRichRun) WithDismissible() TextFieldRichRun {
	run.Dismissible = true
	return run
}

// WithChipEdit marks a dismissible chip so hover also reveals an edit control.
func (run TextFieldRichRun) WithChipEdit() TextFieldRichRun {
	run.ChipEditable = true
	return run
}

// withDismissibleChipHover widens one chip by progress and paints its hover actions.
func withDismissibleChipHover(runs []TextFieldRichRun, start int, theme Theme, progress float32) []TextFieldRichRun {
	if progress <= 0 {
		return runs
	}
	if progress > 1 {
		progress = 1
	}
	next := append([]TextFieldRichRun(nil), runs...)
	for index := range next {
		if !next[index].Dismissible || next[index].Start != start {
			continue
		}
		label := next[index].ChipLabel
		editable := next[index].ChipEditable
		next[index].Advance += tokenChipHoverExtra(next[index]) * progress
		next[index].Paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintTokenChip(displayList, bounds, label, theme, progress, editable)
		}
		return next
	}
	return runs
}
