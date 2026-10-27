//go:build linux

package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/plugin"
	"wox/util"
	"wox/util/window"
)

const (
	linuxLauncherHideWait   = 400 * time.Millisecond
	linuxLauncherHideSettle = 60 * time.Millisecond
)

// tryActivateExistingLinuxApp focuses a window that already belongs to this
// desktop entry. A miss returns false so the caller starts the app.
func tryActivateExistingLinuxApp(ctx context.Context, api plugin.API, info appInfo) bool {
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(info.Path)), ".desktop") {
		return false
	}
	activation, ok := window.PrepareLinuxAppActivation(ctx, info.Path)
	if !ok {
		return false
	}
	// The launcher is an exclusive layer-shell surface. Hyprland returns keyboard
	// focus to the previous window when that surface unmaps, so a focus issued
	// while the launcher is still visible is undone as soon as it hides.
	waitForLinuxLauncherHide(ctx, func() bool {
		return api != nil && api.IsVisible(ctx)
	})
	if activation.Focus(ctx) {
		return true
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("could not focus existing window for %s, launching", info.Name))
	return false
}

// waitForLinuxLauncherHide blocks until the launcher session is hidden, then
// gives the compositor a moment to finish its own focus restore.
func waitForLinuxLauncherHide(ctx context.Context, visible func() bool) {
	if visible == nil || !visible() {
		return
	}
	deadline := time.Now().Add(linuxLauncherHideWait)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for visible() {
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
	timer := time.NewTimer(linuxLauncherHideSettle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
