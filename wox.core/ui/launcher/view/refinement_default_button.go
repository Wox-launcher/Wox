package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RefinementDefaultButtonProps represents the save or remove action for a filter bar.
type RefinementDefaultButtonProps struct {
	Label, Hotkey string
	Saved, Busy   bool
	OnToggle      func()                         `boundary:"stable"`
	OnTooltip     func(bool, string, woxui.Rect) `boundary:"stable"`
}

// Equal compares the visual state of the optional bookmark button.
func (p *RefinementDefaultButtonProps) Equal(other *RefinementDefaultButtonProps) bool {
	if p == nil || other == nil {
		return p == other
	}
	return p.Label == other.Label && p.Hotkey == other.Hotkey && p.Saved == other.Saved && p.Busy == other.Busy
}

// refinementDefaultButton presents the bookmark and shortcut as one click target.
func refinementDefaultButton(button RefinementDefaultButtonProps, props RefinementsProps, height float32) woxwidget.Widget {
	s := func(n float32) float32 { return scaledLauncherSize(n, props.DensityScale) }
	style := refinementSummaryStyle(props.DensityScale)
	gap, iconSize := s(6), s(15)
	width := s(16) + iconSize + 2*gap + 1 + refinementTextWidth(props.Window, button.Hotkey, style)
	const key woxwidget.Key = "refinement-default"
	control := woxcomponent.Hoverable(key+"/hover", false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
		foreground, iconColor, background, border := props.Theme.RefinementButtonColors(button.Saved, hovered && !button.Busy)
		return woxwidget.Gesture{
			ID: string(key), OnTap: button.OnToggle, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				onHoverAt(inside, bounds)
				if button.OnTooltip != nil {
					button.OnTooltip(inside, button.Label, bounds)
				}
			},
			Child: woxwidget.Container{
				Width: width, Height: height, Radius: s(7), Color: background, BorderColor: border, BorderWidth: 1,
				Padding: woxwidget.Insets{Left: s(8), Right: s(8)}, Child: woxwidget.Align{Vertical: 0.5, Child: woxwidget.Flex{
					Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter,
					Children: []woxwidget.Widget{
						woxcomponent.BookmarkGlyph(iconSize, iconColor, button.Saved),
						woxwidget.Container{Width: 1, Height: s(14), Color: border},
						woxwidget.Text{Value: button.Hotkey, Style: style, Color: foreground},
					},
				}},
			},
		}
	})
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleButton,
		Label: button.Label, Description: button.Hotkey, Selected: button.Saved, Disabled: button.Busy,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && button.OnToggle != nil {
				button.OnToggle()
			}
			return nil
		},
		Child: woxwidget.Focusable{Key: key, Disabled: button.Busy, FocusRingColor: props.Theme.Controls.Focus, FocusRingRadius: s(7), Child: control},
	}
}
