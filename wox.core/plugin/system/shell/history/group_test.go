package history

import (
	"testing"
	"time"
)

// TestGroup uses local midnight and covers yesterday in the previous month.
func TestGroup(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*60*60)
	today := time.Date(2026, time.November, 1, 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)
	now := today.Add(30 * time.Minute)
	for _, tc := range []struct {
		startedAt time.Time
		title     string
		score     int64
	}{
		{today.UTC(), "i18n:plugin_shell_group_today", 90},
		{today.Add(-time.Second), "i18n:plugin_shell_group_yesterday", 80},
		{yesterday, "i18n:plugin_shell_group_yesterday", 80},
		{yesterday.Add(-time.Second), "i18n:plugin_shell_group_history", 10},
	} {
		if title, score := Group(tc.startedAt, now); title != tc.title || score != tc.score {
			t.Errorf("Group(%s, %s) = %q, %d; want %q, %d", tc.startedAt, now, title, score, tc.title, tc.score)
		}
	}
}
