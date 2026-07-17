package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RuntimeStatus contains prepared runtime host presentation data.
type RuntimeStatus struct {
	Runtime      string
	DisplayName  string
	Mark         string
	Version      string
	StatusCode   string
	StatusLabel  string
	Detail       string
	PluginLabel  string
	Actionable   bool
	InstallLabel string
	RestartLabel string
	OnInstall    func()
	OnRestart    func()
}

// RuntimeSettingRow contains one prepared executable setting row.
type RuntimeSettingRow struct {
	ID      string
	Child   woxwidget.Widget
	OnHover func()
	OnTap   func()
}

// RuntimeSettingsProps contains runtime inventory and executable settings.
type RuntimeSettingsProps struct {
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	Loading       bool
	Restarting    bool
	Error         string
	Note          string
	Scroll        float32
	Statuses      []RuntimeStatus
	Settings      []RuntimeSettingRow
	OnRefresh     func()
	OnScroll      func(float32)
	OnSetGeometry func(viewport, content, rowsTop float32)
}

// RuntimeSettingsView builds runtime status cards and executable settings.
func RuntimeSettingsView(props RuntimeSettingsProps) woxwidget.Widget {
	contentWidth := max(float32(0), props.Width-72)
	refreshLabel := "Refresh status"
	if props.Loading {
		refreshLabel = "Refreshing…"
	}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{Title: "Runtime", Description: "Review plugin host status and configure executable paths", Width: max(float32(0), contentWidth-126), Height: 62, TitleSize: 24, Gap: 7, Theme: props.Theme}),
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "runtime-refresh", Label: refreshLabel, Width: 118, Disabled: props.Loading || props.Restarting, OnTap: props.OnRefresh, Theme: props.Theme}),
		}}},
	}
	statusHeight := runtimeStatusGridHeight(props.Statuses, contentWidth)
	children = append(children, runtimeStatusGrid(props, contentWidth, statusHeight))
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{
		Value: "Executable paths", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle,
	}})
	for _, row := range props.Settings {
		row := row
		children = append(children, woxwidget.Gesture{ID: row.ID, OnHover: func(inside bool) {
			if inside && row.OnHover != nil {
				row.OnHover()
			}
		}, OnTap: row.OnTap, Child: row.Child})
	}
	message := props.Note
	messageColor := props.Theme.ResultSubtitle
	if props.Error != "" {
		message = props.Error
		messageColor = props.Theme.ErrorText
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{
		Value: message, Style: woxui.TextStyle{Size: 12}, Color: messageColor,
	}})
	contentHeight := float32(166) + statusHeight + float32(len(props.Settings))*82
	viewportHeight := max(float32(1), props.Height-52)
	rowsTop := float32(132) + statusHeight
	if props.OnSetGeometry != nil {
		props.OnSetGeometry(viewportHeight, contentHeight, rowsTop)
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22}, Child: woxwidget.Gesture{
		ID: "runtime-page-scroll", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
		},
	}}
}

func runtimeStatusGridHeight(statuses []RuntimeStatus, width float32) float32 {
	if len(statuses) == 0 {
		return 86
	}
	columns := runtimeStatusColumns(width)
	height := float32(0)
	for start := 0; start < len(statuses); start += columns {
		rowHeight := float32(160)
		for index := start; index < min(start+columns, len(statuses)); index++ {
			if statuses[index].Actionable {
				rowHeight = 206
			}
		}
		height += rowHeight
		if start+columns < len(statuses) {
			height += 12
		}
	}
	return height
}

func runtimeStatusColumns(width float32) int {
	if width >= 860 {
		return 3
	}
	if width >= 560 {
		return 2
	}
	return 1
}

func runtimeStatusGrid(props RuntimeSettingsProps, width, height float32) woxwidget.Widget {
	if len(props.Statuses) == 0 {
		message := "No runtime hosts reported by Wox core"
		if props.Loading {
			message = "Loading runtime status…"
		}
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 18, Top: 28}, Child: woxwidget.Text{
			Value: message, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
	}
	columns := runtimeStatusColumns(width)
	cardWidth := (width - float32(columns-1)*12) / float32(columns)
	rows := make([]woxwidget.Widget, 0, (len(props.Statuses)+columns-1)/columns)
	for start := 0; start < len(props.Statuses); start += columns {
		end := min(start+columns, len(props.Statuses))
		rowHeight := float32(160)
		for _, status := range props.Statuses[start:end] {
			if status.Actionable {
				rowHeight = 206
			}
		}
		cards := make([]woxwidget.Widget, 0, end-start)
		for _, status := range props.Statuses[start:end] {
			cards = append(cards, runtimeStatusCard(props, status, cardWidth, rowHeight))
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: cards})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: rows}}
}

func runtimeStatusCard(props RuntimeSettingsProps, status RuntimeStatus, width, height float32) woxwidget.Widget {
	theme := props.Theme
	statusColor := runtimeStatusColor(status.StatusCode, theme)
	titleWidth := max(float32(60), width-86)
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 38, Height: 38, Radius: 8, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: woxwidget.Text{
			Value: status.Mark, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.Cursor,
		}},
		woxwidget.Container{Width: titleWidth, Height: 48, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(20), titleWidth-52), Height: 20, Child: woxwidget.Text{Value: status.DisplayName, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
				woxwidget.Text{Value: status.Version, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
			}},
			woxwidget.Container{Width: min(float32(116), titleWidth), Height: 21, Radius: 10, Color: runtimeStatusBackground(status.StatusCode, theme), Padding: woxwidget.Insets{Left: 8, Top: 4}, Child: woxwidget.Text{
				Value: status.StatusLabel, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: statusColor,
			}},
		}}},
	}}
	children := []woxwidget.Widget{
		header,
		woxwidget.TextBlock{Value: status.Detail, Width: max(float32(0), width-28), Height: 38, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: theme.ResultSubtitle},
		woxwidget.Text{Value: status.PluginLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle},
	}
	if status.Actionable {
		buttons := make([]woxwidget.Widget, 0, 2)
		if status.OnInstall != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "runtime-install-" + status.Runtime, Label: status.InstallLabel, Width: 82, Disabled: props.Restarting, OnTap: status.OnInstall, Theme: theme}))
		}
		if status.OnRestart != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "runtime-restart-" + status.Runtime, Label: status.RestartLabel, Width: 104, Disabled: props.Restarting, Variant: woxcomponent.ButtonPrimary, OnTap: status.OnRestart, Theme: theme}))
		}
		children = append(children, woxwidget.Container{Width: max(float32(0), width-28), Height: 38, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}})
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: theme.QueryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 8, Children: children,
	}}
}

func runtimeStatusColor(statusCode string, theme woxcomponent.Theme) woxui.Color {
	switch statusCode {
	case "running":
		return woxui.Color{R: 72, G: 190, B: 112, A: 255}
	case "executable_missing", "unsupported_version", "start_failed":
		return theme.ErrorText
	default:
		return woxui.Color{R: 225, G: 166, B: 64, A: 255}
	}
}

func runtimeStatusBackground(statusCode string, theme woxcomponent.Theme) woxui.Color {
	color := runtimeStatusColor(statusCode, theme)
	color.A = 42
	return color
}
