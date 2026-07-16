package launcher

import (
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

// buildTriggerConflictPreview renders an editable conflict resolver with no native widget dependency.
func (a *App) buildTriggerConflictPreview(result queryResult, preview queryPreview, palette uiPalette, width, height float32) woxwidget.Widget {
	state, err := a.ensureTriggerConflictPreview(result, preview)
	if err != nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(18), Child: woxwidget.TextBlock{
			Value: err.Error(), Width: max(float32(0), width-36), Height: max(float32(0), height-36), Style: woxui.TextStyle{Size: 13}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255},
		}}
	}
	innerWidth := max(float32(0), width-36)
	innerHeight := max(float32(0), height-28)
	titleHeight := float32(30)
	messageHeight := float32(46)
	footerHeight := float32(48)
	errorHeight := float32(0)
	if state.error != "" {
		errorHeight = 30
	}
	bodyHeight := max(float32(56), innerHeight-titleHeight-messageHeight-footerHeight-errorHeight)
	a.setTriggerConflictViewport(state.key, bodyHeight)
	contentHeight := max(bodyHeight, formDefinitionsContentHeight(state.definitions))
	callbacks := formFieldCallbacks{idPrefix: "trigger-conflict", focus: a.focusTriggerConflictField, setCaret: a.setTriggerConflictCaret}
	rows := make([]woxwidget.Widget, 0, len(state.definitions))
	for index, definition := range state.definitions {
		rows = append(rows, a.buildFormField(state.formFieldsSnapshot, callbacks, palette, index, definition, innerWidth, formDefinitionHeight(definition)))
	}
	body := woxwidget.Gesture{ID: "trigger-conflict-scroll", OnScroll: func(delta woxui.Point) {
		a.scrollTriggerConflictPreview(state.key, -delta.Y)
	}, Child: woxwidget.ScrollView{Width: innerWidth, Height: bodyHeight, ContentHeight: contentHeight, Offset: state.scroll, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}}

	dirty := false
	for key, value := range state.values {
		if strings.Join(parseTriggerKeywords(value), ",") != strings.Join(parseTriggerKeywords(state.initial[key]), ",") {
			dirty = true
			break
		}
	}
	saveLabel := a.translate("i18n:ui_save")
	buttonColor := palette.selectedBackground
	if dirty && !state.saving {
		buttonColor = palette.actionSelected
	}
	if state.saving {
		saveLabel += "…"
	}
	button := woxwidget.Gesture{ID: "trigger-conflict-save", OnTap: a.submitTriggerConflictPreview, Child: woxwidget.Container{
		Width: 112, Height: 36, Radius: 8, Color: buttonColor, Padding: woxwidget.Insets{Left: 24, Top: 10}, Child: woxwidget.Text{
			Value: saveLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionSelectedText,
		},
	}}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{woxwidget.Painter{Width: max(float32(0), innerWidth-112), Height: footerHeight}, button}}
	title := state.title
	if title == "" {
		title = fmt.Sprintf("Resolve trigger keyword %q", state.keyword)
	}
	message := state.message
	if message == "" {
		message = "Edit one or more comma-separated keyword lists so each concrete trigger has a single owner."
	}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: innerWidth, Height: titleHeight, Child: woxwidget.Text{Value: title, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: palette.previewText}},
		woxwidget.Container{Width: innerWidth, Height: messageHeight, Child: woxwidget.TextBlock{Value: message, Width: innerWidth, Height: messageHeight, Style: woxui.TextStyle{Size: 12}, LineHeight: 17, Color: palette.resultSubtitle}},
		body,
	}
	if errorHeight > 0 {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: errorHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: state.error, Style: woxui.TextStyle{Size: 11}, Color: woxui.Color{R: 232, G: 95, B: 95, A: 255}}})
	}
	children = append(children, footer)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}
