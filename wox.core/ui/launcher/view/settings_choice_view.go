package view

import (
	"fmt"
	"strings"

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
	ID           string
	Width        float32
	Height       float32
	Anchor       woxui.Rect
	Filterable   bool
	Theme        woxcomponent.Theme
	Window       *woxui.Window
	Title        string
	CurrentValue string
	Choices      []SettingsChoice
	OnChoose     func(int)
	OnCancel     func()
	OnTooltip    func(bool, string, woxui.Rect)
}

// SettingsChoiceView builds a field-anchored dropdown matching the former Flutter settings control.
func SettingsChoiceView(props SettingsChoiceProps) woxwidget.Widget {
	id := props.ID
	if id == "" {
		id = "setting-choice"
		props.ID = id
	}
	return woxwidget.Stateful{
		Key: woxwidget.Key(id), Type: (*settingsChoiceState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &settingsChoiceState{} },
	}
}

type settingsChoiceState struct {
	queryController  *woxwidget.TextEditingController
	queryFocusNode   *woxwidget.FocusNode
	scrollController *woxwidget.ScrollController
	selected         int
}

// InitState creates the dropdown's private query, focus, highlight, and scroll state.
func (s *settingsChoiceState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(SettingsChoiceProps)
	s.queryController = woxwidget.NewTextEditingController("")
	s.queryFocusNode = woxwidget.NewFocusNode()
	s.selected = settingsChoiceCurrentIndex(props.Choices, props.CurrentValue)
	s.scrollController = woxwidget.NewScrollController(max(float32(0), float32(s.selected-4)*settingsChoiceRowHeight))
}

// DidUpdateWidget keeps the highlight aligned when the committed business value changes.
func (s *settingsChoiceState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(SettingsChoiceProps)
	props := newWidget.(SettingsChoiceProps)
	if oldProps.CurrentValue != props.CurrentValue {
		s.selected = settingsChoiceCurrentIndex(props.Choices, props.CurrentValue)
		s.scrollController.JumpTo(max(float32(0), float32(s.selected-4)*settingsChoiceRowHeight))
	}
}

// Build renders the dropdown from retained interaction state and immutable business choices.
func (s *settingsChoiceState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(SettingsChoiceProps)
	visible := filteredSettingsChoices(props.Choices, s.queryController.Text())
	if len(visible) == 0 {
		s.selected = -1
	} else {
		s.selected = min(max(0, s.selected), len(visible)-1)
	}
	return buildSettingsChoiceView(context, props, s, visible)
}

// Dispose releases no external resources; child State objects detach their own controllers.
func (s *settingsChoiceState) Dispose() {}

type visibleSettingsChoice struct {
	choice        SettingsChoice
	originalIndex int
}

// buildSettingsChoiceView lays out the anchored surface while State owns all transient interaction data.
func buildSettingsChoiceView(context woxwidget.StateContext, props SettingsChoiceProps, state *settingsChoiceState, visible []visibleSettingsChoice) woxwidget.Widget {
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
	rowCount := max(1, len(visible))
	maximumMenuHeight := min(settingsChoiceMaxHeight, max(settingsChoiceRowHeight+settingsChoiceMenuPadding*2+searchHeight, props.Height-settingsChoiceMenuMargin*2))
	maximumListHeight := max(settingsChoiceRowHeight, maximumMenuHeight-settingsChoiceMenuPadding*2-searchHeight)
	listHeight := min(float32(rowCount)*settingsChoiceRowHeight, maximumListHeight)
	menuHeight := settingsChoiceMenuPadding*2 + searchHeight + listHeight
	menuTop := settingsChoiceMenuTop(props, anchor, menuHeight, listHeight)
	menu := settingsChoiceMenu(context, props, state, visible, menuWidth, menuHeight, listHeight)
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "setting-choice-backdrop", OnTap: props.OnCancel, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: menuLeft, Top: menuTop, Child: menu},
	}}
}

// settingsChoiceCurrentIndex resolves the committed value without moving that value into component State.
func settingsChoiceCurrentIndex(choices []SettingsChoice, value string) int {
	for index, choice := range choices {
		if choice.Value == value {
			return index
		}
	}
	return 0
}

// filteredSettingsChoices retains original option indexes for business-value callbacks.
func filteredSettingsChoices(choices []SettingsChoice, query string) []visibleSettingsChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	visible := make([]visibleSettingsChoice, 0, len(choices))
	for index, choice := range choices {
		if query == "" || strings.Contains(strings.ToLower(choice.Label), query) || strings.Contains(strings.ToLower(choice.Value), query) {
			visible = append(visible, visibleSettingsChoice{choice: choice, originalIndex: index})
		}
	}
	return visible
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

func settingsChoiceMenu(context woxwidget.StateContext, props SettingsChoiceProps, state *settingsChoiceState, visible []visibleSettingsChoice, width, height, listHeight float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, max(1, len(visible)))
	for index, visibleChoice := range visible {
		index := index
		visibleChoice := visibleChoice
		choice := visibleChoice.choice
		selected := index == state.selected
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
				props.OnChoose(visibleChoice.originalIndex)
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
			if inside && state.selected != index {
				context.SetState(func() {
					state.selected = index
					state.scrollController.EnsureVisible(float32(index)*settingsChoiceRowHeight, float32(index+1)*settingsChoiceRowHeight)
				})
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
	list := woxwidget.ScrollView{
		Key: woxwidget.Key(props.ID + "-list"), ID: props.ID + "-list", Controller: state.scrollController,
		Width: width, Height: listHeight, ContentHeight: max(listHeight, float32(len(rows))*settingsChoiceRowHeight),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}
	children := make([]woxwidget.Widget, 0, 2)
	if props.Filterable {
		search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "setting-choice-search", Label: "Filter choices", Hint: "Filter choices…", Width: width, Height: 40, Radius: 4,
			Padding: woxwidget.Insets{Left: 10, Top: 9, Right: 10, Bottom: 7}, Background: props.Theme.ToolbarBackground,
			Controller: state.queryController, FocusNode: state.queryFocusNode, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
			OnKey: func(event woxui.KeyEvent) bool { return state.handleKey(context, props, visible, event) },
			OnChanged: func(string) {
				context.SetState(func() {
					state.selected = 0
					state.scrollController.JumpTo(0)
				})
			},
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
		surface = woxwidget.Focusable{Key: "setting-choice-menu-focus", Autofocus: true, OnKey: func(event woxui.KeyEvent) bool {
			return state.handleKey(context, props, visible, event)
		}, Child: surface}
	}
	return woxwidget.FocusScope{Key: "setting-choice-scope", Modal: true, Child: surface}
}

// handleKey owns modal navigation while leaving ordinary editing keys to WoxTextField.
func (s *settingsChoiceState) handleKey(context woxwidget.StateContext, props SettingsChoiceProps, visible []visibleSettingsChoice, event woxui.KeyEvent) bool {
	switch event.Key {
	case woxui.KeyEscape:
		if props.OnCancel != nil {
			props.OnCancel()
		}
		return true
	case woxui.KeyArrowUp, woxui.KeyArrowDown:
		if len(visible) == 0 {
			return true
		}
		delta := -1
		if event.Key == woxui.KeyArrowDown {
			delta = 1
		}
		context.SetState(func() {
			s.selected = (s.selected + delta + len(visible)) % len(visible)
			s.scrollController.EnsureVisible(float32(s.selected)*settingsChoiceRowHeight, float32(s.selected+1)*settingsChoiceRowHeight)
		})
		return true
	case woxui.KeyEnter:
		if s.selected >= 0 && s.selected < len(visible) && props.OnChoose != nil {
			props.OnChoose(visible[s.selected].originalIndex)
		}
		return true
	case woxui.KeySpace:
		if !props.Filterable && s.selected >= 0 && s.selected < len(visible) && props.OnChoose != nil {
			props.OnChoose(visible[s.selected].originalIndex)
			return true
		}
	}
	return false
}
