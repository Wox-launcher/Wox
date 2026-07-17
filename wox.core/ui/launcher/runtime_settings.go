package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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
	statuses := make([]launcherview.RuntimeStatus, 0, len(snapshot.runtimeStatuses))
	for _, status := range snapshot.runtimeStatuses {
		status := status
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
			if strings.EqualFold(snapshot.runtimeRestarting, status.Runtime) {
				converted.RestartLabel = a.translate("i18n:ui_runtime_restarting_host")
			}
			converted.RestartIcon = a.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.resultTitle, 32)
			converted.OnRestart = func() { a.restartRuntimeHost(status.Runtime) }
		}
		statuses = append(statuses, converted)
	}
	rows := make([]launcherview.RuntimeSettingRow, 0, len(items))
	for index, item := range items {
		index := index
		item := a.localizedSettingItem(item)
		state := woxui.TextEditingState{Text: item.value}
		focused := snapshot.editKey == item.key
		if focused {
			state = snapshot.editing
		}
		rows = append(rows, launcherview.RuntimeSettingRow{
			ID: "runtime-setting-" + item.key, Title: item.title, Description: item.description, Placeholder: a.runtimeExecutablePlaceholder(item.key),
			State: state, Focused: focused, Disabled: snapshot.saving || item.disabled, Window: a.settingsNativeWindow(),
			OnHover:   func() { a.selectSettingRow(index) },
			OnFocus:   func() { a.selectSettingRow(index); a.startBuiltInSettingEdit(item, -1) },
			OnChanged: func(value string) { a.setBuiltInSettingEditValue(item, value) }, OnKey: a.onBuiltInSettingsEditorKey,
			OnBrowse: func() { a.selectSettingRow(index); a.browseRuntimeExecutable(item) },
			OnClear:  func() { a.selectSettingRow(index); a.saveRuntimeExecutablePath(item, "") },
		})
	}
	return launcherview.RuntimeSettingsView(launcherview.RuntimeSettingsProps{
		Width: width, Height: height, SettingRowHeight: runtimeSettingRowHeight, Theme: snapshot.palette.componentTheme(), Labels: a.runtimeSettingsLabels(), Loading: snapshot.runtimeLoading,
		Restarting: snapshot.runtimeRestarting != "", Error: snapshot.runtimeError, Note: snapshot.note,
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
		Loading:           a.translate("i18n:ui_runtime_status_refresh") + "…",
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
		a.mu.Lock()
		a.settingNote = "Could not select " + item.title + ": " + err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	if path != "" {
		a.saveRuntimeExecutablePath(item, path)
	}
}

// saveRuntimeExecutablePath uses the backend validator as the final authority for picker and clear actions.
func (a *App) saveRuntimeExecutablePath(item settingItem, value string) {
	a.mu.Lock()
	if a.settingSaving {
		a.mu.Unlock()
		return
	}
	a.settingSaving = true
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingNote = "Saving " + item.title + "…"
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	label := value
	if label == "" {
		label = a.translate("i18n:ui_runtime_clear")
	}
	go a.saveSetting(item, settingChoice{value: value, label: label})
}

// runtimeIconSource reuses the colored runtime marks from the Flutter settings implementation.
func runtimeIconSource(runtime string) woxImage {
	const pythonIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#0288d1" d="M9.86 2A2.86 2.86 0 0 0 7 4.86v1.68h4.29c.39 0 .71.57.71.96H4.86A2.86 2.86 0 0 0 2 10.36v3.781a2.86 2.86 0 0 0 2.86 2.86h1.18v-2.68a2.85 2.85 0 0 1 2.85-2.86h5.25c1.58 0 2.86-1.271 2.86-2.851V4.86A2.86 2.86 0 0 0 14.14 2zm-.72 1.61c.4 0 .72.12.72.71s-.32.891-.72.891c-.39 0-.71-.3-.71-.89s.32-.711.71-.711"/><path fill="#fdd835" d="M17.959 7v2.68a2.85 2.85 0 0 1-2.85 2.859H9.86A2.85 2.85 0 0 0 7 15.389v3.75a2.86 2.86 0 0 0 2.86 2.86h4.28A2.86 2.86 0 0 0 17 19.14v-1.68h-4.291c-.39 0-.709-.57-.709-.96h7.14A2.86 2.86 0 0 0 22 13.64V9.86A2.86 2.86 0 0 0 19.14 7zM8.32 11.513l-.004.004l.038-.004zm6.54 7.276c.39 0 .71.3.71.89a.71.71 0 0 1-.71.71c-.4 0-.72-.12-.72-.71s.32-.89.72-.89"/></svg>`
	const nodejsIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><path fill="#8bc34a" d="M16 20.003v2h4a2 2 0 0 0 2-2v-2a2 2 0 0 0-2-2h-2v-2h4v-2h-4a2 2 0 0 0-2 2v2a2 2 0 0 0 2 2h2v2Z"/><path fill="#8bc34a" d="m16 3.003l-12 7v14l4 2h6v-13.5a.5.5 0 0 0-.5-.5h-1a.5.5 0 0 0-.5.5v11.5H8l-2-1.034V11.15l10-5.833l10 5.833v11.703l-10 5.833l-1.745-1.022L13 29.253l3 1.75l12-7v-14Z"/></svg>`
	const scriptIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><g fill="none" stroke-width="1.5"><path fill="#d7e0ff" d="M18 22H2v-6h3V2h17v6h-4z"/><path stroke="#4147d5" d="M14.25 22h3.25H2v-6h12.25v4"/><path stroke="#4147d5" d="M13.5 22H18V4.5M8 8h7m-7 4h7"/><path stroke="#4147d5" d="M5 16V2h17v6h-4"/></g></svg>`
	source := scriptIcon
	switch strings.ToUpper(runtime) {
	case "PYTHON":
		source = pythonIcon
	case "NODEJS":
		source = nodejsIcon
	}
	return woxImage{ImageType: "svg", ImageData: source}
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

// reloadRuntimeStatuses refreshes the runtime inventory while discarding responses superseded by a newer refresh.
func (a *App) reloadRuntimeStatuses() {
	a.mu.Lock()
	a.runtimeRevision++
	revision := a.runtimeRevision
	a.runtimeLoading = true
	a.runtimeError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var statuses []runtimeStatus
	err := a.client.Post(ctx, "/runtime/status", map[string]any{}, &statuses)

	a.mu.Lock()
	if revision != a.runtimeRevision {
		a.mu.Unlock()
		return
	}
	a.runtimeLoading = false
	if err != nil {
		a.runtimeError = err.Error()
	} else {
		a.runtimeStatuses = statuses
		a.runtimeLoaded = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// restartRuntimeHost restarts a recoverable Node.js or Python host and then reloads the authoritative status.
func (a *App) restartRuntimeHost(runtime string) {
	runtime = strings.ToUpper(strings.TrimSpace(runtime))
	a.mu.Lock()
	if runtime == "" || a.runtimeRestarting != "" {
		a.mu.Unlock()
		return
	}
	canRestart := false
	for _, status := range a.runtimeStatuses {
		if strings.EqualFold(status.Runtime, runtime) {
			canRestart = status.CanRestart
			break
		}
	}
	if !canRestart {
		a.mu.Unlock()
		return
	}
	a.runtimeRestarting = runtime
	a.runtimeError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := a.client.Post(ctx, "/runtime/restart", map[string]string{"Runtime": runtime}, nil)
		cancel()
		a.mu.Lock()
		a.runtimeRestarting = ""
		a.mu.Unlock()
		a.reloadRuntimeStatuses()
		if err != nil {
			a.mu.Lock()
			a.runtimeError = fmt.Sprintf("Could not restart %s: %v", runtimeDisplayName(runtime), err)
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
}

// openRuntimeInstallURL delegates installation guidance to the platform browser without owning platform code in the page.
func (a *App) openRuntimeInstallURL(status runtimeStatus) {
	if strings.TrimSpace(status.InstallURL) == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(status.InstallURL); err != nil {
		a.mu.Lock()
		a.runtimeError = "Could not open runtime website: " + err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
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
