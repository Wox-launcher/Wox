package launcher

import (
	"sync/atomic"

	"wox/setting"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// actionPanelController owns filtering, selection, and panel purpose. Execution
// is dispatched through one semantic callback to the selected action's owner.
type actionPanelController struct {
	window                   func() *woxui.Window
	host                     func() *woxwidget.Host
	language                 *languageCatalog
	resultActionPanelEntries func() []actionPanelEntry
	runningVersion           func() string
	actionPanelHotkey        func() string
	navigationBindings       func() setting.NavigationBindingMode
	usePinYin                func() bool
	toggleAboutMenu          func()
	activateActionPanelEntry func(actionPanelEntry)
	beforeOpen               func()
	dismissAboutMenuTooltip  func()
	restoreQueryTextInput    func()
	applyWindowBounds        func() error
	actionPanel              bool
	actionPanelPurpose       actionPanelPurpose
	aboutMenuError           string
	aboutMenuTooltipRevision atomic.Uint64
	actionSelected           int
	actionSelectionKey       string
	actionFilter             *woxui.TextEditor
}

// bindActions wires action sources and cross-feature input handoffs.
func (a *App) bindActions() {
	a.actions.window = func() *woxui.Window { return a.window }
	a.actions.host = func() *woxwidget.Host { return a.host }
	a.actions.language = &a.language
	a.actions.resultActionPanelEntries = a.resultActionPanelEntries
	a.actions.runningVersion = a.runningVersion
	a.actions.actionPanelHotkey = a.actionPanelHotkey
	a.actions.navigationBindings = func() setting.NavigationBindingMode { return a.generalSettings.Data().NavigationBindings }
	a.actions.usePinYin = a.usePinYin
	a.actions.toggleAboutMenu = a.toggleAboutMenu
	a.actions.activateActionPanelEntry = a.activateActionPanelEntry
	a.actions.beforeOpen = func() {
		a.dismissAboutMenuTooltip()
		a.quickSelect.stopQuickSelectLocked()
		a.forms.form = nil
	}
	a.actions.dismissAboutMenuTooltip = a.dismissAboutMenuTooltip
	a.actions.restoreQueryTextInput = a.restoreQueryTextInput
	a.actions.applyWindowBounds = a.applyWindowBounds
}
