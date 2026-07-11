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
	"wox/util"
	"wox/util/mainthread"
)

// playFile dispatches macOS-native playback via NSSound. NSSound must be
// created and played on the main thread for reliable audio output, so we
// dispatch via mainthread.Call.
func playFile(ctx context.Context, path string) error {
	var result C.int
	mainthread.Call(func() {
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
		result = C.playSoundFileMac(cPath)
	})
	util.GetLogger().Info(ctx, fmt.Sprintf("audio: playFile result=%d path=%s", int(result), path))
	if result != 1 {
		return fmt.Errorf("NSSound failed to play %s", path)
	}
	return nil
}
