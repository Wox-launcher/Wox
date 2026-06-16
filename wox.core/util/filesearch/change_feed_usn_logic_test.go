package filesearch

import (
	"testing"
	"time"
)

func TestPrepareUSNVolumeRefreshUsesEarliestFreshCursorAndSchedulesRecovery(t *testing.T) {
	now := time.Now()
	journal := usnJournalState{
		Volume:    `C:\`,
		JournalID: 99,
		FirstUSN:  200,
		NextUSN:   900,
	}

	freshCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeUSN,
		UpdatedAt: now.Add(-time.Hour).UnixMilli(),
		JournalID: 99,
		USN:       450,
		Volume:    `C:\`,
	})
	expiredCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeUSN,
		UpdatedAt: now.Add(-26 * time.Hour).UnixMilli(),
		JournalID: 99,
		USN:       400,
		Volume:    `C:\`,
	})
	truncatedCursor := mustEncodeFeedCursorForTest(t, FeedCursor{
		FeedType:  RootFeedTypeUSN,
		UpdatedAt: now.Add(-time.Hour).UnixMilli(),
		JournalID: 99,
		USN:       150,
		Volume:    `C:\`,
	})

	prepared := prepareUSNVolumeRefresh([]RootRecord{
		{
			ID:         "root-fresh",
			Path:       `C:\fresh`,
			FeedType:   RootFeedTypeUSN,
			FeedCursor: freshCursor,
		},
		{
			ID:         "root-expired",
			Path:       `C:\expired`,
			FeedType:   RootFeedTypeUSN,
			FeedCursor: expiredCursor,
		},
		{
			ID:         "root-truncated",
			Path:       `C:\truncated`,
			FeedType:   RootFeedTypeUSN,
			FeedCursor: truncatedCursor,
		},
		{
			ID:        "root-recovered",
			Path:      `C:\recovered`,
			FeedType:  RootFeedTypeUSN,
			FeedState: RootFeedStateUnavailable,
		},
	}, journal, now, defaultFeedCursorSafeWindow)

	if prepared.startUSN != 450 {
		t.Fatalf("expected earliest fresh usn cursor 450, got %d", prepared.startUSN)
	}
	if len(prepared.roots) != 4 {
		t.Fatalf("expected all roots to remain assigned to the volume, got %d", len(prepared.roots))
	}
	if len(prepared.signals) != 3 {
		t.Fatalf("expected 3 recovery signals, got %d", len(prepared.signals))
	}

	byRoot := map[string]ChangeSignal{}
	for _, signal := range prepared.signals {
		byRoot[signal.RootID] = signal
	}

	if signal, ok := byRoot["root-expired"]; !ok || signal.Kind != ChangeSignalKindRequiresRootReconcile {
		t.Fatalf("expected expired root to require root reconcile, got %#v ok=%t", signal, ok)
	}
	if signal, ok := byRoot["root-truncated"]; !ok || signal.Kind != ChangeSignalKindRequiresRootReconcile {
		t.Fatalf("expected truncated root to require root reconcile, got %#v ok=%t", signal, ok)
	}
	if signal, ok := byRoot["root-recovered"]; !ok || signal.Kind != ChangeSignalKindRequiresRootReconcile {
		t.Fatalf("expected recovered root to require root reconcile, got %#v ok=%t", signal, ok)
	}
}

func TestTranslateUSNDeltaEmitsDirtyPathAndCursor(t *testing.T) {
	journal := usnJournalState{
		Volume:    `C:\`,
		JournalID: 77,
		FirstUSN:  100,
		NextUSN:   888,
	}
	root := RootRecord{
		ID:       "root-usn",
		Path:     `C:\root`,
		FeedType: RootFeedTypeUSN,
	}

	signal := translateUSNDelta(root, journal, `C:\root\dir\child.txt`, false, true, 555, usnReasonFileCreate, time.Unix(100, 0))
	if signal.Kind != ChangeSignalKindDirtyPath {
		t.Fatalf("expected dirty path signal, got %q", signal.Kind)
	}
	if signal.RootID != root.ID || signal.Path != `C:\root\dir\child.txt` {
		t.Fatalf("unexpected usn dirty signal: %#v", signal)
	}
	if !signal.PathTypeKnown || signal.PathIsDir {
		t.Fatalf("expected file dirty signal, got %#v", signal)
	}

	cursor, ok := decodeFeedCursor(signal.Cursor, RootFeedTypeUSN)
	if !ok {
		t.Fatalf("expected usn cursor to decode")
	}
	if cursor.JournalID != journal.JournalID || cursor.USN != 555 || cursor.Volume != journal.Volume {
		t.Fatalf("unexpected usn cursor payload: %#v", cursor)
	}
}
