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
	"context"
	"errors"
	"unsafe"
	"wox/util"
	"wox/util/mainthread"
)

func SwitchInputMethodABC() error {
	abcInputMethodID := "com.apple.keylayout.ABC"
	var switchErr error

	mainthread.Call(func() {
		// mainthread.Call is synchronous on Darwin, so return values must be stored
		// in outer variables instead of being sent through channels from inside the callback.
		defer util.GoRecover(context.Background(), "switch input method panic", func(err error) {
			switchErr = err
		})

		// Fix memory leak: properly free the C-allocated string
		cInputMethod := C.getCurrentInputMethod()
		if cInputMethod == nil {
			switchErr = errors.New("failed to get current input method")
			return
		}
		inputMethod := C.GoString(cInputMethod)
		C.free(unsafe.Pointer(cInputMethod))
		if inputMethod == "" {
			switchErr = errors.New("failed to get current input method")
			return
		}

		if inputMethod == abcInputMethodID {
			return
		}

		inputMethodIDStr := C.CString(abcInputMethodID)
		defer C.free(unsafe.Pointer(inputMethodIDStr))
		C.switchInputMethod(inputMethodIDStr)
	})

	return switchErr
}
