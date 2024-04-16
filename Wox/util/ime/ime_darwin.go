package ime

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

char* getCurrentInputMethod();
void switchInputMethod(const char *inputMethodID);
*/
import "C"
import (
	"errors"
	"unsafe"
)

func SwitchInputMethodABC() error {
	abcInputMethodID := "com.apple.keylayout.ABC"

	inputMethod := C.GoString(C.getCurrentInputMethod())
	if inputMethod == "" {
		return errors.New("failed to get current input method")
	}

	if inputMethod == abcInputMethodID {
		return nil
	}

	inputMethodIDStr := C.CString(abcInputMethodID)
	defer C.free(unsafe.Pointer(inputMethodIDStr))
	C.switchInputMethod(inputMethodIDStr)

	return nil
}
