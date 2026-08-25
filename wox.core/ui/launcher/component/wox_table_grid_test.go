package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxTableGridFrameUsesSingleOuterStroke(t *testing.T) {
	border := woxui.Color{R: 80, G: 90, B: 100, A: 200}
	frame := WoxTableGridFrame(200, 80, border, woxwidget.Container{Width: 200, Height: 80}).(woxwidget.Stack)
	if len(frame.Children) != 2 {
		t.Fatalf("table frame children = %d, want content plus one outer stroke", len(frame.Children))
	}
	outline := frame.Children[1].Child.(woxwidget.Container)
	if outline.BorderWidth != TableGridBorderWidth || outline.BorderColor != border || outline.Color.A != 0 {
		t.Fatalf("outer table stroke = %#v, want a single 1px frame", outline)
	}
}

func TestWoxTableGridCellOmitsSharedEdges(t *testing.T) {
	border := woxui.Color{A: 180}
	interior := WoxTableGridCell(TableGridCellProps{Width: 80, Height: 32, Border: border, Trailing: true, Bottom: true})
	if interior.BorderWidth != 0 || interior.RightBorderWidth != TableGridBorderWidth || interior.BottomBorderWidth != TableGridBorderWidth {
		t.Fatalf("interior cell = %#v, want collapsed right+bottom", interior)
	}
	corner := WoxTableGridCell(TableGridCellProps{Width: 80, Height: 32, Border: border})
	if corner.BorderWidth != 0 || corner.RightBorderWidth != 0 || corner.BottomBorderWidth != 0 {
		t.Fatalf("last cell = %#v, want the outer frame to own that corner", corner)
	}
}
