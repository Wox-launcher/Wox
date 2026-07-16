package launcher

import (
	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

const (
	actionPanelContentWidth = 320
	actionRowHeight         = 40
	actionHeaderHeight      = 16
	actionDividerHeight     = 16
	actionSearchHeight      = 46
	maxVisibleActions       = 8
)

func actionPanelBaseHeightForPalette(palette uiPalette) float32 {
	return actionHeaderHeight + actionDividerHeight + actionSearchHeight + palette.actionPadding.Top + palette.actionPadding.Bottom
}

func (a *App) buildActionPanel(snapshot viewSnapshot, windowWidth, windowHeight, queryHeight, toolbarHeight float32) (woxwidget.Widget, float32, float32) {
	if snapshot.selected < 0 || snapshot.selected >= len(snapshot.results) {
		return nil, 0, 0
	}
	actions := snapshot.results[snapshot.selected].Actions
	if len(actions) == 0 {
		return nil, 0, 0
	}
	panelWidth := min(float32(actionPanelContentWidth)+snapshot.palette.actionPadding.Left+snapshot.palette.actionPadding.Right, max(float32(240), windowWidth-28))
	innerWidth := max(float32(0), panelWidth-snapshot.palette.actionPadding.Left-snapshot.palette.actionPadding.Right)
	visibleRows := max(1, min(len(snapshot.actionIndices), maxVisibleActions))
	panelHeight := actionPanelBaseHeightForPalette(snapshot.palette) + float32(visibleRows*actionRowHeight)
	panelHeight = min(panelHeight, max(float32(100), windowHeight-queryHeight-toolbarHeight-20))
	rows := make([]woxwidget.Widget, 0, max(1, len(snapshot.actionIndices)))
	for _, index := range snapshot.actionIndices {
		action := actions[index]
		selected := index == snapshot.actionSelected
		background := woxui.Color{}
		foreground := snapshot.palette.actionText
		if selected {
			background = snapshot.palette.actionSelected
			foreground = snapshot.palette.actionSelectedText
		}
		var icon woxwidget.Widget = woxwidget.Painter{Width: 22, Height: 22}
		if image := a.imageFor(action.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 22, Height: 22}
		}
		hotkeyWidth := float32(0)
		var hotkey woxwidget.Widget = woxwidget.Painter{}
		if action.Hotkey != "" {
			tailColor := snapshot.palette.resultTail
			chipBackground := snapshot.palette.actionBackground
			if selected {
				tailColor = snapshot.palette.selectedTail
				chipBackground = snapshot.palette.actionSelected
			}
			chip, chipWidth := a.buildHotkeyView(action.Hotkey, tailColor, chipBackground)
			hotkeyWidth = chipWidth + 15
			hotkey = woxwidget.Container{Width: hotkeyWidth, Height: actionRowHeight, Padding: woxwidget.Insets{Left: 10, Top: 6, Right: 5, Bottom: 6}, Child: chip}
		}
		labelWidth := max(float32(40), innerWidth-37-hotkeyWidth)
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
				Width: innerWidth, Height: actionRowHeight, Radius: snapshot.palette.resultItemRadius, Color: background,
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
					woxwidget.Container{Width: 37, Height: actionRowHeight, Padding: woxwidget.Insets{Left: 5, Top: 9, Right: 10, Bottom: 9}, Child: icon},
					woxwidget.Container{Width: labelWidth, Height: actionRowHeight, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{Value: a.translate(action.Name), Style: woxui.TextStyle{Size: 13}, Color: foreground}},
					hotkey,
				}},
			},
		})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: innerWidth, Height: actionRowHeight, Padding: woxwidget.Insets{Left: 8, Top: 13}, Child: woxwidget.Text{
			Value: a.translate("i18n:ui_no_matches"), Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.actionHeader,
		}})
	}
	listHeight := float32(visibleRows * actionRowHeight)
	listContentHeight := float32(len(rows) * actionRowHeight)
	actionOffset := a.configureActionScroll(len(snapshot.actionIndices))
	listChildren := []woxwidget.StackChild{{Child: woxwidget.ScrollView{Width: innerWidth, Height: listHeight, ContentHeight: listContentHeight, Offset: actionOffset, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}}}
	if len(snapshot.actionIndices) > maxVisibleActions {
		thumbHeight := max(float32(24), listHeight*listHeight/listContentHeight)
		thumbTop := (listHeight - thumbHeight) * actionOffset / (listContentHeight - listHeight)
		thumbColor := snapshot.palette.actionHeader
		thumbColor.A = min(150, thumbColor.A)
		listChildren = append(listChildren, woxwidget.StackChild{Left: max(float32(0), innerWidth-5), Top: thumbTop, Child: woxwidget.Container{Width: 3, Height: thumbHeight, Radius: 2, Color: thumbColor}})
	}
	actionList := woxwidget.Gesture{ID: "action-scroll", OnScroll: func(delta woxui.Point) { a.scrollActions(-delta.Y, len(snapshot.actionIndices)) }, Child: woxwidget.Stack{Width: innerWidth, Height: listHeight, Children: listChildren}}
	searchPalette := snapshot.palette
	searchPalette.actionText = snapshot.palette.actionQueryText
	searchStyle := woxui.TextStyle{Size: 12}
	search := woxwidget.Gesture{ID: "action-search", OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.actionEditing, a.window, searchStyle, 1, innerWidth-16, woxui.Point{X: max(float32(0), position.X-8), Y: 0})
		a.setActionFilterCaret(offset)
	}, Child: woxwidget.Container{Width: innerWidth, Height: 40, Radius: snapshot.palette.actionQueryRadius, Color: snapshot.palette.actionQueryBackground, Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 8}, Child: woxwidget.Clip{
		Width: innerWidth - 16, Height: 22, Child: woxwidget.Painter{Width: innerWidth - 16, Height: 22, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			drawFormEditor(displayList, bounds, snapshot.actionEditing, searchStyle, searchPalette, true, 1, a.window)
		}},
	}}}
	return woxwidget.Container{
		Width: panelWidth, Height: panelHeight, Radius: snapshot.palette.actionQueryRadius, Color: snapshot.palette.actionBackground,
		Padding: snapshot.palette.actionPadding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: actionHeaderHeight, Child: woxwidget.Text{Value: a.translate("i18n:ui_actions"), Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.actionHeader}},
			woxwidget.Container{Width: innerWidth, Height: actionDividerHeight, Padding: woxwidget.Insets{Top: 7, Bottom: 8}, Child: woxwidget.Container{Width: innerWidth, Height: 1, Color: snapshot.palette.previewSplit}},
			actionList,
			woxwidget.Container{Width: innerWidth, Height: actionSearchHeight, Padding: woxwidget.Insets{Top: 6}, Child: search},
		}},
	}, panelWidth, panelHeight
}
