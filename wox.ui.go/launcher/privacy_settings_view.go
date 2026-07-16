package launcher

import (
	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildPrivacySettingsPage exposes the telemetry opt-in and core's representative payload.
func (a *App) buildPrivacySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := min(float32(760), max(float32(0), width-72))
	items := settingItemsForSnapshot(snapshot)
	telemetrySelected := snapshot.row == 0
	sampleSelected := snapshot.row == 1
	telemetryBackground := snapshot.palette.queryBackground
	sampleBackground := snapshot.palette.queryBackground
	if telemetrySelected {
		telemetryBackground = snapshot.palette.selectedBackground
	}
	if sampleSelected {
		sampleBackground = snapshot.palette.selectedBackground
	}
	telemetryLabel := "Off"
	if snapshot.data.EnableAnonymousUsageStats {
		telemetryLabel = "On"
	}
	sampleLabel := "View sample"
	if snapshot.privacySample != "" {
		sampleLabel = "Hide sample"
	}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Privacy", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: "Control anonymous product telemetry and inspect what it contains", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
		woxwidget.Gesture{ID: "privacy-telemetry", OnHover: func(inside bool) {
			if inside {
				a.selectSettingRow(0)
			}
		}, OnTap: func() {
			a.selectSettingRow(0)
			a.activateSetting(1)
		}, Child: privacySettingCard(items[0].title, items[0].description, telemetryLabel, contentWidth, telemetryBackground, snapshot.palette)},
		woxwidget.Gesture{ID: "privacy-sample", OnHover: func(inside bool) {
			if inside {
				a.selectSettingRow(1)
			}
		}, OnTap: func() {
			a.selectSettingRow(1)
			a.togglePrivacySample()
		}, Child: privacySettingCard(items[1].title, items[1].description, sampleLabel, contentWidth, sampleBackground, snapshot.palette)},
	}
	if snapshot.privacySample != "" {
		children = append(children, a.buildPrivacySampleCard(snapshot, contentWidth))
	}
	if snapshot.privacyError != "" {
		children = append(children, woxwidget.TextBlock{Value: snapshot.privacyError, Width: contentWidth, Height: 32, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: snapshot.palette.resultSubtitle})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}}
}

func privacySettingCard(title, description, value string, width float32, background woxui.Color, palette uiPalette) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 70, Radius: 10, Color: background, Padding: woxwidget.Insets{Left: 18, Top: 9, Right: 16, Bottom: 9}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(180), width-208), Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: palette.resultTitle},
			woxwidget.Text{Value: description, Style: woxui.TextStyle{Size: 12}, Color: palette.resultSubtitle},
		}}},
		woxwidget.Container{Width: 160, Height: 38, Radius: 8, Color: palette.toolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 10}, Child: woxwidget.Text{Value: value, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.cursor}},
	}}}
}

func (a *App) buildPrivacySampleCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-28)
	return woxwidget.Container{Width: width, Height: 282, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(120), innerWidth-100), Height: 30, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: "Anonymous telemetry payload", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
			a.buildFormTableButton("privacy-copy", "Copy", 90, true, false, a.copyPrivacySample, snapshot.palette),
		}}},
		woxwidget.Container{Width: innerWidth, Height: 210, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.TextBlock{
			Value: snapshot.privacySample, Width: innerWidth - 28, Height: 182, Style: woxui.TextStyle{Size: 12}, LineHeight: 20, Color: snapshot.palette.resultTitle,
		}},
	}}}
}
