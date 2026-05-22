package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
	"wox/analytics"
	"wox/common"
	"wox/database"
	appplugin "wox/plugin/system/app"
	"wox/util"

	"gorm.io/gorm"
)

const defaultUsageStatsPeriod = "30d"

type usageStatsRequest struct {
	Period string `json:"Period"`
}

type usageStatsItem struct {
	Id    string `json:"Id"`
	Name  string `json:"Name"`
	Count int64  `json:"Count"`
	// Icon is filled after the raw usage queries because the analytics rows only
	// contain id/name/count. GORM must ignore this response-only struct field;
	// otherwise it treats WoxImage as a database relation and logs an invalid field error.
	Icon common.WoxImage `json:"Icon" gorm:"-"`
}

type usageStatsBucket struct {
	Key   int   `json:"Key"`
	Count int64 `json:"Count"`
}

type usageStatsDayBucket struct {
	Date  string `json:"Date"`
	Count int64  `json:"Count"`
}

type usageStatsResponse struct {
	TotalOpened               int64                 `json:"TotalOpened"`
	TotalAppLaunch            int64                 `json:"TotalAppLaunch"`
	TotalActions              int64                 `json:"TotalActions"`
	TotalAppsUsed             int64                 `json:"TotalAppsUsed"`
	UsageDays                 int                   `json:"UsageDays"`
	Period                    string                `json:"Period"`
	PeriodDays                int                   `json:"PeriodDays"`
	PeriodOpened              int64                 `json:"PeriodOpened"`
	PreviousPeriodOpened      int64                 `json:"PreviousPeriodOpened"`
	OpenedChangePercent       *float64              `json:"OpenedChangePercent"`
	PeriodAppLaunch           int64                 `json:"PeriodAppLaunch"`
	PreviousPeriodAppLaunch   int64                 `json:"PreviousPeriodAppLaunch"`
	AppLaunchChangePercent    *float64              `json:"AppLaunchChangePercent"`
	PeriodAppsUsed            int64                 `json:"PeriodAppsUsed"`
	PreviousPeriodAppsUsed    int64                 `json:"PreviousPeriodAppsUsed"`
	AppsUsedChangePercent     *float64              `json:"AppsUsedChangePercent"`
	PeriodActions             int64                 `json:"PeriodActions"`
	PreviousPeriodActions     int64                 `json:"PreviousPeriodActions"`
	ActionsChangePercent      *float64              `json:"ActionsChangePercent"`
	MostActiveHour            int                   `json:"MostActiveHour"`
	MostActiveDay             int                   `json:"MostActiveDay"`
	OpenedByHour              []int                 `json:"OpenedByHour"`
	OpenedByWeekday           []int                 `json:"OpenedByWeekday"`
	OpenedByDay               []usageStatsDayBucket `json:"OpenedByDay"`
	TopApps                   []usageStatsItem      `json:"TopApps"`
	TopPlugins                []usageStatsItem      `json:"TopPlugins"`
	currentPeriodStartUnixMs  int64
	previousPeriodStartUnixMs int64
	currentPeriodEndUnixMs    int64
	previousPeriodEndUnixMs   int64
}

func handleUsageStats(w http.ResponseWriter, r *http.Request) {
	ctx := util.NewTraceContext()
	db := database.GetDB()
	if db == nil {
		writeErrorResponse(w, "db not initialized")
		return
	}

	var resp usageStatsResponse
	req := readUsageStatsRequest(r)
	configureUsageStatsPeriod(&resp, req.Period)
	resp.OpenedByHour = make([]int, 24)
	resp.OpenedByWeekday = make([]int, 7)

	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeUIOpened).Count(&resp.TotalOpened).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeAppLaunched).Count(&resp.TotalAppLaunch).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeActionExecuted).Count(&resp.TotalActions).Error
	_ = db.Model(&analytics.Event{}).Where("event_type = ?", analytics.EventTypeAppLaunched).Distinct("subject_id").Count(&resp.TotalAppsUsed).Error

	resp.MostActiveHour = -1
	resp.MostActiveDay = -1

	fillUsageDays(ctx, &resp)
	fillPeriodMetrics(ctx, &resp)
	fillOpenedBuckets(ctx, &resp)
	fillOpenedByDay(ctx, &resp)
	fillTopItems(ctx, &resp)

	writeSuccessResponse(w, resp)
}

func readUsageStatsRequest(r *http.Request) usageStatsRequest {
	var req usageStatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		return usageStatsRequest{Period: defaultUsageStatsPeriod}
	}
	return req
}

func configureUsageStatsPeriod(resp *usageStatsResponse, requestedPeriod string) {
	now := time.Now()
	resp.currentPeriodEndUnixMs = now.UnixMilli()
	resp.Period = defaultUsageStatsPeriod
	resp.PeriodDays = 30

	switch requestedPeriod {
	case "7d":
		resp.Period = "7d"
		resp.PeriodDays = 7
	case "365d":
		resp.Period = "365d"
		resp.PeriodDays = 365
	case "all":
		resp.Period = "all"
		resp.PeriodDays = 0
		resp.currentPeriodStartUnixMs = 0
		resp.previousPeriodStartUnixMs = 0
		resp.previousPeriodEndUnixMs = 0
		return
	case "30d", "":
		resp.Period = defaultUsageStatsPeriod
		resp.PeriodDays = 30
	default:
		resp.Period = defaultUsageStatsPeriod
		resp.PeriodDays = 30
	}

	// Periods are equal-length windows so trend chips can compare the current report against the
	// previous equivalent range: 7d vs 7d, 30d vs 30d, and 365d vs 365d.
	currentPeriodStart := now.AddDate(0, 0, -resp.PeriodDays)
	previousPeriodStart := now.AddDate(0, 0, -resp.PeriodDays*2)
	resp.currentPeriodStartUnixMs = currentPeriodStart.UnixMilli()
	resp.previousPeriodStartUnixMs = previousPeriodStart.UnixMilli()
	resp.previousPeriodEndUnixMs = currentPeriodStart.UnixMilli()
}

func fillPeriodMetrics(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	if resp.Period == "all" {
		resp.PeriodOpened = countEventsInWindow(db, analytics.EventTypeUIOpened, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
		resp.PeriodAppLaunch = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
		resp.PeriodAppsUsed = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, true)
		resp.PeriodActions = countEventsInWindow(db, analytics.EventTypeActionExecuted, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
		return
	}

	// The visible KPI cards are driven by the selected period and its previous equivalent window.
	// All-time totals remain in the response for older UI/share consumers that still read them.
	resp.PeriodOpened = countEventsInWindow(db, analytics.EventTypeUIOpened, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
	resp.PreviousPeriodOpened = countEventsInWindow(db, analytics.EventTypeUIOpened, "", resp.previousPeriodStartUnixMs, resp.previousPeriodEndUnixMs, false)
	resp.OpenedChangePercent = calculateUsageChangePercent(resp.PeriodOpened, resp.PreviousPeriodOpened)

	resp.PeriodAppLaunch = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
	resp.PreviousPeriodAppLaunch = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.previousPeriodStartUnixMs, resp.previousPeriodEndUnixMs, false)
	resp.AppLaunchChangePercent = calculateUsageChangePercent(resp.PeriodAppLaunch, resp.PreviousPeriodAppLaunch)

	resp.PeriodAppsUsed = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, true)
	resp.PreviousPeriodAppsUsed = countEventsInWindow(db, analytics.EventTypeAppLaunched, "", resp.previousPeriodStartUnixMs, resp.previousPeriodEndUnixMs, true)
	resp.AppsUsedChangePercent = calculateUsageChangePercent(resp.PeriodAppsUsed, resp.PreviousPeriodAppsUsed)

	resp.PeriodActions = countEventsInWindow(db, analytics.EventTypeActionExecuted, "", resp.currentPeriodStartUnixMs, resp.currentPeriodEndUnixMs, false)
	resp.PreviousPeriodActions = countEventsInWindow(db, analytics.EventTypeActionExecuted, "", resp.previousPeriodStartUnixMs, resp.previousPeriodEndUnixMs, false)
	resp.ActionsChangePercent = calculateUsageChangePercent(resp.PeriodActions, resp.PreviousPeriodActions)
}

func countEventsInWindow(db *gorm.DB, eventType analytics.EventType, subjectType analytics.SubjectType, startUnixMs int64, endUnixMs int64, distinctSubjects bool) int64 {
	query := db.Model(&analytics.Event{}).Where("event_type = ? AND timestamp >= ? AND timestamp < ?", eventType, startUnixMs, endUnixMs)
	if subjectType != "" {
		query = query.Where("subject_type = ?", subjectType)
	}
	if distinctSubjects {
		query = query.Distinct("subject_id")
	}

	var count int64
	_ = query.Count(&count).Error
	return count
}

func calculateUsageChangePercent(current int64, previous int64) *float64 {
	if previous == 0 {
		if current == 0 {
			zero := 0.0
			return &zero
		}
		return nil
	}

	change := (float64(current-previous) / float64(previous)) * 100
	return &change
}

func fillUsageDays(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	var firstTimestamp int64
	if err := db.Model(&analytics.Event{}).Select("MIN(timestamp)").Scan(&firstTimestamp).Error; err != nil || firstTimestamp <= 0 {
		resp.UsageDays = 0
		return
	}

	// Usage days are derived from the first real analytics event instead of install metadata because
	// Wox does not persist an install timestamp. Rounding up makes a same-day first use show as 1 day,
	// which matches how users read "already used Wox for N days".
	elapsed := time.Since(time.UnixMilli(firstTimestamp))
	if elapsed <= 0 {
		resp.UsageDays = 1
		return
	}
	resp.UsageDays = int(elapsed/(24*time.Hour)) + 1
}

func fillOpenedBuckets(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	var byHour []usageStatsBucket
	_ = db.Raw(
		"SELECT CAST(strftime('%H', timestamp/1000, 'unixepoch', 'localtime') AS INTEGER) AS key, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND timestamp >= ? AND timestamp < ? GROUP BY key",
		analytics.EventTypeUIOpened,
		resp.currentPeriodStartUnixMs,
		resp.currentPeriodEndUnixMs,
	).Scan(&byHour).Error
	for _, b := range byHour {
		if b.Key >= 0 && b.Key < 24 {
			resp.OpenedByHour[b.Key] = int(b.Count)
		}
	}

	var byWeekday []usageStatsBucket
	_ = db.Raw(
		"SELECT CAST(strftime('%w', timestamp/1000, 'unixepoch', 'localtime') AS INTEGER) AS key, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND timestamp >= ? AND timestamp < ? GROUP BY key",
		analytics.EventTypeUIOpened,
		resp.currentPeriodStartUnixMs,
		resp.currentPeriodEndUnixMs,
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

	if resp.PeriodOpened == 0 {
		resp.MostActiveHour = -1
		resp.MostActiveDay = -1
	}
}

// fillOpenedByDay always expands the latest year into daily cells so the heatmap stays stable when the period filter changes.
func fillOpenedByDay(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	now := time.Now()
	start := now.AddDate(-1, 0, 0).Local()
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	end := now.Local()
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	startUnixMs := startDay.UnixMilli()
	endUnixMs := now.UnixMilli()

	var openedDays []usageStatsDayBucket
	_ = db.Raw(
		"SELECT strftime('%Y-%m-%d', timestamp/1000, 'unixepoch', 'localtime') AS date, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND timestamp >= ? AND timestamp < ? GROUP BY date ORDER BY date",
		analytics.EventTypeUIOpened,
		startUnixMs,
		endUnixMs,
	).Scan(&openedDays).Error

	countsByDay := make(map[string]int64, len(openedDays))
	for _, day := range openedDays {
		countsByDay[day.Date] = day.Count
	}

	resp.OpenedByDay = make([]usageStatsDayBucket, 0)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		resp.OpenedByDay = append(resp.OpenedByDay, usageStatsDayBucket{Date: date, Count: countsByDay[date]})
	}
}

func fillTopItems(ctx context.Context, resp *usageStatsResponse) {
	db := database.GetDB()
	if db == nil {
		return
	}

	_ = db.Raw(
		"SELECT subject_id AS id, MAX(subject_name) AS name, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND timestamp >= ? AND timestamp < ? GROUP BY subject_id ORDER BY count DESC LIMIT 10",
		analytics.EventTypeAppLaunched,
		resp.currentPeriodStartUnixMs,
		resp.currentPeriodEndUnixMs,
	).Scan(&resp.TopApps).Error

	_ = db.Raw(
		"SELECT subject_id AS id, MAX(subject_name) AS name, COUNT(*) AS count "+
			"FROM events WHERE event_type = ? AND subject_type = ? AND timestamp >= ? AND timestamp < ? GROUP BY subject_id ORDER BY count DESC LIMIT 10",
		analytics.EventTypeActionExecuted,
		analytics.SubjectTypePlugin,
		resp.currentPeriodStartUnixMs,
		resp.currentPeriodEndUnixMs,
	).Scan(&resp.TopPlugins).Error

	appSubjectIds := make([]string, 0, len(resp.TopApps))
	for i := range resp.TopApps {
		if resp.TopApps[i].Name == "" {
			resp.TopApps[i].Name = resp.TopApps[i].Id
		}
		appSubjectIds = append(appSubjectIds, resp.TopApps[i].Id)
	}

	appIcons := appplugin.GetUsageAppIcons(ctx, appSubjectIds)
	for i := range resp.TopApps {
		if icon, ok := appIcons[resp.TopApps[i].Id]; ok {
			resp.TopApps[i].Icon = icon
		}
	}
	for i := range resp.TopPlugins {
		if resp.TopPlugins[i].Name == "" {
			resp.TopPlugins[i].Name = resp.TopPlugins[i].Id
		}
	}
}
