package launcher

import (
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type settingsDemoState struct {
	kind   string
	anchor woxui.Rect
}

// setSettingsDemoHover shows a shared animated preview and delays dismissal while the pointer crosses into it.
func (a *App) setSettingsDemoHover(kind string, inside bool, anchor woxui.Rect) {
	if inside {
		a.settingsDemoRevision.Add(1)
		a.settingsDemo = &settingsDemoState{kind: kind, anchor: anchor}
		a.preloadDemoWallpaper(true)
		a.invalidateSettingsWindow()
		return
	}
	a.scheduleSettingsDemoHide(kind)
}

// scheduleSettingsDemoHide leaves time for the pointer to cross the popover gap.
func (a *App) scheduleSettingsDemoHide(kind string) {
	revision := a.settingsDemoRevision.Add(1)
	util.Go(a.lifecycleCtx, "hide settings demo", func() {
		timer := time.NewTimer(160 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-a.lifecycleCtx.Done():
			return
		case <-timer.C:
		}
		_ = a.runOnUI("hide settings demo", func() {
			if revision != a.settingsDemoRevision.Load() || a.settingsDemo == nil || a.settingsDemo.kind != kind {
				return
			}
			a.settingsDemo = nil
			a.invalidateSettingsWindow()
		})
	})
}

// keepSettingsDemoHover cancels target-exit dismissal while the preview owns the pointer.
func (a *App) keepSettingsDemoHover(kind string, inside bool) {
	if inside {
		a.settingsDemoRevision.Add(1)
		return
	}
	a.scheduleSettingsDemoHide(kind)
}

// buildSettingsDemoOverlay maps each title affordance to the shared onboarding demo component.
func (a *App) buildSettingsDemoOverlay(snapshot settingsSnapshot, width, height float32) (woxwidget.Widget, float32, float32) {
	if a.settingsDemo == nil {
		return nil, 0, 0
	}
	step := launcherview.OnboardingStep{ID: "queryHotkeys", Title: a.translate("i18n:ui_query_hotkeys"), Accent: woxui.Color{R: 244, G: 63, B: 94, A: 255}}
	switch a.settingsDemo.kind {
	case "query-hotkey-preset-normal":
		step = launcherview.OnboardingStep{ID: "queryHotkeysNormal", Title: a.translate("i18n:ui_query_hotkeys_preset_normal"), Accent: woxui.Color{R: 59, G: 130, B: 246, A: 255}}
	case "query-hotkey-preset-web-panel":
		step = launcherview.OnboardingStep{ID: "queryHotkeysWebPanel", Title: a.translate("i18n:ui_query_hotkeys_preset_web_panel"), Accent: woxui.Color{R: 244, G: 63, B: 94, A: 255}}
	case "query-hotkey-preset-silent":
		step = launcherview.OnboardingStep{ID: "queryHotkeysSilent", Title: a.translate("i18n:ui_query_hotkeys_preset_silent"), Accent: woxui.Color{R: 34, G: 197, B: 94, A: 255}}
	case "query-shortcuts":
		step = launcherview.OnboardingStep{ID: "queryShortcuts", Title: a.translate("i18n:ui_query_shortcuts"), Accent: woxui.Color{R: 167, G: 139, B: 250, A: 255}}
	case "tray-queries":
		step = launcherview.OnboardingStep{ID: "trayQueries", Title: a.translate("i18n:ui_tray_queries"), Accent: woxui.Color{R: 34, G: 197, B: 94, A: 255}}
	}
	props := launcherview.OnboardingProps{
		Wallpaper: snapshot.theme.ThemeWallpaperImage, WallpaperBlurred: snapshot.theme.ThemeWallpaperBlurred,
		Labels: a.onboardingLabels(), Theme: snapshot.palette.componentTheme(),
	}
	kind := a.settingsDemo.kind
	return launcherview.SettingsDemoOverlay(props, step, a.settingsDemo.anchor, width, height, snapshot.palette.componentTheme(), func(inside bool) {
		a.keepSettingsDemoHover(kind, inside)
	})
}
