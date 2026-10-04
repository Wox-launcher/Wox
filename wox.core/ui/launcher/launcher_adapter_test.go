package launcher

import "testing"

func TestResultTailWidthBudget(t *testing.T) {
	app := &App{}
	// A nil window uses zero text metrics; enough chips still fill the budget
	// through their padding without requiring native font measurement.
	tails := make([]resultTail, 40)
	for i := range tails {
		tails[i] = resultTail{Type: "text", Text: "metadata"}
	}
	for _, tc := range []struct {
		width, scale, want float32
	}{
		{900, 1, 585},
		{240, 1, 145},
		{20, 1, 0},
		{900, 0.9, 586},
		{900, 1.1, 583},
	} {
		metrics := launcherDensityMetrics{scale: tc.scale}
		_, width, _ := app.resultTailViewProps(tails, tc.width, metrics, 1.5)
		if width != tc.want {
			t.Fatalf("row %.0f, density %.1f: tail width %.0f, want %.0f", tc.width, tc.scale, width, tc.want)
		}
	}
	_, width, _ := app.resultTailViewProps(tails[:1], 900, launcherDensityMetrics{scale: 1}, 1)
	if width != 24 {
		t.Fatalf("one small tail should shrink to its content, got width %.0f", width)
	}
}
