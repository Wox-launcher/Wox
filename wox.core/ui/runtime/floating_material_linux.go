//go:build linux

package woxui

// Linux compositor blur (ext-background-effect-v1) is a whole-surface effect
// and cannot be scoped to a panel rectangle, so the launcher keeps painting an
// opaque panel wash. A GL blur of the launcher framebuffer under the panel
// would slot in behind this same API.
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
