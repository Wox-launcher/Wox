//go:build linux && cgo

package screen

import "fmt"

// getHyprlandMouseScreen maps the compositor cursor into GTK's logical workarea.
func getHyprlandMouseScreen() (Size, string, error) {
	x, y, err := getHyprlandCursorPosition()
	if err != nil {
		return Size{}, "", fmt.Errorf("hyprland-ipc-failed err=%v", err)
	}
	size, err := getScreenGtkAtPoint(x, y)
	if err != nil {
		return Size{}, "", fmt.Errorf("hyprland-ipc-screen-failed pointer=%d,%d err=%v", x, y, err)
	}
	return size, fmt.Sprintf("source=hyprland-ipc pointer=%d,%d screen=%d,%d %dx%d", x, y, size.X, size.Y, size.Width, size.Height), nil
}
