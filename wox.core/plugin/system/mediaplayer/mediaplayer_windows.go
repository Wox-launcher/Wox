package mediaplayer

/*
#cgo LDFLAGS: -lruntimeobject -lole32
#include <stdlib.h>
#include "mediaplayer_windows.h"
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
	"wox/plugin"
)

var mediaRetriever = &WindowsRetriever{}

type WindowsRetriever struct {
	api plugin.API
}

func (w *WindowsRetriever) UpdateAPI(api plugin.API) {
	w.api = api
}

func (w *WindowsRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	info := C.wox_get_media_info()
	defer C.wox_free_media_info(&info)

	if info.error != nil {
		return nil, fmt.Errorf("%s", C.GoString(info.error))
	}
	if info.has_media == 0 {
		return nil, nil
	}

	mediaInfo := &MediaInfo{
		Title:       C.GoString(info.title),
		Artist:      C.GoString(info.artist),
		Album:       C.GoString(info.album),
		Duration:    int64(info.duration),
		Position:    int64(info.position),
		State:       parseWindowsPlaybackState(int(info.playback_status)),
		AppName:     C.GoString(info.app_name),
		AppBundleID: C.GoString(info.app_id),
	}
	if info.artwork != nil && info.artwork_len > 0 {
		mediaInfo.Artwork = C.GoBytes(unsafe.Pointer(info.artwork), info.artwork_len)
	}
	return mediaInfo, nil
}

func (w *WindowsRetriever) ControlMedia(ctx context.Context, command string) error {
	cCommand := C.CString(command)
	defer C.free(unsafe.Pointer(cCommand))

	var cError *C.char
	ok := C.wox_control_media(cCommand, &cError)
	if cError != nil {
		defer C.wox_free_string(cError)
	}
	if ok == 0 {
		if cError != nil {
			return fmt.Errorf("%s", C.GoString(cError))
		}
		return fmt.Errorf("Windows media control %s was not accepted", command)
	}
	return nil
}

func (w *WindowsRetriever) TogglePlayPause(ctx context.Context) error {
	return w.ControlMedia(ctx, mediaControlToggle)
}

func parseWindowsPlaybackState(status int) PlaybackState {
	switch status {
	case 4:
		return PlaybackStatePlaying
	case 5:
		return PlaybackStatePaused
	case 3:
		return PlaybackStateStopped
	default:
		return PlaybackStateUnknown
	}
}
