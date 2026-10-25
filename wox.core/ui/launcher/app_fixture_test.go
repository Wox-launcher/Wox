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
	app.bindControllers()
	return app
}
