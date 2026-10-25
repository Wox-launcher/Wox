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
		Width: width, Height: height, Theme: snapshot.palette,
		Title: a.language.translate("i18n:ui_privacy"), Description: a.language.translate("i18n:ui_privacy_description"),
		PrivateModeTitle: a.language.translate("i18n:ui_privacy_mode_title"), PrivateModeDescription: a.language.translate("i18n:ui_privacy_mode_description"),
		PrivateModeEnabled: snapshot.general.Data.EnablePrivacyMode,
		OfflineTitle:       a.language.translate("i18n:ui_offline_title"), OfflineDescription: a.language.translate("i18n:ui_offline_description"), OfflineEnabled: snapshot.general.Data.EnableOfflineMode, OfflineReason: a.language.translate("i18n:ui_offline_paused"),
		OnToggleOffline: func() { a.generalSettings.selectSettingRow(1); a.generalSettings.activateSetting(1) },
		TelemetryTitle:  a.language.translate("i18n:ui_privacy_anonymous_stats_title"), TelemetryDescription: a.language.translate("i18n:ui_privacy_anonymous_stats_description"),
		TelemetryEnabled: snapshot.general.Data.EnableAnonymousUsageStats, ViewSampleLabel: a.language.translate("i18n:ui_privacy_view_sample"), Error: snapshot.privacy.Error,
		OnTogglePrivateMode: func() {
			a.generalSettings.selectSettingRow(0)
			a.generalSettings.activateSetting(1)
		},
		OnToggleTelemetry: func() {
			if snapshot.general.Data.EnableOfflineMode {
				return
			}
			a.generalSettings.selectSettingRow(2)
			a.generalSettings.activateSetting(1)
		},
		OnViewSample: a.togglePrivacySample,
	})
}

// buildPrivacySampleOverlay adapts the Flutter-compatible payload dialog to the settings window overlay.
func (a *App) buildPrivacySampleOverlay(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	return launcherview.PrivacySampleDialog(launcherview.PrivacySampleDialogProps{
		Width: width, Height: height, Theme: snapshot.palette,
		Title: a.language.translate("i18n:ui_privacy_sample_title"), Sample: snapshot.privacy.Sample,
		CopyLabel: a.language.translate("i18n:toolbar_copy"), ConfirmLabel: a.language.translate("i18n:ui_ok"), Error: snapshot.privacy.Error,
		OnCopy: a.copyPrivacySample, OnClose: a.togglePrivacySample,
	})
}

// onPrivacySettingsKey keeps the modal sample dialog from driving settings behind it.
func (a *App) onPrivacySettingsKey(event woxui.KeyEvent) bool {
	settingsOpen := a.settingsWindow.open
	tab := a.settingsWindow.tab
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
	a.privacySettings.CopySample(a.settingsWindow.settingsNativeWindow().WriteClipboardText)
}
