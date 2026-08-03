package widget

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestWrapTextLinesUsesAvailableWidthForMixedCJKText(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 2}

	mixed := wrapTextLines(measurer, "帮助我们了解 Wox 的整体使用情况", style, 16, 0)
	if mixed[0] != "帮助我们了解 Wox 的整体使用" {
		t.Fatalf("mixed CJK first line = %q, want available width filled", mixed[0])
	}

	latin := wrapTextLines(measurer, "alpha beta gamma", style, 11, 0)
	if latin[0] != "alpha beta" {
		t.Fatalf("Latin first line = %q, want whitespace boundary preserved", latin[0])
	}
}
