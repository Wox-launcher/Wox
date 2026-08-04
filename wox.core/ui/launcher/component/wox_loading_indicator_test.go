package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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
