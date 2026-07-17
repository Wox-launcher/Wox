package launcher

import (
	"log"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const settingsChangedTopic = "settings.changed"

// ensureSettingsWindow creates the independent settings host once per native lifetime.
func (a *App) ensureSettingsWindow() (*woxui.ManagedWindow, error) {
	if !a.isPrimary && a.primary != nil {
		return a.primary.ensureSettingsWindow()
	}
	a.mu.RLock()
	existing := a.settingsView
	a.mu.RUnlock()
	if existing != nil && existing.Lifecycle() != woxui.WindowLifecycleClosed {
		return existing, nil
	}

	host := woxwidget.NewHost(a.buildSettings)
	var managed *woxui.ManagedWindow
	var openErr error
	if err := woxui.Call(func() {
		managed, _, openErr = a.windows.Open(settingsWindowID, woxui.WindowOptions{
			Title:     a.translate("i18n:ui_tray_open_setting_window"),
			Size:      woxui.Size{Width: settingsWindowWidth, Height: settingsWindowHeight},
			Role:      woxui.WindowRoleApplication,
			OnFrame:   host.Frame,
			OnPointer: host.Pointer,
			OnKey: func(event woxui.KeyEvent) bool {
				if host.Key(event) {
					return true
				}
				return a.onSettingsWindowKey(event)
			},
			OnTextInput: func(event woxui.TextInputEvent) {
				if !host.TextInput(event) {
					a.onSettingsWindowTextInput(event)
				}
			},
			OnCloseRequested: func() {
				go func() {
					if err := a.closeSettings(); err != nil {
						log.Printf("close requested settings window: %v", err)
					}
				}()
			},
			OnClosed: a.onSettingsWindowClosed,
		})
		if openErr == nil {
			host.Attach(managed.Window())
		}
	}); err != nil {
		return nil, err
	}
	if openErr != nil {
		return nil, openErr
	}

	a.mu.Lock()
	a.settingsView = managed
	a.settingsHost = host
	a.settingsOpen = true
	fontFamily := a.settings.AppFontFamily
	isDark := themeColorIsDark(a.palette.background)
	a.mu.Unlock()
	if err := managed.Window().SetAppearance(isDark); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if err := managed.Window().SetFontFamily(fontFamily); err != nil {
		_ = managed.Close()
		return nil, err
	}
	return managed, nil
}

func (a *App) settingsNativeWindow() *woxui.Window {
	a.mu.RLock()
	managed := a.settingsView
	launcherWindow := a.window
	a.mu.RUnlock()
	if managed == nil {
		return launcherWindow
	}
	return managed.Window()
}

func (a *App) invalidateSettingsWindow() {
	if window := a.settingsNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (a *App) invalidateAllWindows() {
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	if settingsWindow := a.settingsNativeWindow(); settingsWindow != nil && settingsWindow != a.window {
		_ = settingsWindow.Invalidate()
	}
}

func (a *App) updateSettingsTextInput(enabled bool) {
	window := a.settingsNativeWindow()
	if window == nil {
		return
	}
	state := woxui.TextInputState{}
	a.mu.RLock()
	searchFocused := a.settingSearchFocused || a.pluginSearchFocused
	a.mu.RUnlock()
	if enabled || searchFocused {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 240, Y: 180, Width: 1, Height: 24}}
	}
	_ = window.SetTextInputState(state)
}

func (a *App) themeEditorUsesSettingsWindow() bool {
	a.mu.RLock()
	usesSettings := a.themeEditor != nil && strings.HasPrefix(a.themeEditor.key, "settings-theme|")
	a.mu.RUnlock()
	return usesSettings
}

func (a *App) themeEditorNativeWindow() *woxui.Window {
	if a.themeEditorUsesSettingsWindow() {
		return a.settingsNativeWindow()
	}
	return a.window
}

func (a *App) invalidateThemeEditorWindow() {
	if window := a.themeEditorNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (a *App) updateThemeEditorTextInput(enabled bool) {
	if a.themeEditorUsesSettingsWindow() {
		a.updateSettingsTextInput(enabled)
		return
	}
	a.updateFormTextInput(enabled)
}

func (a *App) restoreThemeEditorTextInput() {
	if a.themeEditorUsesSettingsWindow() {
		a.updateSettingsTextInput(false)
		return
	}
	a.restoreQueryTextInput()
}

func (a *App) formTableUsesSettingsWindow() bool {
	a.mu.RLock()
	state := a.tableEditor
	usesSettings := state != nil && a.formTableTargetUsesSettingsLocked(state.target)
	a.mu.RUnlock()
	return usesSettings
}

func (a *App) formTableTargetUsesSettingsLocked(target *formFieldsState) bool {
	return target != nil && ((a.pluginForm != nil && target == &a.pluginForm.formFieldsState) || target == a.aiSettingsForm || target == a.hotkeySettingsForm)
}

func (a *App) formTableNativeWindow() *woxui.Window {
	if a.formTableUsesSettingsWindow() {
		return a.settingsNativeWindow()
	}
	return a.window
}

func (a *App) invalidateFormTableWindow() {
	if window := a.formTableNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (a *App) updateFormTableTextInput(enabled bool) {
	if a.formTableUsesSettingsWindow() {
		a.updateSettingsTextInput(enabled)
		return
	}
	a.updateFormTextInput(enabled)
}

func (a *App) hotkeyRecordingUsesSettingsWindow() bool {
	a.mu.RLock()
	state := a.hotkeyRecording
	usesSettings := false
	if state != nil {
		usesSettings = state.target == a.hotkeySettingsForm || (a.pluginForm != nil && state.target == &a.pluginForm.formFieldsState)
		if !usesSettings && a.tableEditor != nil && a.tableEditor.rowForm == state.target {
			usesSettings = a.formTableTargetUsesSettingsLocked(a.tableEditor.target)
		}
	}
	a.mu.RUnlock()
	return usesSettings
}

func (a *App) hotkeyRecordingNativeWindow() *woxui.Window {
	if a.hotkeyRecordingUsesSettingsWindow() {
		return a.settingsNativeWindow()
	}
	return a.window
}

func (a *App) invalidateHotkeyWindows() {
	a.invalidateAllWindows()
}

func (a *App) formFieldNativeWindow(idPrefix string) *woxui.Window {
	switch idPrefix {
	case "plugin-settings", "hotkey-settings", "ai-settings", "cloud-form":
		return a.settingsNativeWindow()
	case "theme-editor":
		return a.themeEditorNativeWindow()
	case "form-table-row":
		return a.formTableNativeWindow()
	default:
		return a.window
	}
}

// publishSettingsChanged keeps every top-level surface on the same shared settings snapshot.
func (a *App) publishSettingsChanged(payload any) {
	if a.windows == nil {
		return
	}
	if err := a.windows.Publish(woxui.WindowMessage{Source: settingsWindowID, Topic: settingsChangedTopic, Payload: payload}); err != nil {
		log.Printf("publish settings change: %v", err)
	}
}

func (a *App) onSettingsWindowKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if a.hotkeyRecordingUsesSettingsWindow() && a.onHotkeyRecordingKey(event) {
		return true
	}
	if a.formTableUsesSettingsWindow() && a.onFormTableKey(event) {
		return true
	}
	return a.onSettingsKey(event)
}

func (a *App) onSettingsWindowTextInput(event woxui.TextInputEvent) {
	if a.formTableUsesSettingsWindow() && a.onFormTableTextInput(event) {
		return
	}
	if a.onPluginSearchTextInput(event) {
		return
	}
	if a.onSettingsSearchTextInput(event) {
		return
	}
	if a.onThemeEditorPreviewTextInput(event) {
		return
	}
	if a.onCloudFormTextInput(event) {
		return
	}
	if a.onBuiltInSettingsTextInput(event) {
		return
	}
	a.onPluginSettingsTextInput(event)
}

func (a *App) onLauncherWindowClosed() {
	a.mu.Lock()
	wasVisible := a.visible
	a.launcher = nil
	a.host = nil
	a.visible = false
	isPrimary := a.isPrimary
	if !isPrimary {
		a.destroyed = true
	}
	a.mu.Unlock()
	if !isPrimary {
		go func() {
			if wasVisible {
				if err := a.notifyHidden(); err != nil {
					log.Printf("notify Wox core after secondary close: %v", err)
				}
			}
			a.destroySecondary()
		}()
	}
}

// onSettingsWindowClosed releases window-owned interaction state before notifying core.
func (a *App) onSettingsWindowClosed() {
	a.mu.Lock()
	wasOpen := a.settingsOpen
	wasRecording := a.hotkeyRecording != nil
	a.settingsOpen = false
	a.settingsTitleBarHover = ""
	a.settingsView = nil
	a.settingsHost = nil
	a.settingSaving = false
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingSearchEditor = nil
	a.settingSearchFocused = false
	a.settingSearchPanel = false
	a.settingSearchSelected = 0
	a.settingSearchScroll = 0
	a.pluginSearchEditor = nil
	a.pluginSearchFocused = false
	a.pluginDetailTab = "settings"
	a.themeSearchEditor = nil
	a.themeSearchFocused = false
	a.themeDetailTab = "preview"
	a.releaseThemeEditorWallpaperLocked()
	a.settingChoicePicker = nil
	a.cloudForm = nil
	a.cloudActionMenu = ""
	a.tableEditor = nil
	a.modelManager = nil
	a.hotkeyRecording = nil
	a.settingsHotkeyFocus = false
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		a.pluginForm.active = false
	}
	if a.themeEditor != nil {
		a.themeEditor.active = false
	}
	launcherVisible := a.visible
	a.mu.Unlock()
	a.setSettingChoiceTooltip(false, "", woxui.Rect{})

	if wasRecording {
		a.postHotkeyRecordingStopped()
	}
	if wasOpen {
		if err := a.notifySettingViewChanged(false); err != nil {
			log.Printf("notify Wox core after settings close: %v", err)
		}
		if !launcherVisible {
			if err := a.notifyHidden(); err != nil {
				log.Printf("notify Wox core after final window hide: %v", err)
			}
		}
	}
	go a.refreshGlance("settingsChanged", "", nil)
}
