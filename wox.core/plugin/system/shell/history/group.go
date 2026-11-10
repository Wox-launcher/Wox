package history

import "time"

// Group classifies Shell runs by local calendar day.
func Group(startedAt, now time.Time) (string, int64) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case !startedAt.Before(today):
		return "i18n:plugin_shell_group_today", 90
	case !startedAt.Before(today.AddDate(0, 0, -1)):
		return "i18n:plugin_shell_group_yesterday", 80
	default:
		return "i18n:plugin_shell_group_history", 10
	}
}
