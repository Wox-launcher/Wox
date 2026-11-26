package ui

import (
	"testing"
	"time"

	woxui "wox/ui/runtime"
	"wox/util/screen"
)

func TestPerformanceDiagnosticsPreserveDisplayUnitsAndFramePhases(t *testing.T) {
	primary := screen.Display{Primary: true, Scale: 1, Bounds: screen.Rect{Width: 1920}, PixelBounds: screen.Rect{Width: 1920}}
	external := screen.Display{Scale: 1.5, Bounds: screen.Rect{X: -1280, Width: 1280}, PixelBounds: screen.Rect{X: -1920, Width: 1920}}
	frames := woxui.FrameMetricsSnapshot{PresentedFrameCount: 3, NativePresent: woxui.FramePhaseMetrics{Samples: 3, TotalMicroseconds: 4250}}
	got := buildPerformanceSnapshot(RenderModeSoftware, []screen.Display{primary, external}, []time.Duration{1500 * time.Microsecond}, frames)
	if got.RenderMode != RenderModeSoftware || len(got.Displays) != 2 || !got.Displays[0].Primary ||
		got.Displays[1].LogicalBounds != external.Bounds || got.Displays[1].PixelBounds != external.PixelBounds ||
		got.Displays[1].Scale != 1.5 || got.WakeMilliseconds[0] != 1.5 {
		t.Fatalf("incorrect display or wake diagnostics: %+v", got)
	}
	if got.PresentedFrameCount != 3 || got.FramePhases[5].Phase != woxui.FrameMetricNativePresent ||
		got.FramePhases[5].FramePhaseMetrics != frames.NativePresent {
		t.Fatalf("incorrect frame diagnostics: %+v", got)
	}
}
