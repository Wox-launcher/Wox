package widget

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

// DebugRepaintEnvironment selects Boundary cache verification or repaint visualization.
const DebugRepaintEnvironment = "WOX_DEBUG_REPAINT"

// RepaintDebugMode controls optional incremental-rendering diagnostics.
type RepaintDebugMode string

const (
	RepaintDebugOff     RepaintDebugMode = "off"
	RepaintDebugRainbow RepaintDebugMode = "rainbow"
	RepaintDebugCounts  RepaintDebugMode = "counts"
	RepaintDebugVerify  RepaintDebugMode = "verify"
)

type boundaryRepaint struct {
	node         *node
	repaintCount uint64
	recentCount  int
}

type repaintDebugFrame struct {
	mode          RepaintDebugMode
	now           time.Time
	repaints      []boundaryRepaint
	repaintRegion woxui.Rect
	repaintCount  uint64
}

func parseRepaintDebugMode(value string) (RepaintDebugMode, error) {
	switch RepaintDebugMode(strings.ToLower(strings.TrimSpace(value))) {
	case "", RepaintDebugOff:
		return RepaintDebugOff, nil
	case RepaintDebugRainbow:
		return RepaintDebugRainbow, nil
	case RepaintDebugCounts:
		return RepaintDebugCounts, nil
	case RepaintDebugVerify:
		return RepaintDebugVerify, nil
	default:
		return RepaintDebugOff, fmt.Errorf("unsupported repaint debug mode %q", value)
	}
}

func repaintDebugModeFromEnvironment() RepaintDebugMode {
	mode, err := parseRepaintDebugMode(os.Getenv(DebugRepaintEnvironment))
	if err != nil {
		return RepaintDebugOff
	}
	return mode
}

func (f *repaintDebugFrame) draw(displayList *woxui.DisplayList) {
	if f == nil || displayList == nil {
		return
	}
	switch f.mode {
	case RepaintDebugRainbow:
		if f.repaintRegion.Width > 0 && f.repaintRegion.Height > 0 {
			displayList.StrokeRoundedRect(f.repaintRegion, 0, 2, repaintRainbowColor(f.repaintCount))
		}
	case RepaintDebugCounts:
		for _, repaint := range f.repaints {
			if repaint.node != nil {
				alpha := uint8(min(180, 28+repaint.recentCount*18))
				displayList.FillRoundedRect(repaint.node.bounds, 3, woxui.Color{R: 255, G: 64, A: alpha})
			}
		}
	}
}

func repaintRainbowColor(repaintCount uint64) woxui.Color {
	hue := math.Mod(float64(repaintCount)*0.6180339887498949, 1)
	sector := hue * 6
	fraction := sector - math.Floor(sector)
	low := uint8(math.Round((1 - fraction) * 255))
	high := uint8(math.Round(fraction * 255))
	switch int(sector) % 6 {
	case 0:
		return woxui.Color{R: 255, G: high, A: 255}
	case 1:
		return woxui.Color{R: low, G: 255, A: 255}
	case 2:
		return woxui.Color{G: 255, B: high, A: 255}
	case 3:
		return woxui.Color{G: low, B: 255, A: 255}
	case 4:
		return woxui.Color{R: high, B: 255, A: 255}
	default:
		return woxui.Color{R: 255, B: low, A: 255}
	}
}
