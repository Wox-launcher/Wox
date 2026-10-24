package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"wox/analytics"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
)

var errNewWindowUnsupported = errors.New("application does not expose a supported new-window command")

// buildNewWindowAction keeps explicit window creation separate from normal window activation.
func (a *ApplicationPlugin) buildNewWindowAction(info appInfo, displayName string, contextData map[string]string, openAction plugin.QueryResultAction) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name:        "i18n:plugin_app_open_new_window",
		Icon:        icons.Get(icons.ActionNewWindow),
		ContextData: contextData,
		Hotkey:      "ctrl+enter",
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			util.GetLogger().Debug(ctx, fmt.Sprintf("app_new_window stage=start path=%q", info.Path))
			// Keep the action discoverable for every app; resolve support only when invoked.
			if err := openAppNewWindow(info); err != nil {
				if errors.Is(err, errNewWindowUnsupported) {
					util.GetLogger().Debug(ctx, fmt.Sprintf("app_new_window stage=fallback path=%q reason=%s", info.Path, err))
					// Reuse Open itself so activation, platform routing and error handling remain identical.
					openAction.Action(ctx, actionContext)
					return
				}
				util.GetLogger().Error(ctx, fmt.Sprintf("error opening new app window %q: %s", info.Path, err))
				a.api.Notify(ctx, fmt.Sprintf(a.api.GetTranslation(ctx, "plugin_app_open_failed_description"), err.Error()))
				return
			}
			// Dispatch success does not guarantee that the application created a visible window.
			util.GetLogger().Debug(ctx, fmt.Sprintf("app_new_window stage=dispatched path=%q", info.Path))
			analytics.TrackAppLaunched(ctx, fmt.Sprintf("%s:%s", info.Type, info.Name), displayName)
		},
	}
}

// appNewWindowArguments matches executable identities, never localized result titles.
func appNewWindowArguments(executable string) []string {
	switch strings.ToLower(executable) {
	case "firefox", "firefox.exe":
		return []string{"--new-window", "about:blank"}
	case "chrome", "chrome.exe", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "msedge", "msedge.exe", "microsoft-edge", "microsoft-edge-stable", "brave", "brave.exe", "brave-browser", "vivaldi", "vivaldi.exe", "vivaldi-stable":
		return []string{"--new-window"}
	case "code", "code.exe", "code-insiders", "code - insiders.exe":
		return []string{"--new-window"}
	case "wt.exe", "windowsterminal.exe":
		return []string{"-w", "new"}
	}
	return nil
}

// supportsNewWindowArguments excludes launch modes whose semantics conflict with a normal new window.
func supportsNewWindowArguments(args []string) bool {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		key := strings.ToLower(strings.SplitN(arg, "=", 2)[0])
		switch strings.TrimLeft(key, "-") {
		case "app", "app-id", "kiosk", "headless", "osint", "new-instance", "no-remote", "new-tab", "private-window", "reuse-window", "r", "w", "window":
			return false
		}
		if arg == "--" {
			return false
		}
	}
	return true
}
