package filesearch

import (
	"testing"
	"time"
)

func TestDecodeFeedCursorRejectsWrongFeedType(t *testing.T) {
	cursorText := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeUSN,
		UpdatedAt: time.Now().Add(-time.Hour).UnixMilli(),
		USN:       42,
	})

	if _, ok := decodeFeedCursor(cursorText, RootFeedTypeFSEvents); ok {
		t.Fatalf("expected mismatched feed type cursor to be rejected")
	}
}

func TestFeedCursorFreshnessUsesSafeWindow(t *testing.T) {
	now := time.Now()

	fresh := FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: now.Add(-23 * time.Hour).UnixMilli(),
		FSEventID: 123,
	}
	if !feedCursorFresh(fresh, now, defaultFeedCursorSafeWindow) {
		t.Fatalf("expected cursor inside safe window to be fresh")
	}

	expired := FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: now.Add(-25 * time.Hour).UnixMilli(),
		FSEventID: 456,
	}
	if feedCursorFresh(expired, now, defaultFeedCursorSafeWindow) {
		t.Fatalf("expected cursor outside safe window to expire")
	}
}

func mustEncodeFeedCursorForTest(t *testing.T, cursor FeedCursor) string {
	t.Helper()

	text, err := encodeFeedCursor(cursor)
	if err != nil {
		t.Fatalf("encode feed cursor: %v", err)
	}
	return text
}
