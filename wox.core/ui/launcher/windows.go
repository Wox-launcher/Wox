package launcher

import (
	"context"
	"log"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const settingsChangedTopic = "settings.changed"

// ensureSettingsWindow creates the independent settings host once per native lifetime.
func (a *settingsWindowController) ensureSettingsWindow() (*woxui.ManagedWindow, error) {
	if primary := a.primaryOwner(); primary != nil {
		return primary.ensureSettingsWindow()
	}

	var managed *woxui.ManagedWindow
	var openErr error
	var fontFamily string
	created := false
	if err := woxui.Call(func() {
		if existing := a.view; existing != nil && existing.Lifecycle() != woxui.WindowLifecycleClosed {
			managed = existing
			return
		}
		host := woxwidget.NewHost(a.buildSettings)
		managed, _, openErr = a.windows.Open(settingsWindowID, woxui.WindowOptions{
			Title:            a.settingsWindowTitle(),
			Size:             woxui.Size{Width: settingsWindowWidth, Height: settingsWindowHeight},
			Role:             woxui.WindowRoleApplication,
			TitleBarControls: woxui.TitleBarControls{Height: launcherview.SettingsTitleBarHeight, Close: true, Minimize: true},
			OnFrame:          host.Frame,
			OnPointer: func(event woxui.PointerEvent) {
				host.Pointer(event)
				a.finishSettingsPointer(event)
			},
			OnFocus: func(event woxui.FocusEvent) {
				host.SetWindowFocused(event.Active)
			},
			OnKey: func(event woxui.KeyEvent) bool {
				// Recording captures combos before focused buttons can consume Space or Enter.
				if a.hotkeyRecordingUsesSettingsWindow() && a.onSettingsWindowKey(event) {
					return true
				}
				return a.dispatchWindowKey(event, host, "", a.requestSettingsClose, a.onSettingsWindowKey)
			},
			OnTextInput: func(event woxui.TextInputEvent) {
				if !host.TextInput(event) {
					a.onSettingsWindowTextInput(event)
				}
			},
			OnCloseRequested: a.requestSettingsClose,
			OnClosed: func() {
				host.Dispose()
				a.onSettingsWindowClosed()
			},
		})
		if openErr == nil {
			host.Attach(managed.Window())
			a.view = managed
			a.host = host
			a.open = true
			fontFamily = a.fontFamily()
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
	if err := managed.Window().SetAppearance(true); err != nil {
		_ = managed.Close()
		return nil, err
	}
	// Fixed Settings colors still use the system material behind the translucent tint.
	if err := managed.Window().SetWindowChrome(false, nil); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if err := managed.Window().SetFontFamily(fontFamily); err != nil {
		_ = managed.Close()
		return nil, err
	}
	return managed, nil
}

// ensureOnboardingWindow creates the first-run guide as its own managed window.
func (c *onboardingController) ensureOnboardingWindow() (*woxui.ManagedWindow, error) {
	if primary := c.primaryOwner(); primary != nil {
		return primary.ensureOnboardingWindow()
	}

	var managed *woxui.ManagedWindow
	var openErr error
	var fontFamily string
	created := false
	if err := woxui.Call(func() {
		if existing := c.view; existing != nil && existing.Lifecycle() != woxui.WindowLifecycleClosed {
			managed = existing
			return
		}
		host := woxwidget.NewHost(c.buildOnboarding)
		managed, _, openErr = c.windows.Open(onboardingWindowID, woxui.WindowOptions{
			Title:     c.language.translate("i18n:onboarding_title"),
			Size:      woxui.Size{Width: onboardingWindowWidth, Height: onboardingWindowHeight},
			Role:      woxui.WindowRoleApplication,
			OnFrame:   host.Frame,
			OnPointer: host.Pointer,
			OnFocus: func(event woxui.FocusEvent) {
				host.SetWindowFocused(event.Active)
				c.onOnboardingWindowFocus(event)
			},
			OnKey: func(event woxui.KeyEvent) bool {
				if c.hotkeyRecordingUsesOnboardingWindow() && c.onOnboardingWindowKey(event) {
					return true
				}
				if host.Key(event) {
					return true
				}
				return c.onOnboardingWindowKey(event)
			},
			OnTextInput: func(event woxui.TextInputEvent) {
				host.TextInput(event)
			},
			// First-run setup must complete through its finish action.
			OnCloseRequested: func() {},
			OnClosed: func() {
				host.Dispose()
				c.onOnboardingWindowClosed()
				c.view = nil
				c.host = nil
			},
		})
		if openErr == nil {
			host.Attach(managed.Window())
			c.view = managed
			c.host = host
			fontFamily = c.fontFamily()
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
	if err := managed.Window().SetAppearance(true); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if err := managed.Window().SetWindowChrome(false, nil); err != nil {
		_ = managed.Close()
		return nil, err
	}
	if err := managed.Window().SetFontFamily(fontFamily); err != nil {
		_ = managed.Close()
		return nil, err
	}
	return managed, nil
}

func (a *settingsWindowController) settingsNativeWindow() *woxui.Window {
	var managed *woxui.ManagedWindow
	if err := a.runOnUI("resolve settings native window", func() {
		managed = a.view
	}); err != nil {
		log.Printf("resolve settings native window: %v", err)
		return a.window()
	}
	if managed == nil {
		return a.window()
	}
	return managed.Window()
}

func (c *onboardingController) onboardingNativeWindow() *woxui.Window {
	var managed *woxui.ManagedWindow
	if err := c.runOnUI("resolve onboarding native window", func() {
		managed = c.view
	}); err != nil {
		log.Printf("resolve onboarding native window: %v", err)
		return c.window()
	}
	if managed == nil {
		return c.window()
	}
	return managed.Window()
}

func (a *settingsWindowController) invalidateSettingsWindow() {
	if window := a.settingsNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (c *onboardingController) invalidateOnboardingWindow() {
	if window := c.onboardingNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

// invalidateLauncherBoundary uses retained geometry for controller-driven local updates.
func (a *App) invalidateLauncherBoundary(key woxwidget.Key) {
	if a.host != nil && a.host.InvalidateBoundary(key) {
		return
	}
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

func (a *App) invalidateAllWindows() {
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	if settingsWindow := a.settingsWindow.settingsNativeWindow(); settingsWindow != nil && settingsWindow != a.window {
		_ = settingsWindow.Invalidate()
	}
	if onboardingWindow := a.onboarding.onboardingNativeWindow(); onboardingWindow != nil && onboardingWindow != a.window {
		_ = onboardingWindow.Invalidate()
	}
	a.chatWindow.invalidateChatWindow()
	for _, controller := range a.notes.noteWindows {
		if controller.managed != nil {
			_ = controller.managed.Window().Invalidate()
		}
	}
}

func (a *App) updateSettingsTextInput(enabled bool) {
	window := a.settingsWindow.settingsNativeWindow()
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

func (a *App) hotkeyRecordingUsesSettingsWindow() bool {
	state := a.hotkeySettings.Recording()
	usesSettings := false
	if state != nil {
		pluginForm := a.pluginSettings.Form()
		usesSettings = (!a.onboarding.isOpen() && state.target == a.hotkeySettings.Form()) || (pluginForm != nil && state.target == &pluginForm.formFieldsState)
		if !usesSettings && a.formTables.settingsEditor != nil && a.formTables.settingsEditor.rowForm == state.target {
			usesSettings = true
		}
	}
	return usesSettings
}

func (a *App) hotkeyRecordingUsesOnboardingWindow() bool {
	state := a.hotkeySettings.Recording()
	return state != nil && a.onboarding.isOpen() && (state.target == a.hotkeySettings.Form() || a.onboarding.ownsHotkeyTarget(state.target))
}

func (a *App) hotkeyRecordingNativeWindow() *woxui.Window {
	if a.hotkeyRecordingUsesOnboardingWindow() {
		return a.onboarding.onboardingNativeWindow()
	}
	if a.hotkeyRecordingUsesSettingsWindow() {
		return a.settingsWindow.settingsNativeWindow()
	}
	return a.window
}

func (a *App) invalidateHotkeyWindows() {
	a.invalidateAllWindows()
}

func (a *App) formFieldNativeWindow(idPrefix string) *woxui.Window {
	switch idPrefix {
	case "hotkey-settings", "onboarding-query-hotkey":
		if a.onboarding.isOpen() {
			return a.onboarding.onboardingNativeWindow()
		}
		return a.settingsWindow.settingsNativeWindow()
	case "plugin-settings", "ai-settings", "cloud-form":
		return a.settingsWindow.settingsNativeWindow()
	case "theme-editor", "theme-editor-dialog":
		return a.themeSettings.themeEditorNativeWindow()
	case "form-table-row":
		return a.formTables.formTableNativeWindow()
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
	if a.formTables.formTableUsesSettingsWindow() {
		return a.formTables.onFormTableKey(event)
	}
	return a.onSettingsKey(event)
}

func (a *App) onSettingsWindowTextInput(event woxui.TextInputEvent) {
	if a.formTables.formTableUsesSettingsWindow() && a.formTables.onFormTableTextInput(event) {
		return
	}
	if a.pluginSettings.onPluginSearchTextInput(event) {
		return
	}
	if a.onSettingsSearchTextInput(event) {
		return
	}
	if a.themeSettings.onThemeEditorPreviewTextInput(event) {
		return
	}
	if a.cloudSettings.onCloudFormTextInput(event) {
		return
	}
	if a.onBuiltInSettingsTextInput(event) {
		return
	}
	a.pluginSettings.onPluginSettingsTextInput(event)
}

func (a *App) onOnboardingWindowKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if a.hotkeyRecordingUsesOnboardingWindow() && a.onHotkeyRecordingKey(event) {
		return true
	}
	return a.onboarding.onOnboardingKey(event)
}

func (a *App) onLauncherWindowClosed() {
	a.previews.releasePinnedPreviewImage()
	wasVisible := a.visible
	a.dictation.deactivateDictationAudio()
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
	wasRecording := a.hotkeyRecordingUsesSettingsWindow()
	wasOpen := a.settingsWindow.endSessionLocked()
	a.formTables.discardSettingsEditor()
	a.generalSettings.EndEdit()
	a.settingsSearch.ReleaseWindowMemory()
	a.pluginSettings.ReleaseWindowMemory()
	a.themeSettings.SetThemeSearchEditor(nil)
	a.themeSettings.SetThemeSearchFocused(false)
	a.themeSettings.SetThemeDetailTab("preview")
	a.themeSettings.releaseDemoWallpaperLocked()
	a.generalSettings.SetChoicePicker(nil)
	a.generalSettings.SetShowDisplayPicker(nil)
	a.generalSettings.ClearShowDisplays()
	a.cloudSettings.SetForm(nil)
	a.cloudSettings.SetActionMenu("")
	a.cloudSettings.SetPluginDialog(nil)
	// The cloud tab reloads its snapshot on open, so the installed plugin catalog it
	// fetched for the exclusion list does not need to outlive the window.
	a.cloudSettings.SetPlugins(nil)
	a.cloudSettings.SetLoaded(false)
	a.aiSettings.SetForm(nil)
	a.abandonModelManager()
	a.generalSettings.SetForm(nil)
	if !a.onboarding.isOpen() {
		a.hotkeySettings.ReleaseWindowMemory()
	} else {
		a.hotkeySettings.SetForm(nil)
		a.hotkeySettings.ClearRecording()
		a.hotkeySettings.SetFocused(false)
	}
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
	}
	if themeEditor := a.themeSettings.ThemeEditor(); themeEditor != nil {
		if themeEditor.ai.cancel != nil {
			themeEditor.ai.cancel()
			themeEditor.ai.cancel = nil
			themeEditor.ai.busy = false
			themeEditor.ai.request++
			themeEditor.ai.status = ""
		}
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
		a.glance.refreshGlance("settingsChanged", "", nil)
	})
}

// openOnboarding replaces Settings with the first-run guide and hands the hotkey
// recorder to the guide's hotkey page.
func (a *App) openOnboarding() error {
	if err := a.reloadSettingsForWindow(); err != nil {
		return err
	}
	wasSettings := false
	var settingsView *woxui.ManagedWindow
	installations := detectOnboardingMigrations(context.Background())
	if err := a.runOnUI("prepare onboarding", func() {
		wasSettings = a.settingsWindow.open
		a.settingsWindow.open = false
		a.onboarding.beginSessionLocked(installations)
		a.stopHotkeyRecording()
		settingsView = a.settingsWindow.view
		if form := a.hotkeySettings.Form(); form != nil {
			form.active = true
			setFormFieldsFocusLocked(form, 0)
		}
	}); err != nil {
		return err
	}
	if wasSettings {
		if settingsView != nil {
			_ = settingsView.Hide()
		}
		if err := a.notifySettingViewChanged(false); err != nil {
			return err
		}
	}
	return a.onboarding.present()
}

func (a *App) onOnboardingWindowClosed() {
	wasRecording := a.hotkeyRecordingUsesOnboardingWindow()
	wasOpen := a.onboarding.leaveLocked()
	a.themeSettings.releaseDemoWallpaperLocked()
	if wasRecording {
		a.hotkeySettings.ClearRecording()
		a.postHotkeyRecordingStopped()
	}
	if wasOpen {
		if err := a.onboarding.notifyOnboardingViewChanged(false); err != nil {
			log.Printf("notify Wox core after onboarding close: %v", err)
		}
	}
}
