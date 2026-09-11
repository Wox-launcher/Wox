//go:build windows

package woxui

// Windows has no per-region backdrop: DWM Acrylic and Mica apply to a whole HWND.
// The Direct2D renderer blurs its own back buffer under the surface instead (see
// wox_renderer_floating_material), which samples the Go content beneath the panel
// but not the desktop behind the window; the window material already covers that.
func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialRendered
}
