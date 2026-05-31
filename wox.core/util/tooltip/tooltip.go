package tooltip

import (
	"context"
	"math"
	"unicode"
	"unicode/utf8"

	"wox/util/overlay"
)

const tooltipOverlayPrefix = "wox_tooltip_"
const (
	tooltipFontSizePt       = 10
	tooltipMaxWidthDip      = 300
	tooltipMinWidthDip      = 100
	tooltipPaddingXDip      = 24
	tooltipPaddingYDip      = 20
	tooltipLineHeightDip    = 14
	tooltipHeightSlackDip   = 4
	tooltipAsciiWidthFactor = 0.56
	tooltipWideWidthFactor  = 1.0
	tooltipSpaceWidthFactor = 0.34
)

// OverlayOptions describes a lightweight native tooltip request.
type OverlayOptions struct {
	Name          string
	Text          string
	X             float64
	Y             float64
	TooltipWidth  float64
	TooltipHeight float64
	AnchorX       float64
	AnchorY       float64
	AnchorWidth   float64
	AnchorHeight  float64
}

// Show renders a native tooltip window that is independent of the Flutter
// launcher surface, so it can extend beyond the launcher bounds.
func Show(ctx context.Context, opts OverlayOptions) {
	if opts.Text == "" {
		return
	}
	if opts.Name == "" {
		opts.Name = tooltipOverlayPrefix + "default"
	}
	width, trackingHeight := estimateBounds(opts.Text)

	overlay.Show(overlay.OverlayOptions{
		Name:             opts.Name,
		Title:            "Wox tooltip",
		Message:          opts.Text,
		Topmost:          true,
		AbsolutePosition: true,
		Anchor:           overlay.AnchorTopLeft,
		OffsetX:          opts.X,
		OffsetY:          opts.Y,
		Width:            width,
		FontSize:         tooltipFontSizePt,
		CornerRadius:     8,
	})
	startVisibilityTracking(opts.withBounds(width, trackingHeight))

	_ = ctx
}

// Close hides a previously shown native tooltip window.
func Close(name string) {
	if name == "" {
		return
	}
	stopVisibilityTracking(name)
	overlay.Close(name)
}

func (opts OverlayOptions) withBounds(width float64, height float64) OverlayOptions {
	return OverlayOptions{
		Name:          opts.Name,
		Text:          opts.Text,
		X:             opts.X,
		Y:             opts.Y,
		TooltipWidth:  width,
		TooltipHeight: height,
		AnchorX:       opts.AnchorX,
		AnchorY:       opts.AnchorY,
		AnchorWidth:   opts.AnchorWidth,
		AnchorHeight:  opts.AnchorHeight,
	}
}

func estimateBounds(text string) (float64, float64) {
	contentMaxWidth := float64(tooltipMaxWidthDip - tooltipPaddingXDip)
	maxContentWidth := 0.0
	lineCount := 0.0

	for _, line := range splitTooltipLines(text) {
		lineWidth := estimateLineWidth(line)
		if lineWidth > maxContentWidth {
			maxContentWidth = lineWidth
		}
		wrappedLines := math.Ceil(lineWidth / contentMaxWidth)
		if wrappedLines < 1 {
			wrappedLines = 1
		}
		lineCount += wrappedLines
	}

	if lineCount < 1 {
		lineCount = 1
	}

	width := maxTooltipDimension(tooltipMinWidthDip, math.Min(tooltipMaxWidthDip, maxContentWidth+tooltipPaddingXDip))
	height := lineCount*tooltipLineHeightDip + tooltipPaddingYDip + tooltipHeightSlackDip
	return width, height
}

func splitTooltipLines(text string) []string {
	lines := []string{}
	current := ""
	for _, r := range text {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
			continue
		}
		current += string(r)
	}
	lines = append(lines, current)
	return lines
}

func estimateLineWidth(text string) float64 {
	if text == "" {
		return 0
	}

	width := 0.0
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			width += tooltipFontSizePt * tooltipSpaceWidthFactor
		case r <= unicode.MaxASCII:
			width += tooltipFontSizePt * tooltipAsciiWidthFactor
		default:
			width += tooltipFontSizePt * tooltipWideWidthFactor
		}
	}

	if width == 0 {
		return float64(utf8.RuneCountInString(text)) * tooltipFontSizePt * tooltipAsciiWidthFactor
	}
	return width
}

func maxTooltipDimension(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
