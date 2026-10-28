//go:build linux

package screenshot

/*
#include <stdint.h>
int32_t wox_screenshot_cursor_position(float *x, float *y);
*/
import "C"

// linuxX11ScreenshotCursorPosition preserves root-window pointer queries for the existing X11 path.
func linuxX11ScreenshotCursorPosition() *Point {
	var x, y C.float
	if C.wox_screenshot_cursor_position(&x, &y) != 0 {
		return nil
	}
	return &Point{X: float32(x), Y: float32(y)}
}
