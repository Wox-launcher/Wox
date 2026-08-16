package timeroverlay

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestTimerSizeExpandsOnceDetailsAreVisible(t *testing.T) {
	compact := timerSize(woxui.Size{Width: 60, Height: 24}, woxui.Size{}, false, true)
	expanded := timerSize(woxui.Size{Width: 60, Height: 24}, woxui.Size{Width: 90, Height: 14}, true, true)
	if expanded.Width <= compact.Width || expanded.Height <= compact.Height {
		t.Fatalf("expanded size = %+v, compact size = %+v", expanded, compact)
	}
}

func TestTimerSizeLeavesTextMeasurementSlack(t *testing.T) {
	countdown := woxui.Size{Width: 60, Height: 24}
	compact := timerSize(countdown, woxui.Size{}, false, true)
	textWidth := compact.Width - timerHorizontalPadding*2
	if textWidth < countdown.Width+timerTextSlack {
		t.Fatalf("timer text width = %v, want at least %v", textWidth, countdown.Width+timerTextSlack)
	}
}

func TestTimerHoverChangesOnlyOnBoundaryTransitions(t *testing.T) {
	if !nextTimerHovered(false, woxui.PointerEnter) {
		t.Fatal("pointer enter should expand the timer")
	}
	if nextTimerHovered(true, woxui.PointerLeave) {
		t.Fatal("pointer leave should collapse the timer")
	}
	if nextTimerHovered(false, woxui.PointerMove) {
		t.Fatal("queued pointer move after leave should not expand the timer")
	}
}
