//go:build linux && cgo

package selection

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"wox/util/keyboard"
	"wox/util/window"
)

var kdeSelectionCaptureMu sync.Mutex

// getSelectedLinuxKDE keeps one copy outstanding so overlapping queries cannot
// consume each other's clipboard offer.
func getSelectedLinuxKDE(ctx context.Context) (Selection, error) {
	if !kdeSelectionCaptureMu.TryLock() {
		return Selection{}, noSelection
	}
	defer kdeSelectionCaptureMu.Unlock()

	pid, windowID, _ := window.GetActiveWindowIdentity(ctx)
	if windowID == "" || pid <= 0 {
		return Selection{}, fmt.Errorf("KWin could not identify the current selection window: %w", noSelection)
	}
	shift := kdeSelectionCopyUsesShift(window.GetProcessIdentity(pid))
	return captureWaylandSelection(func() error {
		return keyboard.SimulateKDECopy(ctx, shift, func() error {
			// A focus change while the keyboard starts invalidates the query.
			activePID, activeID, _ := window.GetActiveWindowIdentity(ctx)
			if activeID != windowID || activePID != pid {
				return fmt.Errorf("KWin selection window lost focus: %w", noSelection)
			}
			return nil
		})
	})
}

// kdeSelectionCopyUsesShift avoids interrupting a terminal foreground job with Ctrl+C.
func kdeSelectionCopyUsesShift(identity string) bool {
	switch strings.TrimSuffix(strings.ToLower(identity), ".desktop") {
	case "konsole", "org.kde.konsole", "yakuake", "org.kde.yakuake",
		"foot", "footclient", "kitty", "alacritty", "org.wezfurlong.wezterm", "wezterm",
		"com.mitchellh.ghostty", "ghostty", "gnome-terminal", "org.gnome.terminal", "xterm":
		return true
	default:
		return false
	}
}
