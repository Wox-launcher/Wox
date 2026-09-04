package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	ActionPanelContentWidth = 320
	ActionRowHeight         = 40
	// ActionHeaderHeight is the 18px CJK line already used by action labels.
	// A 16px Text slot clips the native ascent of 13px Chinese titles such as 操作.
	ActionHeaderHeight  = 18
	ActionDividerHeight = 16
	// ActionGroupDividerHeight matches the title divider so plugin and system groups share one hairline slot.
	ActionGroupDividerHeight = ActionDividerHeight
	ActionSearchHeight       = 46
	MaxVisibleActions        = 8
)

// ActionItemKind distinguishes selectable actions from non-interactive group chrome.
type ActionItemKind uint8

const (
	ActionItemKindAction ActionItemKind = iota
	ActionItemKindSeparator
)

// ActionItem contains resolved presentation data for one result action or group chrome.
type ActionItem struct {
	Kind         ActionItemKind
	Index        int
	ID           string
	Label        string
	Icon         *woxui.Image
	HotkeyLabels []string
}

// Equal compares every prepared visual field for one action item.
func (i ActionItem) Equal(other ActionItem) bool {
	if i.Kind != other.Kind || i.Index != other.Index || i.ID != other.ID || i.Label != other.Label || i.Icon != other.Icon || len(i.HotkeyLabels) != len(other.HotkeyLabels) {
		return false
	}
	for index := range i.HotkeyLabels {
		if i.HotkeyLabels[index] != other.HotkeyLabels[index] {
			return false
		}
	}
	return true
}

// ActionItemHeight returns the list-slot height for one action or group divider.
func ActionItemHeight(item ActionItem) float32 {
	if item.Kind == ActionItemKindSeparator {
		return ActionGroupDividerHeight
	}
	return ActionRowHeight
}

// ActionPanelListHeight sizes the visible action list, counting at most MaxVisibleActions rows.
func ActionPanelListHeight(items []ActionItem) float32 {
	if len(items) == 0 {
		return ActionRowHeight
	}
	visibleActions := 0
	height := float32(0)
	for _, item := range items {
		if item.Kind == ActionItemKindSeparator {
			if visibleActions > 0 && visibleActions < MaxVisibleActions {
				height += ActionGroupDividerHeight
			}
			continue
		}
		if visibleActions >= MaxVisibleActions {
			break
		}
		height += ActionRowHeight
		visibleActions++
	}
	if visibleActions == 0 {
		return ActionRowHeight
	}
	return height
}

// ActionsProps contains the action panel state and callbacks.
type ActionsProps struct {
	Revision              uint64
	Window                *woxui.Window
	WindowWidth           float32
	WindowHeight          float32
	QueryHeight           float32
	ToolbarHeight         float32
	DensityScale          float32
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
	Filter                string
	OnSelect              func(int)                 `boundary:"stable"`
	OnActivate            func()                    `boundary:"stable"`
	OnFilterChanged       func(string)              `boundary:"stable"`
	OnFilterKey           func(woxui.KeyEvent) bool `boundary:"stable"`
}

// Equal compares every render dependency for the floating action panel.
func (p ActionsProps) Equal(other ActionsProps) bool {
	if p.Revision != other.Revision || p.Window != other.Window || p.WindowWidth != other.WindowWidth || p.WindowHeight != other.WindowHeight || p.QueryHeight != other.QueryHeight || p.ToolbarHeight != other.ToolbarHeight || p.DensityScale != other.DensityScale || p.Theme != other.Theme || p.ActionHeader != other.ActionHeader || p.ActionQueryBackground != other.ActionQueryBackground || p.ActionQueryText != other.ActionQueryText || p.ResultTail != other.ResultTail || p.SelectedTail != other.SelectedTail || p.ResultItemRadius != other.ResultItemRadius || p.ActionQueryRadius != other.ActionQueryRadius || p.ActionPadding != other.ActionPadding || p.HeaderLabel != other.HeaderLabel || p.NoMatchesLabel != other.NoMatchesLabel || p.Selected != other.Selected || p.Filter != other.Filter || len(p.Items) != len(other.Items) {
		return false
	}
	for index := range p.Items {
		if !p.Items[index].Equal(other.Items[index]) {
			return false
		}
	}
	return true
}

// ActionPanelBaseHeight returns the non-list height used by launcher window sizing.
func ActionPanelBaseHeight(padding woxwidget.Insets) float32 {
	return ActionHeaderHeight + ActionDividerHeight + ActionSearchHeight + padding.Top + padding.Bottom
}

// ActionPanelWidth returns the floating panel width for the current launcher geometry.
func ActionPanelWidth(padding woxwidget.Insets, windowWidth float32) float32 {
	return min(float32(ActionPanelContentWidth)+padding.Left+padding.Right, max(float32(240), windowWidth-28))
}

type actionsViewState struct {
	scrollController *woxwidget.ScrollController
}

// ActionsView creates the retained floating action picker and returns its geometry.
func ActionsView(props ActionsProps) (woxwidget.Widget, float32, float32) {
	panelWidth, _, panelHeight, _ := actionPanelGeometry(props)
	view := woxwidget.Stateful{
		Key: "actions-view", Type: (*actionsViewState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &actionsViewState{} },
	}
	return view, panelWidth, panelHeight
}

// ActionsBoundary composes the floating action panel. Scroll chrome stays outside
// any Boundary so WOX_DEBUG_REPAINT=verify does not compare a live scrollbar
// animation with a shadow rebuild at rest, which would rebuild the whole panel
// on every caret blink and report panel-sized idle damage.
func ActionsBoundary(props ActionsProps) (woxwidget.Widget, float32, float32) {
	return ActionsView(props)
}

// InitState creates the action list controller when the panel enters the Host tree.
func (s *actionsViewState) InitState(_ woxwidget.StateContext, _ any) {
	s.scrollController = woxwidget.NewScrollController(0)
}

// DidUpdateWidget preserves the action list controller across immutable prop updates.
func (s *actionsViewState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build keeps transient scrolling inside the action panel while selection remains controller-owned.
func (s *actionsViewState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	return buildActionsView(context, widget.(ActionsProps), s.scrollController)
}

// Dispose leaves controller detachment to the nested retained ScrollView.
func (s *actionsViewState) Dispose() {}

// actionPanelGeometry calculates the stable panel and list extents used by both the adapter and retained State.
func actionPanelGeometry(props ActionsProps) (panelWidth, innerWidth, panelHeight, listHeight float32) {
	panelWidth = ActionPanelWidth(props.ActionPadding, props.WindowWidth)
	innerWidth = max(float32(0), panelWidth-props.ActionPadding.Left-props.ActionPadding.Right)
	listHeight = ActionPanelListHeight(props.Items)
	panelHeight = ActionPanelBaseHeight(props.ActionPadding) + listHeight
	panelHeight = min(panelHeight, max(float32(100), props.WindowHeight-props.QueryHeight-props.ToolbarHeight-20))
	return panelWidth, innerWidth, panelHeight, listHeight
}

// buildActionsView composes the current immutable action rows around the retained scroll controller.
func buildActionsView(context woxwidget.StateContext, props ActionsProps, scrollController *woxwidget.ScrollController) woxwidget.Widget {
	panelWidth, innerWidth, panelHeight, listHeight := actionPanelGeometry(props)
	actionTitleFontSize := scaledLauncherSize(woxcomponent.ActionTitleFontSize, props.DensityScale)
	actionHeaderFontSize := scaledLauncherSize(woxcomponent.ActionHeaderFontSize, props.DensityScale)
	actionFilterFontSize := scaledLauncherSize(woxcomponent.ActionFilterFontSize, props.DensityScale)
	emptyFontSize := scaledLauncherSize(woxcomponent.ListEmptyFontSize, props.DensityScale)
	headerLineHeight := scaledLauncherSize(ActionHeaderHeight, props.DensityScale)
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Items)))
	for _, item := range props.Items {
		if item.Kind == ActionItemKindSeparator {
			rows = append(rows, actionPanelDivider(innerWidth, props.Theme.PreviewSplit))
			continue
		}
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
				Labels: item.HotkeyLabels, Foreground: tailColor, Background: chipBackground,
				FontSize: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale), Window: props.Window,
			})
			hotkeyWidth = chipWidth + 15
			hotkey = woxwidget.Align{Width: hotkeyWidth, Height: ActionRowHeight, Vertical: 0.5, Child: woxwidget.Container{
				Width: hotkeyWidth, Padding: woxwidget.Insets{Left: 10, Right: 5}, Child: chip,
			}}
		}
		labelWidth := max(float32(40), innerWidth-37-hotkeyWidth)
		labelLineHeight := scaledLauncherSize(18, props.DensityScale)
		activate := func() {
			if props.OnSelect != nil {
				props.OnSelect(item.Index)
			}
			if props.OnActivate != nil {
				props.OnActivate()
			}
		}
		automationID := "action-" + item.ID
		row := woxwidget.Gesture{
			ID: "action-" + item.ID,
			OnHover: func(inside bool) {
				if inside && props.OnSelect != nil {
					props.OnSelect(item.Index)
				}
			},
			OnTap: activate,
			Child: woxwidget.Container{Width: innerWidth, Height: ActionRowHeight, Radius: props.ResultItemRadius, Color: background, Child: woxwidget.Flex{
				Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
					woxwidget.Align{Width: 37, Height: ActionRowHeight, Vertical: 0.5, Child: woxwidget.Container{
						Width: 37, Padding: woxwidget.Insets{Left: 5, Right: 10}, Child: icon,
					}},
					woxwidget.Align{Width: labelWidth, Height: ActionRowHeight, Vertical: 0.5, Child: woxwidget.TextBlock{
						Value: item.Label, Width: labelWidth, Height: labelLineHeight, LineHeight: labelLineHeight, MaxLines: 1, AlignmentY: 0.5,
						Style: woxui.TextStyle{Size: actionTitleFontSize}, Color: foreground,
					}},
					hotkey,
				},
			}},
		}
		rows = append(rows, woxwidget.Semantics{
			Key: woxwidget.Key(automationID), AutomationID: automationID, Role: woxui.AccessibilityRoleMenuItem, Label: item.Label, Selected: selected,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action == woxui.AccessibilityActionActivate {
					activate()
				}
				return nil
			},
			Child: row,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Align{Width: innerWidth, Height: ActionRowHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxcomponent.SearchGlyph(scaledLauncherSize(18, props.DensityScale), props.ActionHeader),
				woxwidget.Text{Value: props.NoMatchesLabel, Style: woxui.TextStyle{Size: emptyFontSize}, Color: props.ActionHeader},
			},
		}})
	}
	var keepVisible *woxwidget.ScrollRange
	offset := float32(0)
	for _, item := range props.Items {
		height := ActionItemHeight(item)
		if item.Kind == ActionItemKindAction && item.Index == props.Selected {
			keepVisible = &woxwidget.ScrollRange{Start: offset, End: offset + height}
			break
		}
		offset += height
	}
	actionList := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "action-scroll", Controller: scrollController, KeepVisible: keepVisible, Width: innerWidth, Height: listHeight,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.ActionHeader,
	})
	search := actionSearchBoundary(actionSearchProps{
		Width: innerWidth, Height: 40, Radius: props.ActionQueryRadius,
		Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 8}, Background: props.ActionQueryBackground,
		Style: woxui.TextStyle{Size: actionFilterFontSize}, TextColor: props.ActionQueryText, Filter: props.Filter,
		Window: props.Window, Theme: props.Theme, OnChanged: props.OnFilterChanged, OnKey: props.OnFilterKey,
	})
	panel := woxwidget.Container{
		Width: panelWidth, Height: panelHeight, Radius: props.ActionQueryRadius, Color: props.Theme.ActionBackground,
		Padding: props.ActionPadding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Align{Width: innerWidth, Height: ActionHeaderHeight, Vertical: 0.5, Child: woxwidget.TextBlock{
				Value: props.HeaderLabel, Width: innerWidth, Height: headerLineHeight, LineHeight: headerLineHeight, MaxLines: 1, AlignmentY: 0.5,
				Style: woxui.TextStyle{Size: actionHeaderFontSize}, Color: props.ActionHeader,
			}},
			actionPanelDivider(innerWidth, props.Theme.PreviewSplit),
			actionList,
			woxwidget.Container{Width: innerWidth, Height: ActionSearchHeight, Padding: woxwidget.Insets{Top: 6}, Child: search},
		}},
	}
	// Keep non-interactive panel chrome opaque to pointer hit testing so native composition content cannot receive clicks through it.
	return woxwidget.Gesture{ID: "action-panel-surface", OnTap: func() {}, Child: panel}
}

// actionPanelDivider shares the centered hairline used below the title and between action groups.
func actionPanelDivider(width float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Align{
		Width: width, Height: ActionDividerHeight, Vertical: 0.5,
		Child: woxwidget.Container{Width: width, Height: 1, Color: color},
	}
}

type actionSearchProps struct {
	Width      float32
	Height     float32
	Radius     float32
	Padding    woxwidget.Insets
	Background woxui.Color
	Style      woxui.TextStyle
	TextColor  woxui.Color
	Filter     string
	Window     *woxui.Window
	Theme      woxcomponent.Theme
	OnChanged  func(string)              `boundary:"stable"`
	OnKey      func(woxui.KeyEvent) bool `boundary:"stable"`
}

// Equal compares every render dependency for the retained action filter.
func (p actionSearchProps) Equal(other actionSearchProps) bool {
	return p.Width == other.Width && p.Height == other.Height && p.Radius == other.Radius && p.Padding == other.Padding && p.Background == other.Background && p.Style == other.Style && p.TextColor == other.TextColor && p.Filter == other.Filter && p.Window == other.Window && p.Theme == other.Theme
}

// actionSearchBoundary retains the action filter so caret blinks and text-field
// invalidations stay inside the input instead of walking to a full-width ancestor.
func actionSearchBoundary(props actionSearchProps) woxwidget.Widget {
	return woxwidget.Boundary[actionSearchProps]{
		Key: "launcher-actions-search-boundary", Label: "actions:search", Props: props,
		Build: func(props actionSearchProps) woxwidget.Widget {
			return woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
				ID: "action-search", Label: "Filter actions", Width: props.Width, Height: props.Height, Radius: props.Radius,
				Padding: props.Padding, Background: props.Background, Style: props.Style, TextColor: props.TextColor,
				Value: props.Filter, Focused: true, Autofocus: true, DisableHover: true, MaxLines: 1,
				Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
			})
		},
	}
}
