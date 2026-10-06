package launcher

import (
	"fmt"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// dispatchWindowKey reserves window commands before focused controls and feature actions.
// Callers handle hotkey recording first so reserved combinations can still be recorded.
func (a *App) dispatchWindowKey(event woxui.KeyEvent, host *woxwidget.Host, settingsPluginID string, closeRequested func(), fallback func(woxui.KeyEvent) bool) bool {
	if a.onWindowShortcut(event, settingsPluginID, closeRequested) {
		return true
	}
	if host != nil && host.Key(event) {
		return true
	}
	return fallback(event)
}

// onWindowShortcut consumes repeats without scheduling another window transition.
func (a *App) onWindowShortcut(event woxui.KeyEvent, settingsPluginID string, closeRequested func()) bool {
	if !event.Down || event.Composing || !woxui.IsWindowShortcut(event.Key, event.Modifiers) {
		return false
	}
	if !event.Repeat {
		if event.Key == woxui.Key(",") {
			a.requestSettingsFromShortcut(settingsPluginID)
		} else {
			closeRequested()
		}
	}
	return true
}

// requestSettingsFromShortcut focuses generic opens and routes plugin windows through existing draft-aware navigation.
func (a *App) requestSettingsFromShortcut(pluginID string) {
	if !a.isPrimary && a.primary != nil {
		a.primary.requestSettingsFromShortcut(pluginID)
		return
	}
	if !a.openingSettingsShortcut.CompareAndSwap(false, true) {
		return
	}
	util.Go(a.lifecycleCtx, "open settings from shortcut", func() {
		defer a.openingSettingsShortcut.Store(false)
		var existing *woxui.ManagedWindow
		err := a.runOnUI("resolve settings shortcut target", func() {
			if a.settingsView != nil && a.settingsView.Lifecycle() != woxui.WindowLifecycleClosed {
				existing = a.settingsView
				if pluginID != "" {
					a.activateSettingsSearchResult(settingsSearchResult{kind: settingsSearchPlugin, pluginID: pluginID})
					a.pluginSettings.SetDetailTab("settings")
				}
			}
		})
		if err == nil {
			if existing != nil {
				_, err = existing.Show()
			} else {
				windowContext := common.DefaultSettingWindowContext
				if pluginID != "" {
					windowContext = common.SettingWindowContext{Path: "/plugin/setting", Param: pluginID}
				}
				err = a.OpenSetting(a.lifecycleCtx, windowContext)
			}
		}
		if err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("open settings from shortcut: %v", err))
		}
	})
}

// requestLauncherShortcutClose lets native WebView callbacks unwind before destroying a browser.
func (a *App) requestLauncherShortcutClose() {
	if a.webViewPreviewData != "" {
		queryID, previewData := a.query.QueryID, a.webViewPreviewData
		closePage := func() {
			// A query change or an earlier close may have replaced this page while the callback was queued.
			if !a.destroyed.Load() && a.query.QueryID == queryID && a.webViewPreviewData == previewData {
				a.closeWebViewPage()
			}
		}
		if a.uiPost == nil {
			closePage()
		} else if err := a.uiPost(closePage); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("schedule webview close: %v", err))
		}
		return
	}
	a.closePreviewWindow()
}

// requestSettingsClose shares the caption and keyboard close path, including plugin form submission.
func (a *App) requestSettingsClose() {
	util.Go(a.lifecycleCtx, "close requested settings window", func() {
		if err := a.closeSettings(); err != nil {
			util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("close requested settings window: %v", err))
		}
	})
}
