package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TextEditContextAction identifies one standard text-field context menu command.
type TextEditContextAction uint8

const (
	TextEditContextCut TextEditContextAction = iota
	TextEditContextCopy
	TextEditContextPaste
	TextEditContextSelectAll
)

// TextEditContextMenuProps describes Cut/Copy/Paste/Select All enablement for one menu.
type TextEditContextMenuProps struct {
	ID           string
	Theme        Theme
	CanCut       bool
	CanCopy      bool
	CanPaste     bool
	CanSelectAll bool
	OnAction     func(TextEditContextAction)
}

type textEditContextMenuItemProps struct {
	ID              string
	Label           string
	Enabled         bool
	TextColor       woxui.Color
	HoverTextColor  woxui.Color
	HoverBackground woxui.Color
	OnTap           func()
}

type textEditContextMenuItemState struct {
	hovered bool
}

// TextEditContextMenuItemID returns the stable automation ID for one menu command.
func TextEditContextMenuItemID(menuID string, action TextEditContextAction) string {
	switch action {
	case TextEditContextCut:
		return menuID + ".cut"
	case TextEditContextCopy:
		return menuID + ".copy"
	case TextEditContextPaste:
		return menuID + ".paste"
	case TextEditContextSelectAll:
		return menuID + ".selectAll"
	default:
		return menuID + ".unknown"
	}
}

// BuildTextEditContextMenu builds the shared Cut/Copy/Paste/Select All popup content.
func BuildTextEditContextMenu(props TextEditContextMenuProps) woxwidget.Widget {
	items := []struct {
		label   string
		action  TextEditContextAction
		enabled bool
	}{
		{label: "Cut", action: TextEditContextCut, enabled: props.CanCut},
		{label: "Copy", action: TextEditContextCopy, enabled: props.CanCopy},
		{label: "Paste", action: TextEditContextPaste, enabled: props.CanPaste},
		{label: "Select All", action: TextEditContextSelectAll, enabled: props.CanSelectAll},
	}
	// Floating menus must stay opaque; QueryBackground is often translucent for acrylic windows.
	background := props.Theme.ActionBackground
	if background.A == 0 {
		background = props.Theme.QueryBackground
	}
	background.A = 255
	border := props.Theme.ResultSubtitle
	if border.A == 0 {
		border = props.Theme.ActionText
	}
	border.A = 140
	textColor := props.Theme.ActionText
	if textColor.A == 0 {
		textColor = props.Theme.QueryText
	}
	hoverBackground := props.Theme.ActionSelected
	if hoverBackground.A == 0 {
		hoverBackground = props.Theme.SelectedBackground
	}
	hoverText := props.Theme.ActionSelectedText
	if hoverText.A == 0 {
		hoverText = textColor
	}
	children := make([]woxwidget.Widget, 0, len(items))
	for _, item := range items {
		label := item.label
		action := item.action
		enabled := item.enabled
		color := textColor
		if !enabled {
			color.A = 96
		}
		itemID := TextEditContextMenuItemID(props.ID, action)
		children = append(children, woxwidget.Stateful{
			Key: woxwidget.Key(itemID), Type: (*textEditContextMenuItemState)(nil),
			Widget: textEditContextMenuItemProps{
				ID: itemID, Label: label, Enabled: enabled, TextColor: color,
				HoverTextColor: hoverText, HoverBackground: hoverBackground,
				OnTap: func() {
					if enabled && props.OnAction != nil {
						props.OnAction(action)
					}
				},
			},
			CreateState: func() woxwidget.State { return &textEditContextMenuItemState{} },
		})
	}
	return woxwidget.Container{
		Width: textFieldContextMenuWidth, Height: textFieldContextMenuRowH * float32(len(children)), Radius: 6,
		Color: background, BorderColor: border, BorderWidth: 1,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}

func (s *textEditContextMenuItemState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *textEditContextMenuItemState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

func (s *textEditContextMenuItemState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(textEditContextMenuItemProps)
	background := woxui.Color{}
	foreground := props.TextColor
	if props.Enabled && s.hovered {
		background = props.HoverBackground
		foreground = props.HoverTextColor
	}
	var onTap func()
	var actions []woxui.AccessibilityAction
	if props.Enabled {
		onTap = props.OnTap
		actions = []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
	}
	content := woxwidget.Gesture{
		ID:    props.ID,
		OnTap: onTap,
		OnHover: func(inside bool) {
			if !props.Enabled {
				inside = false
			}
			if inside != s.hovered {
				context.SetState(func() { s.hovered = inside })
			}
		},
		Child: woxwidget.Container{
			Width: textFieldContextMenuWidth, Height: textFieldContextMenuRowH,
			Padding: woxwidget.Insets{Left: 4, Right: 4, Top: 2, Bottom: 2},
			Child: woxwidget.Container{
				Width: textFieldContextMenuWidth - 8, Height: textFieldContextMenuRowH - 4,
				Radius: 4, Color: background, Padding: woxwidget.Insets{Left: 8, Right: 8},
				Child: woxwidget.Align{
					Width: textFieldContextMenuWidth - 24, Height: textFieldContextMenuRowH - 4, Vertical: 0.5,
					Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 13}, Color: foreground},
				},
			},
		},
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(props.ID), AutomationID: props.ID, Role: woxui.AccessibilityRoleMenuItem,
		Label: props.Label, Disabled: !props.Enabled, Selected: props.Enabled && s.hovered, Actions: actions,
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && onTap != nil {
				onTap()
			}
			return nil
		},
		Child: content,
	}
}

func (s *textEditContextMenuItemState) Dispose() {}

// PlaceTextEditContextMenu builds a full-window dismissible overlay around a context menu.
func PlaceTextEditContextMenu(frame woxui.Size, windowPos woxui.Point, ownerID string, menu woxwidget.Widget, onDismiss func()) woxwidget.Widget {
	menuHeight := textFieldContextMenuRowH * 4
	left := windowPos.X
	top := windowPos.Y
	if frame.Width > 0 {
		left = min(max(float32(4), left), max(float32(4), frame.Width-textFieldContextMenuWidth-4))
	}
	if frame.Height > 0 {
		if top+menuHeight > frame.Height-4 {
			top = max(float32(4), windowPos.Y-menuHeight)
		}
		top = min(max(float32(4), top), max(float32(4), frame.Height-menuHeight-4))
	}
	return woxwidget.Stack{
		Width: frame.Width, Height: frame.Height,
		Children: []woxwidget.StackChild{
			{Child: woxwidget.Gesture{
				ID:    ownerID + "-context-dismiss",
				OnTap: onDismiss,
				Child: woxwidget.Container{Width: frame.Width, Height: frame.Height},
			}},
			{Left: left, Top: top, Child: menu},
		},
	}
}
