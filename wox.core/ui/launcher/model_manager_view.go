package launcher

import (
	"fmt"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) buildModelManagerOverlay(snapshot *modelManagerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(780), width-28))
	panelHeight := max(float32(0), min(float32(660), height-28))
	left := max(float32(14), (width-panelWidth)/2)
	top := max(float32(14), (height-panelHeight)/2)
	shade := palette.background
	shade.A = 210
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "model-manager-shade", OnTap: func() {}, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: width, Height: height, Color: shade}}},
		{Left: left, Top: top, Child: a.buildModelManagerPanel(snapshot, palette, panelWidth, panelHeight)},
	}}
}

func (a *App) buildModelManagerPanel(snapshot *modelManagerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	headerHeight := float32(54)
	engineHeight := float32(72)
	footerHeight := float32(58)
	statusHeight := float32(28)
	viewportHeight := max(float32(82), height-headerHeight-engineHeight-footerHeight-statusHeight-32)
	a.setModelManagerViewport(viewportHeight)
	title := "Dictation models"
	if snapshot.kind == "ocrModel" {
		title = "OCR models"
	}
	header := woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
		woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
		woxwidget.Text{Value: "Core owns model files and downloads; this portable page owns selection and progress state.", Style: woxui.TextStyle{Size: 10}, Color: palette.actionHeader},
	}}}
	engine := a.buildModelEngineCard(snapshot, palette, innerWidth, engineHeight)
	rows := make([]woxwidget.Widget, 0, len(snapshot.options))
	for index, option := range snapshot.options {
		index := index
		option := option
		rows = append(rows, a.buildModelManagerRow(snapshot, palette, index, option, innerWidth))
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: innerWidth, Height: viewportHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No model options were returned by the plugin.", Style: woxui.TextStyle{Size: 12}, Color: palette.actionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "model-manager-list", OnScroll: func(delta woxui.Point) { a.scrollModelManager(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*modelManagerRowHeight), Offset: snapshot.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	status := snapshot.error
	if status == "" {
		if snapshot.loading {
			status = "Refreshing model and engine status…"
		} else {
			status = "↑↓ select · Enter download/select · Delete removes a dictation model · Ctrl+R refresh"
		}
	}
	statusColor := palette.actionHeader
	if snapshot.error != "" {
		statusColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-216), Height: 40},
		a.buildFormTableButton("model-manager-refresh", "Refresh", 104, !snapshot.loading && snapshot.busy == "", false, func() {
			a.mu.RLock()
			state := a.modelManager
			a.mu.RUnlock()
			if state != nil {
				go a.refreshModelManager(state)
			}
		}, palette),
		a.buildFormTableButton("model-manager-close", "Close", 104, true, true, a.closeModelManager, palette),
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 12, Color: palette.actionBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		header,
		engine,
		list,
		woxwidget.Container{Width: innerWidth, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{Value: status, Width: innerWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
		woxwidget.Container{Width: innerWidth, Height: footerHeight, Padding: woxwidget.Insets{Top: 10}, Child: footer},
	}}}
}

func (a *App) buildModelEngineCard(snapshot *modelManagerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	label := "Checking inference engine…"
	canDownload := false
	buttonLabel := "Download engine"
	if snapshot.engine.Known {
		if snapshot.engine.Ready {
			label = "Inference engine ready"
		} else {
			switch snapshot.engine.State {
			case "downloading", "extracting", "finalizing":
				label = fmt.Sprintf("Engine %s · %d%%", snapshot.engine.State, snapshot.engine.Progress)
			case "failed":
				label = "Engine failed"
				buttonLabel = "Retry engine"
				canDownload = snapshot.busy == "" && !snapshot.loading
			default:
				label = "Inference engine is not installed"
				canDownload = snapshot.busy == "" && !snapshot.loading
			}
		}
	}
	if snapshot.engine.Error != "" {
		label += " · " + snapshot.engine.Error
	}
	button := a.buildFormTableButton("model-manager-engine", buttonLabel, 132, canDownload, false, func() { a.runModelManagerAction("engine", -1) }, palette)
	return woxwidget.Container{Width: width, Height: height - 8, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(120), width-156), Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Runtime engine", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
			woxwidget.TextBlock{Value: label, Width: max(float32(100), width-156), Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: palette.actionHeader},
		}}},
		button,
	}}}
}

func (a *App) buildModelManagerRow(snapshot *modelManagerSnapshot, palette uiPalette, index int, option formOption, width float32) woxwidget.Widget {
	background := palette.queryBackground
	foreground := palette.actionText
	if index == snapshot.selectedRow {
		background = palette.selectedBackground
		foreground = palette.selectedTitle
	}
	selected := modelOptionID(option) == snapshot.selected
	usable := modelOptionUsable(snapshot.kind, option)
	actionLabel := "Download"
	actionEnabled := snapshot.busy == "" && !snapshot.loading
	action := func() { a.runModelManagerAction("download", index) }
	if usable {
		actionLabel = "Select"
		actionEnabled = actionEnabled && !selected
		action = func() { a.chooseManagedModel(index) }
	} else if snapshot.kind == "ocrModel" && option.Status == "downloaded" && !option.Available {
		actionLabel = "Unavailable"
		actionEnabled = false
	} else if option.Status == "downloading" || option.Status == "extracting" || option.Status == "finalizing" {
		actionLabel = fmt.Sprintf("%d%%", option.DownloadProgress)
		actionEnabled = false
	} else if option.Status == "failed" {
		actionLabel = "Retry"
	}
	if selected {
		actionLabel = "Selected"
	}
	deleteWidth := float32(0)
	buttons := []woxwidget.Widget{}
	if snapshot.kind == "dictationModel" && option.Status == "downloaded" {
		deleteWidth = 76
		buttons = append(buttons, a.buildFormTableButton(fmt.Sprintf("model-delete-%d", index), "Delete", deleteWidth, snapshot.busy == "" && !snapshot.loading, false, func() { a.runModelManagerAction("delete", index) }, palette))
	}
	buttons = append(buttons, a.buildFormTableButton(fmt.Sprintf("model-action-%d", index), actionLabel, 96, actionEnabled, usable, action, palette))
	buttonWidth := float32(96) + deleteWidth
	if deleteWidth > 0 {
		buttonWidth += 8
	}
	detailWidth := max(float32(120), width-buttonWidth-42)
	detail := strings.TrimSpace(option.Description)
	if option.Languages != "" {
		if detail != "" {
			detail += " · "
		}
		detail += option.Languages
	}
	if detail == "" {
		detail = modelStatusLabel(option)
	}
	name := modelOptionLabel(option)
	if option.Recommended {
		name += " · Recommended"
	}
	return woxwidget.Gesture{ID: fmt.Sprintf("model-row-%d", index), OnTap: func() { a.selectModelManagerRow(index) }, Child: woxwidget.Container{
		Width: width, Height: modelManagerRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: detailWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
				woxwidget.Text{Value: name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
				woxwidget.TextBlock{Value: detail, Width: detailWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: palette.actionHeader},
				woxwidget.TextBlock{Value: modelStatusLabel(option), Width: detailWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: palette.cursor},
			}}},
			woxwidget.Container{Width: buttonWidth, Height: 48, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
		},
		}}}
}
