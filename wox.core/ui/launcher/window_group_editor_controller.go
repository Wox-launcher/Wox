package launcher

import (
	"context"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

// windowGroupEditorController drives the Window Manager plugin's workspace group editor,
// which replaces the generic row form inside the shared table editor. Its per-edit state
// lives on formTableEditorState.windowGroupEditor so it is discarded with the table
// editor; this controller only holds the collaborators needed to edit and save it.
type windowGroupEditorController struct {
	services     contract.WindowManagerSettingsServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	language     *languageCatalog
	formTables   *formTableController

	pluginSettings  func() *pluginSettingsController
	hotkeySettings  func() *hotkeySettingsController
	imageForTint    func(woxImage, *woxui.Color, int) *woxui.Image
	imageForSurface func(woxImage, int, woxui.Color) *woxui.Image
}

// bindWindowGroupEditor wires the group editor to the table editor, plugin form and app catalog.
func (a *App) bindWindowGroupEditor() {
	c := &a.windowGroups
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.language = &a.language
	c.formTables = &a.formTables
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.hotkeySettings = func() *hotkeySettingsController { return a.hotkeySettings }
	c.imageForTint = a.imageForTint
	c.imageForSurface = a.imageForSurface
}
