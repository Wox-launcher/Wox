//go:build linux && cgo

package keyboard

import (
	"fmt"
	"wox/util"
)

func init() {
	registerGlobalHotkeysPlatform = registerGlobalHotkeysLinux
	isWaylandGlobalShortcutsPortalAvailablePlatform = isWaylandGlobalShortcutsPortalAvailableLinux
}

func RegisterGlobalHotkey(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	if IsWaylandSession() {
		// On Hyprland, the portal backend cannot deliver key events without
		// manual compositor-side bind configuration. Use the native Hyprland
		// Lua bind backend instead, which auto-registers via hyprctl.
		if isHyprlandSession() {
			reg, _, err := registerGlobalHotkeysLinuxHyprland([]GlobalHotkeySpec{
				{Modifiers: modifiers, Key: key, Callback: callback},
			})
			return reg, err
		}
		reg, err := registerGlobalHotkeyLinuxWayland(modifiers, key, callback)
		if err == nil {
			return reg, nil
		}

		if util.IsGnomeDesktopSession() {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[hotkey] wayland portal unavailable (%v), falling back to GNOME custom keybindings", err))
			return registerGlobalHotkeyLinuxGnome(modifiers, key, callback)
		}

		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal unavailable (%v), no desktop-specific fallback available", err))
		return nil, fmt.Errorf("wayland global shortcuts portal unavailable: %w", err)
	}
	return registerGlobalHotkeyLinuxX11(modifiers, key, callback)
}

func registerGlobalHotkeysLinux(specs []GlobalHotkeySpec) (HotkeyRegistration, bool, error) {
	if !IsWaylandSession() {
		return nil, false, nil
	}

	// On Hyprland, prefer the native Lua bind backend over the portal backend.
	if isHyprlandSession() {
		return registerGlobalHotkeysLinuxHyprland(specs)
	}

	registration, err := registerGlobalHotkeysLinuxWayland(specs)
	if err == nil {
		return registration, true, nil
	}

	if util.IsGnomeDesktopSession() {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] wayland portal unavailable (%v), falling back to GNOME custom keybindings", err))
		return nil, false, nil
	}

	util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] wayland portal unavailable (%v), no desktop-specific fallback available", err))
	return nil, true, fmt.Errorf("wayland global shortcuts portal unavailable: %w", err)
}

func AddRawKeyListener(handler RawKeyHandler) (RawKeySubscription, error) {
	if IsWaylandSession() {
		// On Wayland, the display server does not expose raw key events to
		// applications. Try evdev direct-read as a fallback so double-modifier
		// and CapsLock-combo hotkeys can still work when the user has read
		// access to /dev/input/event* (membership in the 'input' group).
		if IsEvdevReadAvailable() {
			return addRawKeyListenerLinuxEvdev(handler)
		}
		return addRawKeyListenerLinuxWayland(handler)
	}
	return addRawKeyListenerLinuxX11(handler)
}

func IsWaylandSession() bool {
	return util.IsLinuxWaylandSession()
}

func unsupportedWaylandRawListenerError() error {
	return fmt.Errorf("raw keyboard listeners are not supported on Wayland; double modifier hotkeys are unavailable")
}
