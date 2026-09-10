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
	"sync"
	"time"
	"unsafe"
	"wox/plugin"
	"wox/util"

	"golang.org/x/sync/singleflight"
)

var mediaRetriever = &WindowsRetriever{}

// Burst queries (play → enter media, refresh overlapping a query) reuse the last
// SMTC snapshot instead of issuing another RequestAsync that can stall for seconds.
const windowsMediaCacheFreshFor = 500 * time.Millisecond

// WindowsRetriever reads SMTC sessions. Concurrent RequestAsync calls can hang
// for seconds, so lookups are single-flighted and the last snapshot is reused briefly.
type WindowsRetriever struct {
	api plugin.API

	fetchGroup singleflight.Group

	mu       sync.Mutex
	cached   *MediaInfo
	cachedAt time.Time
}

func (w *WindowsRetriever) UpdateAPI(api plugin.API) {
	w.api = api
}

func (w *WindowsRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	now := time.Now()
	if cached := w.snapshotCachedIfFresh(now, windowsMediaCacheFreshFor); cached != nil {
		return cached, nil
	}
	if err := ctx.Err(); err != nil {
		if cached := w.snapshotCached(now); cached != nil {
			return cached, nil
		}
		return nil, err
	}

	v, err, _ := w.fetchGroup.Do("current", func() (interface{}, error) {
		return w.fetchCurrentMedia(ctx)
	})
	if err != nil {
		if cached := w.snapshotCached(time.Now()); cached != nil {
			return cached, nil
		}
		return nil, err
	}
	info, _ := v.(*MediaInfo)
	return cloneMediaInfo(info), nil
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

// fetchCurrentMedia reads SMTC once and reuses cached artwork for the same track.
func (w *WindowsRetriever) fetchCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	cached := w.snapshotCached(time.Now())
	needArtwork := cached == nil || len(cached.Artwork) == 0

	start := time.Now()
	info, err := w.readNativeMedia(needArtwork)
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Windows SMTC GetCurrentMedia cost %s artwork=%v", elapsed, needArtwork))
	}
	if err != nil {
		return nil, err
	}
	if info == nil {
		w.storeCache(nil)
		return nil, nil
	}

	if cached != nil && sameMediaTrack(info, cached) && len(info.Artwork) == 0 {
		info.Artwork = append([]byte(nil), cached.Artwork...)
	} else if !needArtwork && cached != nil && !sameMediaTrack(info, cached) {
		// Track changed while we skipped artwork; fetch once so the new cover can appear.
		withArtwork, artworkErr := w.readNativeMedia(true)
		if artworkErr == nil && withArtwork != nil && sameMediaTrack(withArtwork, info) {
			info.Artwork = withArtwork.Artwork
		}
	}

	w.storeCache(info)
	return cloneMediaInfo(info), nil
}

func (w *WindowsRetriever) readNativeMedia(includeArtwork bool) (*MediaInfo, error) {
	flag := C.int(0)
	if includeArtwork {
		flag = 1
	}
	info := C.wox_get_media_info(flag)
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

func (w *WindowsRetriever) storeCache(info *MediaInfo) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cached = cloneMediaInfo(info)
	w.cachedAt = time.Now()
}

func (w *WindowsRetriever) snapshotCached(now time.Time) *MediaInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cached == nil {
		return nil
	}
	info := cloneMediaInfo(w.cached)
	if info.State == PlaybackStatePlaying {
		advancePlaybackPosition(info, now.Sub(w.cachedAt))
	}
	return info
}

func (w *WindowsRetriever) snapshotCachedIfFresh(now time.Time, maxAge time.Duration) *MediaInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cached == nil || now.Sub(w.cachedAt) >= maxAge {
		return nil
	}
	info := cloneMediaInfo(w.cached)
	if info.State == PlaybackStatePlaying {
		advancePlaybackPosition(info, now.Sub(w.cachedAt))
	}
	return info
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
