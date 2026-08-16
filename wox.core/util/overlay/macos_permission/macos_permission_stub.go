//go:build !darwin

package macospermission

func openPermissionSettings(string) {}

func permissionSettingsWindow() (Rect, Rect, bool) { return Rect{}, Rect{}, false }

func permissionApplicationPath() string { return "" }
