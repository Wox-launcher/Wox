package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	settingsChoiceRowHeight    = float32(48)
	settingsChoiceSearchHeight = float32(48)
	settingsChoiceMenuPadding  = float32(8)
	settingsChoiceMenuMargin   = float32(12)
	settingsChoiceMaxHeight    = float32(360)
)

// SettingsChoice is one value displayed by the settings dropdown.
type SettingsChoice struct {
	Value    string
	Label    string
	Trailing string
	Tooltip  string
}

// SettingsChoiceProps contains the immutable state and actions rendered by the settings dropdown.
type SettingsChoiceProps struct {
	Width         float32
	Height        float32
	Anchor        woxui.Rect
	Filterable    bool
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	Title         string
	CurrentValue  string
	Query         woxui.TextEditingState
	Choices       []SettingsChoice
	Selected      int
	Scroll        float32
	OnCaret       func(int)
	OnSetQuery    func(string) error
	OnKey         func(woxui.KeyEvent) bool
	OnTextInput   func(woxui.TextInputEvent) bool
	OnSelect      func(int)
	OnChoose      func(int)
	OnCancel      func()
	OnScroll      func(float32)
	OnSetViewport func(float32)
	OnTooltip     func(bool, string, woxui.Rect)
}

// SettingsChoiceView builds a field-anchored dropdown matching the former Flutter settings control.
func SettingsChoiceView(props SettingsChoiceProps) woxwidget.Widget {
	anchor := props.Anchor
	if anchor.Width <= 0 || anchor.Height <= 0 {
		anchor.Width = min(float32(300), max(float32(190), props.Width-settingsChoiceMenuMargin*2))
		anchor.Height = 38
		anchor.X = max(settingsChoiceMenuMargin, props.Width-anchor.Width-settingsChoiceMenuMargin)
		anchor.Y = 72
	}
	menuWidth := min(anchor.Width, max(float32(1), props.Width-settingsChoiceMenuMargin*2))
	menuLeft := min(max(settingsChoiceMenuMargin, anchor.X), max(settingsChoiceMenuMargin, props.Width-menuWidth-settingsChoiceMenuMargin))
	searchHeight := float32(0)
	if props.Filterable {
		searchHeight = settingsChoiceSearchHeight
	}
	rowCount := max(1, len(props.Choices))
	maximumMenuHeight := min(settingsChoiceMaxHeight, max(settingsChoiceRowHeight+settingsChoiceMenuPadding*2+searchHeight, props.Height-settingsChoiceMenuMargin*2))
	maximumListHeight := max(settingsChoiceRowHeight, maximumMenuHeight-settingsChoiceMenuPadding*2-searchHeight)
	listHeight := min(float32(rowCount)*settingsChoiceRowHeight, maximumListHeight)
	menuHeight := settingsChoiceMenuPadding*2 + searchHeight + listHeight
	menuTop := settingsChoiceMenuTop(props, anchor, menuHeight, listHeight)
	menu := settingsChoiceMenu(props, menuWidth, menuHeight, listHeight)
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "setting-choice-backdrop", OnTap: props.OnCancel, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: menuLeft, Top: menuTop, Child: menu},
	}}
}

func settingsChoiceMenuTop(props SettingsChoiceProps, anchor woxui.Rect, menuHeight, listHeight float32) float32 {
	if props.Filterable {
		top := anchor.Y + anchor.Height
		if top+menuHeight > props.Height-settingsChoiceMenuMargin {
			top = anchor.Y - menuHeight
		}
		return min(max(settingsChoiceMenuMargin, top), max(settingsChoiceMenuMargin, props.Height-menuHeight-settingsChoiceMenuMargin))
	}
	currentIndex := 0
	for index, choice := range props.Choices {
		if choice.Value == props.CurrentValue {
			currentIndex = index
			break
		}
	}
	initialScroll := max(float32(0), float32(currentIndex-4)*settingsChoiceRowHeight)
	initialScroll = min(initialScroll, max(float32(0), float32(len(props.Choices))*settingsChoiceRowHeight-listHeight))
	selectedCenter := settingsChoiceMenuPadding + float32(currentIndex)*settingsChoiceRowHeight - initialScroll + settingsChoiceRowHeight/2
	top := anchor.Y + anchor.Height/2 - selectedCenter
	return min(max(settingsChoiceMenuMargin, top), max(settingsChoiceMenuMargin, props.Height-menuHeight-settingsChoiceMenuMargin))
}

func settingsChoiceMenu(props SettingsChoiceProps, width, height, listHeight float32) woxwidget.Widget {
	if props.OnSetViewport != nil {
		props.OnSetViewport(listHeight)
	}
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Choices)))
	for index, choice := range props.Choices {
		index := index
		choice := choice
		selected := index == props.Selected
		background := props.Theme.ActionBackground
		foreground := props.Theme.ActionText
		if selected {
			background = props.Theme.SelectedBackground
			background.A = min(uint8(80), background.A)
			foreground = props.Theme.SelectedTitle
		}
		contentWidth := max(float32(0), width-32)
		trailingWidth := float32(0)
		if choice.Trailing != "" {
			trailingWidth = min(float32(80), max(float32(0), contentWidth-80))
			contentWidth = max(float32(0), contentWidth-trailingWidth-12)
		}
		var tooltip woxwidget.Widget = woxwidget.Painter{}
		if choice.Tooltip != "" {
			contentWidth = max(float32(0), contentWidth-28)
			tooltip = woxwidget.Gesture{ID: fmt.Sprintf("setting-choice-tooltip-%d", index), OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnTooltip != nil {
					props.OnTooltip(inside, choice.Tooltip, bounds)
				}
			}, Child: woxwidget.Container{Width: 28, Height: settingsChoiceRowHeight, Padding: woxwidget.Insets{Left: 6, Top: 14}, Child: woxwidget.Text{
				Value: "ⓘ", Style: woxui.TextStyle{Size: 14}, Color: foreground,
			}}}
		}
		activate := func() {
			if props.OnChoose != nil {
				props.OnChoose(index)
			}
		}
		key := woxwidget.Key(fmt.Sprintf("setting-choice-%d", index))
		rowChildren := []woxwidget.Widget{
			woxwidget.Container{Width: contentWidth, Height: settingsChoiceRowHeight, Padding: woxwidget.Insets{Top: 15}, Child: woxwidget.Text{
				Value: choice.Label, Style: woxui.TextStyle{Size: 13}, Color: foreground,
			}},
		}
		if trailingWidth > 0 {
			rowChildren = append(rowChildren,
				woxwidget.Container{Width: 12, Height: settingsChoiceRowHeight},
				woxwidget.Align{Width: trailingWidth, Height: settingsChoiceRowHeight, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{
					Value: choice.Trailing, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
				}},
			)
		}
		rowChildren = append(rowChildren, tooltip)
		row := woxwidget.Gesture{ID: string(key), OnHover: func(inside bool) {
			if inside && props.OnSelect != nil {
				props.OnSelect(index)
			}
		}, OnTap: activate, Child: woxwidget.Container{
			Width: width, Height: settingsChoiceRowHeight, Color: background, Padding: woxwidget.Insets{Left: 16},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: rowChildren},
		}}
		rows = append(rows, woxwidget.Semantics{
			Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleMenuItem, Label: choice.Label,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, Selected: selected,
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate {
					activate()
				}
				return nil
			}, Child: row,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: settingsChoiceRowHeight, Padding: woxwidget.Insets{Left: 16, Top: 15}, Child: woxwidget.Text{
			Value: "No matching choices", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
		}})
	}
	list := woxwidget.Gesture{ID: "setting-choice-list", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: width, Height: listHeight, ContentHeight: max(listHeight, float32(len(rows))*settingsChoiceRowHeight), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	children := make([]woxwidget.Widget, 0, 2)
	if props.Filterable {
		search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "setting-choice-search", Label: "Filter choices", Hint: "Filter choices…", Width: width, Height: 40, Radius: 4,
			Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 10, Bottom: 7}, Background: props.Theme.ToolbarBackground,
			State: props.Query, Focused: true, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
			OnCaret: props.OnCaret, OnSetValue: props.OnSetQuery, OnKey: props.OnKey, OnTextInput: props.OnTextInput,
		})
		children = append(children, woxwidget.Container{Width: width, Height: settingsChoiceSearchHeight, Padding: woxwidget.Insets{Bottom: 8}, Child: search})
	}
	children = append(children, list)
	menuContent := woxwidget.Container{Width: width, Height: height, Radius: 4, Color: props.Theme.ActionBackground,
		Padding: woxwidget.Insets{Top: settingsChoiceMenuPadding, Bottom: settingsChoiceMenuPadding},
		Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	// Paint the border after the rows so their full-width backgrounds cannot cover the inset stroke.
	menuBorder := woxwidget.Container{Width: width, Height: height, Radius: 4, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1}
	var surface woxwidget.Widget = woxwidget.Semantics{
		Key: "setting-choice-menu", AutomationID: "setting-choice-menu", Role: woxui.AccessibilityRoleMenu, Label: props.Title,
		Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{{Child: menuContent}, {Child: menuBorder}}},
	}
	if !props.Filterable {
		surface = woxwidget.Focusable{Key: "setting-choice-menu-focus", Autofocus: true, OnKey: props.OnKey, Child: surface}
	}
	return woxwidget.FocusScope{Key: "setting-choice-scope", Modal: true, Child: surface}
}
