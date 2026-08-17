package widget

import (
	"math"
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

func TestTextBlockPaintsWhenFontLineHeightExceedsBox(t *testing.T) {
	style := woxui.TextStyle{Size: 13}
	color := woxui.Color{R: 240, G: 240, B: 240, A: 255}
	value := "继续上次查询"
	root := (TextBlock{Value: value, Style: style, Width: 120, Height: 18, MaxLines: 1, Color: color}).layout(
		context{window: tallLineMeasurer{height: 22}}, constraints{width: 120, height: 18},
	)

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)

	expected := &woxui.DisplayList{}
	expected.DrawText(value, woxui.Rect{Width: 120, Height: 18}, style, color)
	if err := actual.Compare(expected); err != nil {
		t.Fatalf("tall CJK line in 18px slot: %v", err)
	}
}

func TestTextBlockAlignmentYCentersTallCJKLineInSlot(t *testing.T) {
	style := woxui.TextStyle{Size: 13}
	color := woxui.Color{R: 240, G: 240, B: 240, A: 255}
	value := "快捷键"
	root := (TextBlock{Value: value, Style: style, Width: 120, Height: 18, LineHeight: 18, MaxLines: 1, AlignmentY: 0.5, Color: color}).layout(
		context{window: tallLineMeasurer{height: 22}}, constraints{width: 120, height: 18},
	)

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)

	expected := &woxui.DisplayList{}
	expected.PushClipRect(woxui.Rect{Width: 120, Height: 18})
	expected.DrawText(value, woxui.Rect{Y: -2, Width: 120, Height: 22}, style, color)
	expected.PopClipRect()
	if err := actual.Compare(expected); err != nil {
		t.Fatalf("aligned CJK line in 18px slot: %v", err)
	}
}

func TestTextBlockSkipsLinesThatStartPastTheBox(t *testing.T) {
	style := woxui.TextStyle{Size: 10}
	color := woxui.Color{A: 255}
	layout := TextBlockLayout{
		Lines: []string{"alpha", "beta"}, Size: woxui.Size{Width: 30, Height: 36},
		LineHeight: 18, ConstraintWidth: 30, HasConstraintWidth: true,
	}
	root := (TextBlock{Value: "alpha beta", Style: style, Width: 30, Height: 18, LineHeight: 18, MaxLines: 2, Color: color, Layout: &layout}).layout(
		context{window: &fakeHostServices{}}, constraints{width: 30, height: 18},
	)

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)

	expected := &woxui.DisplayList{}
	expected.DrawText("alpha", woxui.Rect{Width: 30, Height: 18}, style, color)
	if err := actual.Compare(expected); err != nil {
		t.Fatalf("overflowing second line: %v", err)
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

func TestTextBlockUnboundedHeightHonorsMaxLines(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 10}
	value := "alpha beta gamma delta"
	block := TextBlock{Value: value, Style: style, Width: 25, MaxLines: 2, LineHeight: 10}

	root := block.layout(context{window: measurer}, constraints{width: 25, height: math.MaxFloat32})
	expected := layoutTextBlock(measurer, value, style, 25, 2, 10)
	if root.bounds.Height != expected.Size.Height || expected.Size.Height != 20 {
		t.Fatalf("unbounded MaxLines 2 height = %.0f lines %d, want 20px across %d wrapped lines", root.bounds.Height, len(expected.Lines), 2)
	}

	unlimited := TextBlock{Value: value, Style: style, Width: 25, LineHeight: 10}.layout(
		context{window: measurer}, constraints{width: 25, height: math.MaxFloat32},
	)
	full := layoutTextBlock(measurer, value, style, 25, 0, 10)
	if unlimited.bounds.Height != full.Size.Height || full.Size.Height <= 20 {
		t.Fatalf("unbounded unlimited height = %.0f, want full wrap %.0f taller than two lines", unlimited.bounds.Height, full.Size.Height)
	}
}

func TestTextBlockBoundedHeightStillClipsMaxLines(t *testing.T) {
	measurer := &fakeHostServices{}
	style := woxui.TextStyle{Size: 10}
	root := (TextBlock{Value: "alpha beta gamma", Style: style, Width: 25, MaxLines: 2, LineHeight: 10}).layout(
		context{window: measurer}, constraints{width: 25, height: 10},
	)
	if root.bounds.Height != 10 {
		t.Fatalf("bounded height = %.0f, want one 10px line", root.bounds.Height)
	}
}

type tallLineMeasurer struct {
	height float32
}

func (m tallLineMeasurer) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(len([]rune(text))) * max(style.Size/2, 1), Height: m.height}}, nil
}
