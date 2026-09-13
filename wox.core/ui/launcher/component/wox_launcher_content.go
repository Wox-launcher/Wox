package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherContentBounds reserves logical window space without depending on display scale or desktop origin.
func LauncherContentBounds(width, height, inset float32) woxui.Rect {
	inset = min(max(0, inset), max(0, min(width, height)/2))
	return woxui.Rect{X: inset, Y: inset, Width: max(0, width-2*inset), Height: max(0, height-2*inset)}
}

// WoxLauncherContent paints the inner panel independently of native window chrome.
func WoxLauncherContent(width, height float32, theme Theme, child woxwidget.Widget) woxwidget.Widget {
	if theme.AppContentInset == 0 && theme.AppContentBackground.A == 0 && theme.AppContentBorderRadius == 0 {
		return child
	}
	bounds := LauncherContentBounds(width, height, theme.AppContentInset)
	radius := min(max(0, theme.AppContentBorderRadius), min(bounds.Width, bounds.Height)/2)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(bounds.X), Child: woxwidget.Clip{
		Width: bounds.Width, Height: bounds.Height,
		Child: woxwidget.Container{Width: bounds.Width, Height: bounds.Height, Color: theme.AppContentBackground, Radius: radius, Child: child},
	}}
}
