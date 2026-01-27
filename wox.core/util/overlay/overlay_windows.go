//go:build windows

package overlay

/*
#cgo LDFLAGS: -luser32 -lgdi32 -ldwmapi -luxtheme -lmsimg32 -lole32

extern void overlayClickCallbackCGO();
extern void explorerActivationCallbackCGO(int x, int y, int width, int height);

void showExplorerHint(int x, int y, int width, int height, const char* message, const unsigned char* bgra, int iconWidth, int iconHeight);
void hideExplorerHint();
void startAppActivationListener();
void stopAppActivationListener();
*/
import "C"
import (
	"unsafe"
)

var overlayClickCallback OverlayClickCallback
var explorerActivationCallback func(windowFrame Rect)

//export overlayClickCallbackCGO
func overlayClickCallbackCGO() {
	if overlayClickCallback != nil {
		overlayClickCallback()
	}
}

//export explorerActivationCallbackCGO
func explorerActivationCallbackCGO(x C.int, y C.int, width C.int, height C.int) {
	if explorerActivationCallback != nil {
		explorerActivationCallback(Rect{
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

	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cMessage))

	var cIconData *C.uchar
	var cIconWidth, cIconHeight C.int
	if len(iconData) > 0 {
		// Assume square icon, calculate dimensions from BGRA data
		// For 32x32 icon: 32*32*4 = 4096 bytes
		pixelCount := len(iconData) / 4
		size := int(float64(pixelCount) + 0.5) // sqrt approximation
		if size*size*4 == len(iconData) {
			cIconData = (*C.uchar)(unsafe.Pointer(&iconData[0]))
			cIconWidth = C.int(size)
			cIconHeight = C.int(size)
		}
	}

	// Calculate bottom-right position for hint window
	hintWidth := 400
	hintHeight := 60
	hintX := windowFrame.X + windowFrame.Width - hintWidth - 10
	hintY := windowFrame.Y + windowFrame.Height - hintHeight - 10

	C.showExplorerHint(
		C.int(hintX),
		C.int(hintY),
		C.int(hintWidth),
		C.int(hintHeight),
		cMessage,
		cIconData,
		cIconWidth,
		cIconHeight,
	)
}

// HideExplorerHint hides the currently displayed explorer hint overlay.
func HideExplorerHint() {
	C.hideExplorerHint()
}

// StartAppActivationListener starts listening for app activation events.
func StartAppActivationListener(onExplorerActivated func(windowFrame Rect)) {
	explorerActivationCallback = onExplorerActivated
	C.startAppActivationListener()
}

// StopAppActivationListener stops listening for app activation events.
func StopAppActivationListener() {
	C.stopAppActivationListener()
	explorerActivationCallback = nil
}
