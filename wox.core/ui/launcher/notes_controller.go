package launcher

import (
	"context"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// notesController owns the independent Notes windows and active-note routing.
// Each document controller shares only Notes services and presentation capabilities.
type notesController struct {
	isDestroyed          func() bool
	services             contract.NotesServices
	lifecycleCtx         context.Context
	windows              *woxui.WindowManager
	language             *languageCatalog
	tooltips             *tooltipController
	palette              func() uiPalette
	fontFamily           func() string
	densityScale         func() float32
	runOnUI              func(string, func()) error
	dispatchWindowKey    func(woxui.KeyEvent, *woxwidget.Host, string, func(), func(woxui.KeyEvent) bool) bool
	imageForViewport     func(woxImage, int) *woxui.Image
	releaseViewportImage func(woxImage, int)
	noteWindows          map[string]*notesWindowController
	activeNote           *notesWindowController
}

// bindNotes supplies document-independent native and rendering capabilities.
func (a *App) bindNotes() {
	c := &a.notes
	c.isDestroyed = a.isDestroyed
	c.services = a.services
	c.lifecycleCtx = a.lifecycleCtx
	c.windows = a.windows
	c.language = &a.language
	c.tooltips = &a.tooltips
	c.palette = func() uiPalette { return a.appearance.palette }
	c.fontFamily = func() string { return a.generalSettings.Data().AppFontFamily }
	c.densityScale = a.chatDensityScale
	c.runOnUI = a.runOnUI
	c.dispatchWindowKey = a.dispatchWindowKey
	c.imageForViewport = a.imageForViewport
	c.releaseViewportImage = a.releaseViewportImage
}

// dispose preserves the final save attempt on the UI thread before stopping autosave work.
func (c *notesController) dispose() {
	// A conflict save may replace the document ID and update the window registry.
	windows := make([]*notesWindowController, 0, len(c.noteWindows))
	for _, window := range c.noteWindows {
		windows = append(windows, window)
	}
	for _, window := range windows {
		if err := window.flush(); err != nil {
			util.GetLogger().Error(c.lifecycleCtx, "save note on shutdown: "+err.Error())
		}
	}
}
