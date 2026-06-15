//go:build linux && cgo

package keyboard

import (
	"fmt"
	"os"
	"strings"
	"wox/util"
)

func init() {
	registerGlobalHotkeysPlatform = registerGlobalHotkeysLinux
	isWaylandGlobalShortcutsPortalAvailablePlatform = isWaylandGlobalShortcutsPortalAvailableLinux
}

func RegisterGlobalHotkey(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	if IsWaylandSession() {
		reg, err := registerGlobalHotkeyLinuxWayland(modifiers, key, callback)
		if err == nil {
			return reg, nil
		}

		if isGnomeDesktopSession() {
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

	registration, err := registerGlobalHotkeysLinuxWayland(specs)
	if err == nil {
		return registration, true, nil
	}

	if isGnomeDesktopSession() {
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
		return addRawKeyListenerLinuxWayland(handler)
	}
	return addRawKeyListenerLinuxX11(handler)
}

func IsWaylandSession() bool {
	return strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland") || os.Getenv("WAYLAND_DISPLAY") != ""
}

func isGnomeDesktopSession() bool {
	for _, value := range []string{
		os.Getenv("XDG_CURRENT_DESKTOP"),
		os.Getenv("DESKTOP_SESSION"),
		os.Getenv("GDMSESSION"),
	} {
		if strings.Contains(strings.ToLower(value), "gnome") {
			return true
		}
	}
	return false
}

func unsupportedWaylandRawListenerError() error {
	return fmt.Errorf("raw keyboard listeners are not supported on Wayland; double modifier hotkeys are unavailable")
}
