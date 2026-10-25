package launcher

import (
	"context"
	"fmt"
	"strings"

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
func (c *runtimeSettingsController) buildRuntimeSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height, imageScale float32) woxwidget.Widget {
	statuses := make([]launcherview.RuntimeStatus, 0, len(snapshot.runtime.Statuses))
	for _, status := range snapshot.runtime.Statuses {
		version := strings.TrimSpace(status.HostVersion)
		if version != "" && !strings.HasPrefix(strings.ToLower(version), "v") {
			version = "v" + version
		}
		displayName := c.localizedRuntimeDisplayName(status.Runtime)
		pluginLabel := strings.ReplaceAll(c.language.translate("i18n:ui_runtime_status_plugin_count"), "{count}", fmt.Sprintf("%d", status.LoadedPluginCount))
		converted := launcherview.RuntimeStatus{
			Runtime: status.Runtime, DisplayName: displayName, Mark: runtimeFallbackMark(status.Runtime), Icon: c.imageForSurface(runtimeIconSource(status.Runtime), physicalImageSize(22, imageScale), settingsPalette().Background), Version: version,
			StatusCode: status.StatusCode, StatusLabel: c.localizedRuntimeStatusLabel(status), Detail: runtimeStatusDetail(status), PluginLabel: pluginLabel,
			Actionable: runtimeStatusActionable(status),
		}
		if status.InstallURL != "" && (status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version") {
			labelKey := "ui_runtime_install_runtime"
			if status.StatusCode == "unsupported_version" {
				labelKey = "ui_runtime_upgrade_runtime"
			}
			converted.InstallLabel = strings.ReplaceAll(c.language.translate("i18n:"+labelKey), "{runtime}", displayName)
			converted.InstallIcon = c.imageForTint(settingControlIconSource("external"), &snapshot.palette.Text, physicalImageSize(14, imageScale))
			converted.OnInstall = func() { c.openRuntimeInstallURL(status) }
		}
		if runtimeStatusRefreshable(status) {
			converted.RefreshLabel = c.language.translate("i18n:ui_runtime_status_refresh")
			if strings.EqualFold(snapshot.runtime.Refreshing, status.Runtime) {
				converted.RefreshLabel = c.language.translate("i18n:ui_runtime_refreshing_host")
			}
			converted.RefreshIcon = c.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.Text, physicalImageSize(14, imageScale))
			converted.OnRefresh = func() { c.refreshRuntimeHost(status.Runtime) }
		}
		if status.CanRestart {
			converted.RestartLabel = c.language.translate("i18n:ui_runtime_restart_host")
			if strings.EqualFold(snapshot.runtime.Restarting, status.Runtime) {
				converted.RestartLabel = c.language.translate("i18n:ui_runtime_restarting_host")
			}
			converted.RestartIcon = c.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.Text, physicalImageSize(14, imageScale))
			converted.OnRestart = func() { c.restartRuntimeHost(status.Runtime) }
		}
		statuses = append(statuses, converted)
	}
	rows := make([]launcherview.RuntimeSettingRow, 0, len(items))
	for index, item := range items {
		item := c.localizedSettingItem(item)
		state := woxui.TextEditingState{Text: item.value}
		focused := snapshot.general.EditKey == item.key
		if focused {
			state = snapshot.general.Editing
		}
		rows = append(rows, launcherview.RuntimeSettingRow{
			ID: "runtime-setting-" + item.key, Title: item.title, Description: item.description, Placeholder: c.runtimeExecutablePlaceholder(item.key),
			State: state, Focused: focused, Disabled: snapshot.saving || snapshot.runtime.Restarting != "" || snapshot.runtime.Refreshing != "" || item.disabled, Highlighted: snapshot.highlight == "built-in:"+item.key, Window: c.settings.settingsNativeWindow(),
			OnHover:   func() { c.selectSettingRow(index) },
			OnFocus:   func() { c.selectSettingRow(index); c.startBuiltInSettingEdit(item, -1) },
			OnChanged: func(value string) { c.setBuiltInSettingEditValue(item, value) }, OnKey: c.onBuiltInSettingsEditorKey,
			OnBrowse: func() { c.selectSettingRow(index); c.browseRuntimeExecutable(item) },
			OnSave: func() {
				if c.generalSettings().EditKey() != item.key || c.settings.saving {
					return
				}
				// Leaving an unchanged path must not restart its runtime host.
				if c.generalSettings().Editor().State().Text == item.value {
					c.cancelBuiltInSettingEdit()
					return
				}
				c.submitBuiltInSettingEdit()
			},
			OnClear: func() { c.selectSettingRow(index); c.saveRuntimeExecutablePath(item, "") },
		})
	}
	return launcherview.RuntimeSettingsView(launcherview.RuntimeSettingsProps{
		Width: width, Height: height, SettingRowHeight: runtimeSettingRowHeight, Theme: snapshot.palette, Labels: c.runtimeSettingsLabels(), Loading: snapshot.runtime.Loading,
		Restarting: snapshot.runtime.Restarting != "", Refreshing: snapshot.runtime.Refreshing != "", Error: snapshot.runtime.Error,
		Editing: snapshot.general.EditKey != "", Saving: snapshot.saving, OnTooltip: c.setSettingChoiceTooltip,
		Selected: snapshot.row, Statuses: statuses, Settings: rows,
	})
}

// runtimeSettingsLabels resolves Flutter-compatible page copy before entering the pure view layer.
func (c *runtimeSettingsController) runtimeSettingsLabels() launcherview.RuntimeSettingsLabels {
	return launcherview.RuntimeSettingsLabels{
		Title:             c.language.translate("i18n:ui_runtime_settings"),
		Description:       c.language.translate("i18n:ui_runtime_settings_description"),
		StatusSection:     c.language.translate("i18n:ui_runtime_status"),
		ExecutableSection: c.language.translate("i18n:ui_runtime_executable_paths"),
		ExecutableHelp:    c.language.translate("i18n:ui_runtime_executable_help"),
		Browse:            c.language.translate("i18n:ui_runtime_browse"),
		Clear:             c.language.translate("i18n:ui_runtime_clear"),
		Empty:             c.language.translate("i18n:ui_runtime_status_empty"),
	}
}

// localizedRuntimeDisplayName keeps card names consistent with the former Flutter page.
func (c *runtimeSettingsController) localizedRuntimeDisplayName(runtime string) string {
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
	case "FLOWJSONRPC":
		key = "ui_runtime_name_flowjsonrpc"
	case "FLOWDOTNET":
		key = "ui_runtime_name_flowdotnet"
	}
	if key == "" {
		return runtime
	}
	return c.language.translate("i18n:" + key)
}

// localizedRuntimeStatusLabel maps backend diagnosis codes to translated card pills.
func (c *runtimeSettingsController) localizedRuntimeStatusLabel(status runtimeStatus) string {
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
	return c.language.translate("i18n:" + key)
}

func (c *runtimeSettingsController) runtimeExecutablePlaceholder(key string) string {
	if key == "CustomPythonPath" {
		return c.language.translate("i18n:ui_runtime_python_path_placeholder")
	}
	return c.language.translate("i18n:ui_runtime_nodejs_path_placeholder")
}

// browseRuntimeExecutable persists a selected executable immediately, matching Flutter's picker flow.
func (c *runtimeSettingsController) browseRuntimeExecutable(item settingItem) {
	window := c.settings.settingsNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{})
	if err != nil {
		c.SetError("Could not select " + item.title + ": " + err.Error())
		return
	}
	if path != "" {
		c.saveRuntimeExecutablePath(item, path)
	}
}

// saveRuntimeExecutablePath uses the backend validator as the final authority for picker and clear actions.
func (c *runtimeSettingsController) saveRuntimeExecutablePath(item settingItem, value string) {
	if c.settings.saving {
		return
	}
	c.generalSettings().EndEdit()
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	label := value
	if label == "" {
		label = c.language.translate("i18n:ui_runtime_clear")
	}
	c.startGeneralSettingSave("save runtime executable path", item, settingChoice{value: value, label: label})
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
	case "FLOWJSONRPC", "FLOWDOTNET":
		return "FL"
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
func (c *runtimeSettingsController) reloadRuntimeStatuses() {
	c.Reload(context.Background(), c.services, c.sessionID)
}

// restartRuntimeHost restarts a recoverable Node.js or Python host and then reloads the authoritative status.
func (c *runtimeSettingsController) restartRuntimeHost(runtime string) {
	c.Restart(context.Background(), c.services, c.sessionID, runtime, c.reloadRuntimeStatuses)
}

// refreshRuntimeHost re-detects a Node.js or Python interpreter after the user installs it.
func (c *runtimeSettingsController) refreshRuntimeHost(runtime string) {
	c.Refresh(context.Background(), c.services, c.sessionID, runtime, nil)
}

// openRuntimeInstallURL delegates installation guidance to the platform browser without owning platform code in the page.
func (c *runtimeSettingsController) openRuntimeInstallURL(status runtimeStatus) {
	if strings.TrimSpace(status.InstallURL) == "" {
		return
	}
	if err := c.settings.settingsNativeWindow().OpenExternalURL(status.InstallURL); err != nil {
		c.SetError("Could not open runtime website: " + err.Error())
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

// runtimeStatusRefreshable shows Refresh next to Install so users can re-detect
// a newly installed Python or Node.js interpreter without restarting Wox.
func runtimeStatusRefreshable(status runtimeStatus) bool {
	if !strings.EqualFold(status.Runtime, "PYTHON") && !strings.EqualFold(status.Runtime, "NODEJS") {
		return false
	}
	return runtimeStatusActionable(status)
}
