package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
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

// buildRuntimeSettingsPage prepares runtime status and setting rows for the pure page view.
func (a *App) buildRuntimeSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height float32) woxwidget.Widget {
	statuses := make([]launcherview.RuntimeStatus, 0, len(snapshot.runtimeStatuses))
	for _, status := range snapshot.runtimeStatuses {
		status := status
		version := strings.TrimSpace(status.HostVersion)
		if version != "" && !strings.HasPrefix(strings.ToLower(version), "v") {
			version = "v" + version
		}
		mark := strings.ToUpper(runtimeDisplayName(status.Runtime))
		if len(mark) > 2 {
			mark = mark[:2]
		}
		pluginLabel := fmt.Sprintf("%d loaded plugins", status.LoadedPluginCount)
		if status.LoadedPluginCount == 1 {
			pluginLabel = "1 loaded plugin"
		}
		converted := launcherview.RuntimeStatus{
			Runtime: status.Runtime, DisplayName: runtimeDisplayName(status.Runtime), Mark: mark, Version: version,
			StatusCode: status.StatusCode, StatusLabel: runtimeStatusLabel(status), Detail: runtimeStatusDetail(status),
			PluginLabel: pluginLabel, Actionable: runtimeStatusActionable(status),
		}
		if status.InstallURL != "" && (status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version") {
			converted.InstallLabel = "Install ↗"
			if status.StatusCode == "unsupported_version" {
				converted.InstallLabel = "Upgrade ↗"
			}
			converted.OnInstall = func() { a.openRuntimeInstallURL(status) }
		}
		if status.CanRestart {
			converted.RestartLabel = "Restart host"
			if strings.EqualFold(snapshot.runtimeRestarting, status.Runtime) {
				converted.RestartLabel = "Restarting…"
			}
			converted.OnRestart = func() { a.restartRuntimeHost(status.Runtime) }
		}
		statuses = append(statuses, converted)
	}
	contentWidth := max(float32(0), width-72)
	rows := make([]launcherview.RuntimeSettingRow, 0, len(items))
	for index, item := range items {
		index := index
		item := item
		background := snapshot.palette.queryBackground
		if index == snapshot.row {
			background = snapshot.palette.selectedBackground
		}
		rows = append(rows, launcherview.RuntimeSettingRow{
			ID: "runtime-setting-" + item.key, Child: a.buildSettingRow(snapshot, item, index, contentWidth, background),
			OnHover: func() { a.selectSettingRow(index) },
			OnTap: func() {
				a.selectSettingRow(index)
				a.openOrActivateSetting()
			},
		})
	}
	return launcherview.RuntimeSettingsView(launcherview.RuntimeSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Loading: snapshot.runtimeLoading,
		Restarting: snapshot.runtimeRestarting != "", Error: snapshot.runtimeError, Note: snapshot.note,
		Scroll: snapshot.runtimePageScroll, Statuses: statuses, Settings: rows,
		OnRefresh: a.reloadRuntimeStatuses, OnScroll: a.scrollRuntimePage, OnSetGeometry: a.setRuntimePageGeometry,
	})
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

// runtimeStatusLabel maps backend diagnosis codes to short card labels.
func runtimeStatusLabel(status runtimeStatus) string {
	switch status.StatusCode {
	case "running":
		return "Running"
	case "executable_missing":
		return "Missing"
	case "unsupported_version":
		return "Upgrade required"
	case "start_failed":
		return "Start failed"
	default:
		return "Stopped"
	}
}

// runtimeStatusDetail prefers technical startup errors and resolved executable paths over generic text.
func runtimeStatusDetail(status runtimeStatus) string {
	if status.StatusCode == "start_failed" && strings.TrimSpace(status.LastStartError) != "" {
		return status.LastStartError
	}
	if status.StatusCode == "running" && strings.TrimSpace(status.ExecutablePath) != "" {
		return status.ExecutablePath
	}
	if strings.TrimSpace(status.StatusMessage) != "" {
		return status.StatusMessage
	}
	return status.ExecutablePath
}

func runtimeStatusActionable(status runtimeStatus) bool {
	return status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version" || status.StatusCode == "start_failed"
}

// setRuntimePageGeometry keeps the specialized runtime page scroll range aligned with its responsive status grid.
func (a *App) setRuntimePageGeometry(viewport, content, rowsTop float32) {
	a.mu.Lock()
	a.runtimePageViewport = max(float32(1), viewport)
	a.runtimePageContent = max(content, viewport)
	a.runtimeRowsTop = rowsTop
	a.clampRuntimePageScrollLocked()
	a.mu.Unlock()
}

// scrollRuntimePage clamps wheel input against the responsive page height.
func (a *App) scrollRuntimePage(delta float32) {
	a.mu.Lock()
	a.runtimePageScroll += delta
	a.clampRuntimePageScrollLocked()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) clampRuntimePageScrollLocked() {
	maximum := max(float32(0), a.runtimePageContent-a.runtimePageViewport)
	a.runtimePageScroll = min(max(float32(0), a.runtimePageScroll), maximum)
}

// ensureRuntimeSettingRowVisibleLocked accounts for status cards above the shared setting rows.
func (a *App) ensureRuntimeSettingRowVisibleLocked() {
	viewport := max(float32(1), a.runtimePageViewport)
	top := a.runtimeRowsTop + float32(a.settingRow)*82
	bottom := top + 70
	if top < a.runtimePageScroll {
		a.runtimePageScroll = top
	} else if bottom > a.runtimePageScroll+viewport {
		a.runtimePageScroll = bottom - viewport
	}
	a.clampRuntimePageScrollLocked()
}
