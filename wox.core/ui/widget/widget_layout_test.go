package widget

import (
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
		Child: LayoutBuilder{Build: func(size woxui.Size) Widget {
			builtWith = size
			return Container{Color: woxui.Color{A: 255}}
		}},
	}}}).layout(context{window: &fakeHostServices{}}, constraints{width: 200, height: 100})

	child := root.children[0].bounds
	if builtWith.Width != 150 || builtWith.Height != 75 || child.X != 20 || child.Y != 10 || child.Width != 150 || child.Height != 75 {
		t.Fatalf("stretched stack child = constraints %+v bounds %+v, want 150x75 at 20,10", builtWith, child)
	}
}

func TestFlexExpandedUsesRemainingMainAxisExtent(t *testing.T) {
	root := (Flex{Axis: Horizontal, Gap: 10, Children: []Widget{
		Container{Width: 20, Height: 12},
		Expanded{Child: Container{Height: 12}},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 30})

	if root.bounds.Width != 100 || len(root.children) != 2 {
		t.Fatalf("expanded flex = bounds %+v children %d, want width 100 and two children", root.bounds, len(root.children))
	}
	if first, expanded := root.children[0].bounds, root.children[1].bounds; first.X != 0 || first.Width != 20 || expanded.X != 30 || expanded.Width != 70 {
		t.Fatalf("expanded children = first %+v expanded %+v, want widths 20/70 at x 0/30", first, expanded)
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
	root := (Flex{Axis: Horizontal, CrossAxisAlignment: CrossAxisStretch, Children: []Widget{
		Container{Width: 20, Height: 10},
		Container{Width: 30, Height: 15},
	}}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: 40})

	if root.bounds.Height != 40 || root.children[0].bounds.Height != 40 || root.children[1].bounds.Height != 40 {
		t.Fatalf("stretched flex = root %+v first %+v second %+v, want height 40", root.bounds, root.children[0].bounds, root.children[1].bounds)
	}
}

func TestConstrainedAppliesBoundsAndFill(t *testing.T) {
	bounded := (Constrained{MinWidth: 30, MaxHeight: 20, Child: Container{Width: 10, Height: 40}}).layout(
		context{window: &fakeHostServices{}}, constraints{width: 100, height: 100},
	)
	if bounded.bounds.Width != 30 || bounded.bounds.Height != 20 {
		t.Fatalf("constrained bounds = %+v, want 30x20", bounded.bounds)
	}

	filled := (Constrained{FillWidth: true, FillHeight: true, Child: Container{Width: 10, Height: 10}}).layout(
		context{window: &fakeHostServices{}}, constraints{width: 80, height: 50},
	)
	if filled.bounds.Width != 80 || filled.bounds.Height != 50 {
		t.Fatalf("filled bounds = %+v, want 80x50", filled.bounds)
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
