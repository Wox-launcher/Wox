package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxsvg "wox/util/svg"
)

func TestWoxLoadingIndicatorUsesLoopAnimation(t *testing.T) {
	indicator := WoxLoadingIndicator(20, woxui.Color{R: 10, G: 20, B: 30, A: 255}).(woxwidget.LoopAnimation)
	if indicator.Duration <= 0 {
		t.Fatal("loading indicator does not animate")
	}
	if _, ok := indicator.Builder(0.5).(woxwidget.Painter); !ok {
		t.Fatal("loading indicator does not paint its rotating ring")
	}
}

func TestWoxProgressIndicatorKeepsDeterminateAndIndeterminateStatesDistinct(t *testing.T) {
	color := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	determinate := WoxProgressIndicator(14, 10, false, color).(woxwidget.Image)
	if determinate.Source == nil || determinate.Width != 14 || determinate.Height != 14 {
		t.Fatal("determinate toolbar progress did not render its compact ring")
	}
	if _, ok := WoxProgressIndicator(14, 10, true, color).(woxwidget.LoopAnimation); !ok {
		t.Fatal("indeterminate toolbar progress did not animate")
	}
}

func TestWoxProgressIndicatorGrowsClockwise(t *testing.T) {
	image, err := woxsvg.Render(toolbarProgressSVG(25), 64, 64)
	if err != nil {
		t.Fatalf("render toolbar progress: %v", err)
	}
	rightAlpha := image.RGBAAt(56, 32).A
	leftAlpha := image.RGBAAt(8, 32).A
	if rightAlpha <= leftAlpha {
		t.Fatalf("25%% progress alpha right=%d left=%d, want clockwise growth", rightAlpha, leftAlpha)
	}
}
