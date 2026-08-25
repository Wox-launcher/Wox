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
	tableSurfaceBorderWidth  = woxcomponent.TableGridBorderWidth
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

// tableSurfaceCell maps Settings table colors onto the shared collapsed grid cell.
func tableSurfaceCell(width, height float32, fill woxui.Color, style tableSurfaceStyle, trailing, bottom bool, padding woxwidget.Insets, child woxwidget.Widget) woxwidget.Container {
	return woxcomponent.WoxTableGridCell(woxcomponent.TableGridCellProps{
		Width: width, Height: height, Color: fill, Border: style.border,
		Trailing: trailing, Bottom: bottom, Padding: padding, Child: child,
	})
}

// formTableGridChrome wraps a Settings table in the shared 1px outer frame.
func formTableGridChrome(props FormTableFieldProps, width, height float32, child woxwidget.Widget) woxwidget.Widget {
	return woxcomponent.WoxTableGridFrame(width, height, newTableSurfaceStyle(props.Theme).border, child)
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
