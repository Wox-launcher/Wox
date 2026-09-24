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
	settingsChoiceGroupHeight  = float32(28)
	settingsChoiceSearchHeight = float32(48)
	settingsChoiceMenuPadding  = float32(8)
	settingsChoiceMenuMargin   = float32(12)
	settingsChoiceMaxHeight    = float32(360)
	settingsChoiceMenuMaxWidth = float32(360)
)

// SettingsChoice is one value displayed by the settings dropdown.
type SettingsChoice struct {
	Value        string
	Label        string
	GroupTooltip string
	Group        string
	Leading      *woxui.Image
	Trailing     string
	Tooltip      string
}

// SettingsChoiceProps contains the immutable state and actions rendered by the settings dropdown.
type SettingsChoiceProps struct {
	ID           string
	Width        float32
	Height       float32
	Anchor       woxui.Rect
	Filterable   bool
	Theme        woxcomponent.ControlTheme
	Window       *woxui.Window
	Title        string
	FilterHint   string
	SearchIcon   *woxui.Image
	InfoIcon     *woxui.Image
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
	hovered          int
	keyboardSelected bool
}

// InitState creates the dropdown's private query, focus, highlight, and scroll state.
func (s *settingsChoiceState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(SettingsChoiceProps)
	s.queryController = woxwidget.NewTextEditingController("")
	s.queryFocusNode = woxwidget.NewFocusNode()
	visible := filteredSettingsChoices(props.Choices, "")
	s.selected = settingsChoiceVisibleIndex(visible, props.CurrentValue)
	s.hovered = -1
	offset := settingsChoiceOffset(visible, s.selected)
	s.scrollController = woxwidget.NewScrollController(max(float32(0), offset-4*settingsChoiceRowHeight))
}

// DidUpdateWidget keeps the highlight aligned when the committed business value changes.
func (s *settingsChoiceState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(SettingsChoiceProps)
	props := newWidget.(SettingsChoiceProps)
	if oldProps.CurrentValue != props.CurrentValue {
		visible := filteredSettingsChoices(props.Choices, s.queryController.Text())
		s.selected = settingsChoiceVisibleIndex(visible, props.CurrentValue)
		s.hovered = -1
		s.keyboardSelected = false
		offset := settingsChoiceOffset(visible, s.selected)
		s.scrollController.JumpTo(max(float32(0), offset-4*settingsChoiceRowHeight))
	}
}

// Build renders the dropdown from retained interaction state and immutable business choices.
func (s *settingsChoiceState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(SettingsChoiceProps)
	visible := filteredSettingsChoices(props.Choices, s.queryController.Text())
	if len(visible) == 0 {
		s.selected = -1
		s.hovered = -1
	} else {
		s.selected = min(max(0, s.selected), len(visible)-1)
		if s.selected >= 0 && visible[s.selected].header {
			s.selected = settingsChoiceNearestSelectable(visible, s.selected, 1)
		}
		if s.hovered >= len(visible) || (s.hovered >= 0 && visible[s.hovered].header) {
			s.hovered = -1
		}
	}
	return buildSettingsChoiceView(context, props, s, visible)
}

// Dispose releases no external resources; child State objects detach their own controllers.
func (s *settingsChoiceState) Dispose() {}

type visibleSettingsChoice struct {
	choice        SettingsChoice
	originalIndex int
	header        bool
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
	menuWidth := settingsChoicePreferredMenuWidth(props, visible, anchor)
	menuLeft := min(max(settingsChoiceMenuMargin, anchor.X), max(settingsChoiceMenuMargin, props.Width-menuWidth-settingsChoiceMenuMargin))
	searchHeight := float32(0)
	menuPadding := settingsChoiceMenuPadding
	if props.Filterable {
		searchHeight = settingsChoiceSearchHeight
		menuPadding = 0
	}
	maximumMenuHeight := min(settingsChoiceMaxHeight, max(searchHeight+menuPadding*2, props.Height-settingsChoiceMenuMargin*2))
	maximumListHeight := max(float32(0), maximumMenuHeight-menuPadding*2-searchHeight)
	listHeight := min(settingsChoiceContentHeight(visible), maximumListHeight)
	menuHeight := menuPadding*2 + searchHeight + listHeight
	menuTop := settingsChoiceMenuTop(props, anchor, menuHeight, listHeight)
	menu := settingsChoiceMenu(context, props, state, visible, menuWidth, menuHeight, listHeight, menuPadding)
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "setting-choice-backdrop", OnTap: props.OnCancel, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: menuLeft, Top: menuTop, Child: menu},
	}}
}

// settingsChoiceVisibleIndex finds the committed value in the rendered list, skipping group headers.
func settingsChoiceVisibleIndex(visible []visibleSettingsChoice, value string) int {
	for index, item := range visible {
		if !item.header && item.choice.Value == value {
			return index
		}
	}
	return settingsChoiceFirstSelectable(visible)
}

// settingsChoiceFirstSelectable returns the first option row, or -1 when the list is empty.
func settingsChoiceFirstSelectable(visible []visibleSettingsChoice) int {
	return settingsChoiceNearestSelectable(visible, -1, 1)
}

// settingsChoiceNearestSelectable walks from start in direction until it finds an option row.
func settingsChoiceNearestSelectable(visible []visibleSettingsChoice, start, delta int) int {
	if len(visible) == 0 || delta == 0 {
		return -1
	}
	index := start
	for range visible {
		index += delta
		if index < 0 {
			index = len(visible) - 1
		} else if index >= len(visible) {
			index = 0
		}
		if !visible[index].header {
			return index
		}
	}
	if start >= 0 && start < len(visible) && !visible[start].header {
		return start
	}
	return -1
}

func settingsChoiceItemHeight(item visibleSettingsChoice) float32 {
	if item.header {
		return settingsChoiceGroupHeight
	}
	return settingsChoiceRowHeight
}

func settingsChoiceOffset(visible []visibleSettingsChoice, index int) float32 {
	offset := float32(0)
	for i := 0; i < index && i < len(visible); i++ {
		offset += settingsChoiceItemHeight(visible[i])
	}
	return offset
}

func settingsChoiceContentHeight(visible []visibleSettingsChoice) float32 {
	return settingsChoiceOffset(visible, len(visible))
}

// filteredSettingsChoices retains original option indexes and inserts group headers.
func filteredSettingsChoices(choices []SettingsChoice, query string) []visibleSettingsChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	visible := make([]visibleSettingsChoice, 0, len(choices))
	lastGroup := ""
	for index, choice := range choices {
		if query != "" && !settingsChoiceMatches(choice, query) {
			continue
		}
		if choice.Group != "" && choice.Group != lastGroup {
			visible = append(visible, visibleSettingsChoice{
				choice: SettingsChoice{Label: choice.Group, Group: choice.Group, GroupTooltip: choice.GroupTooltip},
				header: true,
			})
			lastGroup = choice.Group
		}
		visible = append(visible, visibleSettingsChoice{choice: choice, originalIndex: index})
	}
	return visible
}

func settingsChoiceMatches(choice SettingsChoice, query string) bool {
	return strings.Contains(strings.ToLower(choice.Label), query) ||
		strings.Contains(strings.ToLower(choice.Trailing), query) ||
		strings.Contains(strings.ToLower(choice.Tooltip), query) ||
		strings.Contains(strings.ToLower(choice.Group), query)
}

// settingsChoicePreferredMenuWidth grows past the 200-wide Settings trigger when
// labels, version trailers, and info icons would otherwise clip inside that slot.
func settingsChoicePreferredMenuWidth(props SettingsChoiceProps, visible []visibleSettingsChoice, anchor woxui.Rect) float32 {
	maxAllowed := min(settingsChoiceMenuMaxWidth, max(float32(1), props.Width-settingsChoiceMenuMargin*2))
	width := max(float32(190), anchor.Width)
	for _, item := range visible {
		width = max(width, settingsChoiceRowPreferredWidth(item, props))
	}
	return min(maxAllowed, width)
}

func settingsChoiceRowPreferredWidth(item visibleSettingsChoice, props SettingsChoiceProps) float32 {
	const inset = float32(24)
	if item.header {
		width := inset + settingsChoiceEstimateTextWidth(item.choice.Label, woxcomponent.SettingsSectionTitleFontSize)
		if item.choice.GroupTooltip != "" && props.InfoIcon != nil {
			width += 28
		}
		return width
	}
	width := inset
	if item.choice.Leading != nil {
		width += 26
	}
	width += settingsChoiceEstimateTextWidth(item.choice.Label, 13)
	if item.choice.Trailing != "" {
		width += 12 + settingsChoiceEstimateTextWidth(item.choice.Trailing, 12)
	}
	if item.choice.Tooltip != "" {
		width += 28
	}
	return width
}

func settingsChoiceEstimateTextWidth(text string, size float32) float32 {
	width := float32(0)
	for _, r := range text {
		if r <= 0x7F {
			width += size * 0.6
			continue
		}
		width += size
	}
	return width
}

func settingsChoiceMenuTop(props SettingsChoiceProps, anchor woxui.Rect, menuHeight, listHeight float32) float32 {
	if props.Filterable {
		top := anchor.Y + anchor.Height
		if top+menuHeight > props.Height-settingsChoiceMenuMargin {
			top = anchor.Y - menuHeight
		}
		return min(max(settingsChoiceMenuMargin, top), max(settingsChoiceMenuMargin, props.Height-menuHeight-settingsChoiceMenuMargin))
	}
	visible := filteredSettingsChoices(props.Choices, "")
	currentIndex := settingsChoiceVisibleIndex(visible, props.CurrentValue)
	contentHeight := settingsChoiceContentHeight(visible)
	currentOffset := settingsChoiceOffset(visible, currentIndex)
	initialScroll := max(float32(0), currentOffset-4*settingsChoiceRowHeight)
	initialScroll = min(initialScroll, max(float32(0), contentHeight-listHeight))
	selectedCenter := settingsChoiceMenuPadding + currentOffset - initialScroll + settingsChoiceRowHeight/2
	top := anchor.Y + anchor.Height/2 - selectedCenter
	return min(max(settingsChoiceMenuMargin, top), max(settingsChoiceMenuMargin, props.Height-menuHeight-settingsChoiceMenuMargin))
}

func settingsChoiceMenu(context woxwidget.StateContext, props SettingsChoiceProps, state *settingsChoiceState, visible []visibleSettingsChoice, width, height, listHeight, menuPadding float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, max(1, len(visible)))
	for index, visibleChoice := range visible {
		choice := visibleChoice.choice
		if visibleChoice.header {
			rows = append(rows, settingsChoiceGroupHeader(width, choice, index, props))
			continue
		}
		selected := choice.Value == props.CurrentValue
		// The floating menu owns the translucent fill; repainting it here darkens only the rows.
		background := woxui.Color{}
		foreground := props.Theme.Text
		if selected {
			background = props.Theme.SelectionBackground
			foreground = props.Theme.SelectionText
		} else if state.hovered == index || (state.hovered < 0 && state.keyboardSelected && state.selected == index) {
			background = props.Theme.SelectionBackground
			background.A = uint8(float32(background.A)*0.25 + 0.5)
		}
		var tooltip woxwidget.Widget = woxwidget.Painter{}
		if choice.Tooltip != "" {
			tooltip = woxwidget.Gesture{ID: fmt.Sprintf("setting-choice-tooltip-%d", index), OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnTooltip != nil {
					props.OnTooltip(inside, choice.Tooltip, bounds)
				}
			}, Child: woxwidget.Container{Width: 28, Height: settingsChoiceRowHeight, Padding: woxwidget.Insets{Left: 6}, Child: woxwidget.Align{
				Width: 22, Height: settingsChoiceRowHeight, Vertical: 0.5, Child: woxwidget.Text{
					Value: "ⓘ", Style: woxui.TextStyle{Size: props.Theme.Scaled(14)}, Color: foreground,
				},
			}}}
		}
		activate := func() {
			if props.OnChoose != nil {
				props.OnChoose(visibleChoice.originalIndex)
			}
		}
		key := woxwidget.Key(fmt.Sprintf("setting-choice-%d", index))
		rowChildren := make([]woxwidget.Widget, 0, 6)
		if choice.Leading != nil {
			rowChildren = append(rowChildren,
				woxwidget.Align{Width: 18, Height: settingsChoiceRowHeight, Vertical: 0.5, Child: woxwidget.Image{Source: choice.Leading, Width: 18, Height: 18}},
				woxwidget.Container{Width: 8, Height: settingsChoiceRowHeight},
			)
		}
		rowChildren = append(rowChildren, woxwidget.Expanded{Child: woxwidget.Align{Height: settingsChoiceRowHeight, Vertical: 0.5, Child: woxwidget.TextBlock{
			Value: choice.Label, Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: foreground,
		}}})
		if choice.Trailing != "" {
			rowChildren = append(rowChildren,
				woxwidget.Container{Width: 12, Height: settingsChoiceRowHeight},
				woxwidget.Text{Value: choice.Trailing, Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: foreground},
			)
		}
		rowChildren = append(rowChildren, tooltip)
		rowContent := woxwidget.Container{
			Width: width, Height: settingsChoiceRowHeight, Padding: woxwidget.Insets{Left: 16, Right: 8},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: rowChildren},
		}
		var rowBackground woxwidget.Widget = woxwidget.Container{Width: width, Height: settingsChoiceRowHeight, Color: background}
		if menuPadding == 0 && index == len(visible)-1 {
			rowBackground = settingsChoiceRoundedEndBackground(width, settingsChoiceRowHeight, background, false)
		}
		row := woxwidget.Gesture{ID: string(key), OnHover: func(inside bool) {
			if inside && (state.hovered != index || state.selected != index) {
				context.SetState(func() {
					state.hovered = index
					state.selected = index
					state.keyboardSelected = false
					start := settingsChoiceOffset(visible, index)
					state.scrollController.EnsureVisible(start, start+settingsChoiceRowHeight)
				})
			} else if !inside && state.hovered == index {
				context.SetState(func() { state.hovered = -1 })
			}
		}, OnTap: activate, Child: woxwidget.Stack{Width: width, Height: settingsChoiceRowHeight, Children: []woxwidget.StackChild{{Child: rowBackground}, {Child: rowContent}}}}
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
	children := make([]woxwidget.Widget, 0, 2)
	if props.Filterable {
		filterHint := props.FilterHint
		if filterHint == "" {
			filterHint = "Filter..."
		}
		iconWidth := float32(0)
		var icon woxwidget.Widget = woxwidget.Painter{}
		if props.SearchIcon != nil {
			iconWidth = 28
			icon = woxwidget.Align{Width: iconWidth, Height: settingsChoiceSearchHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.SearchIcon, Width: 16, Height: 16}}
		}
		search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "setting-choice-search", Label: filterHint, Hint: filterHint, Width: max(float32(0), width-16-iconWidth), Height: 40, Radius: 0,
			Padding: woxwidget.Insets{Left: 2, Top: 9, Right: 8, Bottom: 7}, TextAlignmentY: 0.5, Transparent: true,
			Controller: state.queryController, FocusNode: state.queryFocusNode, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
			OnKey: func(event woxui.KeyEvent) bool { return state.handleKey(context, props, visible, event) },
			OnChanged: func(string) {
				context.SetState(func() {
					state.selected = settingsChoiceFirstSelectable(filteredSettingsChoices(props.Choices, state.queryController.Text()))
					state.hovered = -1
					state.keyboardSelected = false
					state.scrollController.JumpTo(0)
				})
			},
		})
		searchContent := woxwidget.Container{Width: width, Height: settingsChoiceSearchHeight, Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{icon, search}}}
		children = append(children, woxwidget.Stack{Width: width, Height: settingsChoiceSearchHeight, Children: []woxwidget.StackChild{
			{Child: settingsChoiceRoundedEndBackground(width, settingsChoiceSearchHeight, props.Theme.Background, true)},
			{Child: searchContent},
		}})
	}
	if len(rows) > 0 {
		children = append(children, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: woxwidget.Key(props.ID + "-scroll"), Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, Width: width, Height: listHeight,
			Controller: state.scrollController, Theme: props.Theme, ThumbColor: props.Theme.Text,
		}))
	}
	menuContent := woxwidget.Container{Width: width, Height: height, Radius: 4, Floating: true, Color: props.Theme.Surface,
		Padding: woxwidget.Insets{Top: menuPadding, Bottom: menuPadding},
		Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	// Paint the border after the rows so their full-width backgrounds cannot cover the inset stroke.
	// This also stays in Go rather than on the floating material, whose edge sits below the rows.
	menuBorder := woxwidget.Container{Width: width, Height: height, Radius: 4, BorderColor: props.Theme.Border, BorderWidth: 1}
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

// settingsChoiceGroupHeader paints a non-selectable category label above option rows.
func settingsChoiceGroupHeader(width float32, choice SettingsChoice, index int, props SettingsChoiceProps) woxwidget.Widget {
	key := woxwidget.Key(fmt.Sprintf("setting-choice-group-%d", index))
	children := []woxwidget.Widget{woxwidget.Text{
		Value: strings.ToUpper(choice.Label), Style: woxui.TextStyle{Size: props.Theme.Scaled(woxcomponent.SettingsSectionTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: props.Theme.TextSecondary,
	}}
	if choice.GroupTooltip != "" && props.InfoIcon != nil && props.OnTooltip != nil {
		children = append(children, woxwidget.Semantics{
			AutomationID: string(key) + "-tooltip", Role: woxui.AccessibilityRoleGroup, Label: choice.GroupTooltip,
			Child: woxwidget.Gesture{ID: string(key) + "-tooltip", OnHoverAt: func(inside bool, bounds woxui.Rect) {
				props.OnTooltip(inside, choice.GroupTooltip, bounds)
			}, Child: woxwidget.Align{Width: 28, Height: settingsChoiceGroupHeight, Horizontal: 0.5, Vertical: 0.5,
				Child: woxwidget.Image{Source: props.InfoIcon, Width: 14, Height: 14},
			}},
		})
	}
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleGroup, Label: choice.Label, Description: choice.GroupTooltip,
		Child: woxwidget.Container{
			Width: width, Height: settingsChoiceGroupHeight, Padding: woxwidget.Insets{Left: 16, Right: 16},
			Child: woxwidget.Align{Height: settingsChoiceGroupHeight, Vertical: 0.5, Child: woxwidget.Flex{
				Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
			}},
		},
	}
}

// settingsChoiceRoundedEndBackground keeps menu corner pixels transparent without requiring rounded subtree clipping.
func settingsChoiceRoundedEndBackground(width, height float32, color woxui.Color, top bool) woxwidget.Widget {
	const radius = float32(4)
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		displayList.FillRoundedRect(bounds, radius, color)
		if top {
			displayList.FillRect(woxui.Rect{X: bounds.X, Y: bounds.Y + radius, Width: bounds.Width, Height: max(float32(0), bounds.Height-radius)}, color)
		} else {
			displayList.FillRect(woxui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: max(float32(0), bounds.Height-radius)}, color)
		}
	}}
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
			s.hovered = -1
			s.keyboardSelected = true
			s.selected = settingsChoiceNearestSelectable(visible, s.selected, delta)
			start := settingsChoiceOffset(visible, s.selected)
			s.scrollController.EnsureVisible(start, start+settingsChoiceRowHeight)
		})
		return true
	case woxui.KeyEnter:
		if s.selected >= 0 && s.selected < len(visible) && !visible[s.selected].header && props.OnChoose != nil {
			props.OnChoose(visible[s.selected].originalIndex)
		}
		return true
	case woxui.KeySpace:
		if !props.Filterable && s.selected >= 0 && s.selected < len(visible) && !visible[s.selected].header && props.OnChoose != nil {
			props.OnChoose(visible[s.selected].originalIndex)
			return true
		}
	}
	return false
}
