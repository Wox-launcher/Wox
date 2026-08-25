package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	tableSurfaceHeaderHeight = float32(36)
	tableSurfaceRowHeight    = float32(36)
	tableSurfaceEmptyHeight  = float32(82)
	tableSurfaceBorderWidth  = float32(1)
)

// tableSurfaceStyle keeps every column-based table on the same theme-derived visual tokens.
type tableSurfaceStyle struct {
	headerBackground woxui.Color
	bodyBackground   woxui.Color
	headerText       woxui.Color
	border           woxui.Color
}

// newTableSurfaceStyle resolves the shared table colors for the active theme.
func newTableSurfaceStyle(theme woxcomponent.Theme) tableSurfaceStyle {
	return tableSurfaceStyle{
		headerBackground: tableSurfaceAlpha(theme.ResultTitle, 14),
		bodyBackground:   tableSurfaceAlpha(theme.ResultTitle, 5),
		headerText:       tableSurfaceAlpha(theme.ResultTitle, 224),
		border:           theme.PreviewSplit,
	}
}

func tableSurfaceAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// tableSurfaceCell paints one cell with collapsed separators: only the trailing
// and bottom edges. A full per-cell border would stack into a 2px internal line.
func tableSurfaceCell(width, height float32, fill woxui.Color, style tableSurfaceStyle, trailing, bottom bool, padding woxwidget.Insets, child woxwidget.Widget) woxwidget.Container {
	cell := woxwidget.Container{Width: width, Height: height, Color: fill, Padding: padding, Child: child}
	if trailing {
		cell.RightBorderColor = style.border
		cell.RightBorderWidth = tableSurfaceBorderWidth
	}
	if bottom {
		cell.BottomBorderColor = style.border
		cell.BottomBorderWidth = tableSurfaceBorderWidth
	}
	return cell
}

// formTableHasTrailingSeparator reports whether this column should draw the
// vertical line at its right edge. The last column leaves that edge to the
// table's outer frame.
func formTableHasTrailingSeparator(readOnly bool, columnCount, columnIndex int) bool {
	if !readOnly {
		return columnIndex < columnCount
	}
	return columnIndex < columnCount-1
}
