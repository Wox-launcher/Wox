package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	tableSurfaceHeaderHeight = float32(36)
	tableSurfaceRowHeight    = float32(40)
	tableSurfaceBorderWidth  = woxcomponent.TableGridBorderWidth
)

// tableSurfaceStyle keeps every column-based table on the same theme-derived visual tokens.
type tableSurfaceStyle struct {
	headerBackground woxui.Color
	bodyBackground   woxui.Color
	headerText       woxui.Color
	border           woxui.Color
	rowDivider       woxui.Color
}

// newTableSurfaceStyle resolves the shared table colors for the active theme.
func newTableSurfaceStyle(theme woxcomponent.Theme) tableSurfaceStyle {
	return tableSurfaceStyle{
		headerBackground: tableSurfaceAlpha(theme.ResultTitle, 14),
		bodyBackground:   tableSurfaceAlpha(theme.ResultTitle, 5),
		headerText:       theme.ResultSubtitle,
		border:           tableSurfaceAlpha(theme.PreviewSplit, min(theme.PreviewSplit.A, 40)),
		rowDivider:       tableSurfaceAlpha(theme.PreviewSplit, min(theme.PreviewSplit.A, 26)),
	}
}

func tableSurfaceAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// tableSurfaceCell maps Settings table colors onto the shared collapsed grid cell.
func tableSurfaceCell(width, height float32, style tableSurfaceStyle, bottom bool, padding woxwidget.Insets, child woxwidget.Widget) woxwidget.Container {
	border := style.rowDivider
	if height == tableSurfaceHeaderHeight {
		border = style.border
	}
	return woxcomponent.WoxTableGridCell(woxcomponent.TableGridCellProps{
		Width: width, Height: height, Border: border,
		Bottom: bottom, Padding: padding, Child: child,
	})
}

// formTableGridChrome wraps a Settings table in the shared 1px outer frame.
func formTableGridChrome(props FormTableFieldProps, width, height float32, child woxwidget.Widget) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	return woxcomponent.WoxSettingsTableFrame(width, height, tableSurfaceHeaderHeight, style.border, style.headerBackground, style.bodyBackground, child)
}
