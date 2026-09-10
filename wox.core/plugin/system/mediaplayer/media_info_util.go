package mediaplayer

import "time"

// cloneMediaInfo returns an independent copy so cached snapshots can be reused safely.
func cloneMediaInfo(info *MediaInfo) *MediaInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	if len(info.Artwork) > 0 {
		cloned.Artwork = append([]byte(nil), info.Artwork...)
	}
	return &cloned
}

// mediaTrackKey identifies a now-playing item so artwork can be reused across refreshes.
func mediaTrackKey(info *MediaInfo) string {
	if info == nil {
		return ""
	}
	return info.Title + "\x00" + info.Artist + "\x00" + info.Album + "\x00" + info.AppBundleID
}

func sameMediaTrack(a, b *MediaInfo) bool {
	if a == nil || b == nil {
		return a == b
	}
	return mediaTrackKey(a) == mediaTrackKey(b)
}

// advancePlaybackPosition estimates the current position from a cached snapshot
// so a fresh cache hit can still show a moving progress bar.
func advancePlaybackPosition(info *MediaInfo, elapsed time.Duration) {
	if info == nil || elapsed <= 0 {
		return
	}
	info.Position += int64(elapsed / time.Second)
	if info.Position < 0 {
		info.Position = 0
	}
	if info.Duration > 0 && info.Position > info.Duration {
		info.Position = info.Duration
	}
}
