package launcher

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	launcherview "wox/ui/launcher/view"
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
	items := settingItemsForSnapshot(snapshot)
	return launcherview.PrivacySettingsView(launcherview.PrivacySettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Selected: snapshot.row,
		TelemetryTitle: items[0].title, TelemetryDescription: items[0].description, TelemetryEnabled: snapshot.data.EnableAnonymousUsageStats,
		SampleTitle: items[1].title, SampleDescription: items[1].description, Sample: snapshot.privacySample, Error: snapshot.privacyError,
		OnSelect: a.selectSettingRow,
		OnToggleTelemetry: func() {
			a.activateSetting(1)
		},
		OnToggleSample: a.togglePrivacySample,
		OnCopySample:   a.copyPrivacySample,
	})
}

// togglePrivacySample snapshots one representative telemetry payload so its timestamp stays stable while visible.
func (a *App) togglePrivacySample() {
	a.mu.Lock()
	if a.privacySample != "" {
		a.privacySample = ""
		a.privacyError = ""
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	version := a.aboutVersion
	if version == "" {
		version = "current Wox version"
	}
	payload := privacySamplePayload{
		SchemaVersion: 1,
		InstallHash:   "sha256(install_id) - a 64-character hexadecimal string",
		OSFamily:      runtime.GOOS,
		WoxVersion:    version,
		SentAt:        time.Now().UnixMilli(),
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		a.privacyError = err.Error()
	} else {
		a.privacySample = string(encoded)
		a.privacyError = ""
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// copyPrivacySample publishes the visible sample through the portable clipboard boundary.
func (a *App) copyPrivacySample() {
	a.mu.RLock()
	value := a.privacySample
	a.mu.RUnlock()
	if value == "" {
		return
	}
	err := a.settingsNativeWindow().WriteClipboardText(value)
	a.mu.Lock()
	if err != nil {
		a.privacyError = fmt.Sprintf("Could not copy sample: %v", err)
	} else {
		a.privacyError = "Sample copied to clipboard."
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}
