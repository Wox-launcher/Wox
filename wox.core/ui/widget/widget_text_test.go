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

func TestTextBlockShrinkWrapUsesContentWidthUpToLimit(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 10}

	short := MeasureStateless(measurer, TextBlock{Value: "sync", Style: style, Width: 100, MaxLines: 1, ShrinkWrap: true}, 100)
	if short.Width != 20 {
		t.Fatalf("short shrink-wrapped width = %v, want 20", short.Width)
	}

	long := MeasureStateless(measurer, TextBlock{Value: "a long sync mode label", Style: style, Width: 40, MaxLines: 1, ShrinkWrap: true}, 100)
	if long.Width != 40 {
		t.Fatalf("long shrink-wrapped width = %v, want truncation limit 40", long.Width)
	}
}
