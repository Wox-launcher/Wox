package view

import (
	"fmt"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// UsagePeriod describes one report period selector.
type UsagePeriod struct {
	ID       string
	Label    string
	Selected bool
	OnSelect func()
}

// UsageKPI contains one usage summary value.
type UsageKPI struct {
	Label string
	Value int64
}

// UsageDay contains one daily activity bucket.
type UsageDay struct {
	Date  string
	Count int64
}

// UsageRankingItem contains one ranked app or plugin.
type UsageRankingItem struct {
	Name  string
	Count int64
}

// UsageSettingsProps contains the local usage report presentation data.
type UsageSettingsProps struct {
	Width       float32
	Height      float32
	Theme       woxcomponent.Theme
	PeriodLabel string
	Periods     []UsagePeriod
	Error       string
	Loading     bool
	KPIs        []UsageKPI
	Days        []UsageDay
	TopApps     []UsageRankingItem
	TopPlugins  []UsageRankingItem
}

// UsageSettingsView builds the local usage dashboard.
func UsageSettingsView(props UsageSettingsProps) woxwidget.Widget {
	contentWidth := max(float32(0), props.Width-72)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 58, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Usage", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
			woxwidget.Text{Value: "A local report built from Wox usage events", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
		}}},
		usagePeriodSelector(props, contentWidth),
	}
	if props.Error != "" {
		children = append(children, woxwidget.Text{Value: "Could not load usage: " + props.Error, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText})
	}
	if props.Loading {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 120, Padding: woxwidget.Insets{Top: 40}, Child: woxwidget.Text{Value: "Loading usage report…", Style: woxui.TextStyle{Size: 14}, Color: props.Theme.ResultSubtitle}})
	} else {
		children = append(children, usageKPIs(props, contentWidth), usageActivity(props, contentWidth), usageRankings(props, contentWidth))
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}}
}

func usagePeriodSelector(props UsageSettingsProps, width float32) woxwidget.Widget {
	buttons := []woxwidget.Widget{woxwidget.Container{Width: max(float32(120), width-390), Height: 34, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
		Value: "Overview · " + props.PeriodLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle,
	}}}
	for _, period := range props.Periods {
		background := props.Theme.QueryBackground
		foreground := props.Theme.ResultSubtitle
		if period.Selected {
			background = props.Theme.SelectedBackground
			foreground = props.Theme.SelectedTitle
		}
		buttons = append(buttons, woxwidget.Gesture{ID: "usage-period-" + period.ID, OnTap: period.OnSelect, Child: woxwidget.Container{
			Width: 86, Height: 34, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 9}, Child: woxwidget.Text{Value: period.Label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}})
	}
	return woxwidget.Container{Width: width, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 7, Children: buttons}}
}

func usageKPIs(props UsageSettingsProps, width float32) woxwidget.Widget {
	cardWidth := max(float32(100), (width-30)/4)
	cards := make([]woxwidget.Widget, 0, len(props.KPIs))
	for _, item := range props.KPIs {
		cards = append(cards, woxwidget.Container{Width: cardWidth, Height: 88, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 15}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 9, Children: []woxwidget.Widget{
			woxwidget.Text{Value: item.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle},
			woxwidget.Text{Value: fmt.Sprintf("%d", item.Value), Style: woxui.TextStyle{Size: 25, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		}}})
	}
	return woxwidget.Container{Width: width, Height: 88, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: cards}}
}

func usageActivity(props UsageSettingsProps, width float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	return woxwidget.Container{Width: width, Height: 166, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 13, Right: 16, Bottom: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Text{Value: "Open activity · latest year", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		woxwidget.Painter{Width: innerWidth, Height: 116, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			drawUsageHeatmap(displayList, bounds, props.Days, props.Theme)
		}},
	}}}
}

// drawUsageHeatmap maps daily buckets into Monday-first weeks without a chart dependency.
func drawUsageHeatmap(displayList *woxui.DisplayList, bounds woxui.Rect, days []UsageDay, theme woxcomponent.Theme) {
	if len(days) == 0 {
		displayList.DrawText("No activity yet", bounds, woxui.TextStyle{Size: 12}, theme.ResultSubtitle)
		return
	}
	firstOffset := 0
	if first, err := time.Parse("2006-01-02", days[0].Date); err == nil {
		firstOffset = (int(first.Weekday()) + 6) % 7
	}
	columns := max(1, (firstOffset+len(days)+6)/7)
	const gap = float32(2)
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
		color := theme.ToolbarBackground
		if day.Count > 0 {
			color = theme.Cursor
			color.A = uint8(60 + day.Count*195/maxCount)
		}
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + float32(position/7)*(cell+gap), Y: bounds.Y + float32(position%7)*(cell+gap), Width: cell, Height: cell}, 2, color)
	}
}

func usageRankings(props UsageSettingsProps, width float32) woxwidget.Widget {
	const gap = float32(12)
	panelWidth := max(float32(180), (width-gap)/2)
	return woxwidget.Container{Width: width, Height: 204, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		usageRankingPanel("Top apps", props.TopApps, panelWidth, props.Theme),
		usageRankingPanel("Top plugins", props.TopPlugins, panelWidth, props.Theme),
	}}}
}

func usageRankingPanel(title string, items []UsageRankingItem, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	rows := []woxwidget.Widget{woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}}
	if len(items) == 0 {
		rows = append(rows, woxwidget.Container{Width: width - 30, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: "No activity yet", Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}})
	}
	for index, item := range items {
		rows = append(rows, woxwidget.Container{Width: width - 30, Height: 27, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 88, Height: 27, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: fmt.Sprintf("%d  %s", index+1, item.Name), Style: woxui.TextStyle{Size: 12}, Color: theme.ResultTitle}},
			woxwidget.Container{Width: 42, Height: 27, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: fmt.Sprintf("%d", item.Count), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.Cursor}},
		}}})
	}
	return woxwidget.Container{Width: width, Height: 204, Radius: 10, Color: theme.QueryBackground, Padding: woxwidget.Insets{Left: 15, Top: 13, Right: 15, Bottom: 10}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: rows}}
}
