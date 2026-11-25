package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

const (
	settingsDialogActionHeight = float32(32)
	settingsDialogActionGap    = float32(12)
	// SettingsDialogActionsHeight includes the action row and its top spacing.
	SettingsDialogActionsHeight = float32(44)
)

type settingsDialogAction struct {
	ID       string
	Label    string
	Disabled bool
	OnTap    func()
}

// settingsScrollableDialog keeps the footer visible when measured content exceeds the panel.
func settingsScrollableDialog(props woxcomponent.DialogProps, body, actions woxwidget.Widget, bodyHeight, minHeight, maxHeight float32) woxwidget.Widget {
	const gap = float32(12)
	chromeHeight := props.Padding.Top + props.Padding.Bottom + gap + SettingsDialogActionsHeight
	props.Height = max(0, min(maxHeight, props.OverlayHeight-56, max(minHeight, bodyHeight+chromeHeight)))
	scroll := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key(props.ID + "-content"), Width: max(0, props.Width-props.Padding.Left-props.Padding.Right),
		Height:  max(1, min(bodyHeight, props.Height-chromeHeight)),
		Content: body, Theme: props.Theme, ThumbColor: props.Theme.TextSecondary,
	})
	props.Child = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: gap, Children: []woxwidget.Widget{scroll, actions}}
	return woxcomponent.WoxDialog(props)
}

// settingsDialogActions builds the shared right-aligned cancel/confirm footer used by settings dialogs.
func settingsDialogActions(width float32, theme woxcomponent.ControlTheme, cancel, confirm settingsDialogAction) woxwidget.Widget {
	button := func(action settingsDialogAction, variant woxcomponent.ButtonVariant) woxwidget.Widget {
		return woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: action.ID, Label: action.Label, Radius: 4, FontSize: 13,
			Variant: variant, Disabled: action.Disabled, OnTap: action.OnTap, Theme: theme,
		})
	}
	return woxwidget.Container{
		Width: width, Height: SettingsDialogActionsHeight, Padding: woxwidget.Insets{Top: SettingsDialogActionsHeight - settingsDialogActionHeight},
		Child: woxwidget.Align{
			Width: width, Height: settingsDialogActionHeight, Horizontal: 1, Vertical: 0.5,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: settingsDialogActionGap, Children: []woxwidget.Widget{
				button(cancel, woxcomponent.ButtonSecondary),
				button(confirm, woxcomponent.ButtonPrimary),
			}},
		},
	}
}
