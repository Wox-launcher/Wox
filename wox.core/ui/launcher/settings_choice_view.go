package launcher

import (
	"fmt"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) buildSettingChoicePickerOverlay(snapshot *settingChoicePickerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(620), width-40)
	panelHeight := min(float32(650), height-40)
	left := max(float32(20), (width-panelWidth)/2)
	top := max(float32(20), (height-panelHeight)/2)
	shade := palette.background
	shade.A = 210
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "setting-choice-shade", OnTap: func() {}, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: width, Height: height, Color: shade}}},
		{Left: left, Top: top, Child: a.buildSettingChoicePickerPanel(snapshot, palette, panelWidth, panelHeight)},
	}}
}

func (a *App) buildSettingChoicePickerPanel(snapshot *settingChoicePickerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	window := a.settingsNativeWindow()
	innerWidth := width - 32
	headerHeight := float32(46)
	searchHeight := float32(48)
	footerHeight := float32(52)
	viewportHeight := max(float32(46), height-headerHeight-searchHeight-footerHeight-32)
	a.setSettingChoicePickerViewport(viewportHeight)
	style := woxui.TextStyle{Size: 13}
	search := woxwidget.Gesture{ID: "setting-choice-search", OnTapAt: func(position woxui.Point) {
		offset := formTextOffsetAt(snapshot.query, window, style, 1, innerWidth-24, woxui.Point{X: max(float32(0), position.X-12), Y: max(float32(0), position.Y-9)})
		a.setSettingChoicePickerCaret(offset)
	}, Child: woxwidget.Container{Width: innerWidth, Height: 40, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 7}, Child: woxwidget.Painter{
		Width: innerWidth - 24, Height: 24, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			state := snapshot.query
			if state.Text == "" {
				displayList.DrawText("Filter choices…", bounds, style, palette.resultSubtitle)
				_ = window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: bounds.X, Y: bounds.Y, Width: 1, Height: 22}})
				return
			}
			drawFormEditor(displayList, bounds, state, style, palette, true, 1, window)
		},
	}}}
	rows := make([]woxwidget.Widget, 0, len(snapshot.choices))
	for index, choice := range snapshot.choices {
		index := index
		background := palette.queryBackground
		foreground := palette.actionText
		if index == snapshot.selected {
			background = palette.selectedBackground
			foreground = palette.selectedTitle
		}
		mark := ""
		if choice.value == snapshot.item.value {
			mark = "  ✓"
		}
		rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("setting-choice-%d", index), OnTap: func() { a.chooseSettingChoice(index) }, Child: woxwidget.Container{
			Width: innerWidth, Height: settingChoicePickerRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 14, Top: 14, Right: 12},
			Child: woxwidget.Text{Value: choice.label + mark, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: innerWidth, Height: viewportHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No matching choices", Style: woxui.TextStyle{Size: 12}, Color: palette.actionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "setting-choice-list", OnScroll: func(delta woxui.Point) { a.scrollSettingChoicePicker(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*settingChoicePickerRowHeight), Offset: snapshot.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), innerWidth-112), Height: 40, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{
			Value: fmt.Sprintf("%d choices · type to filter · ↑↓ move · Enter select", len(snapshot.choices)), Style: woxui.TextStyle{Size: 10}, Color: palette.actionHeader,
		}},
		a.buildFormTableButton("setting-choice-cancel", "Cancel", 104, true, false, a.closeSettingChoicePicker, palette),
	}}
	return woxwidget.Container{Width: width, Height: height, Radius: 12, Color: palette.actionBackground, Padding: woxwidget.UniformInsets(16), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Text{Value: snapshot.item.title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: palette.actionText}},
		woxwidget.Container{Width: innerWidth, Height: searchHeight, Child: search},
		list,
		woxwidget.Container{Width: innerWidth, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: footer},
	}}}
}
