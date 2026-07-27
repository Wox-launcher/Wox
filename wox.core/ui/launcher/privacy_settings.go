package launcher

import (
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type privacySamplePayload struct {
	SchemaVersion int    `json:"schema_version"`
	InstallHash   string `json:"install_hash"`
	OSFamily      string `json:"os_family"`
	WoxVersion    string `json:"wox_version"`
	SentAt        int64  `json:"sent_at"`
}

// buildPrivacySettingsPage adapts controller state to the package-independent privacy view.
func (a *App) buildPrivacySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	return launcherview.PrivacySettingsView(launcherview.PrivacySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(),
		Title: a.translate("i18n:ui_privacy"), Description: a.translate("i18n:ui_privacy_description"),
		TelemetryTitle: a.translate("i18n:ui_privacy_anonymous_stats_title"), TelemetryDescription: a.translate("i18n:ui_privacy_anonymous_stats_description"),
		TelemetryEnabled: snapshot.general.Data.EnableAnonymousUsageStats, ViewSampleLabel: a.translate("i18n:ui_privacy_view_sample"), Error: snapshot.privacy.Error,
		OnToggleTelemetry: func() {
			a.selectSettingRow(0)
			a.activateSetting(1)
		},
		OnViewSample: a.togglePrivacySample,
	})
}

// buildPrivacySampleOverlay adapts the Flutter-compatible payload dialog to the settings window overlay.
func (a *App) buildPrivacySampleOverlay(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	return launcherview.PrivacySampleDialog(launcherview.PrivacySampleDialogProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(),
		Title: a.translate("i18n:ui_privacy_sample_title"), Sample: snapshot.privacy.Sample,
		CopyLabel: a.translate("i18n:toolbar_copy"), ConfirmLabel: a.translate("i18n:ui_ok"), Error: snapshot.privacy.Error,
		OnCopy: a.copyPrivacySample, OnClose: a.togglePrivacySample,
	})
}

// onPrivacySettingsKey keeps the modal sample dialog from driving settings behind it.
func (a *App) onPrivacySettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	settingsOpen := a.settingsOpen
	tab := a.settingTab
	a.mu.RUnlock()
	if !settingsOpen || tab != "privacy" || !a.privacySettings.SampleVisible() {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.togglePrivacySample()
	}
	return true
}

// togglePrivacySample snapshots one representative telemetry payload so its timestamp stays stable while visible.
func (a *App) togglePrivacySample() {
	a.privacySettings.ToggleSample(func() string { return a.aboutSettings.Snapshot().Version })
}

// copyPrivacySample publishes the visible sample through the portable clipboard boundary.
func (a *App) copyPrivacySample() {
	a.privacySettings.CopySample(a.settingsNativeWindow().WriteClipboardText)
}
