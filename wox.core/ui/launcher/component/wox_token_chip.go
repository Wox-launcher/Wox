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
	tokenChipIconSize    = float32(12)
	tokenChipIconGap     = float32(4)
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

func tokenChipIconSlot(icon *woxui.Image) float32 {
	if icon == nil {
		return 0
	}
	return tokenChipIconSize + tokenChipIconGap
}

// MeasureTokenChip returns the inline advance reserved for a compact token chip.
func MeasureTokenChip(window textFieldMeasurer, label string) float32 {
	return MeasureTokenChipWithIcon(window, label, nil)
}

// MeasureTokenChipWithIcon reserves space for an optional leading chip icon.
func MeasureTokenChipWithIcon(window textFieldMeasurer, label string, icon *woxui.Image) float32 {
	width := float32(len([]rune(label))) * 7
	if window != nil {
		if metrics, err := window.MeasureText(label, woxui.TextStyle{Size: tokenChipFontSize}); err == nil {
			width = metrics.Size.Width
		}
	}
	return max(tokenChipMinWidth, width+tokenChipPadX*2+tokenChipIconSlot(icon))
}

// PaintTokenChip draws a quiet pill that replaces a backing placeholder in the editor.
func PaintTokenChip(displayList *woxui.DisplayList, bounds woxui.Rect, label string, theme ControlTheme) {
	paintTokenChip(displayList, bounds, label, nil, nil, theme, 0, false)
}

func tokenChipLabelHeight(window textFieldMeasurer, label string, chipHeight float32) float32 {
	style := woxui.TextStyle{Size: tokenChipFontSize}
	height := tokenChipFontSize
	if window != nil {
		if metrics, err := window.MeasureText(label, style); err == nil && metrics.Size.Height > 0 {
			height = metrics.Size.Height
		}
	}
	return min(chipHeight, height)
}

func paintTokenChip(displayList *woxui.DisplayList, bounds woxui.Rect, label string, icon *woxui.Image, window textFieldMeasurer, theme ControlTheme, progress float32, editable bool) {
	if displayList == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	fill := theme.TextSecondary
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
	iconSlot := tokenChipIconSlot(icon)
	if icon != nil {
		displayList.DrawImage(icon, woxui.Rect{
			X: chip.X + tokenChipPadX, Y: chip.Y + (chip.Height-tokenChipIconSize)/2,
			Width: tokenChipIconSize, Height: tokenChipIconSize,
		})
	}
	labelWidth := max(float32(0), chip.Width-tokenChipPadX*2-extra*progress-iconSlot)
	labelHeight := tokenChipLabelHeight(window, label, chip.Height)
	displayList.DrawText(label, woxui.Rect{
		X: chip.X + tokenChipPadX + iconSlot, Y: chip.Y + (chip.Height-labelHeight)/2,
		Width: labelWidth, Height: labelHeight,
	}, woxui.TextStyle{Size: tokenChipFontSize}, theme.Text)
	if progress > 0 {
		if editable {
			paintTokenChipEdit(displayList, chip, theme, progress)
		}
		paintTokenChipClose(displayList, chip, theme, progress)
	}
}

// tokenChipCloseColor uses the theme danger color, with the shared window-close red as fallback.
func tokenChipCloseColor(theme ControlTheme) woxui.Color {
	if theme.Error.A > 0 {
		return theme.Error
	}
	return woxui.Color{R: 232, G: 17, B: 35, A: 255}
}

// paintTokenChipEdit draws the quiet circular edit control to the left of close.
func paintTokenChipEdit(displayList *woxui.DisplayList, chip woxui.Rect, theme ControlTheme, progress float32) {
	circle := theme.Text
	circle.A = uint8(float32(theme.Text.A) * 0.16)
	paintTokenChipAction(displayList, chip, circle, theme.Text, "control.edit", progress, tokenChipCloseSlot)
}

// paintTokenChipClose draws the red circular close control, fading and scaling with hover progress.
func paintTokenChipClose(displayList *woxui.DisplayList, chip woxui.Rect, theme ControlTheme, progress float32) {
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
func NewTokenChipRun(start, end int, label string, window textFieldMeasurer, theme ControlTheme) TextFieldRichRun {
	return NewTokenChipRunWithIcon(start, end, label, nil, window, theme)
}

// NewTokenChipRunWithIcon paints a compact chip with an optional leading icon.
func NewTokenChipRunWithIcon(start, end int, label string, icon *woxui.Image, window textFieldMeasurer, theme ControlTheme) TextFieldRichRun {
	return TextFieldRichRun{
		Start: start, End: end, Advance: MeasureTokenChipWithIcon(window, label, icon), HideText: true, ChipLabel: label, ChipIcon: icon,
		Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintTokenChip(displayList, bounds, label, icon, window, theme, 0, false)
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
func withDismissibleChipHover(runs []TextFieldRichRun, start int, theme ControlTheme, progress float32) []TextFieldRichRun {
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
		icon := next[index].ChipIcon
		editable := next[index].ChipEditable
		next[index].Advance += tokenChipHoverExtra(next[index]) * progress
		next[index].Paint = func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			paintTokenChip(displayList, bounds, label, icon, nil, theme, progress, editable)
		}
		return next
	}
	return runs
}
