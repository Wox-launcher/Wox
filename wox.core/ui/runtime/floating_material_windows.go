//go:build windows

package woxui

// Windows has no per-region backdrop: DWM Acrylic and Mica apply to a whole
// HWND, so the launcher keeps painting an opaque panel wash there. A future
// implementation would blur the Direct2D back buffer under the panel rectangle
// (or host a child HWND with its own backdrop) behind this same API.
func nativeFloatingMaterialAvailable() bool {
	return false
}

// showFloatingMaterial succeeds as a no-op so layout code can call it unconditionally.
func (w *platformWindow) showFloatingMaterial(bounds Rect, style FloatingMaterialStyle) error {
	return nil
}

func (w *platformWindow) hideFloatingMaterial() error {
	return nil
}
