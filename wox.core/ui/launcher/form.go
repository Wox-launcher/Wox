package launcher

import (
	"context"
	"log"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type formFieldsState struct {
	definitions []formDefinition
	values      map[string]string
	focused     int
	editor      *woxui.TextEditor
	active      bool
}

type formFieldsSnapshot struct {
	definitions []formDefinition
	values      map[string]string
	focused     int
	editing     woxui.TextEditingState
	active      bool
}

type formState struct {
	formFieldsState
	resultID string
	queryID  string
	action   resultAction
}

type formSnapshot struct {
	formFieldsSnapshot
	resultID string
	queryID  string
	action   resultAction
}

func newFormFieldsState(definitions []formDefinition, initialValues map[string]string, active bool) formFieldsState {
	values := make(map[string]string)
	focused := -1
	for index, definition := range definitions {
		if definition.Value.Key != "" {
			value, ok := initialValues[definition.Value.Key]
			if !ok {
				value = definition.Value.DefaultValue
			}
			if formDefinitionTextEditable(definition) && definition.Value.MaxLines > 1 {
				value = strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
			}
			values[definition.Value.Key] = value
		}
		if focused < 0 && formDefinitionFocusable(definition) {
			focused = index
		}
	}
	fields := formFieldsState{definitions: append([]formDefinition(nil), definitions...), values: values, focused: focused, active: active}
	if focused >= 0 && formDefinitionTextEditable(definitions[focused]) {
		fields.editor = woxui.NewTextEditor(values[definitions[focused].Value.Key])
	}
	return fields
}

func snapshotFormFieldsLocked(state *formFieldsState) formFieldsSnapshot {
	if state == nil {
		return formFieldsSnapshot{focused: -1}
	}
	values := make(map[string]string, len(state.values))
	for key, value := range state.values {
		values[key] = value
	}
	snapshot := formFieldsSnapshot{
		definitions: append([]formDefinition(nil), state.definitions...),
		values:      values,
		focused:     state.focused,
		active:      state.active,
	}
	if state.editor != nil {
		snapshot.editing = state.editor.State()
	}
	return snapshot
}

// setFormFieldsTextLocked updates committed form data without importing selection or IME state from the widget.
func setFormFieldsTextLocked(state *formFieldsState, index int, value string) bool {
	if state == nil || index < 0 || index >= len(state.definitions) || !formDefinitionTextEditable(state.definitions[index]) {
		return false
	}
	key := state.definitions[index].Value.Key
	if key == "" || state.values[key] == value {
		return false
	}
	state.values[key] = value
	if state.focused == index {
		if state.editor == nil {
			state.editor = woxui.NewTextEditor(value)
		} else {
			state.editor.SetText(value, false)
		}
	}
	return true
}

// formFieldsKeepVisible returns the focused field interval for a retained form scroll view.
func formFieldsKeepVisible(fields formFieldsSnapshot) *woxwidget.ScrollRange {
	if fields.focused < 0 || fields.focused >= len(fields.definitions) {
		return nil
	}
	start := float32(0)
	for index := 0; index < fields.focused; index++ {
		start += formDefinitionHeight(fields.definitions[index], fields.values)
	}
	return &woxwidget.ScrollRange{Start: start, End: start + formDefinitionHeight(fields.definitions[fields.focused], fields.values)}
}

func snapshotFormLocked(state *formState) *formSnapshot {
	if state == nil {
		return nil
	}
	return &formSnapshot{formFieldsSnapshot: snapshotFormFieldsLocked(&state.formFieldsState), resultID: state.resultID, queryID: state.queryID, action: state.action}
}

func formPanelHeight(state *formState) int {
	if state == nil {
		return 0
	}
	return formDefinitionsPanelHeight(state.action.Form, state.values)
}

func formDefinitionsPanelHeight(definitions []formDefinition, valueMaps ...map[string]string) int {
	height := 100
	for _, definition := range definitions {
		height += int(formDefinitionHeight(definition, valueMaps...))
	}
	return min(max(height, 160), 520)
}

type formTextLine struct {
	start int
	end   int
	text  string
}

// ponytail: Multiline fields wrap only at explicit newlines; add soft-wrap offsets when a real setting needs paragraph editing.
func formTextLines(value string) []formTextLine {
	runes := []rune(value)
	lines := make([]formTextLine, 0, strings.Count(value, "\n")+1)
	start := 0
	for index, current := range runes {
		if current == '\n' {
			lines = append(lines, formTextLine{start: start, end: index, text: string(runes[start:index])})
			start = index + 1
		}
	}
	lines = append(lines, formTextLine{start: start, end: len(runes), text: string(runes[start:])})
	return lines
}

func formTextLineIndex(lines []formTextLine, offset int) int {
	for index, line := range lines {
		if offset <= line.end || index == len(lines)-1 {
			return index
		}
	}
	return 0
}

func handleFormEditorKey(editor *woxui.TextEditor, definition formDefinition, event woxui.KeyEvent) (bool, bool) {
	if editor == nil {
		return false, false
	}
	if !formDefinitionTextEditable(definition) || definition.Value.MaxLines <= 1 {
		return editor.HandleKey(event)
	}
	state := editor.State()
	lines := formTextLines(state.Text)
	lineIndex := formTextLineIndex(lines, state.Selection.Focus)
	line := lines[lineIndex]
	extend := event.Modifiers&woxui.KeyModifierShift != 0
	setFocus := func(offset int) {
		if extend {
			editor.SetSelection(state.Selection.Anchor, offset)
		} else {
			editor.SetCaret(offset)
		}
	}
	switch event.Key {
	case woxui.KeyEnter:
		return true, editor.InsertText("\n")
	case woxui.KeyArrowUp, woxui.KeyArrowDown:
		target := lineIndex - 1
		if event.Key == woxui.KeyArrowDown {
			target = lineIndex + 1
		}
		if target < 0 || target >= len(lines) {
			return true, false
		}
		column := state.Selection.Focus - line.start
		setFocus(lines[target].start + min(column, lines[target].end-lines[target].start))
		return true, false
	case woxui.KeyHome:
		setFocus(line.start)
		return true, false
	case woxui.KeyEnd:
		setFocus(line.end)
		return true, false
	default:
		return editor.HandleKey(event)
	}
}

func formDefinitionsContentHeight(definitions []formDefinition, valueMaps ...map[string]string) float32 {
	height := float32(0)
	for _, definition := range definitions {
		height += formDefinitionHeight(definition, valueMaps...)
	}
	return height
}

func formDefinitionFocusable(definition formDefinition) bool {
	return formDefinitionTextEditable(definition) || definition.Type == "checkbox" || definition.Type == "select" || definition.Type == "selectAIModel" || definition.Type == "hotkey" || definition.Type == "dictationHotkey" || definition.Type == "app" || definition.Type == "table" || definition.Type == "dictationModel" || definition.Type == "ocrModel"
}

func formDefinitionTextEditable(definition formDefinition) bool {
	return definition.Type == "textbox" || definition.Type == "password" || definition.Type == "dirPath" || definition.Type == "woxImage"
}

func syncFormFieldsEditorLocked(fields *formFieldsState) {
	if fields == nil || fields.editor == nil || fields.focused < 0 || fields.focused >= len(fields.definitions) {
		return
	}
	definition := fields.definitions[fields.focused]
	if formDefinitionTextEditable(definition) && definition.Value.Key != "" {
		fields.values[definition.Value.Key] = fields.editor.State().Text
	}
}

func setFormFieldsFocusLocked(fields *formFieldsState, index int) {
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	fields.focused = index
	fields.active = true
	definition := fields.definitions[index]
	if formDefinitionTextEditable(definition) {
		fields.editor = woxui.NewTextEditor(fields.values[definition.Value.Key])
	} else {
		fields.editor = nil
	}
}

func changeFormFieldsChoiceLocked(fields *formFieldsState, index, delta int) {
	if fields == nil || index < 0 || index >= len(fields.definitions) {
		return
	}
	syncFormFieldsEditorLocked(fields)
	setFormFieldsFocusLocked(fields, index)
	definition := fields.definitions[index]
	key := definition.Value.Key
	switch definition.Type {
	case "checkbox":
		if fields.values[key] == "true" {
			fields.values[key] = "false"
		} else {
			fields.values[key] = "true"
		}
	case "select", "selectAIModel":
		if len(definition.Value.Options) == 0 {
			return
		}
		current := -1
		for optionIndex, option := range definition.Value.Options {
			if option.Value == fields.values[key] {
				current = optionIndex
				break
			}
		}
		if current < 0 && delta < 0 {
			current = 0
		}
		current = (current + delta + len(definition.Value.Options)) % len(definition.Value.Options)
		fields.values[key] = definition.Value.Options[current].Value
	}
}

func (a *App) openFormAction(result queryResult, action resultAction) {
	state := &formState{formFieldsState: newFormFieldsState(action.Form, nil, true), resultID: result.ID, queryID: result.QueryID, action: action}
	a.mu.Lock()
	a.form = state
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.mu.Unlock()
	a.updateFormTextInput(state.editor != nil)
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

func (a *App) closeFormAction() {
	a.mu.Lock()
	if a.form == nil {
		a.mu.Unlock()
		return
	}
	a.form = nil
	a.mu.Unlock()
	a.restoreQueryTextInput()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

func (a *App) submitFormAction() {
	a.mu.Lock()
	if a.form == nil {
		a.mu.Unlock()
		return
	}
	a.syncFormEditorLocked()
	state := a.form
	values := make(map[string]string, len(state.values))
	for key, value := range state.values {
		values[key] = value
	}
	a.form = nil
	a.mu.Unlock()
	if err := a.services.SubmitFormAction(context.Background(), a.sessionID, state.queryID, state.resultID, state.action.ID, values); err != nil {
		log.Printf("submit form action: %v", err)
	}
	a.restoreQueryTextInput()
	_ = a.applyWindowBounds()
	_ = a.window.Invalidate()
}

func (a *App) onFormKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.form != nil
	focused := -1
	fieldType := ""
	multiline := false
	if active {
		focused = a.form.focused
		if focused >= 0 && focused < len(a.form.definitions) {
			fieldType = a.form.definitions[focused].Type
			multiline = fieldType == "textbox" && a.form.definitions[focused].Value.MaxLines > 1
		}
	}
	a.mu.RUnlock()
	if !active {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.closeFormAction()
		return true
	}
	if event.Key == woxui.KeyEnter && event.Modifiers.HasPrimary() {
		a.submitFormAction()
		return true
	}
	textEditable := fieldType == "textbox" || fieldType == "password" || fieldType == "dirPath"
	if textEditable {
		switch event.Key {
		case woxui.KeyTab:
			delta := 1
			if event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -1
			}
			a.moveFormFocus(delta)
			return true
		case woxui.KeyArrowDown:
			if !multiline {
				a.moveFormFocus(1)
				return true
			}
		case woxui.KeyArrowUp:
			if !multiline {
				a.moveFormFocus(-1)
				return true
			}
		case woxui.KeyEnter:
			return !multiline
		}
		return false
	}
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		if event.Key == woxui.KeyArrowDown && multiline {
			a.editFormKey(event)
			break
		}
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveFormFocus(delta)
	case woxui.KeyArrowUp:
		if multiline {
			a.editFormKey(event)
		} else {
			a.moveFormFocus(-1)
		}
	case woxui.KeyArrowLeft:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormChoice(focused, -1)
		} else {
			a.editFormKey(event)
		}
	case woxui.KeyArrowRight:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormChoice(focused, 1)
		} else {
			a.editFormKey(event)
		}
	case woxui.KeySpace, woxui.KeyEnter:
		if event.Key == woxui.KeyEnter && multiline {
			a.editFormKey(event)
		} else if fieldType == "table" {
			a.openActionFormTable(focused)
		} else if fieldType == "checkbox" || fieldType == "select" || fieldType == "selectAIModel" {
			a.changeFormChoice(focused, 1)
		}
	default:
		a.editFormKey(event)
	}
	return true
}

func (a *App) setFormText(index int, value string) {
	a.mu.Lock()
	changed := a.form != nil && setFormFieldsTextLocked(&a.form.formFieldsState, index, value)
	a.mu.Unlock()
	if changed {
		_ = a.applyWindowBounds()
		_ = a.window.Invalidate()
	}
}

func (a *App) onFormTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	active := a.form != nil
	a.mu.RUnlock()
	return active
}

func (a *App) editFormKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if a.form != nil && a.form.editor != nil && a.form.focused >= 0 && a.form.focused < len(a.form.definitions) {
		_, changed := handleFormEditorKey(a.form.editor, a.form.definitions[a.form.focused], event)
		if changed {
			a.syncFormEditorLocked()
		}
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) moveFormFocus(delta int) {
	a.mu.Lock()
	if a.form == nil || len(a.form.definitions) == 0 {
		a.mu.Unlock()
		return
	}
	a.syncFormEditorLocked()
	index := a.form.focused
	for step := 0; step < len(a.form.definitions); step++ {
		index = (index + delta + len(a.form.definitions)) % len(a.form.definitions)
		if formDefinitionFocusable(a.form.definitions[index]) {
			a.setFormFocusLocked(index)
			break
		}
	}
	textInput := a.form.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) focusFormField(index int) {
	a.mu.Lock()
	if a.form == nil || index < 0 || index >= len(a.form.definitions) || !formDefinitionFocusable(a.form.definitions[index]) {
		a.mu.Unlock()
		return
	}
	a.syncFormEditorLocked()
	a.setFormFocusLocked(index)
	textInput := a.form.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(textInput)
	_ = a.window.Invalidate()
}

func (a *App) changeFormChoice(index, delta int) {
	a.mu.Lock()
	if a.form == nil || index < 0 || index >= len(a.form.definitions) {
		a.mu.Unlock()
		return
	}
	changeFormFieldsChoiceLocked(&a.form.formFieldsState, index, delta)
	a.mu.Unlock()
	a.updateFormTextInput(false)
	_ = a.window.Invalidate()
}

func (a *App) setFormFocusLocked(index int) {
	setFormFieldsFocusLocked(&a.form.formFieldsState, index)
}

func (a *App) syncFormEditorLocked() {
	if a.form != nil {
		syncFormFieldsEditorLocked(&a.form.formFieldsState)
	}
}

func (a *App) updateFormTextInput(enabled bool) {
	state := woxui.TextInputState{}
	if enabled {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 240, Y: 180, Width: 1, Height: 24}}
	}
	_ = a.window.SetTextInputState(state)
}

func (a *App) restoreQueryTextInput() {
	a.mu.RLock()
	enabled := !a.show.HideQueryBox && !a.chatFullscreen && a.form == nil && (a.requirementForm == nil || !a.requirementForm.active) && (a.triggerConflict == nil || !a.triggerConflict.active) && (a.themeEditor == nil || !a.themeEditor.active) && (a.chatPreview == nil || !a.chatPreview.active) && (a.terminalPreview == nil || !a.terminalPreview.SearchOpen)
	a.mu.RUnlock()
	state := woxui.TextInputState{}
	if enabled {
		state = woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: 130, Y: 29, Width: 1, Height: 24}}
	}
	_ = a.window.SetTextInputState(state)
}
