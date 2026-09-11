//go:build linux

package woxui

// Linux compositor blur (ext-background-effect-v1) is a whole-window desktop
// backdrop and cannot be scoped to a surface rectangle. The OpenGL renderer
// blurs its own back buffer under the surface instead (see
// wox_linux_window_floating_material), which samples the Go content beneath
// the panel but not the desktop behind the window; the window material already
// covers that.
func nativeFloatingMaterialMode() floatingMaterialMode {
	return floatingMaterialRendered
}
