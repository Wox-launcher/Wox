package overlay

/*
#cgo CFLAGS: -DUNICODE -D_UNICODE
#include <stdlib.h>
#include <stdbool.h>

typedef struct {
    char* name;
    char* title;
    char* message;
    unsigned char* iconData;
    int iconLen;
    bool closable;
    int stickyWindowPid;
    int anchor;
    float offsetX;
    float offsetY;
    float width;
    float height;
} OverlayOptions;

void ShowOverlay(OverlayOptions opts);
void CloseOverlay(char* name);
void overlayClickCallbackCGO(char* name);
*/
import "C"

// Stub for click callbacks on Windows
// In reality, you'd implement the same map logic as Darwin.
// For now, to match the interface:

func Show(opts OverlayOptions) {
	// Basic Stub for Windows compilation
}

func Close(name string) {
	// Stub
}

//export overlayClickCallbackCGO
func overlayClickCallbackCGO(cName *C.char) {
	// Callback
}
