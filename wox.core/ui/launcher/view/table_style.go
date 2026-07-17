package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

const (
	tableSurfaceHeaderHeight   = float32(36)
	tableSurfaceRowHeight      = float32(36)
	tableSurfaceEmptyHeight    = float32(82)
	tableSurfaceBorderWidth    = float32(0.5)
	tableSurfaceHeaderFontSize = float32(12)
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
