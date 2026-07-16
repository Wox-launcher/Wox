package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildRuntimeSettingsPage combines the live runtime inventory with the shared core-backed executable settings.
func (a *App) buildRuntimeSettingsPage(snapshot settingsSnapshot, items []settingItem, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-72)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), contentWidth-126), Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Runtime", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: "Review plugin host status and configure executable paths", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
			}}},
			a.buildFormTableButton("runtime-refresh", runtimeRefreshLabel(snapshot), 118, !snapshot.runtimeLoading && snapshot.runtimeRestarting == "", false, a.reloadRuntimeStatuses, snapshot.palette),
		}}},
	}

	statusHeight := runtimeStatusGridHeight(snapshot.runtimeStatuses, contentWidth)
	children = append(children, a.buildRuntimeStatusGrid(snapshot, contentWidth, statusHeight))
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Text{
		Value: "Executable paths", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle,
	}})
	for index, item := range items {
		index := index
		item := item
		background := snapshot.palette.queryBackground
		if index == snapshot.row {
			background = snapshot.palette.selectedBackground
		}
		children = append(children, woxwidget.Gesture{
			ID: "runtime-setting-" + item.key,
			OnHover: func(inside bool) {
				if inside {
					a.selectSettingRow(index)
				}
			},
			OnTap: func() {
				a.selectSettingRow(index)
				a.openOrActivateSetting()
			},
			Child: a.buildSettingRow(snapshot, item, contentWidth, background),
		})
	}
	message := snapshot.note
	messageColor := snapshot.palette.resultSubtitle
	if snapshot.runtimeError != "" {
		message = snapshot.runtimeError
		messageColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{
		Value: message, Style: woxui.TextStyle{Size: 12}, Color: messageColor,
	}})

	contentHeight := float32(166) + statusHeight + float32(len(items))*82
	viewportHeight := max(float32(1), height-52)
	rowsTop := float32(132) + statusHeight
	a.setRuntimePageGeometry(viewportHeight, contentHeight, rowsTop)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22}, Child: woxwidget.Gesture{
		ID: "runtime-page-scroll", OnScroll: func(delta woxui.Point) { a.scrollRuntimePage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.runtimePageScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
		},
	}}
}

func runtimeRefreshLabel(snapshot settingsSnapshot) string {
	if snapshot.runtimeLoading {
		return "Refreshing…"
	}
	return "Refresh status"
}

func runtimeStatusGridHeight(statuses []runtimeStatus, width float32) float32 {
	if len(statuses) == 0 {
		return 86
	}
	columns := runtimeStatusColumns(width)
	height := float32(0)
	for start := 0; start < len(statuses); start += columns {
		rowHeight := float32(160)
		for index := start; index < min(start+columns, len(statuses)); index++ {
			if runtimeStatusActionable(statuses[index]) {
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

// buildRuntimeStatusGrid keeps card heights aligned within each responsive row.
func (a *App) buildRuntimeStatusGrid(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if len(snapshot.runtimeStatuses) == 0 {
		message := "No runtime hosts reported by Wox core"
		if snapshot.runtimeLoading {
			message = "Loading runtime status…"
		}
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 28}, Child: woxwidget.Text{
			Value: message, Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	columns := runtimeStatusColumns(width)
	cardWidth := (width - float32(columns-1)*12) / float32(columns)
	rows := make([]woxwidget.Widget, 0, (len(snapshot.runtimeStatuses)+columns-1)/columns)
	for start := 0; start < len(snapshot.runtimeStatuses); start += columns {
		end := min(start+columns, len(snapshot.runtimeStatuses))
		rowHeight := float32(160)
		for _, status := range snapshot.runtimeStatuses[start:end] {
			if runtimeStatusActionable(status) {
				rowHeight = 206
			}
		}
		cards := make([]woxwidget.Widget, 0, end-start)
		for _, status := range snapshot.runtimeStatuses[start:end] {
			cards = append(cards, a.buildRuntimeStatusCard(snapshot, status, cardWidth, rowHeight))
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: cards})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: rows}}
}

// buildRuntimeStatusCard presents diagnosis and recovery actions without platform-specific widgets.
func (a *App) buildRuntimeStatusCard(snapshot settingsSnapshot, status runtimeStatus, width, height float32) woxwidget.Widget {
	statusColor := runtimeStatusColor(status.StatusCode)
	version := strings.TrimSpace(status.HostVersion)
	if version != "" && !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	mark := strings.ToUpper(runtimeDisplayName(status.Runtime))
	if len(mark) > 2 {
		mark = mark[:2]
	}
	titleWidth := max(float32(60), width-86)
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 38, Height: 38, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 9, Top: 10}, Child: woxwidget.Text{
			Value: mark, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.cursor,
		}},
		woxwidget.Container{Width: titleWidth, Height: 48, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(20), titleWidth-52), Height: 20, Child: woxwidget.Text{Value: runtimeDisplayName(status.Runtime), Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
				woxwidget.Text{Value: version, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
			}},
			woxwidget.Container{Width: min(float32(116), titleWidth), Height: 21, Radius: 10, Color: runtimeStatusBackground(status.StatusCode), Padding: woxwidget.Insets{Left: 8, Top: 4}, Child: woxwidget.Text{
				Value: runtimeStatusLabel(status), Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: statusColor,
			}},
		}}},
	}}
	detail := woxwidget.TextBlock{Value: runtimeStatusDetail(status), Width: max(float32(0), width-28), Height: 38, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: snapshot.palette.resultSubtitle}
	pluginLabel := fmt.Sprintf("%d loaded plugins", status.LoadedPluginCount)
	if status.LoadedPluginCount == 1 {
		pluginLabel = "1 loaded plugin"
	}
	children := []woxwidget.Widget{
		header,
		detail,
		woxwidget.Text{Value: pluginLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle},
	}
	if runtimeStatusActionable(status) {
		buttons := make([]woxwidget.Widget, 0, 2)
		if status.InstallURL != "" && (status.StatusCode == "executable_missing" || status.StatusCode == "unsupported_version") {
			label := "Install"
			if status.StatusCode == "unsupported_version" {
				label = "Upgrade"
			}
			current := status
			buttons = append(buttons, a.buildFormTableButton("runtime-install-"+status.Runtime, label+" ↗", 82, snapshot.runtimeRestarting == "", false, func() { a.openRuntimeInstallURL(current) }, snapshot.palette))
		}
		if status.CanRestart {
			currentRuntime := status.Runtime
			label := "Restart host"
			if strings.EqualFold(snapshot.runtimeRestarting, status.Runtime) {
				label = "Restarting…"
			}
			buttons = append(buttons, a.buildFormTableButton("runtime-restart-"+status.Runtime, label, 104, snapshot.runtimeRestarting == "", true, func() { a.restartRuntimeHost(currentRuntime) }, snapshot.palette))
		}
		children = append(children, woxwidget.Container{Width: max(float32(0), width-28), Height: 38, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}})
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 14, Top: 14, Right: 14, Bottom: 14}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 8, Children: children,
	}}
}

func runtimeStatusColor(statusCode string) woxui.Color {
	switch statusCode {
	case "running":
		return woxui.Color{R: 72, G: 190, B: 112, A: 255}
	case "executable_missing", "unsupported_version", "start_failed":
		return woxui.Color{R: 232, G: 95, B: 95, A: 255}
	default:
		return woxui.Color{R: 225, G: 166, B: 64, A: 255}
	}
}

func runtimeStatusBackground(statusCode string) woxui.Color {
	color := runtimeStatusColor(statusCode)
	color.A = 42
	return color
}
