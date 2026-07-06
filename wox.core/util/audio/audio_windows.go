package audio

/*
#cgo LDFLAGS: -lwinmm
#include <windows.h>
#include <mmsystem.h>

// playSoundFileWin plays a wav file synchronously via PlaySoundW.
// Returns 1 on success, 0 on failure.
int playSoundFileWin(const wchar_t* filePath) {
    BOOL ok = PlaySoundW(filePath, NULL, SND_FILENAME | SND_NODEFAULT);
    return ok ? 1 : 0;
}
*/
import "C"

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"
)

// playFile uses Windows PlaySoundW (winmm). SND_FILENAME plays from file path;
// playback is asynchronous when combined with SND_ASYNC, but we use the
// default synchronous call inside a goroutine so it doesn't block the caller.
func playFile(ctx context.Context, name, path string) error {
	go func() {
		wpath, _ := syscall.UTF16PtrFromString(path)
		if C.playSoundFileWin((*C.wchar_t)(unsafe.Pointer(wpath))) != 1 {
			logErr(ctx, name, fmt.Errorf("PlaySoundW failed for %s", path))
		}
	}()
	return nil
}
