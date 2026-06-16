//go:build darwin

package mouse

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdbool.h>

#import <Cocoa/Cocoa.h>

typedef struct {
    double x;
    double y;
    bool ok;
} MousePosition;

static MousePosition getCurrentMousePosition() {
    MousePosition position = {0};
    @autoreleasepool {
        NSScreen *mainScreen = [NSScreen mainScreen];
        if (mainScreen == nil) {
            return position;
        }

        NSPoint mouseLocation = [NSEvent mouseLocation];
        position.x = mouseLocation.x;
        position.y = mainScreen.frame.size.height - mouseLocation.y;
        position.ok = true;
        return position;
    }
}
*/
import "C"

// CurrentPosition returns the pointer position as desktop top-left coordinates.
func CurrentPosition() (Point, bool) {
	position := C.getCurrentMousePosition()
	if !bool(position.ok) {
		return Point{}, false
	}

	return Point{
		X: float64(position.x),
		Y: float64(position.y),
	}, true
}
