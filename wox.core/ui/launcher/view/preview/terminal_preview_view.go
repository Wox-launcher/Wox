package preview

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TerminalPreviewProps contains the immutable state and actions rendered by a terminal preview.
type TerminalPreviewProps struct {
	Width              float32
	Height             float32
	Theme              woxcomponent.Theme
	Window             *woxui.Window
	SessionID          string
	Command            string
	Status             string
	Error              string
	Text               string
	Scroll             float32
	LoadingHistory     bool
	SearchOpen         bool
	SearchEditing      woxui.TextEditingState
	CaseSensitive      bool
	MatchCount         int
	MatchIndex         int
	LayoutText         func(string, woxui.TextStyle, float32, float32) woxwidget.TextBlockLayout
	OnClampScroll      func(float32)
	OnScroll           func(float32, float32)
	OnOpenSearch       func()
	OnSetSearch        func(string) error
	OnSearchChanged    func(string)
	OnSearchKey        func(woxui.KeyEvent) bool
	OnMoveSearch       func(int)
	OnToggleSearchCase func()
	OnCloseSearch      func()
}

// TerminalPreviewView builds the streaming output and local find surface.
func TerminalPreviewView(props TerminalPreviewProps) woxwidget.Widget {
	const statusHeight = float32(38)
	searchHeight := float32(0)
	if props.SearchOpen {
		searchHeight = 50
	}
	bodyHeight := max(float32(0), props.Height-statusHeight-searchHeight)
	innerWidth := max(float32(0), props.Width-24)
	innerHeight := max(float32(0), bodyHeight-20)
	value := props.Text
	if strings.TrimSpace(value) == "" {
		value = "Waiting for terminal output…"
	}
	if props.Error != "" {
		value += "\n\n" + props.Error
	}
	style := woxui.TextStyle{Size: 12}
	layout := woxwidget.TextBlockLayout{}
	if props.LayoutText != nil {
		layout = props.LayoutText(value, style, innerWidth, 18)
	}
	contentHeight := max(innerHeight, layout.Size.Height)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	offset := min(max(float32(0), props.Scroll), maxOffset)
	if props.OnClampScroll != nil {
		props.OnClampScroll(maxOffset)
	}
	header := terminalHeader(props)
	body := woxwidget.Container{Width: props.Width, Height: bodyHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12, Bottom: 10}, Child: woxwidget.Gesture{
		ID: "terminal-preview-scroll-" + props.SessionID,
		OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y, maxOffset)
			}
		},
		Child: woxwidget.ScrollView{Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset, Child: woxwidget.TextBlock{
			Value: value, Width: innerWidth, Height: contentHeight, Style: style, LineHeight: 18, Color: props.Theme.PreviewText, Layout: &layout,
		}},
	}}
	children := []woxwidget.Widget{header}
	if props.SearchOpen {
		children = append(children, terminalSearchBar(props, searchHeight))
	}
	children = append(children, body)
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

func terminalHeader(props TerminalPreviewProps) woxwidget.Widget {
	statusColor := props.Theme.ResultSubtitle
	switch props.Status {
	case "running":
		statusColor = woxui.Color{R: 68, G: 196, B: 120, A: 255}
	case "failed", "killed":
		statusColor = props.Theme.ErrorText
	}
	command := props.Command
	if command == "" {
		command = "Terminal"
	}
	status := props.Status
	if status == "" {
		status = "idle"
	}
	if props.LoadingHistory {
		status = "history…"
	}
	contentWidth := max(float32(0), props.Width-24)
	searchWidth := float32(50)
	statusWidth := float32(64)
	commandWidth := max(float32(40), contentWidth-searchWidth-statusWidth-34)
	return woxwidget.Container{Width: props.Width, Height: 38, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12}, Child: woxwidget.Stack{Width: contentWidth, Height: 22, Children: []woxwidget.StackChild{
		{Top: 7, Child: woxwidget.Container{Width: 8, Height: 8, Radius: 4, Color: statusColor}},
		{Left: 17, Top: 3, Child: woxwidget.Container{Width: commandWidth, Height: 18, Child: woxwidget.Text{Value: command, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}}},
		{Left: contentWidth - searchWidth - statusWidth - 8, Top: 4, Child: woxwidget.Container{Width: statusWidth, Height: 18, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}}},
		{Left: contentWidth - searchWidth, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "terminal-search-open-" + props.SessionID, Label: "Find", Width: searchWidth, Height: 22, Radius: 6,
			Padding: woxwidget.Insets{Left: 9}, FontSize: 9, Variant: woxcomponent.ButtonSurface, OnTap: props.OnOpenSearch, Theme: props.Theme,
		})},
	}}}
}

func terminalSearchBar(props TerminalPreviewProps, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-20)
	actionWidth := float32(32)
	countWidth := float32(46)
	gap := float32(5)
	inputWidth := max(float32(90), innerWidth-countWidth-actionWidth*4-gap*5)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "terminal-search-input-" + props.SessionID, Label: "Find in terminal output", Width: inputWidth, Height: 34,
		Radius: 7, Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 5}, Value: props.SearchEditing.Text,
		Focused: true, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
		OnChanged: props.OnSearchChanged, OnSetValue: props.OnSetSearch, OnKey: props.OnSearchKey,
	})
	count := "0/0"
	if props.MatchCount > 0 {
		count = fmt.Sprintf("%d/%d", props.MatchIndex+1, props.MatchCount)
	}
	button := func(id, label string, selected bool, action func()) woxwidget.Widget {
		variant := woxcomponent.ButtonSecondary
		if selected {
			variant = woxcomponent.ButtonSelected
		}
		return woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: id + "-" + props.SessionID, Label: label, Width: actionWidth, Height: 34, Radius: 7,
			Padding: woxwidget.Insets{Left: 9}, Variant: variant, OnTap: action, Theme: props.Theme,
		})
	}
	return woxwidget.Container{Width: props.Width, Height: height, Radius: 8, Color: props.Theme.ActionBackground, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		input,
		woxwidget.Container{Width: countWidth, Height: 34, Padding: woxwidget.Insets{Left: 5, Top: 10}, Child: woxwidget.Text{Value: count, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}},
		button("terminal-search-previous", "↑", false, func() {
			if props.OnMoveSearch != nil {
				props.OnMoveSearch(-1)
			}
		}),
		button("terminal-search-next", "↓", false, func() {
			if props.OnMoveSearch != nil {
				props.OnMoveSearch(1)
			}
		}),
		button("terminal-search-case", "Aa", props.CaseSensitive, props.OnToggleSearchCase),
		button("terminal-search-close", "×", false, props.OnCloseSearch),
	}}}
}
