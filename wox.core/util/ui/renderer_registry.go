//go:build cgo

package ui

import "sync"

// activeRenderers maps windowId → *baseRenderer so C→Go callbacks
// (uiEventCallback, uiDarwinOnDraw) can find the right renderer without
// passing Go pointers through C. Maintained by NewNativeRenderer / Close.
var activeRenderers sync.Map

// findRenderer looks up a renderer by its native window ID.
// Returns nil if no renderer is registered for the given ID.
func findRenderer(windowID int32) *baseRenderer {
	val, ok := activeRenderers.Load(windowID)
	if !ok {
		return nil
	}
	r, _ := val.(*baseRenderer)
	return r
}