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
