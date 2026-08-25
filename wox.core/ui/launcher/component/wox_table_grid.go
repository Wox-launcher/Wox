package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TableGridBorderWidth is the shared 1-unit stroke for table frames and separators.
const TableGridBorderWidth = float32(1)

// TableGridCellProps describes one collapsed table cell.
type TableGridCellProps struct {
	Width    float32
	Height   float32
	Color    woxui.Color
	Border   woxui.Color
	Trailing bool
	Bottom   bool
	Padding  woxwidget.Insets
	Child    woxwidget.Widget
}

// WoxTableGridFrame draws one outer 1px stroke above the cells so scrolling
// keeps a stable edge and adjacent cells do not stack into a 2px line.
func WoxTableGridFrame(width, height float32, border woxui.Color, child woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: child},
		{Child: woxwidget.Container{Width: width, Height: height, BorderColor: border, BorderWidth: TableGridBorderWidth}},
	}}
}

// WoxTableGridCell paints only the trailing and bottom separators. The last row
// and column leave those edges to WoxTableGridFrame.
func WoxTableGridCell(props TableGridCellProps) woxwidget.Container {
	cell := woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Color, Padding: props.Padding, Child: props.Child}
	if props.Trailing {
		cell.RightBorderColor = props.Border
		cell.RightBorderWidth = TableGridBorderWidth
	}
	if props.Bottom {
		cell.BottomBorderColor = props.Border
		cell.BottomBorderWidth = TableGridBorderWidth
	}
	return cell
}
