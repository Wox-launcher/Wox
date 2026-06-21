//go:build linux

package plugin

import (
	"context"
	"os/exec"
	"strings"
	"wox/util"
	"wox/util/browser"
)

const gnomeAppIndicatorExtensionURL = "https://extensions.gnome.org/extension/615/appindicator-support/"

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
