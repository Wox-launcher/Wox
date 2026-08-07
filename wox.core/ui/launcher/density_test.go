package launcher

import (
	"testing"
)

func TestLauncherDensityMetricsMatchFlutterBuckets(t *testing.T) {
	tests := []struct {
		density                            string
		query, result, toolbar, refinement float32
	}{
		{density: "compact", query: 50, result: 45, toolbar: 36, refinement: 40},
		{density: "normal", query: 55, result: 50, toolbar: 40, refinement: 44},
		{density: "comfortable", query: 61, result: 55, toolbar: 44, refinement: 48},
	}

	for _, test := range tests {
		metrics := launcherDensityMetricsFor(test.density)
		if metrics.queryBoxHeight != test.query || metrics.resultRowBaseHeight != test.result || metrics.toolbarHeight != test.toolbar || metrics.refinementBarHeight != test.refinement {
			t.Fatalf("%s metrics = %.0f/%.0f/%.0f/%.0f, want %.0f/%.0f/%.0f/%.0f", test.density, metrics.queryBoxHeight, metrics.resultRowBaseHeight, metrics.toolbarHeight, metrics.refinementBarHeight, test.query, test.result, test.toolbar, test.refinement)
		}
	}
}

func TestLauncherQueryHeightFollowsMultilineLimit(t *testing.T) {
	metrics := launcherDensityMetricsFor("normal")
	want := float32(123)
	if got := metrics.queryBoxHeightForText("one\ntwo\nthree", metrics.queryLineHeight(0)); got != want {
		t.Fatalf("three-line query height = %v, want %v", got, want)
	}
	if got := launcherQueryLineCount("one\ntwo\nthree\nfour\nfive"); got != launcherQueryMaxLines {
		t.Fatalf("query line count = %d, want max %d", got, launcherQueryMaxLines)
	}
}

func TestLauncherQueryLineHeightExpandsForConfiguredFont(t *testing.T) {
	metrics := launcherDensityMetricsFor("normal")
	if got := metrics.queryLineHeight(37.2); got != 38 {
		t.Fatalf("measured query line height = %v, want 38", got)
	}
}
