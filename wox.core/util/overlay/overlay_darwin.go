//go:build darwin

package overlay

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>

extern void overlayClickCallbackCGO();
extern void finderActivationCallbackCGO(int x, int y, int width, int height);

void showExplorerHint(int x, int y, int width, int height, const char* message, const unsigned char* iconData, int iconLen, void (*callback)());
void hideExplorerHint();
void startAppActivationListener(void (*callback)(int x, int y, int width, int height));
void stopAppActivationListener();
*/
import "C"
import (
	"unsafe"

	"golang.design/x/hotkey/mainthread"
)

var overlayClickCallback OverlayClickCallback
var finderActivationCallback func(windowFrame Rect)

//export overlayClickCallbackCGO
func overlayClickCallbackCGO() {
	if overlayClickCallback != nil {
		overlayClickCallback()
	}
}

//export finderActivationCallbackCGO
func finderActivationCallbackCGO(x C.int, y C.int, width C.int, height C.int) {
	if finderActivationCallback != nil {
		finderActivationCallback(Rect{
			X:      int(x),
			Y:      int(y),
			Width:  int(width),
			Height: int(height),
		})
	}
}

// ShowExplorerHint displays an explorer hint overlay window.
func ShowExplorerHint(windowFrame Rect, message string, iconData []byte, onClick OverlayClickCallback) {
	overlayClickCallback = onClick

	mainthread.Call(func() {
		cMessage := C.CString(message)
		defer C.free(unsafe.Pointer(cMessage))

		var cIconData *C.uchar
		var cIconLen C.int
		if len(iconData) > 0 {
			cIconData = (*C.uchar)(unsafe.Pointer(&iconData[0]))
			cIconLen = C.int(len(iconData))
		}

		C.showExplorerHint(
			C.int(windowFrame.X),
			C.int(windowFrame.Y),
			C.int(windowFrame.Width),
			C.int(windowFrame.Height),
			cMessage,
			cIconData,
			cIconLen,
			nil,
		)
	})
}

// HideExplorerHint hides the currently displayed explorer hint overlay.
func HideExplorerHint() {
	mainthread.Call(func() {
		C.hideExplorerHint()
	})
}

// StartAppActivationListener starts listening for app activation events.
func StartAppActivationListener(onFinderActivated func(windowFrame Rect)) {
	finderActivationCallback = onFinderActivated

	mainthread.Call(func() {
		C.startAppActivationListener(nil)
	})
}

// StopAppActivationListener stops listening for app activation events.
func StopAppActivationListener() {
	mainthread.Call(func() {
		C.stopAppActivationListener()
	})
	finderActivationCallback = nil
}
