package launcher

import (
	"context"

	"wox/common"
)

type currentThemeServices interface {
	CurrentTheme(context.Context, string) (common.Theme, error)
}

// appearanceController owns resolved launcher colors and logical density metrics.
// Window chrome changes are applied by the composition root after publication.
type appearanceController struct {
	services       currentThemeServices
	sessionID      string
	runOnUI        func(string, func()) error
	applyTheme     func(themeData)
	palette        uiPalette
	densityMetrics launcherDensityMetrics
}

// bindAppearance connects theme loading to coordinated native appearance updates.
func (a *App) bindAppearance() {
	a.appearance.services = a.services
	a.appearance.sessionID = a.sessionID
	a.appearance.runOnUI = a.runOnUI
	a.appearance.applyTheme = a.applyTheme
}
