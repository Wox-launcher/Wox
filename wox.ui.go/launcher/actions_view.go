package launcher

import (
	"fmt"
	"runtime"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

const (
	actionPanelWidth  = 320
	actionRowHeight   = 46
	maxVisibleActions = 8
)

func (a *App) buildActionPanel(snapshot viewSnapshot, windowWidth, windowHeight, queryHeight, toolbarHeight float32) (woxwidget.Widget, float32, float32) {
	if snapshot.selected < 0 || snapshot.selected >= len(snapshot.results) {
		return nil, 0, 0
	}
	actions := snapshot.results[snapshot.selected].Actions
	if len(actions) == 0 {
		return nil, 0, 0
	}
	start := max(0, snapshot.actionSelected-maxVisibleActions+1)
	end := min(len(actions), start+maxVisibleActions)
	if end-start < maxVisibleActions {
		start = max(0, end-maxVisibleActions)
	}
	visible := actions[start:end]
	panelWidth := min(float32(actionPanelWidth), max(float32(240), windowWidth-28))
	panelHeight := float32(54 + len(visible)*actionRowHeight + 12)
	panelHeight = min(panelHeight, max(float32(100), windowHeight-queryHeight-toolbarHeight-20))
	rows := make([]woxwidget.Widget, 0, len(visible))
	for offset, action := range visible {
		index := start + offset
		action := action
		selected := index == snapshot.actionSelected
		background := woxui.Color{}
		foreground := snapshot.palette.actionText
		if selected {
			background = snapshot.palette.actionSelected
			foreground = snapshot.palette.actionSelectedText
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 24, Height: 24, Radius: 6, Color: snapshot.palette.cursor}
		if image := a.imageFor(action.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 24, Height: 24}
		}
		labelWidth := max(float32(80), panelWidth-24-20-68)
		rows = append(rows, woxwidget.Gesture{
			ID: "action-" + action.ID,
			OnHover: func(inside bool) {
				if inside {
					a.selectAction(index)
				}
			},
			OnTap: func() {
				a.selectAction(index)
				a.activateSelectedAction()
			},
			Child: woxwidget.Container{
				Width: panelWidth - 20, Height: actionRowHeight, Radius: 8, Color: background,
				Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10, Bottom: 8},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
					icon,
					woxwidget.Container{Width: labelWidth, Height: 26, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Text{Value: a.translate(action.Name), Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: foreground}},
					woxwidget.Container{Width: 44, Height: 26, Padding: woxwidget.Insets{Top: 3}, Child: woxwidget.Text{Value: action.Hotkey, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.actionHeader}},
				}},
			},
		})
	}
	moreHotkey := "Ctrl+J"
	if runtime.GOOS == "darwin" {
		moreHotkey = "Cmd+J"
	}
	header := fmt.Sprintf("Actions  ·  %s", moreHotkey)
	return woxwidget.Container{
		Width: panelWidth, Height: panelHeight, Radius: 12, Color: snapshot.palette.actionBackground,
		Padding: woxwidget.Insets{Left: 10, Top: 12, Right: 10, Bottom: 10},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Container{Width: panelWidth - 20, Height: 34, Padding: woxwidget.Insets{Left: 8, Top: 5}, Child: woxwidget.Text{Value: header, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionHeader}},
			woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}},
	}, panelWidth, panelHeight
}
