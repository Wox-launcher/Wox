package interactive

import (
	"strings"
	"testing"
)

// TestCommandTracker preserves reported command boundaries across output chunks.
func TestCommandTracker(t *testing.T) {
	var written strings.Builder
	var start, finish int
	tracker := commandTracker{
		prefix:  "\x1b]633;Wox;test;",
		history: HistoryEntry{ID: "interactive:test"},
		onStart: func() { start = written.Len() },
		onFinish: func(_ HistoryEntry, _ string, exitCode int) bool {
			finish = written.Len()
			return exitCode == 1
		},
	}
	write := func(segment []byte) bool {
		written.Write(segment)
		return true
	}
	tracker.observe([]byte("normal\x1b]633;Wox;other;start;ignore\a\x1b]633;Wo"), write)
	tracker.observe([]byte("x;test;start;echo hello; sleep 2"), write)
	if tracker.history.Title != "" {
		t.Fatal("accepted an incomplete command marker")
	}
	tracker.observe([]byte("\aordinary output"), write)
	running := tracker.history
	if running.Title != "echo hello; sleep 2" || running.Status != CommandRunning {
		t.Fatalf("running history = %+v", running)
	}
	tracker.observe([]byte(tracker.prefix+"end;1\x1b\\next prompt"), write)
	if tracker.history.Status != CommandFailed || !tracker.history.Saved {
		t.Fatalf("finished history = %+v", tracker.history)
	}
	if captured := written.String()[start:finish]; captured != "ordinary output" {
		t.Fatalf("output between markers = %q", captured)
	}
	tracker.observe([]byte(tracker.prefix+"start;echo second\asecond output"+tracker.prefix+"end;0\a"), write)
	if entry := tracker.history; entry.ID != running.ID || entry.Title != running.Title ||
		entry.StartedAt != running.StartedAt || entry.Status != CommandCompleted || !entry.Saved {
		t.Fatalf("second command changed the session identity: %+v", entry)
	}
}
