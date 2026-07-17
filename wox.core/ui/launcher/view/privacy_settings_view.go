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
	Selected             int
	TelemetryTitle       string
	TelemetryDescription string
	TelemetryEnabled     bool
	SampleTitle          string
	SampleDescription    string
	Sample               string
	Error                string
	OnSelect             func(int)
	OnToggleTelemetry    func()
	OnToggleSample       func()
	OnCopySample         func()
}

// PrivacySettingsView builds the privacy page without depending on launcher controller state.
func PrivacySettingsView(props PrivacySettingsProps) woxwidget.Widget {
	contentWidth := min(float32(760), max(float32(0), props.Width-72))
	telemetryBackground := props.Theme.QueryBackground
	sampleBackground := props.Theme.QueryBackground
	if props.Selected == 0 {
		telemetryBackground = props.Theme.SelectedBackground
	}
	if props.Selected == 1 {
		sampleBackground = props.Theme.SelectedBackground
	}
	telemetryLabel := "Off"
	if props.TelemetryEnabled {
		telemetryLabel = "On"
	}
	sampleLabel := "View sample"
	if props.Sample != "" {
		sampleLabel = "Hide sample"
	}
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
			Title: "Privacy", Description: "Control anonymous product telemetry and inspect what it contains",
			Width: contentWidth, Height: 62, TitleSize: 24, Gap: 7, Theme: props.Theme,
		}),
		privacySettingCard("privacy-telemetry", props.TelemetryTitle, props.TelemetryDescription, telemetryLabel, contentWidth, telemetryBackground, props.Theme, func() {
			selectPrivacyRow(props, 0)
		}, func() {
			selectPrivacyRow(props, 0)
			if props.OnToggleTelemetry != nil {
				props.OnToggleTelemetry()
			}
		}),
		privacySettingCard("privacy-sample", props.SampleTitle, props.SampleDescription, sampleLabel, contentWidth, sampleBackground, props.Theme, func() {
			selectPrivacyRow(props, 1)
		}, func() {
			selectPrivacyRow(props, 1)
			if props.OnToggleSample != nil {
				props.OnToggleSample()
			}
		}),
	}
	if props.Sample != "" {
		children = append(children, privacySampleCard(props, contentWidth))
	}
	if props.Error != "" {
		children = append(children, woxwidget.TextBlock{
			Value: props.Error, Width: contentWidth, Height: 32, MaxLines: 2,
			Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: props.Theme.ResultSubtitle,
		})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	}
}

func selectPrivacyRow(props PrivacySettingsProps, row int) {
	if props.OnSelect != nil {
		props.OnSelect(row)
	}
}

func privacySettingCard(id, title, description, value string, width float32, background woxui.Color, theme woxcomponent.Theme, onHover, onTap func()) woxwidget.Widget {
	return woxwidget.Gesture{ID: id, OnHover: func(inside bool) {
		if inside && onHover != nil {
			onHover()
		}
	}, OnTap: onTap, Child: woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: 70, Radius: 10, Color: background, Padding: woxwidget.Insets{Left: 18, Top: 9, Right: 16, Bottom: 9}, Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(180), width-208), Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.Text{Value: description, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle},
			}}},
			woxwidget.Container{Width: 160, Height: 38, Radius: 8, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 10}, Child: woxwidget.Text{
				Value: value, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.Cursor,
			}},
		}},
	})}
}

func privacySampleCard(props PrivacySettingsProps, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-28)
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: 282, Radius: 10, Padding: woxwidget.UniformInsets(14), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(120), innerWidth-100), Height: 30, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
					Value: "Anonymous telemetry payload", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle,
				}},
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "privacy-copy", Label: "Copy", Width: 90, OnTap: props.OnCopySample, Theme: props.Theme}),
			}}},
			woxwidget.Container{Width: innerWidth, Height: 210, Radius: 8, Color: props.Theme.ToolbarBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.TextBlock{
				Value: props.Sample, Width: innerWidth - 28, Height: 182, Style: woxui.TextStyle{Size: 12}, LineHeight: 20, Color: props.Theme.ResultTitle,
			}},
		}},
	})
}
