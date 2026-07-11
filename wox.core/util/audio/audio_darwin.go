package audio

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AVFoundation
#include <stdlib.h>

// prepareSoundFileMac loads and prepares an audio file. Returns 1 on success.
int prepareSoundFileMac(const char* filePath);

// playSoundFileMac plays a prepared audio file. Returns 1 on success.
int playSoundFileMac(const char* filePath);

extern void audioPlaybackFinished(char* filePath, int success);
extern void audioPlaybackDecodeFailed(char* filePath, char* message);
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
	"wox/util"
	"wox/util/mainthread"
)

// prepareFile preloads a clip with AVAudioPlayer so playback does not need to
// parse the file or initialize the player at the dictation start boundary.
func prepareFile(ctx context.Context, path string) error {
	var result C.int
	mainthread.Call(func() {
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
		result = C.prepareSoundFileMac(cPath)
	})
	if result != 1 {
		return fmt.Errorf("AVAudioPlayer failed to prepare %s", path)
	}
	return nil
}

// playFile dispatches macOS-native playback via AVAudioPlayer. The player is
// retained and prepared by its path, avoiding unreliable first-use playback
// during audio capture startup.
func playFile(ctx context.Context, path string) error {
	var result C.int
	mainthread.Call(func() {
		cPath := C.CString(path)
		defer C.free(unsafe.Pointer(cPath))
		result = C.playSoundFileMac(cPath)
	})
	util.GetLogger().Info(ctx, fmt.Sprintf("audio: playback requested result=%d path=%s", int(result), path))
	if result != 1 {
		return fmt.Errorf("AVAudioPlayer failed to play %s", path)
	}
	return nil
}

//export audioPlaybackFinished
func audioPlaybackFinished(path *C.char, success C.int) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf("audio: playback finished success=%t path=%s", success == 1, C.GoString(path)))
}

//export audioPlaybackDecodeFailed
func audioPlaybackDecodeFailed(path *C.char, message *C.char) {
	util.GetLogger().Warn(context.Background(), fmt.Sprintf("audio: playback decode failed path=%s error=%s", C.GoString(path), C.GoString(message)))
}
