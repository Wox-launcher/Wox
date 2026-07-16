package launcher

import (
	"context"
	"fmt"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

type usageStatsData struct {
	Period          string
	PeriodOpened    int64
	PeriodAppLaunch int64
	PeriodAppsUsed  int64
	PeriodActions   int64
	UsageDays       int
	MostActiveHour  int
	MostActiveDay   int
	OpenedByDay     []usageStatsDay
	TopApps         []usageStatsItem
	TopPlugins      []usageStatsItem
}

type usageStatsDay struct {
	Date  string
	Count int64
}

type usageStatsItem struct {
	ID    string `json:"Id"`
	Name  string
	Count int64
}

// cloneUsageStats keeps render snapshots independent from asynchronous report refreshes.
func cloneUsageStats(source usageStatsData) usageStatsData {
	result := source
	result.OpenedByDay = append([]usageStatsDay(nil), source.OpenedByDay...)
	result.TopApps = append([]usageStatsItem(nil), source.TopApps...)
	result.TopPlugins = append([]usageStatsItem(nil), source.TopPlugins...)
	return result
}

func (a *App) currentUsagePeriod() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.usagePeriod == "" {
		return "30d"
	}
	return a.usagePeriod
}

// reloadUsageStats refreshes one report period and ignores responses superseded by a later selection.
func (a *App) reloadUsageStats(period string) {
	period = normalizeUsagePeriod(period)
	a.mu.Lock()
	a.usageRevision++
	revision := a.usageRevision
	a.usagePeriod = period
	a.usageLoading = true
	a.usageError = ""
	a.mu.Unlock()
	_ = a.window.Invalidate()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var data usageStatsData
	err := a.client.Post(ctx, "/usage/stats", map[string]string{"Period": period}, &data)

	a.mu.Lock()
	if revision != a.usageRevision {
		a.mu.Unlock()
		return
	}
	a.usageLoading = false
	if err != nil {
		a.usageError = err.Error()
	} else {
		data.Period = normalizeUsagePeriod(data.Period)
		a.usageStats = data
		a.usagePeriod = data.Period
		a.usageLoaded = true
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func normalizeUsagePeriod(period string) string {
	switch period {
	case "7d", "30d", "365d", "all":
		return period
	default:
		return "30d"
	}
}

func usagePeriodLabel(period string) string {
	switch normalizeUsagePeriod(period) {
	case "7d":
		return "7 days"
	case "365d":
		return "365 days"
	case "all":
		return "All time"
	default:
		return "30 days"
	}
}

// buildUsageSettingsPage renders the same local analytics response on every desktop backend.
func (a *App) buildUsageSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-72)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Usage", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: "A local report built from Wox usage events", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
		a.buildUsagePeriodSelector(snapshot, contentWidth),
	}
	if snapshot.usageError != "" {
		children = append(children, woxwidget.Text{Value: "Could not load usage: " + snapshot.usageError, Style: woxui.TextStyle{Size: 12}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}})
	}
	if snapshot.usageLoading && len(snapshot.usage.OpenedByDay) == 0 {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 120, Padding: woxwidget.Insets{Top: 40}, Child: woxwidget.Text{Value: "Loading usage report…", Style: woxui.TextStyle{Size: 14}, Color: snapshot.palette.resultSubtitle}})
	} else {
		children = append(children,
			a.buildUsageKPIs(snapshot, contentWidth),
			a.buildUsageActivity(snapshot, contentWidth),
			a.buildUsageRankings(snapshot, contentWidth),
		)
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	}
}

func (a *App) buildUsagePeriodSelector(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	periods := []string{"7d", "30d", "365d", "all"}
	buttons := make([]woxwidget.Widget, 0, len(periods)+1)
	buttons = append(buttons, woxwidget.Container{Width: max(float32(120), width-390), Height: 34, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
		Value: fmt.Sprintf("Overview · %s", usagePeriodLabel(snapshot.usagePeriod)), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle,
	}})
	for _, period := range periods {
		period := period
		background := snapshot.palette.queryBackground
		foreground := snapshot.palette.resultSubtitle
		if period == snapshot.usagePeriod {
			background = snapshot.palette.selectedBackground
			foreground = snapshot.palette.selectedTitle
		}
		buttons = append(buttons, woxwidget.Gesture{ID: "usage-period-" + period, OnTap: func() { go a.reloadUsageStats(period) }, Child: woxwidget.Container{
			Width: 86, Height: 34, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 9},
			Child: woxwidget.Text{Value: usagePeriodLabel(period), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}})
	}
	return woxwidget.Container{Width: width, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 7, Children: buttons}}
}

func (a *App) buildUsageKPIs(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	stats := snapshot.usage
	cardWidth := max(float32(100), (width-30)/4)
	cards := []struct {
		label string
		value int64
	}{
		{"Opened", stats.PeriodOpened},
		{"App launches", stats.PeriodAppLaunch},
		{"Apps used", stats.PeriodAppsUsed},
		{"Actions", stats.PeriodActions},
	}
	children := make([]woxwidget.Widget, 0, len(cards))
	for _, card := range cards {
		children = append(children, woxwidget.Container{Width: cardWidth, Height: 88, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 15}, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 9, Children: []woxwidget.Widget{
				woxwidget.Text{Value: card.label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle},
				woxwidget.Text{Value: fmt.Sprintf("%d", card.value), Style: woxui.TextStyle{Size: 25, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			},
		}})
	}
	return woxwidget.Container{Width: width, Height: 88, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: children}}
}

func (a *App) buildUsageActivity(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	return woxwidget.Container{Width: width, Height: 166, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 13, Right: 16, Bottom: 12}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Open activity · latest year", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
			woxwidget.Painter{Width: innerWidth, Height: 116, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				drawUsageHeatmap(displayList, bounds, snapshot.usage.OpenedByDay, snapshot.palette)
			}},
		},
	}}
}

// drawUsageHeatmap maps daily buckets into Monday-first weeks without introducing a chart dependency.
func drawUsageHeatmap(displayList *woxui.DisplayList, bounds woxui.Rect, days []usageStatsDay, palette uiPalette) {
	if len(days) == 0 {
		displayList.DrawText("No activity yet", bounds, woxui.TextStyle{Size: 12}, palette.resultSubtitle)
		return
	}
	firstOffset := 0
	if first, err := time.Parse("2006-01-02", days[0].Date); err == nil {
		firstOffset = (int(first.Weekday()) + 6) % 7
	}
	columns := max(1, (firstOffset+len(days)+6)/7)
	gap := float32(2)
	cell := min(float32(12), min((bounds.Width-float32(columns-1)*gap)/float32(columns), (bounds.Height-6*gap)/7))
	if cell < 2 {
		return
	}
	maxCount := int64(1)
	for _, day := range days {
		maxCount = max(maxCount, day.Count)
	}
	for index, day := range days {
		position := firstOffset + index
		column := position / 7
		row := position % 7
		color := palette.toolbarBackground
		if day.Count > 0 {
			color = palette.cursor
			color.A = uint8(60 + day.Count*195/maxCount)
		}
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + float32(column)*(cell+gap), Y: bounds.Y + float32(row)*(cell+gap), Width: cell, Height: cell}, 2, color)
	}
}

func (a *App) buildUsageRankings(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	gap := float32(12)
	panelWidth := max(float32(180), (width-gap)/2)
	return woxwidget.Container{Width: width, Height: 204, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		a.buildUsageRankingPanel("Top apps", snapshot.usage.TopApps, snapshot.palette, panelWidth),
		a.buildUsageRankingPanel("Top plugins", snapshot.usage.TopPlugins, snapshot.palette, panelWidth),
	}}}
}

func (a *App) buildUsageRankingPanel(title string, items []usageStatsItem, palette uiPalette, width float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, 6)
	rows = append(rows, woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: palette.resultTitle})
	if len(items) == 0 {
		rows = append(rows, woxwidget.Container{Width: width - 30, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: "No activity yet", Style: woxui.TextStyle{Size: 12}, Color: palette.resultSubtitle}})
	}
	for index, item := range items {
		if index >= 5 {
			break
		}
		name := item.Name
		if name == "" {
			name = item.ID
		}
		rows = append(rows, woxwidget.Container{Width: width - 30, Height: 27, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 88, Height: 27, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: fmt.Sprintf("%d  %s", index+1, name), Style: woxui.TextStyle{Size: 12}, Color: palette.resultTitle}},
			woxwidget.Container{Width: 42, Height: 27, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: fmt.Sprintf("%d", item.Count), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.cursor}},
		}}})
	}
	return woxwidget.Container{Width: width, Height: 204, Radius: 10, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 15, Top: 13, Right: 15, Bottom: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: rows}}
}
