//go:build linux

package woxui

// Linux compositor blur (ext-background-effect-v1) is a whole-surface effect
// and cannot be scoped to a surface rectangle, so DisplayList.FloatingMaterial
// paints the surface tint and edge on the main surface there. A GL blur of the
// framebuffer under the rectangle would slot in behind this same API as
// floatingMaterialRendered, like the Windows Direct2D renderer does.
func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialPainted
}
