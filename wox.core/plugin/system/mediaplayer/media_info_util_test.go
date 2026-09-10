package mediaplayer

import (
	"testing"
	"time"
)

func TestCloneMediaInfoCopiesArtwork(t *testing.T) {
	original := &MediaInfo{Title: "song", Artwork: []byte{1, 2, 3}}
	cloned := cloneMediaInfo(original)
	if cloned == original {
		t.Fatal("cloneMediaInfo returned the same pointer")
	}
	cloned.Artwork[0] = 9
	if original.Artwork[0] != 1 {
		t.Fatal("cloneMediaInfo did not copy artwork bytes")
	}
}

func TestSameMediaTrack(t *testing.T) {
	left := &MediaInfo{Title: "a", Artist: "b", Album: "c", AppBundleID: "app"}
	right := &MediaInfo{Title: "a", Artist: "b", Album: "c", AppBundleID: "app"}
	if !sameMediaTrack(left, right) {
		t.Fatal("expected matching tracks")
	}
	right.Artist = "other"
	if sameMediaTrack(left, right) {
		t.Fatal("expected different tracks")
	}
	if sameMediaTrack(left, nil) || sameMediaTrack(nil, left) {
		t.Fatal("nil should not match a track")
	}
	if !sameMediaTrack(nil, nil) {
		t.Fatal("two nil tracks should match")
	}
}

func TestAdvancePlaybackPosition(t *testing.T) {
	info := &MediaInfo{Position: 10, Duration: 12}
	advancePlaybackPosition(info, 2500*time.Millisecond)
	if info.Position != 12 {
		t.Fatalf("position=%d, want 12", info.Position)
	}

	info.Position = 3
	info.Duration = 0
	advancePlaybackPosition(info, time.Second)
	if info.Position != 4 {
		t.Fatalf("position=%d, want 4", info.Position)
	}
}
