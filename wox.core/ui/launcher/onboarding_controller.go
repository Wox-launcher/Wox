package launcher

import (
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// onboardingController owns first-run host and setup-session state.
type onboardingController struct {
	windows                             *woxui.WindowManager
	runOnUI                             func(string, func()) error
	fontFamily                          func() string
	primaryOwner                        func() *onboardingController
	window                              func() *woxui.Window
	language                            *languageCatalog
	buildOnboarding                     func(woxui.FrameInfo) woxwidget.Widget
	onOnboardingWindowFocus             func(woxui.FocusEvent)
	hotkeyRecordingUsesOnboardingWindow func() bool
	onOnboardingWindowKey               func(woxui.KeyEvent) bool
	onOnboardingWindowClosed            func()
	onboardingView                      *woxui.ManagedWindow
	onboardingHost                      *woxwidget.Host
	onboardingOpen                      bool
	onboardingStep                      int
	onboardingChoice                    string
	onboardingChoiceAnchor              woxui.Rect
	onboardingPermission                contract.MacOSPermissionStatus
	onboardingLoading                   bool
	onboardingError                     string
	onboardingQueryHotkey               *onboardingQueryHotkeyState
	onboardingPlugins                   onboardingPluginState
	onboardingTheme                     onboardingThemeState
	onboardingMigration                 onboardingMigrationState
	permissionFlowHost                  *macOSPermissionFlowHost
}

// bindOnboarding supplies rendering and feature handoffs to the native window owner.
func (a *App) bindOnboarding() {
	c := &a.onboarding
	c.windows = a.windows
	c.runOnUI = a.runOnUI
	c.fontFamily = func() string { return a.generalSettings.Data().AppFontFamily }
	c.primaryOwner = func() *onboardingController {
		if !a.isPrimary && a.primary != nil {
			return &a.primary.onboarding
		}
		return nil
	}
	c.window = func() *woxui.Window { return a.window }
	c.language = &a.language
	c.buildOnboarding = a.buildOnboarding
	c.onOnboardingWindowFocus = a.onOnboardingWindowFocus
	c.hotkeyRecordingUsesOnboardingWindow = a.hotkeyRecordingUsesOnboardingWindow
	c.onOnboardingWindowKey = a.onOnboardingWindowKey
	c.onOnboardingWindowClosed = a.onOnboardingWindowClosed
}
