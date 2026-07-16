package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

type formFieldCallbacks struct {
	idPrefix  string
	focus     func(index int)
	change    func(index, delta int)
	setCaret  func(index, offset int)
	openTable func(index int)
	pickDir   func(index int)
	pickApp   func(index int)
	recordKey func(index int)
	openModel func(index int)
}

func (a *App) buildFormPanel(snapshot viewSnapshot, windowWidth float32) (woxwidget.Widget, float32, float32) {
	form := snapshot.form
	panelWidth := min(float32(520), max(float32(320), windowWidth-28))
	panelHeight := float32(formDefinitionsPanelHeight(form.action.Form))
	rows := make([]woxwidget.Widget, 0, len(form.action.Form))
	bodyLimit := panelHeight - 100
	for index, definition := range form.action.Form {
		rowHeight := formDefinitionHeight(definition)
		rows = append(rows, a.buildFormDefinition(snapshot, index, definition, panelWidth-28, rowHeight))
	}
	contentHeight := max(bodyLimit, formDefinitionsContentHeight(form.action.Form))
	body := woxwidget.Gesture{
		ID: "form-scroll",
		OnScroll: func(delta woxui.Point) {
			a.scrollForm(-delta.Y)
		},
		Child: woxwidget.ScrollView{
			Width: panelWidth - 28, Height: bodyLimit, ContentHeight: contentHeight, Offset: form.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		},
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), panelWidth-28-210), Height: 36},
		woxwidget.Gesture{ID: "form-cancel", OnTap: a.closeFormAction, Child: woxwidget.Container{
			Width: 86, Height: 36, Radius: 8, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 10},
			Child: woxwidget.Text{Value: a.translate("i18n:ui_cancel"), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionText},
		}},
		woxwidget.Gesture{ID: "form-save", OnTap: a.submitFormAction, Child: woxwidget.Container{
			Width: 104, Height: 36, Radius: 8, Color: snapshot.palette.actionSelected, Padding: woxwidget.Insets{Left: 20, Top: 10},
			Child: woxwidget.Text{Value: a.translate("i18n:ui_save"), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionSelectedText},
		}},
	}}
	panel := woxwidget.Container{
		Width: panelWidth, Height: panelHeight, Radius: 12, Color: snapshot.palette.actionBackground,
		Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14, Bottom: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Container{Width: panelWidth - 28, Height: 28, Child: woxwidget.Text{
				Value: a.translate(form.action.Name), Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.actionText,
			}},
			body,
			buttons,
		}},
	}
	return panel, panelWidth, panelHeight
}

func formDefinitionHeight(definition formDefinition) float32 {
	switch definition.Type {
	case "head", "label":
		return 34
	case "newline":
		return 12
	case "textbox":
		if definition.Value.MaxLines > 1 {
			// ponytail: Eight visible lines keep compact launcher forms bounded; scrolling handles longer values.
			return 32 + float32(min(definition.Value.MaxLines, 8))*20
		}
		return 56
	case "table":
		return 132
	case "dictationModel", "ocrModel":
		return 70
	default:
		return 56
	}
}

func (a *App) buildFormDefinition(snapshot viewSnapshot, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	callbacks := formFieldCallbacks{idPrefix: "action-form", focus: a.focusFormField, change: a.changeFormChoice, setCaret: a.setFormCaret, openTable: a.openActionFormTable}
	return a.buildFormField(snapshot.form.formFieldsSnapshot, callbacks, snapshot.palette, index, definition, width, height)
}

// buildFormField renders one portable form definition independently of its business owner.
func (a *App) buildFormField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	value := definition.Value
	switch definition.Type {
	case "head":
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
			Value: a.translate(value.Content), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
		}}
	case "label":
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
			Value: a.translate(value.Content), Style: woxui.TextStyle{Size: 12}, Color: palette.actionHeader,
		}}
	case "newline":
		return woxwidget.Painter{Width: width, Height: height}
	case "textbox", "password", "dirPath":
		return a.buildFormTextbox(fields, callbacks, palette, index, definition, width, height)
	case "checkbox":
		return a.buildFormChoice(fields, callbacks, palette, index, definition, width, height, fields.values[value.Key] == "true", "")
	case "hotkey", "dictationHotkey":
		return a.buildFormHotkey(fields, callbacks, palette, index, definition, width, height)
	case "app":
		return a.buildFormApp(fields, callbacks, palette, index, definition, width, height)
	case "select", "selectAIModel":
		selectedLabel := fields.values[value.Key]
		for _, option := range value.Options {
			if option.Value == selectedLabel {
				selectedLabel = a.translate(option.Label)
				break
			}
		}
		return a.buildFormChoice(fields, callbacks, palette, index, definition, width, height, false, selectedLabel)
	case "table":
		return a.buildFormTableField(fields, callbacks, palette, index, definition, width, height)
	case "dictationModel", "ocrModel":
		return a.buildFormModelField(fields, callbacks, palette, index, definition, width, height)
	default:
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{
			Value: fmt.Sprintf("Unsupported form field: %s", definition.Type), Style: woxui.TextStyle{Size: 11}, Color: palette.actionHeader,
		}}
	}
}

// buildFormModelField keeps downloadable model details behind one portable manager overlay.
func (a *App) buildFormModelField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	selectedID := fields.values[definition.Value.Key]
	selectedLabel := selectedID
	status := "Manage models"
	for _, option := range definition.Value.Options {
		if modelOptionID(option) != selectedID {
			continue
		}
		selectedLabel = modelOptionLabel(option)
		if option.Status != "" {
			status = modelStatusLabel(option)
		}
		break
	}
	if strings.TrimSpace(selectedLabel) == "" {
		selectedLabel = "No model selected"
	}
	return woxwidget.Gesture{ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), OnTap: func() {
		callbacks.focus(index)
		if callbacks.openModel != nil {
			callbacks.openModel(index)
		}
	}, Child: woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 132, Height: 56, Padding: woxwidget.Insets{Top: 16}, Child: woxwidget.Text{Value: a.translate(definition.Value.Label), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText}},
		woxwidget.Container{Width: width - 142, Height: 56, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Text{Value: selectedLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
			woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 9}, Color: palette.actionHeader},
		}}},
	}}}}
}

func (a *App) buildFormApp(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	var app ignoredHotkeyApp
	_ = json.Unmarshal([]byte(fields.values[definition.Value.Key]), &app)
	name := app.Name
	if strings.TrimSpace(name) == "" {
		name = "Select application"
	}
	detail := app.Path
	if strings.TrimSpace(detail) == "" {
		detail = app.Identity
	}
	fieldWidth := width - 142
	label := woxwidget.Container{Width: 132, Height: 42, Padding: woxwidget.Insets{Top: 11}, Child: woxwidget.Text{
		Value: a.translate(definition.Value.Label), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
	}}
	value := woxwidget.Gesture{ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), OnTap: func() {
		callbacks.focus(index)
		if callbacks.pickApp != nil {
			callbacks.pickApp(index)
		}
	}, Child: woxwidget.Container{
		Width: fieldWidth, Height: 42, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 7, Right: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
			woxwidget.Text{Value: compactFormTableText(detail, 64), Style: woxui.TextStyle{Size: 9}, Color: palette.actionHeader},
		}},
	}}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{label, value},
	}}
}

// buildFormHotkey leaves raw-key capture to core while keeping focus and rendering portable.
func (a *App) buildFormHotkey(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	value := fields.values[definition.Value.Key]
	recording, status := a.hotkeyRecordingFieldStatus(callbacks.idPrefix, index)
	if recording {
		value = "Recording…"
		if status != "" {
			value = status
		}
	} else if strings.TrimSpace(value) == "" {
		value = "Click to record"
	}
	return woxwidget.Gesture{ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), OnTap: func() {
		callbacks.focus(index)
		if callbacks.recordKey != nil {
			callbacks.recordKey(index)
		}
	}, Child: woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 132, Height: 42, Padding: woxwidget.Insets{Top: 11}, Child: woxwidget.Text{
			Value: a.translate(definition.Value.Label), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
		}},
		woxwidget.Container{Width: width - 142, Height: 42, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 12}, Child: woxwidget.Text{
			Value: value, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
		}},
	}}}}
}

func (a *App) buildFormChoice(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32, checked bool, selectedLabel string) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	valueText := selectedLabel
	if definition.Type == "checkbox" {
		valueText = "Off"
		if checked {
			valueText = "On"
		}
	} else {
		valueText = "‹  " + valueText + "  ›"
	}
	return woxwidget.Gesture{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index),
		OnTap: func() {
			callbacks.focus(index)
			callbacks.change(index, 1)
		},
		Child: woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: 132, Height: 42, Padding: woxwidget.Insets{Top: 11}, Child: woxwidget.Text{
				Value: a.translate(definition.Value.Label), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
			}},
			woxwidget.Container{Width: width - 142, Height: 42, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 12}, Child: woxwidget.Text{
				Value: valueText, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
			}},
		}}},
	}
}

func (a *App) buildFormTextbox(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	background := palette.queryBackground
	if focused {
		background = palette.selectedBackground
	}
	style := woxui.TextStyle{Size: 13}
	fieldWidth := width - 142
	inputWidth := fieldWidth
	if definition.Type == "dirPath" {
		inputWidth = max(float32(80), fieldWidth-92)
	}
	fieldHeight := max(float32(42), height-14)
	editorHeight := max(float32(24), fieldHeight-18)
	maxLines := max(1, definition.Value.MaxLines)
	if maxLines > 8 {
		maxLines = 8
	}
	state := fields.editing
	if !focused {
		value := fields.values[definition.Value.Key]
		state = woxui.TextEditingState{Text: value}
	}
	renderState := state
	if definition.Type == "password" {
		renderState.Text = strings.Repeat("•", len([]rune(state.Text)))
		renderState.Composition = strings.Repeat("•", len([]rune(state.Composition)))
	}
	input := woxwidget.Gesture{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index),
		OnTapAt: func(position woxui.Point) {
			offset := formTextOffsetAt(renderState, a.window, style, maxLines, inputWidth-22, woxui.Point{X: max(float32(0), position.X-12), Y: max(float32(0), position.Y-10)})
			callbacks.focus(index)
			callbacks.setCaret(index, offset)
		},
		Child: woxwidget.Container{Width: inputWidth, Height: fieldHeight, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Clip{
			Width: inputWidth - 22, Height: editorHeight, Child: woxwidget.Painter{Width: inputWidth - 22, Height: editorHeight,
				Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
					drawFormEditor(displayList, bounds, renderState, style, palette, focused, maxLines, a.window)
				},
			},
		}},
	}
	var valueField woxwidget.Widget = input
	if definition.Type == "dirPath" {
		valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			input,
			woxwidget.Gesture{ID: fmt.Sprintf("%s-field-%d-browse", callbacks.idPrefix, index), OnTap: func() {
				if callbacks.pickDir != nil {
					callbacks.pickDir(index)
				}
			}, Child: woxwidget.Container{Width: 84, Height: fieldHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 15, Top: 12}, Child: woxwidget.Text{
				Value: "Browse", Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
			}}},
		}}
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 132, Height: fieldHeight, Padding: woxwidget.Insets{Top: 11}, Child: woxwidget.Text{
			Value: a.translate(definition.Value.Label), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: palette.actionText,
		}},
		valueField,
	}}}
}

func formTextOffsetAt(state woxui.TextEditingState, window *woxui.Window, style woxui.TextStyle, maxLines int, width float32, point woxui.Point) int {
	lines := formTextLines(state.Text)
	caretLine := formTextLineIndex(lines, state.Selection.Focus)
	firstLine := max(0, caretLine-maxLines+1)
	lineIndex := min(len(lines)-1, firstLine+max(0, int(point.Y/20)))
	line := lines[lineIndex]
	runes := []rune(line.text)
	if maxLines == 1 {
		point.X += formTextHorizontalOffset([]rune(state.Text), state.Selection.Focus, style, width, window)
	}
	offset := len(runes)
	previousWidth := float32(0)
	for candidate := 1; candidate <= len(runes); candidate++ {
		metrics, _ := window.MeasureText(string(runes[:candidate]), style)
		if point.X < (previousWidth+metrics.Size.Width)*0.5 {
			offset = candidate - 1
			break
		}
		previousWidth = metrics.Size.Width
	}
	return line.start + offset
}

func formTextHorizontalOffset(runes []rune, focus int, style woxui.TextStyle, width float32, window *woxui.Window) float32 {
	focus = max(0, min(len(runes), focus))
	metrics, _ := window.MeasureText(string(runes[:focus]), style)
	return max(float32(0), metrics.Size.Width-max(float32(0), width-4))
}

func drawFormEditor(displayList *woxui.DisplayList, bounds woxui.Rect, state woxui.TextEditingState, style woxui.TextStyle, palette uiPalette, focused bool, maxLines int, window *woxui.Window) {
	runes := []rune(state.Text)
	start := max(0, min(len(runes), state.Selection.Start()))
	end := max(start, min(len(runes), state.Selection.End()))
	focus := max(0, min(len(runes), state.Selection.Focus))
	displayValue := state.Text
	compositionStart := -1
	compositionEnd := -1
	if state.Composition != "" {
		displayValue = string(runes[:start]) + state.Composition + string(runes[end:])
		compositionStart = start
		compositionEnd = start + len([]rune(state.Composition))
		start = compositionEnd
		end = compositionEnd
		focus = compositionEnd
	}
	displayRunes := []rune(displayValue)
	lines := formTextLines(displayValue)
	caretLine := formTextLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/20)))
	firstLine := max(0, caretLine-visibleLines+1)
	lastLine := min(len(lines), firstLine+visibleLines)
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = formTextHorizontalOffset(displayRunes, focus, style, bounds.Width, window)
	}
	for lineIndex := firstLine; lineIndex < lastLine; lineIndex++ {
		line := lines[lineIndex]
		y := bounds.Y + float32(lineIndex-firstLine)*20
		selectionStart := max(start, line.start)
		selectionEnd := min(end, line.end)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedMetrics, _ := window.MeasureText(string(displayRunes[selectionStart:selectionEnd]), style)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: y, Width: selectedMetrics.Size.Width, Height: 20}, 3, palette.selectionBackground)
		}
		displayList.DrawText(line.text, woxui.Rect{X: bounds.X - horizontalOffset, Y: y, Width: bounds.Width + horizontalOffset, Height: 20}, style, palette.actionText)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedText := string(displayRunes[selectionStart:selectionEnd])
			selectedMetrics, _ := window.MeasureText(selectedText, style)
			displayList.DrawText(selectedText, woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: y, Width: selectedMetrics.Size.Width, Height: 20}, style, palette.selectionText)
		}
	}
	if !focused {
		return
	}
	line := lines[caretLine]
	caretPrefix := string(displayRunes[line.start:focus])
	caretMetrics, _ := window.MeasureText(caretPrefix, style)
	cursorX := bounds.X - horizontalOffset + caretMetrics.Size.Width
	cursorY := bounds.Y + float32(caretLine-firstLine)*20
	if compositionStart >= line.start && compositionEnd <= line.end {
		prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:compositionStart]), style)
		compositionMetrics, _ := window.MeasureText(string(displayRunes[compositionStart:compositionEnd]), style)
		displayList.FillRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: cursorY + 19, Width: compositionMetrics.Size.Width, Height: 1}, palette.cursor)
	}
	displayList.FillRect(woxui.Rect{X: cursorX, Y: cursorY, Width: 1, Height: 20}, palette.cursor)
	_ = window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: cursorX, Y: cursorY, Width: 1, Height: 22}})
}
