package launcher

import (
	"context"
	"fmt"
	"runtime"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	"wox/util"
	macospermission "wox/util/overlay/macos_permission"
	"wox/util/permission"
)

type macOSPermissionFlowHost struct {
	onboarding         *woxui.ManagedWindow
	settings           *woxui.ManagedWindow
	launcherWasVisible bool
	show               showAppParams
	permissionType     permission.MacOSPermissionType
}

func macOSPermissionFlowTitleKey(permissionType string) string {
	switch permission.MacOSPermissionType(permissionType) {
	case permission.MacOSPermissionAccessibility:
		return "onboarding_permission_accessibility_title"
	case permission.MacOSPermissionFullDiskAccess:
		return "onboarding_permission_disk_title"
	default:
		return "onboarding_permissions_title"
	}
}

// openMacOSPermissionFlow hides Wox chrome, opens System Settings, and shows the drag-to-authorize panel.
func (a *App) openMacOSPermissionFlow(permissionType string) error {
	targetPermission := permission.MacOSPermissionType(permissionType)
	if !permission.IsValidMacOSPermissionType(targetPermission) {
		return fmt.Errorf("invalid macOS permission type: %s", permissionType)
	}
	if runtime.GOOS != "darwin" {
		return nil
	}

	var launcherWasVisible bool
	if err := a.runOnUI("hide host for macOS permission flow", func() {
		a.hideHostForMacOSPermissionFlow(targetPermission)
		if a.permissionFlowHost != nil {
			launcherWasVisible = a.permissionFlowHost.launcherWasVisible
		}
	}); err != nil {
		return err
	}
	if launcherWasVisible {
		if err := a.hideWindow(false); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("hide launcher for macOS permission flow: %v", err))
		}
	}
	util.GetLogger().Info(context.Background(), "open macOS permission flow: "+permissionType)
	err := macospermission.Open(macospermission.Options{
		PermissionType:    targetPermission,
		Title:             a.translate("i18n:" + macOSPermissionFlowTitleKey(permissionType)),
		RightInstruction:  a.translate("i18n:macos_permission_flow_drag_left_instruction"),
		BottomInstruction: a.translate("i18n:macos_permission_flow_drag_above_instruction"),
		ManualInstruction: a.translate("i18n:macos_permission_flow_manual_instruction"),
		CloseLabel:        a.translate("i18n:ui_close"),
		Theme:             a.palette.componentTheme(),
		LightAppearance:   !themeColorIsDark(a.palette.background),
		OnClosed: func() {
			_ = a.runOnUI("restore host after macOS permission flow", a.restoreHostAfterMacOSPermissionFlow)
			a.refreshMacOSPermissionFlowStatus()
		},
		OnRefreshRequested: a.refreshMacOSPermissionFlowStatus,
	})
	if err != nil {
		_ = a.runOnUI("restore host after failed macOS permission flow", a.restoreHostAfterMacOSPermissionFlow)
	}
	return err
}

// refreshMacOSPermissionFlowStatus updates onboarding status and keyboard access without flashing the authorize button.
func (a *App) refreshMacOSPermissionFlowStatus() {
	if a == nil {
		return
	}
	util.Go(a.lifecycleCtx, "refresh macOS permission flow status", func() {
		a.loadOnboardingPermissionStatusWithLoading(false)
	})
}

// hideHostForMacOSPermissionFlow conceals Wox windows so the System Settings list stays droppable.
func (a *App) hideHostForMacOSPermissionFlow(permissionType permission.MacOSPermissionType) {
	if a.permissionFlowHost != nil {
		a.permissionFlowHost.permissionType = permissionType
		return
	}
	host := &macOSPermissionFlowHost{permissionType: permissionType}
	if a.onboardingOpen && a.onboardingView != nil && a.onboardingView.Lifecycle() == woxui.WindowLifecycleVisible {
		host.onboarding = a.onboardingView
		if err := a.onboardingView.Hide(); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("hide onboarding for macOS permission flow: %v", err))
		}
	}
	if a.settingsOpen && a.settingsView != nil && a.settingsView.Lifecycle() == woxui.WindowLifecycleVisible {
		host.settings = a.settingsView
		if err := a.settingsView.Hide(); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("hide settings for macOS permission flow: %v", err))
		}
	}
	if a.visible {
		host.launcherWasVisible = true
		host.show = a.show
	}
	a.permissionFlowHost = host
}

// macOSPermissionGranted reports whether the permission targeted by the active guide is ready.
func macOSPermissionGranted(permissionType permission.MacOSPermissionType, status contract.MacOSPermissionStatus) bool {
	switch permissionType {
	case permission.MacOSPermissionAccessibility:
		return status.Accessibility == string(permission.MacOSPermissionGranted)
	case permission.MacOSPermissionFullDiskAccess:
		return status.FullDiskAccess == string(permission.MacOSPermissionGranted)
	default:
		return false
	}
}

// restoreHostAfterMacOSPermissionFlow brings back the windows this flow hid, then refreshes Doctor if needed.
func (a *App) restoreHostAfterMacOSPermissionFlow() {
	host := a.permissionFlowHost
	a.permissionFlowHost = nil
	if host == nil {
		return
	}
	if host.onboarding != nil && a.onboardingOpen {
		if _, err := host.onboarding.Show(); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("restore onboarding after macOS permission flow: %v", err))
		}
	}
	if host.settings != nil && a.settingsOpen {
		if _, err := host.settings.Show(); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("restore settings after macOS permission flow: %v", err))
		}
	}
	if host.launcherWasVisible {
		show := host.show
		util.Go(a.lifecycleCtx, "restore launcher after macOS permission flow", func() {
			if err := a.showWindow(show); err != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("restore launcher after macOS permission flow: %v", err))
				return
			}
			if err := a.RefreshQuery(context.Background(), true); err != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("refresh query after macOS permission flow: %v", err))
			}
		})
	}
}
