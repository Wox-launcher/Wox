package view

import (
	"fmt"
	"sort"
	"time"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	usagePageHorizontalInset = float32(38)
	usagePageRightInset      = float32(44)
	usagePageTopInset        = float32(34)
	usagePageBottomInset     = float32(30)
	usageSectionGap          = float32(18)
	usageCardGap             = float32(12)
	usageKPIHeight           = float32(92)
	usageHeatmapPanelHeight  = float32(252)
)

// UsagePeriod describes one report period selector.
type UsagePeriod struct {
	ID       string
	Label    string
	Selected bool
	OnSelect func()
}

// UsageKPI contains one usage summary value and its semantic visual treatment.
type UsageKPI struct {
	Label  string
	Value  int64
	Icon   *woxui.Image
	Accent woxui.Color
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
	Icon  *woxui.Image
}

// UsageSettingsProps contains the local usage report presentation data.
type UsageSettingsProps struct {
	Width           float32
	Height          float32
	Theme           woxcomponent.Theme
	Title           string
	Overview        string
	ShareLabel      string
	ActivityTitle   string
	TopAppsTitle    string
	TopPluginsTitle string
	EmptyLabel      string
	Periods         []UsagePeriod
	Error           string
	Loading         bool
	KPIs            []UsageKPI
	Days            []UsageDay
	MonthLabels     []string
	TopApps         []UsageRankingItem
	TopPlugins      []UsageRankingItem
	ShareIcon       *woxui.Image
	CalendarIcon    *woxui.Image
	AppsIcon        *woxui.Image
	PluginsIcon     *woxui.Image
	AppFallbackIcon *woxui.Image
	RankIcons       []*woxui.Image
	HeatmapAccent   woxui.Color
	AppAccent       woxui.Color
	PluginAccent    woxui.Color
	OnShare         func()
}

// UsageSettingsView builds the responsive dashboard used by the Usage settings route.
func UsageSettingsView(props UsageSettingsProps) woxwidget.Widget {
	contentWidth := max(float32(0), props.Width-usagePageHorizontalInset-usagePageRightInset)
	viewportHeight := max(float32(1), props.Height-usagePageTopInset-usagePageBottomInset)
	header, headerHeight := usageSummaryHeader(props, contentWidth)
	kpiGrid, kpiHeight := usageKPIGrid(props, contentWidth)
	rankings, rankingsHeight := usageRankings(props, contentWidth)
	children := []woxwidget.Widget{header}
	contentHeight := headerHeight + kpiHeight + usageHeatmapPanelHeight + rankingsHeight + usageSectionGap*3
	if props.Error != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.TextBlock{
			Value: props.Error, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText,
		}})
		contentHeight += 30 + usageSectionGap
	}
	children = append(children, kpiGrid, usageActivityPanel(props, contentWidth), rankings)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height,
		Padding: woxwidget.Insets{Left: usagePageHorizontalInset, Top: usagePageTopInset, Right: usagePageRightInset, Bottom: usagePageBottomInset},
		Child: woxwidget.ScrollView{
			Key: "usage-page-scroll", ID: "usage-page-scroll",
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight),
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: usageSectionGap, Children: children},
		},
	}
}

// usageSummaryHeader keeps the report title, period filter, and share action on one balanced row when space permits.
func usageSummaryHeader(props UsageSettingsProps, width float32) (woxwidget.Widget, float32) {
	selector, selectorWidth := usagePeriodSelector(props)
	share, shareWidth := usageShareButton(props)
	wide := width >= selectorWidth+512
	headerHeight := float32(54)
	if !wide {
		headerHeight = 110
	}
	titleWidth := float32(240)
	if !wide {
		titleWidth = min(float32(320), max(float32(150), width-shareWidth-18))
	}
	title := props.Title
	if props.Loading {
		title += " …"
	}
	titleBlock := woxwidget.Container{Width: titleWidth, Height: 54, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 21, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		woxwidget.Clip{Width: titleWidth, Height: 20, Child: woxwidget.Text{Value: props.Overview, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle}},
	}}}
	children := []woxwidget.StackChild{{Child: titleBlock}, {Left: max(float32(0), width-shareWidth), Child: share}}
	selectorTop := float32(0)
	if !wide {
		selectorTop = 70
	}
	children = append(children, woxwidget.StackChild{Left: max(float32(0), (width-selectorWidth)/2), Top: selectorTop, Child: selector})
	return woxwidget.Stack{Width: width, Height: headerHeight, Children: children}, headerHeight
}

// usagePeriodSelector renders the reporting ranges as the same compact segmented filter used by Flutter.
func usagePeriodSelector(props UsageSettingsProps) (woxwidget.Widget, float32) {
	buttons := make([]woxwidget.Widget, 0, len(props.Periods))
	selectorWidth := float32(6)
	for _, period := range props.Periods {
		buttonWidth := usagePeriodButtonWidth(period.Label)
		selectorWidth += buttonWidth
		background := woxui.Color{}
		foreground := props.Theme.ResultSubtitle
		if period.Selected {
			background = props.Theme.SelectedBackground
			foreground = props.Theme.SelectedTitle
		}
		if props.Loading && !period.Selected {
			foreground = usageWithAlpha(foreground, 120)
		}
		onSelect := period.OnSelect
		if props.Loading || period.Selected {
			onSelect = nil
		}
		buttons = append(buttons, woxwidget.Gesture{ID: "usage-period-" + period.ID, OnTap: onSelect, Child: woxwidget.Container{
			Width: buttonWidth, Height: 32, Radius: 6, Color: background, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Align{
				Width: buttonWidth, Height: 18, Horizontal: 0.5, Child: woxwidget.Text{Value: period.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
			},
		}})
	}
	return woxwidget.Container{
		Width: selectorWidth, Height: 38, Radius: 8, Color: props.Theme.QueryBackground, BorderColor: usageOutlineColor(props.Theme), BorderWidth: 1,
		Padding: woxwidget.UniformInsets(3), Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: buttons},
	}, selectorWidth
}

func usagePeriodButtonWidth(label string) float32 {
	width := float32(24)
	for _, character := range label {
		switch {
		case character == ' ':
			width += 3.5
		case character > 127:
			width += 12
		default:
			width += 6.5
		}
	}
	return min(float32(120), max(float32(58), width))
}

// usageShareButton builds the outlined share action without giving it more visual weight than the page filter.
func usageShareButton(props UsageSettingsProps) (woxwidget.Widget, float32) {
	width := min(float32(168), max(float32(104), float32(utf8.RuneCountInString(props.ShareLabel))*7.5+48))
	foreground := props.Theme.ResultTitle
	onShare := props.OnShare
	if props.Loading {
		foreground = usageWithAlpha(props.Theme.ResultSubtitle, 130)
		onShare = nil
	}
	var icon woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if props.ShareIcon != nil {
		icon = woxwidget.Image{Source: props.ShareIcon, Width: 16, Height: 16}
	}
	return woxwidget.Gesture{ID: "usage-share-x", OnTap: onShare, Child: woxwidget.Container{
		Width: width, Height: 38, Radius: 8, Color: props.Theme.QueryBackground, BorderColor: usageOutlineColor(props.Theme), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			icon, woxwidget.Text{Value: props.ShareLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}},
	}}, width
}

// usageKPIGrid follows Flutter's four-column desktop grid and collapses cleanly on narrower settings windows.
func usageKPIGrid(props UsageSettingsProps, width float32) (woxwidget.Widget, float32) {
	columns := 4
	if width < 760 {
		columns = 2
	}
	if width < 420 {
		columns = 1
	}
	columns = min(columns, max(1, len(props.KPIs)))
	rows := max(1, (len(props.KPIs)+columns-1)/columns)
	cardWidth := max(float32(100), (width-float32(columns-1)*usageCardGap)/float32(columns))
	cards := make([]woxwidget.Widget, 0, len(props.KPIs))
	for _, item := range props.KPIs {
		cards = append(cards, usageKPICard(item, cardWidth, props.Theme))
	}
	height := float32(rows)*usageKPIHeight + float32(rows-1)*usageCardGap
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Wrap{Gap: usageCardGap, RunGap: usageCardGap, Children: cards}}, height
}

func usageKPICard(item UsageKPI, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	iconBackground := usageWithAlpha(item.Accent, 40)
	var icon woxwidget.Widget = woxwidget.Container{Width: 22, Height: 22}
	if item.Icon != nil {
		icon = woxwidget.Image{Source: item.Icon, Width: 22, Height: 22}
	}
	labelWidth := max(float32(20), width-98)
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: usageKPIHeight, Padding: woxwidget.UniformInsets(14), BorderColor: usageOutlineColor(theme), Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Container{Width: 46, Height: 46, Radius: 8, Color: iconBackground, Child: woxwidget.Align{Width: 46, Height: 46, Horizontal: 0.5, Vertical: 0.5, Child: icon}},
			woxwidget.Container{Width: labelWidth, Height: 50, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
				woxwidget.Clip{Width: labelWidth, Height: 18, Child: woxwidget.Text{Value: item.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
				woxwidget.Text{Value: fmt.Sprintf("%d", item.Value), Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
			}}},
		}},
	})
}

// usageActivityPanel gives the yearly heatmap the same full-width card and centered reading area as Flutter.
func usageActivityPanel(props UsageSettingsProps, width float32) woxwidget.Widget {
	var icon woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if props.CalendarIcon != nil {
		icon = woxwidget.Image{Source: props.CalendarIcon, Width: 16, Height: 16}
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		icon, woxwidget.Text{Value: props.ActivityTitle, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
	}}
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: usageHeatmapPanelHeight, Padding: woxwidget.UniformInsets(16), BorderColor: usageOutlineColor(props.Theme), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{
			header,
			woxwidget.Painter{Width: max(float32(0), width-32), Height: 188, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
				drawUsageHeatmap(displayList, bounds, props.Days, props.MonthLabels, props.EmptyLabel, props.HeatmapAccent, props.Theme)
			}},
		}},
	})
}

type usageHeatmapDay struct {
	date  time.Time
	count int64
}

type usageHeatmapThresholds struct {
	low    int64
	medium int64
	high   int64
}

// drawUsageHeatmap maps one year of local dates into Sunday-first weeks with localized month labels.
func drawUsageHeatmap(displayList *woxui.DisplayList, bounds woxui.Rect, source []UsageDay, monthLabels []string, emptyLabel string, accent woxui.Color, theme woxcomponent.Theme) {
	days := make([]usageHeatmapDay, 0, len(source))
	for _, day := range source {
		date, err := time.ParseInLocation("2006-01-02", day.Date, time.Local)
		if err == nil {
			days = append(days, usageHeatmapDay{date: date, count: day.Count})
		}
	}
	if len(days) == 0 {
		displayList.DrawText(emptyLabel, woxui.Rect{X: bounds.X, Y: bounds.Y + bounds.Height/2 - 8, Width: bounds.Width, Height: 18}, woxui.TextStyle{Size: 12}, theme.ResultSubtitle)
		return
	}
	sort.Slice(days, func(i, j int) bool { return days[i].date.Before(days[j].date) })
	firstOffset := int(days[0].date.Weekday())
	weekCount := max(1, (firstOffset+len(days)+6)/7)
	const cellGap = float32(2.5)
	const monthHeight = float32(18)
	availableGridWidth := max(float32(0), bounds.Width-8)
	cellSize := min(float32(13), max(float32(5), (availableGridWidth-cellGap*float32(weekCount-1))/float32(weekCount)))
	gridWidth := cellSize*float32(weekCount) + cellGap*float32(weekCount-1)
	gridHeight := cellSize*7 + cellGap*6
	gridLeft := bounds.X + max(float32(0), (bounds.Width-gridWidth)/2)
	totalHeight := monthHeight + gridHeight
	monthTop := bounds.Y + max(float32(0), (bounds.Height-totalHeight)/2)
	gridTop := monthTop + monthHeight

	maxCount := int64(0)
	positive := make([]int64, 0, len(days))
	for _, day := range days {
		maxCount = max(maxCount, day.count)
		if day.count > 0 {
			positive = append(positive, day.count)
		}
	}
	thresholds := usageHeatmapThresholdValues(positive)
	emptyColor := usageHeatmapEmptyColor(theme)
	outline := usageOutlineColor(theme)
	seenMonths := map[int]bool{}
	for index, day := range days {
		position := firstOffset + index
		column := position / 7
		row := position % 7
		x := gridLeft + float32(column)*(cellSize+cellGap)
		y := gridTop + float32(row)*(cellSize+cellGap)
		color := emptyColor
		if day.count > 0 && maxCount > 0 {
			color = usageHeatmapColor(day.count, thresholds, accent, theme)
		}
		cellBounds := woxui.Rect{X: x, Y: y, Width: cellSize, Height: cellSize}
		displayList.FillRoundedRect(cellBounds, 3, color)
		displayList.StrokeRoundedRect(cellBounds, 3, 1, outline)

		monthKey := day.date.Year()*100 + int(day.date.Month())
		if !seenMonths[monthKey] {
			seenMonths[monthKey] = true
			label := fmt.Sprintf("%d", day.date.Month())
			if monthIndex := int(day.date.Month()) - 1; monthIndex >= 0 && monthIndex < len(monthLabels) {
				label = monthLabels[monthIndex]
			}
			labelX := min(gridLeft+float32(column)*(cellSize+cellGap), gridLeft+max(float32(0), gridWidth-32))
			displayList.DrawText(label, woxui.Rect{X: labelX, Y: monthTop, Width: 32, Height: 14}, woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, theme.ResultSubtitle)
		}
	}
}

func usageHeatmapThresholdValues(positive []int64) usageHeatmapThresholds {
	if len(positive) == 0 {
		return usageHeatmapThresholds{}
	}
	sort.Slice(positive, func(i, j int) bool { return positive[i] < positive[j] })
	percentile := func(value float64) int64 {
		index := int(float64(len(positive)-1) * value)
		return positive[min(max(0, index), len(positive)-1)]
	}
	return usageHeatmapThresholds{low: percentile(0.25), medium: percentile(0.50), high: percentile(0.75)}
}

func usageHeatmapColor(count int64, thresholds usageHeatmapThresholds, accent woxui.Color, theme woxcomponent.Theme) woxui.Color {
	level := 1
	if count > thresholds.high {
		level = 4
	} else if count > thresholds.medium {
		level = 3
	} else if count > thresholds.low {
		level = 2
	}
	baseAlpha := 46
	step := 46
	if usageThemeIsDark(theme) {
		baseAlpha = 56
		step = 41
	}
	return usageWithAlpha(accent, uint8(min(255, baseAlpha+level*step)))
}

// usageRankings preserves the paired desktop panels while stacking them for compact widths.
func usageRankings(props UsageSettingsProps, width float32) (woxwidget.Widget, float32) {
	if width >= 760 {
		panelWidth := max(float32(180), (width-usageCardGap)/2)
		apps, appsHeight := usageRankingPanel(props.TopAppsTitle, props.AppsIcon, props.TopApps, panelWidth, props.EmptyLabel, props.AppAccent, true, props.AppFallbackIcon, props.RankIcons, props.Theme)
		plugins, pluginsHeight := usageRankingPanel(props.TopPluginsTitle, props.PluginsIcon, props.TopPlugins, panelWidth, props.EmptyLabel, props.PluginAccent, false, nil, props.RankIcons, props.Theme)
		height := max(appsHeight, pluginsHeight)
		return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: usageCardGap, Children: []woxwidget.Widget{apps, plugins}}}, height
	}
	apps, appsHeight := usageRankingPanel(props.TopAppsTitle, props.AppsIcon, props.TopApps, width, props.EmptyLabel, props.AppAccent, true, props.AppFallbackIcon, props.RankIcons, props.Theme)
	plugins, pluginsHeight := usageRankingPanel(props.TopPluginsTitle, props.PluginsIcon, props.TopPlugins, width, props.EmptyLabel, props.PluginAccent, false, nil, props.RankIcons, props.Theme)
	height := appsHeight + usageCardGap + pluginsHeight
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: usageCardGap, Children: []woxwidget.Widget{apps, plugins}}}, height
}

// usageRankingPanel combines rank, optional application imagery, a thin progress meter, and the exact count.
func usageRankingPanel(title string, titleIcon *woxui.Image, items []UsageRankingItem, width float32, emptyLabel string, accent woxui.Color, showItemIcons bool, fallbackIcon *woxui.Image, rankIcons []*woxui.Image, theme woxcomponent.Theme) (woxwidget.Widget, float32) {
	panelHeight := float32(136)
	if len(items) > 0 {
		panelHeight = 64 + float32(len(items))*34
	}
	var icon woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if titleIcon != nil {
		icon = woxwidget.Image{Source: titleIcon, Width: 16, Height: 16}
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		icon, woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
	}}
	var body woxwidget.Widget
	if len(items) == 0 {
		body = woxwidget.Align{Width: max(float32(0), width-32), Height: 72, Vertical: 0.5, Child: woxwidget.Text{Value: emptyLabel, Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}}
	} else {
		maxCount := int64(1)
		for _, item := range items {
			maxCount = max(maxCount, item.Count)
		}
		rows := make([]woxwidget.Widget, 0, len(items))
		for index, item := range items {
			rows = append(rows, usageRankingRow(index, item, maxCount, max(float32(0), width-32), accent, showItemIcons, fallbackIcon, rankIcons, theme))
		}
		body = woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}
	}
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: panelHeight, Padding: woxwidget.UniformInsets(16), BorderColor: usageOutlineColor(theme), Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{header, body}},
	}), panelHeight
}

func usageRankingRow(index int, item UsageRankingItem, maxCount int64, width float32, accent woxui.Color, showItemIcon bool, fallbackIcon *woxui.Image, rankIcons []*woxui.Image, theme woxcomponent.Theme) woxwidget.Widget {
	iconSlotWidth := float32(0)
	if showItemIcon {
		iconSlotWidth = 26
	}
	progressWidth := max(float32(54), width*0.34)
	nameWidth := max(float32(56), width-24-iconSlotWidth-12-progressWidth-10-32)
	children := []woxwidget.Widget{usageRankVisual(index, rankIcons, theme)}
	if showItemIcon {
		itemIcon := item.Icon
		if itemIcon == nil {
			itemIcon = fallbackIcon
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 18, Height: 18, Radius: 4, Color: usageWithAlpha(accent, 28)}
		if itemIcon != nil {
			icon = woxwidget.Image{Source: itemIcon, Width: 18, Height: 18}
		}
		children = append(children, woxwidget.Container{Width: 26, Height: 24, Padding: woxwidget.Insets{Top: 3, Right: 8}, Child: icon})
	}
	children = append(children,
		woxwidget.Clip{Width: nameWidth, Height: 24, Child: woxwidget.Container{Width: nameWidth, Height: 24, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultTitle}}},
		woxwidget.Container{Width: 12, Height: 24},
		usageRankingProgress(progressWidth, item.Count, maxCount, accent, theme),
		woxwidget.Container{Width: 10, Height: 24},
		woxwidget.Align{Width: 32, Height: 24, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{Value: fmt.Sprintf("%d", item.Count), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
	)
	return woxwidget.Container{Width: width, Height: 34, Padding: woxwidget.Insets{Top: 5, Bottom: 5}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}}
}

func usageRankVisual(index int, rankIcons []*woxui.Image, theme woxcomponent.Theme) woxwidget.Widget {
	if index < 3 && index < len(rankIcons) && rankIcons[index] != nil {
		return woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: rankIcons[index], Width: 16, Height: 16}}
	}
	return woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: fmt.Sprintf("%d", index+1), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}}
}

func usageRankingProgress(width float32, count, maxCount int64, accent woxui.Color, theme woxcomponent.Theme) woxwidget.Widget {
	progress := float32(0)
	if maxCount > 0 {
		progress = min(float32(1), max(float32(0), float32(count)/float32(maxCount)))
	}
	track := usageWithAlpha(theme.ResultTitle, 18)
	return woxwidget.Container{Width: width, Height: 24, Padding: woxwidget.Insets{Top: 10.5, Bottom: 10.5}, Child: woxwidget.Stack{Width: width, Height: 3, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: width, Height: 3, Radius: 2, Color: track}},
		{Child: woxwidget.Container{Width: width * progress, Height: 3, Radius: 2, Color: usageWithAlpha(accent, 184)}},
	}}}
}

func usageOutlineColor(theme woxcomponent.Theme) woxui.Color {
	color := theme.PreviewSplit
	if color.A == 0 {
		color = theme.ResultSubtitle
	}
	return usageWithAlpha(color, 34)
}

func usageHeatmapEmptyColor(theme woxcomponent.Theme) woxui.Color {
	if usageThemeIsDark(theme) {
		return usageWithAlpha(theme.ResultTitle, 18)
	}
	return woxui.Color{R: 232, G: 237, B: 243, A: 255}
}

func usageThemeIsDark(theme woxcomponent.Theme) bool {
	luminance := int(theme.Background.R)*299 + int(theme.Background.G)*587 + int(theme.Background.B)*114
	return luminance < 128000
}

func usageWithAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
