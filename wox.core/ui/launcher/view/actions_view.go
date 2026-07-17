package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	ActionPanelContentWidth = 320
	ActionRowHeight         = 40
	ActionHeaderHeight      = 16
	ActionDividerHeight     = 16
	ActionSearchHeight      = 46
	MaxVisibleActions       = 8
)

// ActionItem contains resolved presentation data for one result action.
type ActionItem struct {
	Index        int
	ID           string
	Label        string
	Icon         *woxui.Image
	HotkeyLabels []string
}

// ActionsProps contains the action panel state and callbacks.
type ActionsProps struct {
	Window                *woxui.Window
	WindowWidth           float32
	WindowHeight          float32
	QueryHeight           float32
	ToolbarHeight         float32
	Theme                 woxcomponent.Theme
	ActionHeader          woxui.Color
	ActionQueryBackground woxui.Color
	ActionQueryText       woxui.Color
	ResultTail            woxui.Color
	SelectedTail          woxui.Color
	ResultItemRadius      float32
	ActionQueryRadius     float32
	ActionPadding         woxwidget.Insets
	HeaderLabel           string
	NoMatchesLabel        string
	Items                 []ActionItem
	Selected              int
	Editing               woxui.TextEditingState
	Scroll                float32
	OnSelect              func(int)
	OnActivate            func()
	OnScroll              func(float32)
	OnCaret               func(int)
}

// ActionPanelBaseHeight returns the non-list height used by launcher window sizing.
func ActionPanelBaseHeight(padding woxwidget.Insets) float32 {
	return ActionHeaderHeight + ActionDividerHeight + ActionSearchHeight + padding.Top + padding.Bottom
}

// ActionsView builds the floating action picker and returns its geometry.
func ActionsView(props ActionsProps) (woxwidget.Widget, float32, float32) {
	panelWidth := min(float32(ActionPanelContentWidth)+props.ActionPadding.Left+props.ActionPadding.Right, max(float32(240), props.WindowWidth-28))
	innerWidth := max(float32(0), panelWidth-props.ActionPadding.Left-props.ActionPadding.Right)
	visibleRows := max(1, min(len(props.Items), MaxVisibleActions))
	panelHeight := ActionPanelBaseHeight(props.ActionPadding) + float32(visibleRows*ActionRowHeight)
	panelHeight = min(panelHeight, max(float32(100), props.WindowHeight-props.QueryHeight-props.ToolbarHeight-20))
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Items)))
	for _, item := range props.Items {
		item := item
		selected := item.Index == props.Selected
		background := woxui.Color{}
		foreground := props.Theme.ActionText
		if selected {
			background = props.Theme.ActionSelected
			foreground = props.Theme.ActionSelectedText
		}
		var icon woxwidget.Widget = woxwidget.Painter{Width: 22, Height: 22}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 22, Height: 22}
		}
		hotkeyWidth := float32(0)
		var hotkey woxwidget.Widget = woxwidget.Painter{}
		if len(item.HotkeyLabels) > 0 {
			tailColor := props.ResultTail
			chipBackground := props.Theme.ActionBackground
			if selected {
				tailColor = props.SelectedTail
				chipBackground = props.Theme.ActionSelected
			}
			chip, chipWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
				Labels: item.HotkeyLabels, Foreground: tailColor, Background: chipBackground, Window: props.Window,
			})
			hotkeyWidth = chipWidth + 15
			hotkey = woxwidget.Container{Width: hotkeyWidth, Height: ActionRowHeight, Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 5, Bottom: 6}, Child: chip}
		}
		labelWidth := max(float32(40), innerWidth-37-hotkeyWidth)
		rows = append(rows, woxwidget.Gesture{
			ID: "action-" + item.ID,
			OnHover: func(inside bool) {
				if inside && props.OnSelect != nil {
					props.OnSelect(item.Index)
				}
			},
			OnTap: func() {
				if props.OnSelect != nil {
					props.OnSelect(item.Index)
				}
				if props.OnActivate != nil {
					props.OnActivate()
				}
			},
			Child: woxwidget.Container{Width: innerWidth, Height: ActionRowHeight, Radius: props.ResultItemRadius, Color: background, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: 37, Height: ActionRowHeight, Padding: woxwidget.Insets{Left: 5, Top: 9, Right: 10, Bottom: 9}, Child: icon},
				woxwidget.Container{Width: labelWidth, Height: ActionRowHeight, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{Value: item.Label, Style: woxui.TextStyle{Size: 13}, Color: foreground}},
				hotkey,
			}}},
		})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: ActionRowHeight, Padding: woxwidget.Insets{Left: 8, Top: 13}, Child: woxwidget.Text{
			Value: props.NoMatchesLabel, Style: woxui.TextStyle{Size: 12}, Color: props.ActionHeader,
		}})
	}
	listHeight := float32(visibleRows * ActionRowHeight)
	listContentHeight := float32(len(rows) * ActionRowHeight)
	listChildren := []woxwidget.StackChild{{Child: woxwidget.ScrollView{
		Width: innerWidth, Height: listHeight, ContentHeight: listContentHeight, Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}}
	if len(props.Items) > MaxVisibleActions {
		thumbHeight := max(float32(24), listHeight*listHeight/listContentHeight)
		thumbTop := (listHeight - thumbHeight) * props.Scroll / (listContentHeight - listHeight)
		thumbColor := props.ActionHeader
		thumbColor.A = min(150, thumbColor.A)
		listChildren = append(listChildren, woxwidget.StackChild{Left: max(float32(0), innerWidth-5), Top: thumbTop, Child: woxwidget.Container{Width: 3, Height: thumbHeight, Radius: 2, Color: thumbColor}})
	}
	actionList := woxwidget.Gesture{ID: "action-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.Stack{Width: innerWidth, Height: listHeight, Children: listChildren}}
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "action-search", Label: "Filter actions", Width: innerWidth, Height: 40, Radius: props.ActionQueryRadius,
		Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 8}, Background: props.ActionQueryBackground,
		Style: woxui.TextStyle{Size: 12}, TextColor: props.ActionQueryText, State: props.Editing, Focused: true,
		MaxLines: 1, Window: props.Window, Theme: props.Theme, ControllerManagedFocus: true, OnCaret: props.OnCaret,
	})
	return woxwidget.Container{
		Width: panelWidth, Height: panelHeight, Radius: props.ActionQueryRadius, Color: props.Theme.ActionBackground,
		Padding: props.ActionPadding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: ActionHeaderHeight, Child: woxwidget.Text{Value: props.HeaderLabel, Style: woxui.TextStyle{Size: 13}, Color: props.ActionHeader}},
			woxwidget.Container{Width: innerWidth, Height: ActionDividerHeight, Padding: woxwidget.Insets{Top: 7, Bottom: 8}, Child: woxwidget.Container{Width: innerWidth, Height: 1, Color: props.Theme.PreviewSplit}},
			actionList,
			woxwidget.Container{Width: innerWidth, Height: ActionSearchHeight, Padding: woxwidget.Insets{Top: 6}, Child: search},
		}},
	}, panelWidth, panelHeight
}
