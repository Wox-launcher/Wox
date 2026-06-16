package filesearch

import (
	"encoding/json"
	"time"
)

const defaultFeedCursorSafeWindow = 24 * time.Hour

type FeedCursor struct {
	FeedType  RootFeedType `json:"feed_type"`
	UpdatedAt int64        `json:"updated_at"`
	FSEventID uint64       `json:"fs_event_id,omitempty"`
	JournalID uint64       `json:"journal_id,omitempty"`
	USN       int64        `json:"usn,omitempty"`
	Volume    string       `json:"volume,omitempty"`
}

func encodeFeedCursor(cursor FeedCursor) (string, error) {
	if cursor.FeedType == "" {
		return "", nil
	}

	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func decodeFeedCursor(value string, feedType RootFeedType) (FeedCursor, bool) {
	if value == "" {
		return FeedCursor{}, false
	}

	var cursor FeedCursor
	if err := json.Unmarshal([]byte(value), &cursor); err != nil {
		return FeedCursor{}, false
	}
	if cursor.FeedType == "" || cursor.FeedType != feedType {
		return FeedCursor{}, false
	}
	return cursor, true
}

func feedCursorFresh(cursor FeedCursor, now time.Time, safeWindow time.Duration) bool {
	if cursor.UpdatedAt <= 0 {
		return false
	}
	if safeWindow <= 0 {
		safeWindow = defaultFeedCursorSafeWindow
	}
	return now.Sub(time.UnixMilli(cursor.UpdatedAt)) <= safeWindow
}
