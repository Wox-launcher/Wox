package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ShowDisplayPickerProps is the screen-only dialog for choosing where Wox opens.
type ShowDisplayPickerProps struct {
	Width       float32
	Height      float32
	Title       string
	Description string
	CancelLabel string
	Arrangement DisplayArrangementProps
	Theme       woxcomponent.ControlTheme
	OnCancel    func()
}

// ShowDisplayPicker builds the display-position screen picker.
func ShowDisplayPicker(props ShowDisplayPickerProps) woxwidget.Widget {
	panelWidth := min(float32(720), max(float32(560), props.Width-80))
	innerWidth := max(float32(0), panelWidth-48)
	const pickerChrome = float32(180)
	arrangementHeight := float32(360)
	if props.Height > 0 {
		if maxBody := props.Height - pickerChrome; maxBody > 220 && maxBody < arrangementHeight {
			arrangementHeight = maxBody
		}
	}
	panelHeight := pickerChrome + arrangementHeight
	border := windowGroupFadeColor(props.Theme.Border, 230)
	arrangement := props.Arrangement
	arrangement.Width = innerWidth
	arrangement.Height = arrangementHeight
	description := woxwidget.Widget(woxwidget.Container{Height: 0})
	if props.Description != "" {
		description = woxwidget.TextBlock{Value: props.Description, Width: innerWidth, MaxLines: 2, LineHeight: 16, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.TextSecondary}
	}
	cancel := woxwidget.Container{
		Width: innerWidth, Height: settingsDialogActionHeight,
		Child: woxwidget.Align{Width: innerWidth, Height: settingsDialogActionHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "show-display-cancel", Label: props.CancelLabel, Radius: 4, FontSize: 13, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnCancel, Theme: props.Theme,
		})},
	}
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 16, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text},
		description,
		DisplayArrangement(arrangement),
		cancel,
	}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "show-display-picker", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "show-display-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 1,
		OnEscape: props.OnCancel, OnBackdrop: props.OnCancel, Theme: props.Theme, Child: content,
	})
}
