package pluginwindow

import (
	"encoding/json"
	"wox/plugin"
	"wox/util/overlay"
)

// themeChanged refreshes native material and paint even for hidden windows.
// Plugin windows are managed independently of the generic overlay registry.
func (s *session) themeChanged() {
	if s.closed {
		return
	}
	if s.window != nil {
		_ = s.window.SetAppearance(!overlay.CurrentThemeChrome().Light)
		_ = s.window.Invalidate()
	}
	s.themePending = true
	s.flushThemeChange()
}

// flushThemeChange delays notifications until the current document is ready.
// Multiple changes while HTML loads collapse to its latest surface appearance.
func (s *session) flushThemeChange() {
	if s.closed || !s.ready || s.loadFailed || !s.themePending || s.document == "" {
		return
	}
	s.themePending = false
	data, _ := json.Marshal(plugin.WindowThemeChangedData{Dark: !overlay.CurrentThemeChrome().Light})
	s.emit(plugin.WindowEvent{Type: plugin.WindowEventThemeChanged, DocumentID: s.document, Data: data, CoalesceKey: "theme"})
}
