//go:build !linux || !cgo

package keyboard

// InvokeGnomeHotkeyCallback is a no-op on non-Linux platforms.
// The real implementation is in listener_linux_gnome.go.
func InvokeGnomeHotkeyCallback(id string) {}
