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

// settingsDialogActions builds the shared right-aligned cancel/confirm footer used by settings dialogs.
func settingsDialogActions(width float32, theme woxcomponent.Theme, cancel, confirm settingsDialogAction) woxwidget.Widget {
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
				button(cancel, woxcomponent.ButtonOutline),
				button(confirm, woxcomponent.ButtonPrimary),
			}},
		},
	}
}
