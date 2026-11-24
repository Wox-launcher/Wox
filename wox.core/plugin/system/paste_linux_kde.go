//go:build linux

package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
	"wox/util/keyboard"
	"wox/util/window"
)

// pasteToKDEWindow checks the captured target immediately before injecting the chord.
func pasteToKDEWindow(ctx context.Context, api plugin.API, target plugin.QueryEnv) error {
	shift := kdePasteUsesShift(window.GetProcessIdentity(target.ActiveWindowPid))
	err := keyboard.SimulatePasteAfterFocus(shift, func() error {
		if api.IsVisible(ctx) {
			return errors.New("launcher is visible")
		}
		if err := window.FocusKDEWindow(ctx, target.ActiveWindowId, target.ActiveWindowPid); err != nil {
			return err
		}
		if api.IsVisible(ctx) {
			return errors.New("launcher reopened during focus restoration")
		}
		return nil
	})
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("KDE Wayland automatic paste failed: %v", err))
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_paste_manually"))
	}

	api.Log(ctx, plugin.LogLevelInfo, "simulate paste clipboard success")
	return nil
}

func kdePasteUsesShift(identity string) bool {
	switch strings.TrimSuffix(strings.ToLower(identity), ".desktop") {
	case "org.kde.konsole", "konsole":
		return true
	default:
		return false
	}
}
