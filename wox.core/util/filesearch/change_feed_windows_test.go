//go:build windows

package filesearch

import (
	"testing"
	"time"
)

func TestUSNResolvedRecordsEmitOnlyLongestMatchingRoot(t *testing.T) {
	parent := RootRecord{ID: "root-usn-parent", Path: `C:\Users\qian`}
	nested := RootRecord{ID: "root-usn-nested", Path: `C:\Users\qian\dev\Wox`, Kind: RootKindUser}
	var emitted []ChangeSignal
	watcher := &usnWatcherSet{
		emit: func(signal ChangeSignal) {
			emitted = append(emitted, signal)
		},
	}

	roots := []RootRecord{parent, nested}
	watcher.emitResolvedRecords(roots, newRootPathMatcher(roots), usnJournalState{Volume: `C:\`, JournalID: 1}, []usnResolvedRecord{
		{Path: `C:\Users\qian\dev\Wox\main.go`, PathKnown: true, PathIsDir: false, USN: 10},
	})

	if len(emitted) != 1 {
		t.Fatalf("expected one usn signal for longest matching root, got %#v", emitted)
	}
	if emitted[0].RootID != nested.ID {
		t.Fatalf("expected nested root signal, got %#v", emitted[0])
	}
	if emitted[0].Kind != ChangeSignalKindDirtyPath || emitted[0].At.IsZero() || emitted[0].At.After(time.Now().Add(time.Second)) {
		t.Fatalf("unexpected nested usn signal: %#v", emitted[0])
	}
}
