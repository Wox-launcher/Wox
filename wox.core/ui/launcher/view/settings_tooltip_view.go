package view

import (
	"math"
	"strings"
	"unicode"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	settingsInlineTooltipGap        = float32(6)
	settingsInlineTooltipMargin     = float32(8)
	settingsInlineTooltipPaddingX   = float32(11)
	settingsInlineTooltipPaddingY   = float32(8)
	settingsInlineTooltipLineHeight = float32(16)
	// settingsInlineTooltipMaxWidth matches the native tooltip cap.
	settingsInlineTooltipMaxWidth = float32(400)
	settingsInlineTooltipMaxLines = 12
	// Width factors match util/tooltip so a missing measure still tracks Latin and CJK.
	settingsInlineTooltipAsciiWidth = float32(0.68)
	settingsInlineTooltipWideWidth  = float32(1.1)
	settingsInlineTooltipSpaceWidth = float32(0.34)
)

// SettingsInlineTooltipProps contains one settings-window tooltip anchored in window coordinates.
type SettingsInlineTooltipProps struct {
	Width   float32
	Height  float32
	Anchor  woxui.Rect
	Message string
	Side    string
	Theme   woxcomponent.ControlTheme
	// Window measures the label. Without it the width falls back to the shared character estimate.
	Window *woxui.Window
}

// SettingsInlineTooltipOverlay renders Linux fallback tooltips inside the settings window.
func SettingsInlineTooltipOverlay(props SettingsInlineTooltipProps) (woxwidget.Widget, float32, float32) {
	message := strings.TrimSpace(props.Message)
	if message == "" || props.Width <= 0 || props.Height <= 0 {
		return nil, 0, 0
	}

	style := woxui.TextStyle{Size: props.Theme.Scaled(11), Weight: woxui.FontWeightSemibold}
	maxWidth := min(settingsInlineTooltipMaxWidth, max(float32(1), props.Width-settingsInlineTooltipMargin*2))
	contentLimit := max(float32(1), maxWidth-settingsInlineTooltipPaddingX*2)
	contentWidth, lineCount := settingsInlineTooltipTextSize(props.Window, message, style, contentLimit)
	tooltipWidth := contentWidth + settingsInlineTooltipPaddingX*2
	tooltipHeight := settingsInlineTooltipPaddingY*2 + float32(lineCount)*settingsInlineTooltipLineHeight

	left, top := settingsInlineTooltipPosition(props, tooltipWidth, tooltipHeight)

	background := props.Theme.Surface
	border := props.Theme.Border
	border.A = uint8(float32(border.A) * 0.7)
	textColor := props.Theme.Text
	textColor.A = uint8(float32(textColor.A) * 0.96)

	tooltip := woxwidget.Container{
		Width:       tooltipWidth,
		Height:      tooltipHeight,
		Radius:      8,
		Floating:    true,
		Color:       background,
		BorderColor: border,
		BorderWidth: 1,
		Padding: woxwidget.Insets{
			Left:   settingsInlineTooltipPaddingX,
			Top:    settingsInlineTooltipPaddingY,
			Right:  settingsInlineTooltipPaddingX,
			Bottom: settingsInlineTooltipPaddingY,
		},
		Child: woxwidget.TextBlock{
			Value:      message,
			Width:      contentWidth,
			Height:     float32(lineCount) * settingsInlineTooltipLineHeight,
			MaxLines:   lineCount,
			LineHeight: settingsInlineTooltipLineHeight,
			Style:      style,
			Color:      textColor,
		},
	}

	return tooltip, left, top
}

func settingsInlineTooltipPosition(props SettingsInlineTooltipProps, tooltipWidth, tooltipHeight float32) (float32, float32) {
	anchor := props.Anchor
	if anchor.Width <= 0 || anchor.Height <= 0 {
		anchor = woxui.Rect{X: props.Width / 2, Y: props.Height / 2, Width: 1, Height: 1}
	}

	centerY := anchor.Y + anchor.Height/2
	minTop := settingsInlineTooltipMargin
	maxTop := props.Height - settingsInlineTooltipMargin - tooltipHeight
	top := centerY - tooltipHeight/2
	if maxTop < minTop {
		top = minTop
	} else {
		top = min(max(top, minTop), maxTop)
	}

	side := strings.ToLower(strings.TrimSpace(props.Side))
	left := anchor.X - tooltipWidth - settingsInlineTooltipGap
	if side == "right" {
		left = anchor.X + anchor.Width + settingsInlineTooltipGap
	}
	if side == "top" || side == "bottom" {
		left = anchor.X + anchor.Width/2 - tooltipWidth/2
	}

	if side == "top" {
		top = anchor.Y - tooltipHeight - settingsInlineTooltipGap
		if top < minTop {
			top = anchor.Y + anchor.Height + settingsInlineTooltipGap
		}
	}
	if side == "bottom" {
		top = anchor.Y + anchor.Height + settingsInlineTooltipGap
		if top+tooltipHeight > props.Height-settingsInlineTooltipMargin {
			top = anchor.Y - tooltipHeight - settingsInlineTooltipGap
		}
	}

	minLeft := settingsInlineTooltipMargin
	maxLeft := props.Width - settingsInlineTooltipMargin - tooltipWidth
	if maxLeft < minLeft {
		left = minLeft
	} else {
		if side == "left" && left < minLeft {
			left = anchor.X + anchor.Width + settingsInlineTooltipGap
		}
		if side == "right" && left+tooltipWidth > props.Width-settingsInlineTooltipMargin {
			left = anchor.X - tooltipWidth - settingsInlineTooltipGap
		}
		left = min(max(left, minLeft), maxLeft)
	}

	if maxTop < minTop {
		top = minTop
	} else {
		top = min(max(top, minTop), maxTop)
	}

	return left, top
}

// settingsInlineTooltipTextSize returns the content width and wrapped line count.
// A live window uses the same font metrics as the native tooltip. Otherwise the
// width follows the shared Latin and CJK character factors.
func settingsInlineTooltipTextSize(window *woxui.Window, message string, style woxui.TextStyle, contentLimit float32) (float32, int) {
	natural := settingsInlineTooltipNaturalWidth(window, message, style)
	contentWidth := min(max(natural, float32(1)), contentLimit)
	if window != nil {
		layout := woxwidget.LayoutTextBlock(window, message, style, contentWidth, settingsInlineTooltipMaxLines, settingsInlineTooltipLineHeight)
		lineCount := len(layout.Lines)
		if lineCount < 1 {
			lineCount = 1
		}
		if lineCount > settingsInlineTooltipMaxLines {
			lineCount = settingsInlineTooltipMaxLines
		}
		return contentWidth, lineCount
	}
	return contentWidth, settingsInlineTooltipEstimatedLines(message, style.Size, contentWidth)
}

func settingsInlineTooltipNaturalWidth(window *woxui.Window, message string, style woxui.TextStyle) float32 {
	widest := float32(0)
	for _, line := range strings.Split(message, "\n") {
		var width float32
		if window != nil {
			if metrics, err := window.MeasureText(line, style); err == nil {
				width = metrics.Size.Width
			}
		}
		if width <= 0 {
			width = settingsInlineTooltipEstimatedWidth(line, style.Size)
		}
		widest = max(widest, width)
	}
	return widest
}

func settingsInlineTooltipEstimatedWidth(text string, fontSize float32) float32 {
	if fontSize <= 0 {
		fontSize = 11
	}
	width := float32(0)
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			width += fontSize * settingsInlineTooltipSpaceWidth
		case r <= unicode.MaxASCII:
			width += fontSize * settingsInlineTooltipAsciiWidth
		default:
			width += fontSize * settingsInlineTooltipWideWidth
		}
	}
	return width
}

func settingsInlineTooltipEstimatedLines(message string, fontSize, contentWidth float32) int {
	if contentWidth <= 0 {
		contentWidth = 1
	}
	lineCount := 0
	for _, line := range strings.Split(message, "\n") {
		width := settingsInlineTooltipEstimatedWidth(line, fontSize)
		wrapped := int(math.Ceil(float64(width / contentWidth)))
		if wrapped < 1 {
			wrapped = 1
		}
		lineCount += wrapped
	}
	if lineCount < 1 {
		lineCount = 1
	}
	if lineCount > settingsInlineTooltipMaxLines {
		return settingsInlineTooltipMaxLines
	}
	return lineCount
}
