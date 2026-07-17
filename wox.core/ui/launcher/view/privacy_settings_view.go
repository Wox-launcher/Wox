package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PrivacySettingsProps contains the immutable state and actions rendered by the privacy page.
type PrivacySettingsProps struct {
	Width                float32
	Height               float32
	Theme                woxcomponent.Theme
	Title                string
	Description          string
	TelemetryTitle       string
	TelemetryDescription string
	TelemetryEnabled     bool
	ViewSampleLabel      string
	Error                string
	OnToggleTelemetry    func()
	OnViewSample         func()
}

// PrivacySettingsView builds the privacy page without depending on launcher controller state.
func PrivacySettingsView(props PrivacySettingsProps) woxwidget.Widget {
	contentWidth := min(float32(1120), max(float32(0), props.Width-82))
	const controlWidth = float32(178)
	labelWidth := min(float32(550), max(float32(180), contentWidth-controlWidth-32))
	controlAreaWidth := max(controlWidth, contentWidth-labelWidth-32)
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "privacy-view-sample", Label: props.ViewSampleLabel, Width: 126, Height: 30, FontSize: 12,
			Padding: woxwidget.Insets{Right: 8}, Variant: woxcomponent.ButtonText, OnTap: props.OnViewSample, Theme: props.Theme,
		}),
		woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
			ID: "privacy-telemetry-switch", Label: props.TelemetryTitle, Value: props.TelemetryEnabled,
			OnChange: func(bool) {
				if props.OnToggleTelemetry != nil {
					props.OnToggleTelemetry()
				}
			}, Theme: props.Theme,
		}),
	}}
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
			Title: props.Title, Description: props.Description, Width: contentWidth, Theme: props.Theme,
		}),
		woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
			Label: props.TelemetryTitle, Description: props.TelemetryDescription, Width: contentWidth, Height: 84,
			LabelWidth: labelWidth, Gap: 32, DescriptionMaxLines: 3, Theme: props.Theme,
			Child: woxwidget.Align{Width: controlAreaWidth, Height: 84, Horizontal: 1, Vertical: 0.5, Child: controls},
		}),
	}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{
			Value: props.Error, Width: contentWidth, Height: 32, MaxLines: 2,
			Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: props.Theme.ErrorText,
		})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 28},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}

// PrivacySampleDialogProps contains the translated copy and actions for the telemetry sample dialog.
type PrivacySampleDialogProps struct {
	Width        float32
	Height       float32
	Theme        woxcomponent.Theme
	Title        string
	Sample       string
	CopyLabel    string
	ConfirmLabel string
	Error        string
	OnCopy       func()
	OnClose      func()
}

// PrivacySampleDialog builds the modal payload preview used by the privacy page.
func PrivacySampleDialog(props PrivacySampleDialogProps) woxwidget.Widget {
	dialogWidth := max(float32(0), min(float32(500), props.Width-40))
	dialogHeight := max(float32(0), min(float32(370), props.Height-40))
	innerWidth := max(float32(0), dialogWidth-40)
	innerHeight := max(float32(0), dialogHeight-40)
	fixedHeight := float32(90)
	if props.Error != "" {
		fixedHeight = 122
	}
	sampleHeight := max(float32(100), innerHeight-fixedHeight)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 28, Child: woxwidget.Text{
			Value: props.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}},
		woxwidget.Container{Width: innerWidth, Height: sampleHeight, Radius: 8, Color: privacyColorAlpha(props.Theme.ActionText, 13), BorderColor: props.Theme.PreviewSplit, BorderWidth: 1, Padding: woxwidget.UniformInsets(12), Child: woxwidget.TextBlock{
			Value: props.Sample, Width: max(float32(0), innerWidth-24), Height: max(float32(0), sampleHeight-24),
			Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: props.Theme.ActionText,
		}},
	}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{
			Value: props.Error, Width: innerWidth, Height: 20, MaxLines: 1,
			Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText,
		})
	}
	children = append(children, woxwidget.Container{Width: innerWidth, Height: 38, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-190), Height: 38},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "privacy-sample-copy", Label: props.CopyLabel, Width: 92, OnTap: props.OnCopy, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "privacy-sample-close", Label: props.ConfirmLabel, Width: 88, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnClose, Theme: props.Theme}),
	}}})
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "privacy-sample-dialog", Label: props.Title, Width: dialogWidth, Height: dialogHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "privacy-sample-backdrop", BackdropColor: woxui.Color{R: 0, G: 0, B: 0, A: 112},
		Padding: woxwidget.UniformInsets(20), OnDismiss: props.OnClose, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	})
}

func privacyColorAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
