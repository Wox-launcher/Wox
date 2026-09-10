package mediaplayer

import (
	"testing"
	"time"
)

func TestWindowsRetrieverFreshCache(t *testing.T) {
	retriever := &WindowsRetriever{}
	retriever.storeCache(&MediaInfo{
		Title:    "陌生人",
		Position: 5,
		Duration: 20,
		State:    PlaybackStatePlaying,
		Artwork:  []byte{1, 2, 3},
	})

	fresh := retriever.snapshotCachedIfFresh(time.Now(), windowsMediaCacheFreshFor)
	if fresh == nil {
		t.Fatal("expected a fresh cache hit")
	}
	if fresh.Title != "陌生人" {
		t.Fatalf("title=%q", fresh.Title)
	}
	if len(fresh.Artwork) != 3 {
		t.Fatalf("artwork len=%d", len(fresh.Artwork))
	}

	stale := retriever.snapshotCachedIfFresh(time.Now().Add(windowsMediaCacheFreshFor+time.Millisecond), windowsMediaCacheFreshFor)
	if stale != nil {
		t.Fatal("expected a stale cache miss")
	}
}
