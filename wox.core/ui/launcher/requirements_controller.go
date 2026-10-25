package launcher

import (
	"context"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// requirementController owns its preview form, validation, and asynchronous save generation.
type requirementController struct {
	services                      pluginSettingsWriter
	sessionID                     string
	lifecycleCtx                  context.Context
	querySession                  *queryController
	runOnUI                       func(string, func()) error
	window                        func() *woxui.Window
	updateFormTextInput           func(bool)
	restoreQueryTextInput         func()
	applyWindowBounds             func() error
	setQuery                      func(plainQuery)
	sendCurrentQuery              func() error
	host                          func() *woxwidget.Host
	aiSettings                    func() *aiSettingsController
	loadAIModels                  func()
	openRequirementFormTable      func(int)
	translateFormTableFieldErrors func(map[string]string) map[string]string
	requirementForm               *requirementFormState
}

// bindRequirements connects query restart and form input capabilities.
func (a *App) bindRequirements() {
	c := &a.requirements
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.querySession = &a.querySession
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.updateFormTextInput = a.updateFormTextInput
	c.restoreQueryTextInput = a.restoreQueryTextInput
	c.applyWindowBounds = a.applyWindowBounds
	c.setQuery = a.setQuery
	c.sendCurrentQuery = a.sendCurrentQuery
	c.host = func() *woxwidget.Host { return a.host }
	c.aiSettings = func() *aiSettingsController { return a.aiSettings }
	c.loadAIModels = a.loadAIModels
	c.openRequirementFormTable = a.openRequirementFormTable
	c.translateFormTableFieldErrors = a.formTables.translateFormTableFieldErrors
}

// pluginSettingsWriter is the shared save capability used by query guidance forms.
type pluginSettingsWriter interface {
	UpdatePluginSettings(context.Context, string, string, map[string]string) error
}
