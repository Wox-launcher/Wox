package ui

import (
	"context"
	"math"
	"wox/setting"
)

const (
	densityQueryBoxBaseHeight   = 55
	densityResultItemBaseHeight = 50
	densityToolbarBaseHeight    = 40
)

func currentUiDensity(ctx context.Context) setting.UiDensity {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	return setting.NormalizeUiDensity(string(woxSetting.UiDensity.Get()))
}

func scaledDensityHeight(baseHeight int, density setting.UiDensity) int {
	// Core window estimates and UI rendering share setting.UiDensityScale. If
	// only one side changes, compact/comfortable windows can be mispositioned or clipped.
	return int(math.Round(float64(baseHeight) * setting.UiDensityMultiplier(density)))
}

// DensityQueryBoxBaseHeight returns the scaled query-box content height used by
// backend window placement. The previous hard-coded 55px estimate only matched
// the normal UI, so keeping this helper beside other UI sizing constants keeps
// tray, hotkey, and explorer placement aligned with the selected density.
func DensityQueryBoxBaseHeight(ctx context.Context) int {
	return scaledDensityHeight(densityQueryBoxBaseHeight, currentUiDensity(ctx))
}

// DensityResultItemBaseHeight returns the content-only result row height. Theme
// padding stays outside density scaling so user themes keep their explicit
// spacing while the core row body follows compact/normal/comfortable sizing.
func DensityResultItemBaseHeight(ctx context.Context) int {
	return scaledDensityHeight(densityResultItemBaseHeight, currentUiDensity(ctx))
}

// DensityToolbarHeight returns the scaled launcher toolbar height. The toolbar
// uses the same base as action rows, but keeping a separate helper makes call
// sites state whether they are estimating the window toolbar or an action item.
func DensityToolbarHeight(ctx context.Context) int {
	return scaledDensityHeight(densityToolbarBaseHeight, currentUiDensity(ctx))
}
