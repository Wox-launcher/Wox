package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>

// playSoundFileMac plays an audio file via NSSound. Returns 1 on success.
int playSoundFileMac(const char* filePath);
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

// playFile dispatches macOS-native playback via NSSound. NSSound playback is
// asynchronous and returns once the sound has been scheduled.
func playFile(ctx context.Context, name, path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	if C.playSoundFileMac(cPath) != 1 {
		return fmt.Errorf("NSSSound failed to play %s", path)
	}
	return nil
}
