package launcher

import (
	"log"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const settingsChangedTopic = "settings.changed"

// ensureSettingsWindow creates the independent settings host once per native lifetime.
func (a *App) ensureSettingsWindow() (*woxui.ManagedWindow, error) {
	if !a.isPrimary && a.primary != nil {
		return a.primary.ensureSettingsWindow()
	}

	var managed *woxui.ManagedWindow
	var openErr error
	var fontFamily string
	var isDark bool
	created := false
	if err := woxui.Call(func() {
		if existing := a.settingsView; existing != nil && existing.Lifecycle() != woxui.WindowLifecycleClosed {
			managed = existing
			return
		}
		host := woxwidget.NewHost(a.buildSettings)
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
				util.Go(a.lifecycleCtx, "close requested settings window", func() {
					if err := a.closeSettings(); err != nil {
						log.Printf("close requested settings window: %v", err)
					}
				})
			},
			OnClosed: func() {
				host.Dispose()
				a.onSettingsWindowClosed()
			},
		})
		if openErr == nil {
			host.Attach(managed.Window())
			a.settingsView = managed
			a.settingsHost = host
			a.settingsOpen = true
			fontFamily = a.generalSettings.Data().AppFontFamily
			isDark = themeColorIsDark(a.palette.background)
			created = true
		}
	}); err != nil {
		return nil, err
	}
	if openErr != nil {
		return nil, openErr
	}
	if !created {
		return managed, nil
	}
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
	var managed *woxui.ManagedWindow
	if err := a.runOnUI("resolve settings native window", func() {
		managed = a.settingsView
	}); err != nil {
		log.Printf("resolve settings native window: %v", err)
		return a.window
	}
	if managed == nil {
		return a.window
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
	searchFocused := a.settingsSearch.Focused() || a.pluginSettings.SearchFocused()
	if enabled || searchFocused {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 240, Y: 180, Width: 1, Height: 24}}
	}
	_ = window.SetTextInputState(state)
}

func (a *App) themeEditorUsesSettingsWindow() bool {
	editor := a.themeSettings.ThemeEditor()
	return editor != nil && strings.HasPrefix(editor.key, "settings-theme|")
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
	state := a.tableEditor
	usesSettings := state != nil && a.formTableTargetUsesSettingsLocked(state.target)
	return usesSettings
}

func (a *App) formTableTargetUsesSettingsLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return target != nil && ((pluginForm != nil && target == &pluginForm.formFieldsState) || target == a.aiSettings.Form() || target == a.hotkeySettings.Form())
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
	state := a.hotkeySettings.Recording()
	usesSettings := false
	if state != nil {
		pluginForm := a.pluginSettings.Form()
		usesSettings = state.target == a.hotkeySettings.Form() || (pluginForm != nil && state.target == &pluginForm.formFieldsState)
		if !usesSettings && a.tableEditor != nil && a.tableEditor.rowForm == state.target {
			usesSettings = a.formTableTargetUsesSettingsLocked(a.tableEditor.target)
		}
	}
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
	wasVisible := a.visible
	a.launcher = nil
	a.host = nil
	a.visible = false
	isPrimary := a.isPrimary
	if !isPrimary {
		a.destroyed.Store(true)
	}
	if !isPrimary {
		util.Go(a.lifecycleCtx, "destroy secondary launcher after close", func() {
			if wasVisible {
				if err := a.notifyHidden(); err != nil {
					log.Printf("notify Wox core after secondary close: %v", err)
				}
			}
			a.destroySecondary()
		})
	}
}

// onSettingsWindowClosed releases window-owned interaction state before notifying core.
func (a *App) onSettingsWindowClosed() {
	wasOpen := a.settingsOpen
	wasRecording := a.hotkeySettings.Recording() != nil
	a.settingsOpen = false
	a.settingsView = nil
	a.settingsHost = nil
	a.settingSaving = false
	a.generalSettings.EndEdit()
	a.settingsSearch.SetEditor(nil)
	a.settingsSearch.SetFocused(false)
	a.settingsSearch.SetPanel(false)
	a.settingsSearch.SetSelected(0)
	a.pluginSettings.SetSearchEditor(nil)
	a.pluginSettings.SetSearchFocused(false)
	a.pluginSettings.SetDetailTab("settings")
	a.themeSettings.SetThemeSearchEditor(nil)
	a.themeSettings.SetThemeSearchFocused(false)
	a.themeSettings.SetThemeDetailTab("preview")
	a.releaseThemeEditorWallpaperLocked()
	a.generalSettings.SetChoicePicker(nil)
	a.cloudSettings.SetForm(nil)
	a.cloudSettings.SetActionMenu("")
	a.tableEditor = nil
	a.aiSettings.SetModelManager(nil)
	a.hotkeySettings.ClearRecording()
	a.hotkeySettings.SetFocused(false)
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
	}
	if themeEditor := a.themeSettings.ThemeEditor(); themeEditor != nil {
		themeEditor.active = false
	}
	launcherVisible := a.visible
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
	util.Go(a.lifecycleCtx, "refresh glance after settings close", func() {
		a.refreshGlance("settingsChanged", "", nil)
	})
}
