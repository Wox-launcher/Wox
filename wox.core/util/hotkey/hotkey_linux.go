//go:build linux

package hotkey

import (
	"context"
	"fmt"
	"wox/util"
	"wox/util/keyboard"
)

func validateHotkeySpec(spec hotkeySpec) error {
	if !keyboard.IsWaylandSession() {
		return nil
	}

	if spec.isDoubleModifier() {
		if !keyboard.IsEvdevReadAvailable() {
			return fmt.Errorf("double modifier hotkeys require evdev access on Wayland; add user to 'input' group")
		}
	}

	if spec.isCapsLockKey() {
		if !keyboard.IsEvdevReadAvailable() {
			return fmt.Errorf("CapsLock combo hotkeys require evdev access on Wayland; add user to 'input' group")
		}
	}

	return nil
}

func init() {
	// On Wayland, the XDG GlobalShortcuts portal does not have a concept of
	// "hotkey conflicts" — the portal always accepts the registration request and
	// the desktop environment resolves conflicts internally. Running the standard
	// register-probe-unregister cycle (used on X11/macOS/Windows) is harmful
	// here because:
	//   1. If the portal is unavailable (old GNOME/KDE), every probe returns an
	//      error and the UI reports every hotkey as "not available".
	//   2. Even when the portal is available, creating a session only to destroy
	//      it immediately can trigger DE confirmation dialogs or cause spurious
	//      D-Bus errors.
	// Portal-backed Wayland sessions only validate the spec itself. Hyprland has
	// an inspectable compositor bind registry, so it additionally rejects keys
	// already owned by the user's default submap.
	platformHotkeyAvailableCheck = func(_ context.Context, hotkeyStr string) (bool, bool, error) {
		if !keyboard.IsWaylandSession() {
			// Not a Wayland session; fall through to the standard X11 check.
			return false, false, nil
		}

		hk := &Hotkey{}
		spec, parseErr := hk.parseCombineKey(hotkeyStr)
		if parseErr != nil {
			return false, true, nil
		}
		if validateErr := validateHotkeySpec(spec); validateErr != nil {
			return false, true, nil
		}
		if util.IsCosmicDesktopSession() && !spec.isDoubleModifier() && !spec.isCapsLockKey() && !spec.isModifierChord() {
			available, err := keyboard.IsCosmicGlobalHotkeyAvailable(spec.modifiers, spec.key)
			if err != nil {
				util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to check COSMIC hotkey availability: hotkey=%s err=%s", hotkeyStr, err.Error()))
			}
			return available, true, err
		}
		if util.IsHyprlandSession() && !spec.isDoubleModifier() && !spec.isCapsLockKey() && !spec.isModifierChord() {
			available, err := keyboard.IsHyprlandGlobalHotkeyAvailable(spec.modifiers, spec.key)
			if err != nil {
				util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to check Hyprland hotkey availability: hotkey=%s err=%s", hotkeyStr, err.Error()))
				return false, true, nil
			}
			return available, true, nil
		}
		return true, true, nil
	}
}
