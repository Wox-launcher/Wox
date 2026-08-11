package launcher

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestLauncherQueryChromeMetricsKeepsBottomPaddingWithBottomQuery(t *testing.T) {
	padding := woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 10}

	height, headerPadding := launcherQueryChromeMetrics(55, padding, true)
	if height != 65 {
		t.Fatalf("bottom-query chrome height = %.0f, want 65", height)
	}
	if headerPadding.Top != 0 || headerPadding.Bottom != 10 || headerPadding.Left != 8 || headerPadding.Right != 8 {
		t.Fatalf("bottom-query header padding = %+v, want top cleared and bottom kept", headerPadding)
	}

	height, headerPadding = launcherQueryChromeMetrics(55, padding, false)
	if height != 65 {
		t.Fatalf("top-query chrome height = %.0f, want 65", height)
	}
	if headerPadding.Top != 10 || headerPadding.Bottom != 0 || headerPadding.Left != 8 || headerPadding.Right != 8 {
		t.Fatalf("top-query header padding = %+v, want bottom cleared and top kept", headerPadding)
	}
}
