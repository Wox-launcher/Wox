package widget

import (
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

func TestLayoutBuilderReceivesContainerContentConstraints(t *testing.T) {
	builtWith := woxui.Size{}
	root := (Container{Width: 120, Height: 80, Padding: UniformInsets(10), Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
		builtWith = size
		return Container{Width: size.Width, Height: 20}
	}}}).layout(context{window: &fakeHostServices{}}, constraints{width: 200, height: 200})

	if builtWith.Width != 100 || builtWith.Height != 60 {
		t.Fatalf("layout builder constraints = %+v, want 100x60", builtWith)
	}
	if child := root.children[0].bounds; child.X != 10 || child.Y != 10 || child.Width != 100 || child.Height != 20 {
		t.Fatalf("layout builder child = %+v, want x/y 10 and size 100x20", child)
	}
}

func TestUnretainedScrollViewReportsMeasuredGeometry(t *testing.T) {
	viewport := float32(0)
	content := float32(0)
	(ScrollView{Width: 100, Height: 50, Child: Container{Width: 100, Height: 120}, OnGeometryChanged: func(measuredViewport, measuredContent float32) {
		viewport = measuredViewport
		content = measuredContent
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 100})

	if viewport != 50 || content != 120 {
		t.Fatalf("scroll geometry = %.0f/%.0f, want 50/120", viewport, content)
	}
}

func TestStackChildStretchesBetweenInsets(t *testing.T) {
	builtWith := woxui.Size{}
	root := (Stack{Width: 200, Height: 100, Children: []StackChild{{
		Left: 20, Right: 30, Top: 10, Bottom: 15, StretchWidth: true, StretchHeight: true,
		Child: Container{Width: 10, Height: 10, Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
			builtWith = size
			return Container{Width: 10, Height: 10, Color: woxui.Color{A: 255}}
		}}},
	}}}).layout(context{window: &fakeHostServices{}}, constraints{width: 200, height: 100})

	child := root.children[0].bounds
	if builtWith.Width != 150 || builtWith.Height != 75 || child.X != 20 || child.Y != 10 || child.Width != 150 || child.Height != 75 {
		t.Fatalf("stretched stack child = constraints %+v bounds %+v, want 150x75 at 20,10", builtWith, child)
	}
}

func TestFlexExpandedUsesRemainingMainAxisExtent(t *testing.T) {
	builtWith := woxui.Size{}
	root := (Flex{Axis: Horizontal, Gap: 10, Children: []Widget{
		Container{Width: 20, Height: 12},
		Expanded{Child: Container{Width: 10, Height: 12, Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
			builtWith = size
			return Container{Width: 10, Height: 12}
		}}}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 30})

	if root.bounds.Width != 100 || len(root.children) != 2 {
		t.Fatalf("expanded flex = bounds %+v children %d, want width 100 and two children", root.bounds, len(root.children))
	}
	if first, expanded := root.children[0].bounds, root.children[1].bounds; first.X != 0 || first.Width != 20 || expanded.X != 30 || expanded.Width != 70 {
		t.Fatalf("expanded children = first %+v expanded %+v, want widths 20/70 at x 0/30", first, expanded)
	}
	if builtWith.Width != 70 {
		t.Fatalf("expanded nested constraints = %+v, want tight width 70", builtWith)
	}
}

func TestFlexExpandedDistributesRemainingExtentByWeight(t *testing.T) {
	root := (Flex{Axis: Horizontal, Children: []Widget{
		Container{Width: 10, Height: 12},
		Expanded{Flex: 1, Child: Container{Height: 12}},
		Expanded{Flex: 2, Child: Container{Height: 12}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 30})

	if first, second := root.children[1].bounds, root.children[2].bounds; first.X != 10 || first.Width != 30 || second.X != 40 || second.Width != 60 {
		t.Fatalf("weighted expanded children = first %+v second %+v, want 30/60 widths at x 10/40", first, second)
	}
}

func TestFlexFlexibleUsesLooseWeightedShare(t *testing.T) {
	root := (Flex{Axis: Horizontal, Gap: 10, Children: []Widget{
		Container{Width: 20, Height: 12},
		Flexible{Child: Container{Width: 20, Height: 12}},
		Expanded{Child: Container{Width: 10, Height: 12}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 120, height: 30})

	if root.bounds.Width != 120 {
		t.Fatalf("flexible flex bounds = %+v, want width 120", root.bounds)
	}
	if flexible, expanded := root.children[1].bounds, root.children[2].bounds; flexible.X != 30 || flexible.Width != 20 || expanded.X != 60 || expanded.Width != 40 {
		t.Fatalf("flexible children = loose %+v expanded %+v, want widths 20/40 at x 30/60", flexible, expanded)
	}
}

func TestFlexFlexibleSupportsVerticalAxis(t *testing.T) {
	root := (Flex{Axis: Vertical, Gap: 10, Children: []Widget{
		Container{Width: 12, Height: 20},
		Flexible{Child: Container{Width: 12, Height: 15}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 30, height: 100})

	if root.bounds.Height != 100 || root.children[1].bounds.Y != 30 || root.children[1].bounds.Height != 15 {
		t.Fatalf("vertical flexible = root %+v child %+v, want height 100 and loose child height 15 at y 30", root.bounds, root.children[1].bounds)
	}
}

func TestFlexNilFlexibleDoesNotConsumeRemainingShare(t *testing.T) {
	root := (Flex{Axis: Horizontal, Children: []Widget{
		Container{Width: 10, Height: 12},
		Flexible{},
		Expanded{Child: Container{Height: 12}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 30})

	if len(root.children) != 2 || root.children[1].bounds.Width != 90 {
		t.Fatalf("nil flexible children = %d expanded %+v, want two children and width 90", len(root.children), root.children[1].bounds)
	}
}

func TestFlexOverflowReportsDebugDiagnostic(t *testing.T) {
	tree := &elementTree{}
	root := (Flex{Axis: Horizontal, Gap: 10, Children: []Widget{
		Container{Width: 60, Height: 12},
		Container{Width: 50, Height: 12},
	}}).layout(context{window: &fakeHostServices{}, debug: &repaintDebugFrame{mode: RepaintDebugCounts}, elements: tree}, constraints{width: 100, height: 30})

	if root.bounds.Width != 100 {
		t.Fatalf("overflowing flex bounds = %+v, want constrained width 100", root.bounds)
	}
	if len(tree.diagnostics) != 1 || !strings.Contains(tree.diagnostics[0], "horizontal flex overflowed by 20.0") {
		t.Fatalf("overflow diagnostics = %v", tree.diagnostics)
	}
}

func TestFlexMainAxisSpaceBetweenUsesUnusedExtent(t *testing.T) {
	root := (Flex{Axis: Horizontal, MainAxisAlignment: MainAxisSpaceBetween, Children: []Widget{
		Container{Width: 20, Height: 10},
		Container{Width: 30, Height: 10},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 20})

	if root.bounds.Width != 100 || root.children[1].bounds.X != 70 {
		t.Fatalf("space-between flex = bounds %+v second %+v, want width 100 and second x 70", root.bounds, root.children[1].bounds)
	}
}

func TestFlexCrossAxisStretchFillsAvailableExtent(t *testing.T) {
	builtWith := woxui.Size{}
	root := (Flex{Axis: Horizontal, CrossAxisAlignment: CrossAxisStretch, Children: []Widget{
		Container{Width: 20, Height: 10, Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
			builtWith = size
			return Container{Width: 20, Height: 10}
		}}},
		Container{Width: 30, Height: 15},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 40})

	if root.bounds.Height != 40 || root.children[0].bounds.Height != 40 || root.children[1].bounds.Height != 40 {
		t.Fatalf("stretched flex = root %+v first %+v second %+v, want height 40", root.bounds, root.children[0].bounds, root.children[1].bounds)
	}
	if builtWith.Height != 40 {
		t.Fatalf("stretched nested constraints = %+v, want tight height 40", builtWith)
	}
}

func TestConstrainedAppliesBoundsAndFill(t *testing.T) {
	bounded := (Constrained{MinWidth: 30, MaxHeight: 20, Child: Container{Width: 10, Height: 40}}).layout(
		context{window: &fakeHostServices{}}, constraints{width: 100, height: 100},
	)
	if bounded.bounds.Width != 30 || bounded.bounds.Height != 20 {
		t.Fatalf("constrained bounds = %+v, want 30x20", bounded.bounds)
	}

	builtWith := woxui.Size{}
	filled := (Constrained{FillWidth: true, FillHeight: true, Child: Container{Width: 10, Height: 10, Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
		builtWith = size
		return Container{Width: 10, Height: 10}
	}}}}).layout(
		context{window: &fakeHostServices{}}, constraints{width: 80, height: 50},
	)
	if filled.bounds.Width != 80 || filled.bounds.Height != 50 {
		t.Fatalf("filled bounds = %+v, want 80x50", filled.bounds)
	}
	if builtWith != (woxui.Size{Width: 80, Height: 50}) {
		t.Fatalf("filled nested constraints = %+v, want tight 80x50", builtWith)
	}
}

func TestGridPassesTightCellWidthIntoNestedLayout(t *testing.T) {
	builtWith := woxui.Size{}
	(Grid{Columns: 2, ColumnGap: 8, Children: []Widget{
		Container{Width: 10, Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
			builtWith = size
			return Container{Width: 10, Height: 20}
		}}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 120, height: 200})

	if builtWith.Width != 56 {
		t.Fatalf("grid nested constraints = %+v, want tight cell width 56", builtWith)
	}
}

func TestGridDerivesAdaptiveColumnsAndMeasuredRows(t *testing.T) {
	root := (Grid{MinColumnWidth: 100, ColumnGap: 10, RowGap: 5, Children: []Widget{
		Container{Height: 20},
		Container{Height: 30},
		Container{Height: 25},
		Container{Height: 40},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 320, height: 200})

	if root.bounds.Width != 320 || root.bounds.Height != 75 {
		t.Fatalf("adaptive grid bounds = %+v, want 320x75", root.bounds)
	}
	if first, third, fourth := root.children[0].bounds, root.children[2].bounds, root.children[3].bounds; first.Width != 100 || third.X != 220 || fourth.X != 0 || fourth.Y != 35 {
		t.Fatalf("adaptive grid cells = first %+v third %+v fourth %+v", first, third, fourth)
	}
}

func TestGridPreservesFixedCellGeometry(t *testing.T) {
	root := (Grid{Columns: 2, CellWidth: 44, CellHeight: 44, ColumnGap: 8, RowGap: 6, Children: []Widget{
		Container{Width: 44, Height: 44},
		Container{Width: 44, Height: 44},
		Container{Width: 44, Height: 44},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 120, height: 200})

	if root.bounds.Height != 94 || root.children[1].bounds.X != 52 || root.children[2].bounds.Y != 50 {
		t.Fatalf("fixed grid = bounds %+v second %+v third %+v, want height 94 and offsets 52/50", root.bounds, root.children[1].bounds, root.children[2].bounds)
	}
}

func TestGridAndWrapReportVerticalOverflowDiagnostics(t *testing.T) {
	debug := &repaintDebugFrame{mode: RepaintDebugCounts}
	sparseTree := &elementTree{}
	(Grid{Columns: 4, CellWidth: 100, ColumnGap: 10, Children: []Widget{
		Container{Height: 20},
	}}).layout(context{window: &fakeHostServices{}, debug: debug, elements: sparseTree}, constraints{width: 320, height: 100})
	if len(sparseTree.diagnostics) != 0 {
		t.Fatalf("sparse grid overflow diagnostics = %v, want no warning for unused columns", sparseTree.diagnostics)
	}

	gridTree := &elementTree{}
	(Grid{Columns: 1, RowGap: 10, Children: []Widget{
		Container{Height: 60},
		Container{Height: 60},
	}}).layout(context{window: &fakeHostServices{}, debug: debug, elements: gridTree}, constraints{width: 100, height: 100})
	if len(gridTree.diagnostics) != 1 || !strings.Contains(gridTree.diagnostics[0], "vertical grid overflowed by 30.0") {
		t.Fatalf("grid overflow diagnostics = %v", gridTree.diagnostics)
	}

	wrapTree := &elementTree{}
	(Wrap{RunGap: 10, Children: []Widget{
		Container{Width: 60, Height: 60},
		Container{Width: 60, Height: 60},
	}}).layout(context{window: &fakeHostServices{}, debug: debug, elements: wrapTree}, constraints{width: 100, height: 100})
	if len(wrapTree.diagnostics) != 1 || !strings.Contains(wrapTree.diagnostics[0], "vertical wrap overflowed by 30.0") {
		t.Fatalf("wrap overflow diagnostics = %v", wrapTree.diagnostics)
	}
}

func TestContainerPaintsCollapsedSideBorders(t *testing.T) {
	color := woxui.Color{R: 80, G: 90, B: 100, A: 200}
	root := (Container{
		Width: 40, Height: 20,
		RightBorderColor: color, RightBorderWidth: 1,
		BottomBorderColor: color, BottomBorderWidth: 1,
	}).layout(context{}, constraints{width: 40, height: 20})

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)
	expected := &woxui.DisplayList{}
	expected.FillRect(woxui.Rect{Y: 19, Width: 40, Height: 1}, color)
	expected.FillRect(woxui.Rect{X: 39, Width: 1, Height: 19}, color)
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
}

func TestContainerLeftBorderKeepsFullHeightWithoutOtherEdges(t *testing.T) {
	color := woxui.Color{R: 19, G: 121, B: 210, A: 255}
	root := (Container{Width: 80, Height: 24, LeftBorderColor: color, LeftBorderWidth: 3}).layout(
		context{}, constraints{width: 80, height: 24},
	)

	actual := &woxui.DisplayList{}
	root.draw(actual, 0, 0, false, false, false, nil)
	expected := &woxui.DisplayList{}
	expected.FillRect(woxui.Rect{Width: 3, Height: 24}, color)
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
}
