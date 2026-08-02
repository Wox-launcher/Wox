package launcher

import (
	"context"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type runtimeStatus struct {
	Runtime           string
	IsStarted         bool
	HostVersion       string
	StatusCode        string
	StatusMessage     string
	ExecutablePath    string
	LastStartError    string
	CanRestart        bool
	InstallURL        string `json:"InstallUrl"`
	LoadedPluginCount int
	LoadedPluginNames []string
}

const runtimeSettingRowHeight = float32(72)

// buildRuntimeSettingsPage prepares runtime status and setting rows for the pure page view.
func (a *App) buildRuntimeSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height float32) woxwidget.Widget {
	statuses := make([]launcherview.RuntimeStatus, 0, len(snapshot.runtime.Statuses))
	for _, status := range snapshot.runtime.Statuses {
		version := strings.TrimSpace(status.HostVersion)
		if version != "" && !strings.HasPrefix(strings.ToLower(version), "v") {
			version = "v" + version
		}
		displayName := a.localizedRuntimeDisplayName(status.Runtime)
		pluginLabel := strings.ReplaceAll(a.translate("i18n:ui_runtime_status_plugin_count"), "{count}", fmt.Sprintf("%d", status.LoadedPluginCount))
		converted := launcherview.RuntimeStatus{
			Runtime: status.Runtime, DisplayName: displayName, Mark: runtimeFallbackMark(status.Runtime), Icon: a.imageForSize(runtimeIconSource(status.Runtime), 48), Version: version,
			StatusCode: status.StatusCode, StatusLabel: a.localizedRuntimeStatusLabel(status), Detail: runtimeStatusDetail(status), PluginLabel: pluginLabel,
			Actionable: runtimeStatusActionable(status),
		}
		if status.InstallURL != "" && (status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version") {
			labelKey := "ui_runtime_install_runtime"
			if status.StatusCode == "unsupported_version" {
				labelKey = "ui_runtime_upgrade_runtime"
			}
			converted.InstallLabel = strings.ReplaceAll(a.translate("i18n:"+labelKey), "{runtime}", displayName)
			converted.InstallIcon = a.imageForTint(settingControlIconSource("external"), &snapshot.palette.resultTitle, 32)
			converted.OnInstall = func() { a.openRuntimeInstallURL(status) }
		}
		if status.CanRestart {
			converted.RestartLabel = a.translate("i18n:ui_runtime_restart_host")
			if strings.EqualFold(snapshot.runtime.Restarting, status.Runtime) {
				converted.RestartLabel = a.translate("i18n:ui_runtime_restarting_host")
			}
			converted.RestartIcon = a.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.resultTitle, 32)
			converted.OnRestart = func() { a.restartRuntimeHost(status.Runtime) }
		}
		statuses = append(statuses, converted)
	}
	rows := make([]launcherview.RuntimeSettingRow, 0, len(items))
	for index, item := range items {
		item := a.localizedSettingItem(item)
		state := woxui.TextEditingState{Text: item.value}
		focused := snapshot.general.EditKey == item.key
		if focused {
			state = snapshot.general.Editing
		}
		rows = append(rows, launcherview.RuntimeSettingRow{
			ID: "runtime-setting-" + item.key, Title: item.title, Description: item.description, Placeholder: a.runtimeExecutablePlaceholder(item.key),
			State: state, Focused: focused, Disabled: snapshot.saving || item.disabled, Highlighted: snapshot.highlight == "built-in:"+item.key, Window: a.settingsNativeWindow(),
			OnHover:   func() { a.selectSettingRow(index) },
			OnFocus:   func() { a.selectSettingRow(index); a.startBuiltInSettingEdit(item, -1) },
			OnChanged: func(value string) { a.setBuiltInSettingEditValue(item, value) }, OnKey: a.onBuiltInSettingsEditorKey,
			OnBrowse: func() { a.selectSettingRow(index); a.browseRuntimeExecutable(item) },
			OnClear:  func() { a.selectSettingRow(index); a.saveRuntimeExecutablePath(item, "") },
		})
	}
	return launcherview.RuntimeSettingsView(launcherview.RuntimeSettingsProps{
		Width: width, Height: height, SettingRowHeight: runtimeSettingRowHeight, Theme: snapshot.palette.componentTheme(), Labels: a.runtimeSettingsLabels(), Loading: snapshot.runtime.Loading,
		Restarting: snapshot.runtime.Restarting != "", Error: snapshot.runtime.Error,
		Selected: snapshot.row, Statuses: statuses, Settings: rows,
	})
}

// runtimeSettingsLabels resolves Flutter-compatible page copy before entering the pure view layer.
func (a *App) runtimeSettingsLabels() launcherview.RuntimeSettingsLabels {
	return launcherview.RuntimeSettingsLabels{
		Title:             a.translate("i18n:ui_runtime_settings"),
		Description:       a.translate("i18n:ui_runtime_settings_description"),
		StatusSection:     a.translate("i18n:ui_runtime_status"),
		ExecutableSection: a.translate("i18n:ui_runtime_executable_paths"),
		Browse:            a.translate("i18n:ui_runtime_browse"),
		Clear:             a.translate("i18n:ui_runtime_clear"),
		Empty:             a.translate("i18n:ui_runtime_status_empty"),
	}
}

// localizedRuntimeDisplayName keeps card names consistent with the former Flutter page.
func (a *App) localizedRuntimeDisplayName(runtime string) string {
	key := ""
	switch strings.ToUpper(runtime) {
	case "NODEJS":
		key = "ui_runtime_name_nodejs"
	case "PYTHON":
		key = "ui_runtime_name_python"
	case "SCRIPT":
		key = "ui_runtime_name_script"
	case "GO":
		key = "ui_runtime_name_go"
	}
	if key == "" {
		return runtime
	}
	return a.translate("i18n:" + key)
}

// localizedRuntimeStatusLabel maps backend diagnosis codes to translated card pills.
func (a *App) localizedRuntimeStatusLabel(status runtimeStatus) string {
	key := "ui_runtime_status_stopped"
	switch status.StatusCode {
	case "running":
		key = "ui_runtime_status_running"
	case "executable_missing":
		key = "ui_runtime_status_executable_missing"
	case "unsupported_version":
		key = "ui_runtime_status_unsupported_version"
	case "start_failed":
		key = "ui_runtime_status_start_failed"
	}
	return a.translate("i18n:" + key)
}

func (a *App) runtimeExecutablePlaceholder(key string) string {
	if key == "CustomPythonPath" {
		return a.translate("i18n:ui_runtime_python_path_placeholder")
	}
	return a.translate("i18n:ui_runtime_nodejs_path_placeholder")
}

// browseRuntimeExecutable persists a selected executable immediately, matching Flutter's picker flow.
func (a *App) browseRuntimeExecutable(item settingItem) {
	window := a.settingsNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{})
	if err != nil {
		a.runtimeSettings.SetError("Could not select " + item.title + ": " + err.Error())
		return
	}
	if path != "" {
		a.saveRuntimeExecutablePath(item, path)
	}
}

// saveRuntimeExecutablePath uses the backend validator as the final authority for picker and clear actions.
func (a *App) saveRuntimeExecutablePath(item settingItem, value string) {
	if a.settingSaving {
		return
	}
	a.generalSettings.EndEdit()
	a.beginSettingSave()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	label := value
	if label == "" {
		label = a.translate("i18n:ui_runtime_clear")
	}
	util.Go(a.lifecycleCtx, "save runtime executable path", func() {
		a.saveSetting(item, settingChoice{value: value, label: label})
	})
}

// runtimeFallbackMark remains visible during the first asynchronous SVG decode.
func runtimeFallbackMark(runtime string) string {
	switch strings.ToUpper(runtime) {
	case "NODEJS":
		return "JS"
	case "PYTHON":
		return "PY"
	case "SCRIPT":
		return "SC"
	default:
		return "RT"
	}
}

// cloneRuntimeStatuses isolates snapshot rendering from plugin-name slice updates.
func cloneRuntimeStatuses(statuses []runtimeStatus) []runtimeStatus {
	cloned := make([]runtimeStatus, len(statuses))
	for index, status := range statuses {
		cloned[index] = status
		cloned[index].LoadedPluginNames = append([]string(nil), status.LoadedPluginNames...)
	}
	return cloned
}

// reloadRuntimeStatuses refreshes the runtime inventory via the runtime controller.
func (a *App) reloadRuntimeStatuses() {
	a.runtimeSettings.Reload(context.Background(), a.services, a.sessionID)
}

// restartRuntimeHost restarts a recoverable Node.js or Python host and then reloads the authoritative status.
func (a *App) restartRuntimeHost(runtime string) {
	a.runtimeSettings.Restart(context.Background(), a.services, a.sessionID, runtime, a.reloadRuntimeStatuses)
}

// openRuntimeInstallURL delegates installation guidance to the platform browser without owning platform code in the page.
func (a *App) openRuntimeInstallURL(status runtimeStatus) {
	if strings.TrimSpace(status.InstallURL) == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(status.InstallURL); err != nil {
		a.runtimeSettings.SetError("Could not open runtime website: " + err.Error())
	}
}

// runtimeDisplayName converts protocol identifiers into compact product labels.
func runtimeDisplayName(runtime string) string {
	switch strings.ToUpper(runtime) {
	case "NODEJS":
		return "Node.js"
	case "PYTHON":
		return "Python"
	case "SCRIPT":
		return "Script"
	case "GO":
		return "Go"
	default:
		return runtime
	}
}

// runtimeStatusDetail mirrors Flutter's reserved path area while preserving actionable failure details.
func runtimeStatusDetail(status runtimeStatus) string {
	if status.StatusCode == "start_failed" && strings.TrimSpace(status.LastStartError) != "" {
		return status.LastStartError
	}
	if status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version" || status.StatusCode == "start_failed" {
		return status.StatusMessage
	}
	return status.ExecutablePath
}

func runtimeStatusActionable(status runtimeStatus) bool {
	return status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version" || status.StatusCode == "start_failed"
}
