package system

import (
	"context"
	"testing"
	"time"
	"wox/plugin"
	"wox/util"
)

type timerUpdateTestAPI struct {
	plugin.API
	updates int
}

func (a *timerUpdateTestAPI) IsVisible(context.Context) bool {
	return true
}

func (a *timerUpdateTestAPI) UpdateResult(context.Context, plugin.UpdatableResult) bool {
	a.updates++
	return a.updates > 1
}

func TestParseTimerQuery(t *testing.T) {
	tests := []struct {
		input     string
		want      time.Duration
		wantLabel string
		wantNote  string
		ok        bool
	}{
		{"5m", 5 * time.Minute, "5m", "", true},
		{"30s", 30 * time.Second, "30s", "", true},
		{"1h", time.Hour, "1h", "", true},
		{"1h 15m", time.Hour + 15*time.Minute, "1h 15m", "", true},
		{"1h15m", time.Hour + 15*time.Minute, "1h 15m", "", true},
		{"2h 30m 10s", 2*time.Hour + 30*time.Minute + 10*time.Second, "2h 30m 10s", "", true},
		{"1m 这只是一个说明", time.Minute, "1m", "这只是一个说明", true},
		{"5m cooking", 5 * time.Minute, "5m", "cooking", true},
		{"1h 15m stretch break", time.Hour + 15*time.Minute, "1h 15m", "stretch break", true},
		{"1m这只是一个说明", time.Minute, "1m", "这只是一个说明", true},
		{"", 0, "", "", false},
		{"abc", 0, "", "", false},
		{"5x", 0, "", "", false},
		{"0m", 0, "", "", false},
		{"cooking 5m", 0, "", "", false},
	}

	for _, tt := range tests {
		got, label, note, ok := parseTimerQuery(tt.input)
		if ok != tt.ok {
			t.Fatalf("parseTimerQuery(%q) ok=%v want %v", tt.input, ok, tt.ok)
		}
		if !tt.ok {
			continue
		}
		if got != tt.want {
			t.Fatalf("parseTimerQuery(%q)=%v want %v", tt.input, got, tt.want)
		}
		if label != tt.wantLabel {
			t.Fatalf("parseTimerQuery(%q) label=%q want %q", tt.input, label, tt.wantLabel)
		}
		if note != tt.wantNote {
			t.Fatalf("parseTimerQuery(%q) note=%q want %q", tt.input, note, tt.wantNote)
		}
	}
}

func TestFormatTimerRemaining(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "00:45"},
		{4*time.Minute + 59*time.Second, "04:59"},
		{time.Hour + 2*time.Minute + 3*time.Second, "1h 02m 03s"},
		{-time.Second, "00:00"},
	}
	for _, tt := range tests {
		if got := formatTimerRemaining(tt.d); got != tt.want {
			t.Fatalf("formatTimerRemaining(%v)=%q want %q", tt.d, got, tt.want)
		}
	}
}

func TestTimerPauseResumeRemaining(t *testing.T) {
	plugin := &TimerPlugin{timers: make(map[string]*timerEntry)}
	entry := &timerEntry{
		ID:            "t1",
		DurationLabel: "5m",
		Duration:      5 * time.Minute,
		Deadline:      time.Now().Add(90 * time.Second),
	}
	plugin.timers[entry.ID] = entry

	ctx := context.Background()
	plugin.pauseTimer(ctx, entry.ID)
	paused := plugin.getTimer(entry.ID)
	if paused == nil || !paused.Paused {
		t.Fatal("timer should be paused")
	}
	if paused.Remaining <= 0 || paused.Remaining > 90*time.Second+time.Second {
		t.Fatalf("unexpected remaining after pause: %v", paused.Remaining)
	}

	before := paused.Remaining
	time.Sleep(20 * time.Millisecond)
	still := plugin.remainingOf(plugin.getTimer(entry.ID))
	if still > before || before-still > time.Second {
		t.Fatalf("paused remaining should stay stable, before=%v after=%v", before, still)
	}

	plugin.resumeTimer(ctx, entry.ID)
	resumed := plugin.getTimer(entry.ID)
	if resumed == nil || resumed.Paused {
		t.Fatal("timer should be resumed")
	}
	remaining := plugin.remainingOf(resumed)
	if remaining <= 0 || remaining > before+time.Second {
		t.Fatalf("unexpected remaining after resume: %v", remaining)
	}
}

func TestTimerListSortedByRemaining(t *testing.T) {
	plugin := &TimerPlugin{timers: make(map[string]*timerEntry)}
	plugin.timers["far"] = &timerEntry{ID: "far", DurationLabel: "10m", Deadline: time.Now().Add(10 * time.Minute)}
	plugin.timers["near"] = &timerEntry{ID: "near", DurationLabel: "1m", Deadline: time.Now().Add(time.Minute)}
	plugin.timers["paused"] = &timerEntry{ID: "paused", DurationLabel: "30s", Remaining: 30 * time.Second, Paused: true}

	list := plugin.listTimersSorted()
	if len(list) != 3 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].ID != "paused" || list[1].ID != "near" || list[2].ID != "far" {
		t.Fatalf("unexpected order: %s %s %s", list[0].ID, list[1].ID, list[2].ID)
	}
}

// TestDeleteTimerStopsStaleOverlayRefresh keeps a tick that already copied overlay IDs from recreating a deleted timer.
func TestDeleteTimerStopsStaleOverlayRefresh(t *testing.T) {
	timer := &TimerPlugin{timers: map[string]*timerEntry{
		"t1": {ID: "t1", DurationLabel: "2m", Note: "tea", OverlayVisible: true, OverlayPlaced: true, Deadline: time.Now().Add(2 * time.Minute)},
	}}

	timer.mu.Lock()
	staleIDs := make([]string, 0, len(timer.timers))
	for id, entry := range timer.timers {
		if entry.OverlayVisible {
			staleIDs = append(staleIDs, id)
		}
	}
	timer.mu.Unlock()

	timer.deleteTimer(context.Background(), "t1")
	for _, id := range staleIDs {
		if entry := timer.getTimer(id); entry != nil && entry.OverlayVisible {
			t.Fatalf("deleted timer %s is still refreshable", id)
		}
		timer.showOverlay(context.Background(), id, true)
	}
	if timer.getTimer("t1") != nil {
		t.Fatal("deleted timer should not remain after a stale overlay refresh")
	}
}

func TestTimerUpdateNote(t *testing.T) {
	plugin := &TimerPlugin{timers: make(map[string]*timerEntry)}
	plugin.timers["t1"] = &timerEntry{ID: "t1", DurationLabel: "1m", Note: "old"}
	plugin.updateNote(context.Background(), "t1", "new note")
	entry := plugin.getTimer("t1")
	if entry == nil || entry.Note != "new note" {
		t.Fatalf("note=%q", entry.Note)
	}
}

func TestTimerPersistedRoundTrip(t *testing.T) {
	deadline := time.UnixMilli(1_700_000_000_000)
	original := &timerEntry{
		ID:            "abc",
		DurationLabel: "5m",
		Note:          "tea",
		Duration:      5 * time.Minute,
		Deadline:      deadline,
		Remaining:     90 * time.Second,
		Paused:        true,
	}

	restored := timerEntryFromPersisted(timerEntryToPersisted(original))
	if restored.ID != original.ID || restored.DurationLabel != original.DurationLabel || restored.Note != original.Note {
		t.Fatalf("identity mismatch: %+v", restored)
	}
	if restored.Duration != original.Duration || restored.Remaining != original.Remaining || restored.Paused != original.Paused {
		t.Fatalf("timing mismatch: %+v", restored)
	}
	if !restored.Deadline.Equal(original.Deadline) {
		t.Fatalf("deadline=%v want %v", restored.Deadline, original.Deadline)
	}
}

func TestTimerTickRetriesTransientResultMiss(t *testing.T) {
	api := &timerUpdateTestAPI{}
	timer := &TimerPlugin{
		api:            api,
		timers:         map[string]*timerEntry{"t1": {ID: "t1", DurationLabel: "1m", Deadline: time.Now().Add(time.Minute)}},
		trackedResults: util.NewHashMap[string, string](),
	}
	timer.trackedResults.Store("t1", "t1")

	timer.tick(context.Background())
	timer.tick(context.Background())

	if api.updates != 2 {
		t.Fatalf("updates=%d want 2", api.updates)
	}
	if _, ok := timer.trackedResults.Load("t1"); !ok {
		t.Fatal("active timer should remain tracked after a transient update miss")
	}
}
