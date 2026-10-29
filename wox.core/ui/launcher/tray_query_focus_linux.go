//go:build linux

package launcher

import (
	"context"
	"fmt"

	"wox/common"
	"wox/util"
	"wox/util/tray"
)

// releaseTrayQueryKeyboardGrab lets Hyprland deliver outside clicks after initial activation.
func (a *App) releaseTrayQueryKeyboardGrab() {
	if a.window == nil || a.show.ShowSource != string(common.ShowSourceTrayQuery) || !tray.SupportsQueryIcons() {
		return
	}
	// The initial exclusive mode activates the window; on-demand mode then permits focus to leave it.
	if err := a.window.SetExclusiveKeyboard(false); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("release tray query keyboard grab: %v", err))
	}
}
