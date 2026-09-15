package component

// Settings control metrics define the ordinary control and row size tiers.
const (
	// SettingsCompactControlHeight is reserved for dense table and toolbar actions.
	SettingsCompactControlHeight = float32(28)
	SettingsControlHeight        = float32(32)
	SettingsSearchHeight         = float32(40)
	SettingsRowHeight            = float32(64)
	SettingsSwitchWidth          = float32(36)
	// SettingsChoiceControlWidth is the shared trailing slot for switches and dropdowns.
	SettingsChoiceControlWidth = float32(200)
	// SettingsPageHorizontalInset is the left and right padding of ordinary Settings pages.
	SettingsPageHorizontalInset = float32(40)
	// SettingsPageFormMaxWidth caps the reading column on form-style Settings pages.
	SettingsPageFormMaxWidth = float32(700)
	SettingsRailMinWidth     = float32(220)
	SettingsRailMaxWidth     = float32(220)
	// SettingsWindowWidth leaves room for the plugin list and detail panes.
	SettingsWindowWidth  = float32(1100)
	SettingsWindowHeight = float32(760)
	// Settings catalog lists share one column so plugin and theme pages stay aligned.
	SettingsCatalogListMinWidth   = float32(220)
	SettingsCatalogListMaxWidth   = float32(250)
	SettingsCatalogDividerGutter  = float32(21)
	settingsCatalogListWidthRatio = float32(0.30)
	SettingsNavItemHeight         = float32(40)
	SettingsNavGroupHeight        = float32(28)
	SettingsNavItemGap            = float32(4)
	SettingsNavGroupLead          = float32(8)
	// SettingsSwitchOffTrackAlpha keeps an off switch visible on the glass canvas.
	SettingsSwitchOffTrackAlpha = uint8(120)
)

// SettingsPageFrameWidth is the page width that exactly fits the form column.
func SettingsPageFrameWidth() float32 {
	return SettingsPageFormMaxWidth + SettingsPageHorizontalInset*2
}

// SettingsRailWidth keeps the navigation column stable around the default window.
func SettingsRailWidth(windowWidth float32) float32 {
	return min(SettingsRailMaxWidth, max(SettingsRailMinWidth, windowWidth-SettingsPageFrameWidth()))
}

// SettingsCatalogListWidth sizes the shared plugin and theme catalog column.
func SettingsCatalogListWidth(contentWidth float32) float32 {
	width := contentWidth * settingsCatalogListWidthRatio
	return min(SettingsCatalogListMaxWidth, max(SettingsCatalogListMinWidth, float32(int(width+0.5))))
}
