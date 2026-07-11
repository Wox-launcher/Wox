//go:build linux

package plugin

import (
	"context"
	"os/exec"
	"strings"
	"wox/i18n"
	"wox/setting"
	"wox/util"
	"wox/util/browser"
	"wox/util/hotkey"
	"wox/util/keyboard"
)

const gnomeAppIndicatorExtensionURL = "https://extensions.gnome.org/extension/615/appindicator-support/"
const waylandHotkeysHelpAnchor = "wayland-double-modifier-hotkeys"

// waylandHotkeysHelpURL returns the localized FAQ URL for the Wayland hotkey
// permissions guide. The docs site serves English under /guide and Chinese
// under /zh/guide; other languages fall back to the English page.
func waylandHotkeysHelpURL() string {
	base := "https://wox-launcher.github.io/Wox"
	if i18n.GetI18nManager().GetCurrentLangCode() == i18n.LangCodeZhCn {
		return base + "/zh/guide/faq#" + waylandHotkeysHelpAnchor
	}
	return base + "/guide/faq#" + waylandHotkeysHelpAnchor
}

// checkGnomeTrayIndicator verifies the AppIndicator host only when Wox runs under GNOME.
func checkGnomeTrayIndicator(ctx context.Context) (DoctorCheckResult, bool) {
	if !util.IsGnomeDesktopSession() {
		return DoctorCheckResult{}, false
	}

	if isStatusNotifierHostRegistered(ctx) {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_gnome_tray",
			Type:        DoctorCheckGnomeTrayIndicator,
			Passed:      true,
			Description: "i18n:plugin_doctor_gnome_tray_ok",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}, true
	}

	return DoctorCheckResult{
		Name:                   "i18n:plugin_doctor_gnome_tray",
		Type:                   DoctorCheckGnomeTrayIndicator,
		Passed:                 false,
		Description:            "i18n:plugin_doctor_gnome_tray_missing",
		ActionName:             "i18n:plugin_doctor_gnome_tray_open_extension",
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext ActionContext) {
			_ = browser.OpenURL(gnomeAppIndicatorExtensionURL, "")
		},
	}, true
}

// checkWaylandDesktopLaunch verifies that Wayland portal permissions can be
// associated with Wox's stable desktop identity.
func checkWaylandDesktopLaunch(ctx context.Context) (DoctorCheckResult, bool) {
	if !util.IsLinuxWaylandSession() {
		return DoctorCheckResult{}, false
	}

	if util.IsLinuxLaunchedFromStableDesktopEntry() {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_wayland_desktop_launch",
			Type:        DoctorCheckWaylandDesktopLaunch,
			Passed:      true,
			Description: "i18n:plugin_doctor_wayland_desktop_launch_ok",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}, true
	}

	return DoctorCheckResult{
		Name:        "i18n:plugin_doctor_wayland_desktop_launch",
		Type:        DoctorCheckWaylandDesktopLaunch,
		Passed:      false,
		Description: "i18n:plugin_doctor_wayland_desktop_launch_missing",
		ActionName:  "",
		Action:      func(ctx context.Context, actionContext ActionContext) {},
	}, true
}

func isStatusNotifierHostRegistered(ctx context.Context) bool {
	output, err := exec.CommandContext(
		ctx,
		"gdbus",
		"call",
		"--session",
		"--dest",
		"org.kde.StatusNotifierWatcher",
		"--object-path",
		"/StatusNotifierWatcher",
		"--method",
		"org.freedesktop.DBus.Properties.Get",
		"org.kde.StatusNotifierWatcher",
		"IsStatusNotifierHostRegistered",
	).CombinedOutput()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(output)), "true")
}

// checkLinuxInputGroup verifies that the user has read access to evdev keyboard
// devices, which is required for double-modifier and CapsLock-combo hotkeys on
// Wayland. On X11 this check is skipped because raw key events are obtained via
// XQueryKeymap without evdev.
//
// The check only runs when the user has actually configured a hotkey that needs
// evdev read access (a double-modifier or CapsLock combo). Normal portal
// hotkeys (e.g. alt+space) do not use evdev, so the check stays quiet for users
// who only rely on those.
//
// The 'uinput' group (write access to /dev/uinput, needed to restore CapsLock
// state after a CapsLock combo fires) is checked separately by
// checkLinuxUinputGroup, and only when the user has configured a CapsLock combo.
func checkLinuxInputGroup(ctx context.Context) (DoctorCheckResult, bool) {
	if !util.IsLinuxWaylandSession() {
		return DoctorCheckResult{}, false
	}

	if !userHasEvdevDependentHotkey(ctx) {
		return DoctorCheckResult{}, false
	}

	if keyboard.IsEvdevReadAvailable() {
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_linux_input_group",
			Type:        DoctorCheckLinuxInputGroup,
			Passed:      true,
			Description: "i18n:plugin_doctor_linux_input_group_ok",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}, true
	}

	return DoctorCheckResult{
		Name:                   "i18n:plugin_doctor_linux_input_group",
		Type:                   DoctorCheckLinuxInputGroup,
		Passed:                 false,
		Description:            "i18n:plugin_doctor_linux_input_group_missing",
		ActionName:             "i18n:plugin_doctor_linux_input_group_open_help",
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext ActionContext) {
			_ = browser.OpenURL(waylandHotkeysHelpURL(), "")
		},
	}, true
}

// checkLinuxUinputGroup verifies that the user has write access to /dev/uinput
// when they have actually configured a CapsLock combo hotkey. uinput is only
// needed to restore CapsLock state (and delete the stray combo character) after
// a CapsLock combo fires, so the check is skipped entirely when no CapsLock
// combo hotkey is configured. This keeps the doctor output quiet for users who
// do not use CapsLock combos.
func checkLinuxUinputGroup(ctx context.Context) (DoctorCheckResult, bool) {
	if !util.IsLinuxWaylandSession() {
		return DoctorCheckResult{}, false
	}

	if !userHasCapsLockComboHotkey(ctx) {
		return DoctorCheckResult{}, false
	}

	switch keyboard.CheckUinputAccess() {
	case keyboard.UinputAccessOK:
		return DoctorCheckResult{
			Name:        "i18n:plugin_doctor_linux_uinput_group",
			Type:        DoctorCheckLinuxUinputGroup,
			Passed:      true,
			Description: "i18n:plugin_doctor_linux_uinput_group_ok",
			ActionName:  "",
			Action:      func(ctx context.Context, actionContext ActionContext) {},
		}, true
	case keyboard.UinputAccessInGroupNoDevice:
		// The user is already in the 'uinput' group but /dev/uinput is still
		// not writable, which means the device node lacks group permissions.
		// Telling them to join the group again would be misleading.
		return DoctorCheckResult{
			Name:                   "i18n:plugin_doctor_linux_uinput_group",
			Type:                   DoctorCheckLinuxUinputGroup,
			Passed:                 false,
			Description:            "i18n:plugin_doctor_linux_uinput_group_in_group_no_device_permission",
			ActionName:             "i18n:plugin_doctor_linux_uinput_group_open_help",
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext ActionContext) {
				_ = browser.OpenURL(waylandHotkeysHelpURL(), "")
			},
		}, true
	default:
		// NotInGroup: the user is not a member of the 'uinput' group.
		return DoctorCheckResult{
			Name:                   "i18n:plugin_doctor_linux_uinput_group",
			Type:                   DoctorCheckLinuxUinputGroup,
			Passed:                 false,
			Description:            "i18n:plugin_doctor_linux_uinput_group_missing",
			ActionName:             "i18n:plugin_doctor_linux_uinput_group_open_help",
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext ActionContext) {
				_ = browser.OpenURL(waylandHotkeysHelpURL(), "")
			},
		}, true
	}
}

// userHasEvdevDependentHotkey reports whether any of the user's configured
// hotkeys (main, selection, or query) is a double-modifier or CapsLock combo.
// These are the only hotkey types that need evdev read access on Wayland;
// normal portal hotkeys (e.g. alt+space) do not. Used to gate the input-group
// doctor check so it only surfaces for users who actually need evdev.
func userHasEvdevDependentHotkey(ctx context.Context) bool {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if isEvdevDependentHotkey(woxSetting.MainHotkey.Get()) {
		return true
	}
	if isEvdevDependentHotkey(woxSetting.SelectionHotkey.Get()) {
		return true
	}
	for _, qh := range woxSetting.QueryHotkeys.Get() {
		if qh.Disabled {
			continue
		}
		if strings.TrimSpace(qh.Hotkey) == "" {
			continue
		}
		if isEvdevDependentHotkey(qh.Hotkey) {
			return true
		}
	}
	return false
}

// userHasCapsLockComboHotkey reports whether any of the user's configured
// hotkeys (main, selection, or query) is a CapsLock combo. Used to gate the
// uinput doctor check so it only surfaces for users who actually need uinput
// (CapsLock state restoration). Double-modifier hotkeys do not need uinput.
func userHasCapsLockComboHotkey(ctx context.Context) bool {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if hotkey.IsCapsLockHotkeyString(woxSetting.MainHotkey.Get()) {
		return true
	}
	if hotkey.IsCapsLockHotkeyString(woxSetting.SelectionHotkey.Get()) {
		return true
	}
	for _, qh := range woxSetting.QueryHotkeys.Get() {
		if qh.Disabled {
			continue
		}
		if strings.TrimSpace(qh.Hotkey) == "" {
			continue
		}
		if hotkey.IsCapsLockHotkeyString(qh.Hotkey) {
			return true
		}
	}
	return false
}

// isEvdevDependentHotkey reports whether combineKey is a hotkey type that
// requires evdev read access on Wayland (double-modifier or CapsLock combo).
func isEvdevDependentHotkey(combineKey string) bool {
	if strings.TrimSpace(combineKey) == "" {
		return false
	}
	return hotkey.IsDoubleModifierHotkeyString(combineKey) || hotkey.IsCapsLockHotkeyString(combineKey)
}
