package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	settingsInlineTooltipGap            = float32(6)
	settingsInlineTooltipMargin         = float32(8)
	settingsInlineTooltipPaddingX       = float32(11)
	settingsInlineTooltipPaddingY       = float32(8)
	settingsInlineTooltipLineHeight     = float32(16)
	settingsInlineTooltipMinWidth       = float32(120)
	settingsInlineTooltipPreferredWidth = float32(280)
	settingsInlineTooltipMaxWidth       = float32(360)
	settingsInlineTooltipMaxLines       = 12
)

// SettingsInlineTooltipProps contains one settings-window tooltip anchored in window coordinates.
type SettingsInlineTooltipProps struct {
	Width   float32
	Height  float32
	Anchor  woxui.Rect
	Message string
	Side    string
	Theme   woxcomponent.Theme
}

// SettingsInlineTooltipOverlay renders Linux fallback tooltips inside the settings window.
func SettingsInlineTooltipOverlay(props SettingsInlineTooltipProps) (woxwidget.Widget, float32, float32) {
	message := strings.TrimSpace(props.Message)
	if message == "" || props.Width <= 0 || props.Height <= 0 {
		return nil, 0, 0
	}

	tooltipWidth := min(settingsInlineTooltipMaxWidth, max(settingsInlineTooltipMinWidth, settingsInlineTooltipPreferredWidth))
	tooltipWidth = min(tooltipWidth, max(float32(1), props.Width-settingsInlineTooltipMargin*2))
	contentWidth := max(float32(1), tooltipWidth-settingsInlineTooltipPaddingX*2)
	lineCount := settingsInlineTooltipLineCount(message, contentWidth)
	tooltipHeight := settingsInlineTooltipPaddingY*2 + float32(lineCount)*settingsInlineTooltipLineHeight

	left, top := settingsInlineTooltipPosition(props, tooltipWidth, tooltipHeight)

	background := props.Theme.ActionBackground
	border := props.Theme.PreviewSplit
	border.A = uint8(float32(border.A) * 0.7)
	textColor := props.Theme.ActionText
	textColor.A = uint8(float32(textColor.A) * 0.96)

	tooltip := woxwidget.Container{
		Width:       tooltipWidth,
		Height:      tooltipHeight,
		Radius:      8,
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
			Style:      woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold},
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
	}
	if side == "bottom" {
		top = anchor.Y + anchor.Height + settingsInlineTooltipGap
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

func settingsInlineTooltipLineCount(message string, contentWidth float32) int {
	maxCharsPerLine := int(max(float32(8), contentWidth/7))
	if maxCharsPerLine <= 0 {
		maxCharsPerLine = 8
	}
	runes := []rune(message)
	lineCount := (len(runes) + maxCharsPerLine - 1) / maxCharsPerLine
	if lineCount < 1 {
		lineCount = 1
	}
	if lineCount > settingsInlineTooltipMaxLines {
		lineCount = settingsInlineTooltipMaxLines
	}
	return lineCount
}
