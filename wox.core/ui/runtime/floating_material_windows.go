//go:build windows

package woxui

// Windows has no per-region backdrop: DWM Acrylic and Mica apply to a whole
// HWND, so DisplayList.FloatingMaterial paints the surface tint and edge on the
// main surface there. A future implementation would blur the Direct2D back
// buffer under the surface rectangle (or host a child HWND with its own
// backdrop) and reconcile it per frame like the macOS window does.
func nativeFloatingMaterialAvailable() bool {
	return false
}
