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

	"golang.design/x/hotkey/mainthread"
)

func SwitchInputMethodABC() error {
	abcInputMethodID := "com.apple.keylayout.ABC"

	resultChan := make(chan string)
	errorChan := make(chan error)

	mainthread.Call(func() {
		defer util.GoRecover(context.Background(), "switch input method panic", func(err error) {
			errorChan <- err
		})

		inputMethod := C.GoString(C.getCurrentInputMethod())
		if inputMethod == "" {
			errorChan <- errors.New("failed to get current input method")
			return
		}

		if inputMethod == abcInputMethodID {
			resultChan <- inputMethod
			return
		}

		inputMethodIDStr := C.CString(abcInputMethodID)
		defer C.free(unsafe.Pointer(inputMethodIDStr))
		C.switchInputMethod(inputMethodIDStr)

		resultChan <- abcInputMethodID
	})

	select {
	case <-resultChan:
		return nil
	case err := <-errorChan:
		return err
	}
}
