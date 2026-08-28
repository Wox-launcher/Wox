package tooltip

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	woxwidget "wox/ui/widget"
	"wox/util/mouse"
	"wox/util/overlay"
	"wox/util/overlay/textoverlay"
	"wox/util/screen"
)

const tooltipOverlayPrefix = "wox_tooltip_"
const (
	tooltipMaxWidthDip      = 400
	tooltipMaxHeightDip     = 600
	tooltipMinWidthDip      = 1
	tooltipFontSizeDip      = 12
	tooltipPaddingXDip      = 24
	tooltipPaddingYDip      = 22
	tooltipLineHeightDip    = 16
	tooltipHeightSlackDip   = 6
	tooltipGapDip           = 6
	tooltipMarginDip        = 8
	tooltipTrackingSlackDip = 20
	tooltipAsciiWidthFactor = 0.68
	tooltipWideWidthFactor  = 1.1
	tooltipSpaceWidthFactor = 0.34
	// tooltipOwnerLeaveGrace is long enough for the first post-show cursor
	// samples to miss, but short enough that a delayed tooltip cannot stay
	// up after the pointer has already moved to another control.
	tooltipOwnerLeaveGrace = 200 * time.Millisecond
)

const (
	tooltipSideLeft   = "left"
	tooltipSideTop    = "top"
	tooltipSideRight  = "right"
	tooltipSideBottom = "bottom"
)

// Options describes a lightweight native tooltip request.
type Options struct {
	Name          string
	Text          string
	Side          string
	X             float64
	Y             float64
	TooltipWidth  float64
	TooltipHeight float64
	AnchorX       float64
	AnchorY       float64
	AnchorWidth   float64
	AnchorHeight  float64
	OwnerX        float64
	OwnerY        float64
	OwnerWidth    float64
	OwnerHeight   float64
	// IgnoreOwnerLeave skips leftover-owner dismiss. Automation hovers do not
	// move the OS pointer onto the trigger, so an idle cursor resting on the
	// owner would otherwise flash-close the overlay after the grace period.
	IgnoreOwnerLeave bool
}

// Show renders a native tooltip window that is independent of the UI
// launcher surface, so it can extend beyond the launcher bounds.
func Show(ctx context.Context, opts Options) {
	if opts.Text == "" {
		return
	}
	if opts.Name == "" {
		opts.Name = tooltipOverlayPrefix + "default"
	}
	width, estimatedHeight := estimateBounds(opts.Text)
	placement := computePlacement(opts, width, estimatedHeight)

	textoverlay.Show(textoverlay.Options{
		Window: overlay.WindowOptions{
			ID:               opts.Name,
			Topmost:          true,
			AbsolutePosition: true,
			Anchor:           placement.overlayAnchor,
			OffsetX:          placement.offsetX,
			OffsetY:          placement.offsetY,
			MinWidth:         tooltipMinWidthDip,
			MaxWidth:         tooltipMaxWidthDip,
			MaxHeight:        tooltipMaxHeightDip,
			CornerRadius:     8,
		},
		Message:  opts.Text,
		FontSize: tooltipFontSizeDip,
		Padding: woxwidget.Insets{
			Left: 11, Top: 8, Right: 11, Bottom: 8,
		},
	})
	startVisibilityTracking(opts.withBounds(placement.trackingX, placement.trackingY, width, placement.trackingHeight))

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

// tooltipTrackingClose reports whether OS-cursor polling may dismiss the overlay.
// The first samples after show often miss the cursor: creating the tooltip HWND
// generates owner WM_MOUSELEAVE, and automation hovers do not move the OS
// pointer. Closing before the cursor has been seen inside would flash-dismiss
// the overlay and hide-on-lost-focus would treat that as a real focus loss.
//
// After the dwell, the pointer may already be on another control in the same
// owner window. Waiting forever for a later enter would leave that leftover
// tooltip stuck. IgnoreOwnerLeave keeps a synthetic hover visible when the OS
// cursor never entered the trigger but still rests on the owner.
func tooltipTrackingClose(seenInside bool, inside bool, overOwner bool, positionOK bool, elapsed time.Duration, ignoreOwnerLeave bool) (shouldClose bool, nowSeenInside bool) {
	if !positionOK {
		// Wayland sessions can lose the cursor once it leaves every Wox window.
		// After it has been seen on the trigger, a missing sample is a leave.
		if seenInside && elapsed >= tooltipOwnerLeaveGrace {
			return true, seenInside
		}
		return false, seenInside
	}
	if inside {
		return false, true
	}
	if seenInside {
		return true, true
	}
	if overOwner && !ignoreOwnerLeave && elapsed >= tooltipOwnerLeaveGrace {
		return true, seenInside
	}
	return false, seenInside
}

// evaluateVisibility samples the OS cursor once and applies tooltipTrackingClose.
func evaluateVisibility(opts Options, seenInside bool, elapsed time.Duration) (shouldClose bool, nowSeenInside bool) {
	inside, overOwner, ok := cursorTooltipRelation(opts)
	return tooltipTrackingClose(seenInside, inside, overOwner, ok, elapsed, opts.IgnoreOwnerLeave)
}

func cursorTooltipRelation(opts Options) (inside bool, overOwner bool, ok bool) {
	point, ok := mouse.CurrentPosition()
	if !ok {
		return false, false, false
	}
	inside = rectContains(point.X, point.Y, opts.AnchorX, opts.AnchorY, opts.AnchorWidth, opts.AnchorHeight) ||
		rectContains(point.X, point.Y, opts.X, opts.Y, opts.TooltipWidth, opts.TooltipHeight)
	overOwner = rectContains(point.X, point.Y, opts.OwnerX, opts.OwnerY, opts.OwnerWidth, opts.OwnerHeight)
	return inside, overOwner, true
}

func rectContains(cursorX float64, cursorY float64, left float64, top float64, width float64, height float64) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return cursorX >= left && cursorX < left+width && cursorY >= top && cursorY < top+height
}

func (opts Options) withBounds(x float64, y float64, width float64, height float64) Options {
	return Options{
		Name:             opts.Name,
		Text:             opts.Text,
		Side:             opts.Side,
		X:                x,
		Y:                y,
		TooltipWidth:     width,
		TooltipHeight:    height,
		AnchorX:          opts.AnchorX,
		AnchorY:          opts.AnchorY,
		AnchorWidth:      opts.AnchorWidth,
		AnchorHeight:     opts.AnchorHeight,
		OwnerX:           opts.OwnerX,
		OwnerY:           opts.OwnerY,
		OwnerWidth:       opts.OwnerWidth,
		OwnerHeight:      opts.OwnerHeight,
		IgnoreOwnerLeave: opts.IgnoreOwnerLeave,
	}
}

type tooltipRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func (r tooltipRect) right() float64 {
	return r.X + r.Width
}

func (r tooltipRect) bottom() float64 {
	return r.Y + r.Height
}

func (r tooltipRect) isEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

type tooltipPlacement struct {
	overlayAnchor  int
	offsetX        float64
	offsetY        float64
	trackingX      float64
	trackingY      float64
	trackingHeight float64
}

// computePlacement keeps side-specific tooltip positioning in the native tooltip
// layer so UI only needs to report the anchor bounds.
func computePlacement(opts Options, width float64, height float64) tooltipPlacement {
	anchor := tooltipRect{
		X:      opts.AnchorX,
		Y:      opts.AnchorY,
		Width:  opts.AnchorWidth,
		Height: opts.AnchorHeight,
	}
	if anchor.isEmpty() {
		return tooltipPlacement{
			overlayAnchor:  overlay.AnchorTopLeft,
			offsetX:        opts.X,
			offsetY:        opts.Y,
			trackingX:      opts.X,
			trackingY:      opts.Y,
			trackingHeight: expandTrackingHeight(height),
		}
	}

	side, explicitSide := normalizeSide(opts.Side)
	workArea := resolveTooltipWorkArea(anchor)
	if !explicitSide {
		spaceBelow := workArea.bottom() - anchor.bottom()
		spaceAbove := anchor.Y - workArea.Y
		if !workArea.isEmpty() && spaceBelow < height+tooltipGapDip && spaceAbove > spaceBelow {
			side = tooltipSideTop
		} else {
			side = tooltipSideBottom
		}
	}

	trackingHeight := expandTrackingHeight(height)
	centerX := anchor.X + anchor.Width/2
	centerY := anchor.Y + anchor.Height/2
	if !workArea.isEmpty() {
		centerX = clampTooltipCoordinate(centerX, workArea.X+tooltipMarginDip+width/2, workArea.right()-tooltipMarginDip-width/2)
		centerY = clampTooltipCoordinate(centerY, workArea.Y+tooltipMarginDip+trackingHeight/2, workArea.bottom()-tooltipMarginDip-trackingHeight/2)
	}

	switch side {
	case tooltipSideLeft:
		offsetX := anchor.X - tooltipGapDip
		return tooltipPlacement{
			overlayAnchor:  overlay.AnchorRightCenter,
			offsetX:        offsetX,
			offsetY:        centerY,
			trackingX:      offsetX - width,
			trackingY:      centerY - trackingHeight/2,
			trackingHeight: trackingHeight,
		}
	case tooltipSideTop:
		offsetY := anchor.Y - tooltipGapDip
		return tooltipPlacement{
			overlayAnchor:  overlay.AnchorBottomCenter,
			offsetX:        centerX,
			offsetY:        offsetY,
			trackingX:      centerX - width/2,
			trackingY:      offsetY - trackingHeight,
			trackingHeight: trackingHeight,
		}
	case tooltipSideRight:
		offsetX := anchor.right() + tooltipGapDip
		return tooltipPlacement{
			overlayAnchor:  overlay.AnchorLeftCenter,
			offsetX:        offsetX,
			offsetY:        centerY,
			trackingX:      offsetX,
			trackingY:      centerY - trackingHeight/2,
			trackingHeight: trackingHeight,
		}
	default:
		offsetY := anchor.bottom() + tooltipGapDip
		return tooltipPlacement{
			overlayAnchor:  overlay.AnchorTopCenter,
			offsetX:        centerX,
			offsetY:        offsetY,
			trackingX:      centerX - width/2,
			trackingY:      offsetY,
			trackingHeight: trackingHeight,
		}
	}
}

func expandTrackingHeight(height float64) float64 {
	trackingHeight := height + tooltipTrackingSlackDip
	if trackingHeight > tooltipMaxHeightDip {
		return tooltipMaxHeightDip
	}
	return trackingHeight
}

func normalizeSide(side string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case tooltipSideLeft:
		return tooltipSideLeft, true
	case tooltipSideTop:
		return tooltipSideTop, true
	case tooltipSideRight:
		return tooltipSideRight, true
	case tooltipSideBottom:
		return tooltipSideBottom, true
	default:
		return "", false
	}
}

// resolveTooltipWorkArea returns the logical work area for the display that owns
// the anchor, matching the coordinate space reported by the UI launcher.
func resolveTooltipWorkArea(anchor tooltipRect) tooltipRect {
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return tooltipRect{}
	}

	display := selectTooltipDisplay(anchor, displays)
	workArea := display.WorkArea
	if workArea.IsEmpty() {
		workArea = display.Bounds
	}
	return screenRectToTooltipRect(workArea)
}

// selectTooltipDisplay prefers the display containing the anchor and otherwise
// falls back to the nearest display so off-edge anchors still get sane clamping.
func selectTooltipDisplay(anchor tooltipRect, displays []screen.Display) screen.Display {
	centerX := anchor.X + anchor.Width/2
	centerY := anchor.Y + anchor.Height/2
	best := displays[0]
	bestDistance := math.MaxFloat64

	for _, display := range displays {
		if displayRectContainsPoint(display.Bounds, centerX, centerY) {
			return display
		}

		distance := distanceSquaredToDisplay(display.Bounds, centerX, centerY)
		if distance < bestDistance || (distance == bestDistance && display.Primary) {
			best = display
			bestDistance = distance
		}
	}

	return best
}

func screenRectToTooltipRect(rect screen.Rect) tooltipRect {
	return tooltipRect{
		X:      float64(rect.X),
		Y:      float64(rect.Y),
		Width:  float64(rect.Width),
		Height: float64(rect.Height),
	}
}

func displayRectContainsPoint(rect screen.Rect, x float64, y float64) bool {
	return x >= float64(rect.X) && x < float64(rect.Right()) && y >= float64(rect.Y) && y < float64(rect.Bottom())
}

func distanceSquaredToDisplay(rect screen.Rect, x float64, y float64) float64 {
	dx := distanceToRange(x, float64(rect.X), float64(rect.Right()))
	dy := distanceToRange(y, float64(rect.Y), float64(rect.Bottom()))
	return dx*dx + dy*dy
}

func distanceToRange(value float64, minValue float64, maxValue float64) float64 {
	if value < minValue {
		return minValue - value
	}
	if value > maxValue {
		return value - maxValue
	}
	return 0
}

func clampTooltipCoordinate(value float64, minValue float64, maxValue float64) float64 {
	if maxValue < minValue {
		return minValue
	}
	return math.Max(minValue, math.Min(maxValue, value))
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

	fontSize := float64(tooltipFontSizeDip)
	width := 0.0
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			width += fontSize * tooltipSpaceWidthFactor
		case r <= unicode.MaxASCII:
			width += fontSize * tooltipAsciiWidthFactor
		default:
			width += fontSize * tooltipWideWidthFactor
		}
	}

	if width == 0 {
		return float64(utf8.RuneCountInString(text)) * fontSize * tooltipAsciiWidthFactor
	}
	return width
}

func maxTooltipDimension(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
