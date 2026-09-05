package view

import (
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const ChatWindowTitleBarHeight = woxcomponent.TitleBarHeight

// ChatWindowProps contains the dedicated chat chrome and conversation body.
type ChatWindowProps struct {
	Width    float32
	Height   float32
	Title    string
	TitleBar woxwidget.Widget
	Body     woxwidget.Widget
	Theme    woxcomponent.Theme
}

// ChatWindow builds the resizable dedicated chat surface.
func ChatWindow(props ChatWindowProps) woxwidget.Widget {
	bodyHeight := max(float32(0), props.Height-ChatWindowTitleBarHeight)
	body := woxwidget.Container{Width: props.Width, Height: bodyHeight, Child: props.Body}
	return woxwidget.Semantics{
		Key: "chat-window", AutomationID: "chat.window", Role: woxui.AccessibilityRoleWindow, Label: props.Title,
		Child: woxwidget.Container{
			Width: props.Width, Height: props.Height, Color: props.Theme.Background,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{props.TitleBar, body}},
		},
	}
}

// ChatWindowTitleBarProps separates native actions from portable title-bar layout.
type ChatWindowTitleBarProps struct {
	Width                                   float32
	Platform                                string
	Active, Maximized                       bool
	Theme                                   woxcomponent.Theme
	Header                                  *previewview.ChatHeaderProps
	OnDrag, OnMinimize, OnMaximize, OnClose func()
}

// ChatWindowTitleBar lays out conversation actions around the platform caption controls.
func ChatWindowTitleBar(props ChatWindowTitleBarProps) woxwidget.Widget {
	left := float32(0)
	if props.Platform == "darwin" {
		left = 78 // Reserve the native traffic-light cluster in logical units.
	}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Gesture{
			ID: "chat-window-title-drag", OnDragStart: props.OnDrag, OnDoubleTap: props.OnMaximize,
			Child: woxwidget.Container{Width: props.Width, Height: ChatWindowTitleBarHeight},
		}},
		{AnchorBottom: true, StretchWidth: true, Child: woxwidget.Container{Height: 1, Color: woxcomponent.TitleBarAlpha(props.Theme.PreviewSplit, 76)}},
		{Child: woxcomponent.WindowCloseChrome(woxcomponent.WindowCloseChromeProps{
			ID: "chat.window.close", Width: props.Width, Platform: props.Platform, Theme: props.Theme, Active: props.Active, Maximized: props.Maximized,
			OnMinimize: props.OnMinimize, OnMaximize: props.OnMaximize, OnClose: props.OnClose,
		})},
	}
	if props.Header != nil {
		header := *props.Header
		header.Width = max(float32(0), props.Width-left-woxcomponent.TitleBarChromeWidth(props.Platform, true, true))
		header.Height = ChatWindowTitleBarHeight
		children = append(children, woxwidget.StackChild{Left: left, Child: previewview.ChatHeader(header)})
	}
	return woxwidget.Stack{Width: props.Width, Height: ChatWindowTitleBarHeight, Children: children}
}
