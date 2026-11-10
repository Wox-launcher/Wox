//go:build linux && cgo

package screen

import "fmt"

// getKDEMouseScreen maps the compositor output to GTK's logical workarea, including panel insets.
func getKDEMouseScreen() (Size, string, error) {
	display, err := GetKWinActiveDisplay()
	if err != nil {
		return Size{}, "", err
	}
	// GTK's integer scale can differ from Plasma's fractional scale. Locate the
	// output by its logical center, never by scaled desktop origins or image pixels.
	logical := display.Logical
	size, err := getScreenGtkAtPoint(logical.X+logical.Width/2, logical.Y+logical.Height/2)
	if err != nil {
		return Size{}, "", err
	}
	return size, fmt.Sprintf("source=kwin output=%s screen=%d,%d %dx%d", display.Name, size.X, size.Y, size.Width, size.Height), nil
}
