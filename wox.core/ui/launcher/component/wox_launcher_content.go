package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherContentBounds reserves logical window space without depending on display scale or desktop origin.
func LauncherContentBounds(width, height, inset float32, surfaces ...*ThemeSurfaceSet) woxui.Rect {
	inset = min(max(0, inset), max(0, min(width, height)/2))
	padding := woxwidget.UniformInsets(inset)
	if len(surfaces) > 0 && surfaces[0] != nil {
		extra := surfaces[0].ContentInsets
		padding.Left += extra.Left
		padding.Right += extra.Right
		padding.Top += extra.Top
		padding.Bottom += extra.Bottom
	}
	left, top := min(width, padding.Left), min(height, padding.Top)
	return woxui.Rect{X: left, Y: top, Width: max(0, width-left-padding.Right), Height: max(0, height-top-padding.Bottom)}
}

// WoxLauncherContent paints the inner panel independently of native window chrome.
func WoxLauncherContent(width, height float32, theme Theme, child woxwidget.Widget) woxwidget.Widget {
	if theme.AppContentInset == 0 && theme.AppContentBackground.A == 0 && theme.AppContentBorderRadius == 0 && (theme.Surfaces == nil || theme.Surfaces.ContentInsets == (woxwidget.Insets{})) {
		return child
	}
	bounds := LauncherContentBounds(width, height, theme.AppContentInset, theme.Surfaces)
	radius := min(max(0, theme.AppContentBorderRadius), min(bounds.Width, bounds.Height)/2)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: bounds.X, Top: bounds.Y, Right: max(0, width-bounds.X-bounds.Width), Bottom: max(0, height-bounds.Y-bounds.Height)}, Child: woxwidget.Clip{
		Width: bounds.Width, Height: bounds.Height,
		Child: woxwidget.Container{Width: bounds.Width, Height: bounds.Height, Color: theme.AppContentBackground, Radius: radius, Child: child},
	}}
}
