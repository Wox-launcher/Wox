package view

import (
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// OfflineSettingsProps is the immutable replacement for unavailable online catalogs.
type OfflineSettingsProps struct {
	Width, Height                     float32
	Theme                             component.ControlTheme
	Icon                              *ui.Image
	Window                            *ui.Window
	Title, Description, SettingsLabel string
	OnSettings                        func()
}

// OfflineSettingsView keeps the catalog destination visible with an explicit recovery action.
func OfflineSettingsView(props OfflineSettingsProps) widget.Widget {
	width := min(props.Theme.Scaled(440), max(float32(0), props.Width-48))
	content := widget.Container{Width: width, Child: widget.Flex{
		Axis: widget.Vertical, Gap: props.Theme.Scaled(16), CrossAxisAlignment: widget.CrossAxisCenter,
		Children: []widget.Widget{
			widget.Image{Source: props.Icon, Width: props.Theme.Scaled(48), Height: props.Theme.Scaled(48)},
			catalogCenteredText(width, props.Title, ui.TextStyle{Size: props.Theme.Scaled(component.SettingsPageTitleFontSize), Weight: ui.FontWeightSemibold}, props.Theme.Text),
			catalogCenteredTextBlock(props.Window, width, props.Theme.Scaled(20), 0, props.Description, ui.TextStyle{Size: props.Theme.Scaled(component.SettingsHelpFontSize)}, props.Theme.TextSecondary),
			widget.Container{Padding: widget.Insets{Top: props.Theme.Scaled(8)}, Child: component.WoxButton(component.ButtonProps{
				ID: "offline-open-settings", Label: props.SettingsLabel, OnTap: props.OnSettings, Theme: props.Theme,
			})},
		},
	}}
	return widget.ScrollView{ID: "offline-settings-scroll", Width: props.Width, Height: props.Height, Child: widget.Align{
		Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: content,
	}}
}
