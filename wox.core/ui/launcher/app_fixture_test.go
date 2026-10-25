package launcher

// bindTestApp wires domain capabilities while preserving the fixture's explicit state.
// Unlike newApp, it does not subscribe to process services or initialize native windows.
func bindTestApp(app *App) *App {
	if app.sharedEdit == nil {
		app.sharedEdit = newSharedEditState()
	}
	if app.generalSettings == nil {
		app.generalSettings = newGeneralSettingsController(CommonDeps{}, app.sharedEdit)
	}
	if app.appearanceSettings == nil {
		app.appearanceSettings = newAppearanceSettingsController(CommonDeps{})
	}
	if app.pluginSettings == nil {
		app.pluginSettings = newPluginSettingsController(CommonDeps{})
	}
	if app.cloudSettings == nil {
		app.cloudSettings = newCloudSettingsController(CommonDeps{})
	}
	if app.themeSettings == nil {
		app.themeSettings = newThemeSettingsController(CommonDeps{})
	}
	if app.settingsSearch == nil {
		app.settingsSearch = newSettingsSearchController(CommonDeps{})
	}
	if app.dataSettings == nil {
		app.dataSettings = newDataSettingsController(CommonDeps{})
	}
	if app.runtimeSettings == nil {
		app.runtimeSettings = newRuntimeSettingsController(CommonDeps{})
	}
	if app.aiSettings == nil {
		app.aiSettings = newAISettingsController(CommonDeps{})
	}
	app.bindControllers()
	return app
}
