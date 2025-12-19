package ui

import (
	"context"
	"net/http"
	"wox/analytics"
	"wox/database"
	"wox/util"
)

type usageStatsItem struct {
	Id    string `json:"Id"`
	Name  string `json:"Name"`
	Count int64  `json:"Count"`
}

type usageStatsBucket struct {
	Key   int   `json:"Key"`
	Count int64 `json:"Count"`
}

type usageStatsResponse struct {
	TotalOpened     int64            `json:"TotalOpened"`
	TotalAppLaunch  int64            `json:"TotalAppLaunch"`
	TotalActions    int64            `json:"TotalActions"`
	TotalAppsUsed   int64            `json:"TotalAppsUsed"`
	MostActiveHour  int              `json:"MostActiveHour"`
	MostActiveDay   int              `json:"MostActiveDay"`
	OpenedByHour    []int            `json:"OpenedByHour"`
	OpenedByWeekday []int            `json:"OpenedByWeekday"`
	TopApps         []usageStatsItem `json:"TopApps"`
	TopPlugins      []usageStatsItem `json:"TopPlugins"`
}

func handleUsageStats(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
	db := database.GetDB()
	if db == nil {
		writeErrorResponse(w, "db not initialized")
		return
	}

	var resp usageStatsResponse
	resp.OpenedByHour = make([]int, 24)
	resp.OpenedByWeekday = make([]int, 7)

	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeUIOpened).Count(&resp.TotalOpened).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeAppLaunched).Count(&resp.TotalAppLaunch).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeActionExecuted).Count(&resp.TotalActions).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeAppLaunched).Distinct("subject_id").Count(&resp.TotalAppsUsed).Error

	resp.MostActiveHour = -1
	resp.MostActiveDay = -1

	fillOpenedBuckets(ctx, &resp)
	fillTopItems(ctx, &resp)

	writeSuccessResponse(w, resp)
}

func fillOpenedBuckets(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	var byHour []usageStatsBucket
	_ = db.Raw(
		"SELECT CAST(strftime('%H', timestamp/1000, 'unixepoch', 'localtime') AS INTEGER) AS key, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? GROUP BY key",
		analytics.EventTypeUIOpened,
	).Scan(&byHour).Error
	for _, b := range byHour {
		if b.Key >= 0 && b.Key < 24 {
			resp.OpenedByHour[b.Key] = int(b.Count)
		}
	}

	var byWeekday []usageStatsBucket
	_ = db.Raw(
		"SELECT CAST(strftime('%w', timestamp/1000, 'unixepoch', 'localtime') AS INTEGER) AS key, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? GROUP BY key",
		analytics.EventTypeUIOpened,
	).Scan(&byWeekday).Error
	for _, b := range byWeekday {
		if b.Key >= 0 && b.Key < 7 {
			resp.OpenedByWeekday[b.Key] = int(b.Count)
		}
	}

	maxHourCount := int64(0)
	for hour, count := range resp.OpenedByHour {
		if int64(count) > maxHourCount {
			maxHourCount = int64(count)
			resp.MostActiveHour = hour
		}
	}
	maxDayCount := int64(0)
	for day, count := range resp.OpenedByWeekday {
		if int64(count) > maxDayCount {
			maxDayCount = int64(count)
			resp.MostActiveDay = day
		}
	}

	if resp.TotalOpened == 0 {
		resp.MostActiveHour = -1
		resp.MostActiveDay = -1
	}
}

func fillTopItems(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	_ = db.Raw(
		"SELECT subject_id AS id, MAX(subject_name) AS name, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? GROUP BY subject_id ORDER BY count DESC LIMIT 10",
		analytics.EventTypeAppLaunched,
	).Scan(&resp.TopApps).Error

	_ = db.Raw(
		"SELECT subject_id AS id, MAX(subject_name) AS name, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND subject_type = ? GROUP BY subject_id ORDER BY count DESC LIMIT 10",
		analytics.EventTypeActionExecuted,
		analytics.SubjectTypePlugin,
	).Scan(&resp.TopPlugins).Error

	for i := range resp.TopApps {
		if resp.TopApps[i].Name == "" {
			resp.TopApps[i].Name = resp.TopApps[i].Id
		}
	}
	for i := range resp.TopPlugins {
		if resp.TopPlugins[i].Name == "" {
			resp.TopPlugins[i].Name = resp.TopPlugins[i].Id
		}
	}
}
