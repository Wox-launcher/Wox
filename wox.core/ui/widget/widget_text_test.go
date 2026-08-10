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

func TestTextBlockReflowsPrecomputedLayoutWhenWidthChanges(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 10}
	value := "alpha beta gamma delta"
	precomputed := layoutTextBlock(measurer, value, style, 100, 0, 10)
	expected := layoutTextBlock(measurer, value, style, 25, 0, 10)
	tree := &elementTree{}

	root := (TextBlock{Value: value, Style: style, LineHeight: 10, Layout: &precomputed}).layout(
		context{window: measurer, debug: &repaintDebugFrame{mode: RepaintDebugCounts}, elements: tree},
		constraints{width: 25, height: 100},
	)

	if root.bounds.Height != expected.Size.Height || root.bounds.Height == precomputed.Size.Height {
		t.Fatalf("reflowed height = %.0f, want %.0f instead of precomputed %.0f", root.bounds.Height, expected.Size.Height, precomputed.Size.Height)
	}
	if len(tree.diagnostics) != 1 || tree.diagnostics[0] != "text layout measured at width 100.0 was reflowed for width 25.0" {
		t.Fatalf("text layout diagnostics = %v", tree.diagnostics)
	}
}

func TestTextBlockReflowsLayoutMeasuredAtZeroWidth(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 10}
	value := "alpha beta"
	precomputed := layoutTextBlock(measurer, value, style, 0, 0, 10)
	expected := layoutTextBlock(measurer, value, style, 25, 0, 10)

	root := (TextBlock{Value: value, Style: style, LineHeight: 10, Layout: &precomputed}).layout(
		context{window: measurer}, constraints{width: 25, height: 100},
	)

	if !precomputed.HasConstraintWidth || root.bounds.Height != expected.Size.Height {
		t.Fatalf("zero-width layout contract = known %v height %.0f, want known width and height %.0f", precomputed.HasConstraintWidth, root.bounds.Height, expected.Size.Height)
	}
}
