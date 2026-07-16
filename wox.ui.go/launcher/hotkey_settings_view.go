package launcher

import (
	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildHotkeySettingsPage renders raw hotkeys and query tables through the same portable form surface.
func (a *App) buildHotkeySettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if snapshot.hotkeyForm == nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(36), Child: woxwidget.Text{
			Value: "Hotkey settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	innerWidth := max(float32(0), width-72)
	headerHeight := float32(74)
	noteHeight := float32(34)
	bodyHeight := max(float32(80), height-60-headerHeight-noteHeight)
	a.setHotkeySettingsViewport(bodyHeight)
	callbacks := formFieldCallbacks{
		idPrefix: "hotkey-settings", focus: a.focusHotkeySettingsField, openTable: a.openHotkeySettingsTable, recordKey: a.recordHotkeySettingsField,
	}
	rows := make([]woxwidget.Widget, 0, len(snapshot.hotkeyForm.definitions))
	for index, definition := range snapshot.hotkeyForm.definitions {
		rows = append(rows, a.buildFormField(*snapshot.hotkeyForm, callbacks, snapshot.palette, index, definition, innerWidth, formDefinitionHeight(definition)))
	}
	body := woxwidget.Gesture{ID: "hotkey-settings-scroll", OnScroll: func(delta woxui.Point) { a.scrollHotkeySettings(-delta.Y) }, Child: woxwidget.ScrollView{
		Width: innerWidth, Height: bodyHeight, ContentHeight: max(bodyHeight, formDefinitionsContentHeight(snapshot.hotkeyForm.definitions)), Offset: snapshot.hotkeyForm.scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	note := snapshot.note
	if note == "" {
		note = "Core records raw hotkeys; this page only owns focus and persisted values."
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 30}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Hotkeys", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: "Global activation and reusable query launchers", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
			}}},
			body,
			woxwidget.Container{Width: innerWidth, Height: noteHeight, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{Value: note, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle}},
		},
	}}
}
