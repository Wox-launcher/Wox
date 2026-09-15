package component

import (
	"reflect"
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

// TestWoxSettingsTableFrameRoundsContinuousFills guards the actual paint commands,
// including the header clip at a nonzero origin.
func TestWoxSettingsTableFrameRoundsContinuousFills(t *testing.T) {
	border, header, body := woxui.Color{A: 40}, woxui.Color{A: 14}, woxui.Color{A: 5}
	frame := WoxSettingsTableFrame(400, 116, 36, border, header, body, woxwidget.Container{}).(woxwidget.Stack)
	painter := frame.Children[0].Child.(woxwidget.Painter)
	bounds := woxui.Rect{X: 12, Y: 20, Width: 400, Height: 116}
	var got, want woxui.DisplayList
	painter.Paint(&got, bounds)
	want.FillRoundedRect(bounds, 8, body)
	want.PushClipRect(woxui.Rect{X: 12, Y: 20, Width: 400, Height: 36})
	want.FillRoundedRect(bounds, 8, header)
	want.PopClipRect()
	if !reflect.DeepEqual(got, want) {
		t.Fatal("table fills must share a rounded silhouette with the header clipped to its band")
	}
	outline := frame.Children[2].Child.(woxwidget.Container)
	if outline.Radius != 8 || outline.BorderWidth != 1 || outline.BorderColor != border {
		t.Fatalf("unexpected outline: %#v", outline)
	}
}
