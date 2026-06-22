//go:build linux

package plugin

import (
	"context"
	"os/exec"
	"strings"
	"wox/util"
	"wox/util/browser"
	"wox/util/keyboard"
)

const gnomeAppIndicatorExtensionURL = "https://extensions.gnome.org/extension/615/appindicator-support/"
const waylandInputGroupHelpURL = "https://wox-launcher.github.io/Wox/guide/faq#wayland-double-modifier-hotkeys"

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
// This check only verifies the 'input' group (evdev read access). The 'uinput'
// group is checked separately when the user attempts to register a CapsLock
// combo hotkey, since uinput is only needed for CapsLock state restoration and
// not all users need it.
func checkLinuxInputGroup(ctx context.Context) (DoctorCheckResult, bool) {
	if !util.IsLinuxWaylandSession() {
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
			_ = browser.OpenURL(waylandInputGroupHelpURL, "")
		},
	}, true
}
