package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SidebarRowHeight is the normal-density navigation row height.
const SidebarRowHeight = float32(38)

// SidebarTitleIconSize and SidebarTitleFontSize match the dedicated Chat title bar.
const SidebarTitleIconSize = float32(20)
const SidebarTitleFontSize = float32(14)

// SidebarPanel renders the shared in-window sidebar surface around feature-owned content.
func SidebarPanel(width, height float32, padding woxwidget.Insets, theme Theme, child woxwidget.Widget) woxwidget.Widget {
	background, border := theme.ResultSubtitle, theme.ResultSubtitle
	background.A = uint8(float32(background.A) * .04)
	border.A = uint8(float32(border.A) * .12)
	return woxwidget.Container{Width: width, Height: height, Radius: theme.Controls.Scaled(12), Color: background, BorderColor: border, BorderWidth: 1, Padding: padding, Child: child}
}

// SidebarSplit keeps an open sidebar in the same layout flow as its content.
func SidebarSplit(sidebar, content woxwidget.Widget) woxwidget.Widget {
	if sidebar == nil {
		return content
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{sidebar, content}}
}

// SidebarToggle uses the shared icon and button interaction while allowing host chrome sizing.
func SidebarToggle(props IconButtonProps, iconSize float32, color, activeColor woxui.Color, open bool) woxwidget.Widget {
	if open {
		props.Icon = svgIcon("control.sidebar-open", iconSize, activeColor)
	} else {
		props.Icon = SidebarGlyph(iconSize, color)
	}
	return WoxIconButton(props)
}

// SidebarMinimumWidth reserves content space only while the sidebar is open.
func SidebarMinimumWidth(contentWidth, sidebarWidth float32, open bool) float32 {
	if open {
		return contentWidth + sidebarWidth
	}
	return contentWidth
}
