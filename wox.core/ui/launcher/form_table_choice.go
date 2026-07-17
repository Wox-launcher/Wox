package launcher

import (
	"fmt"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formTableChoicePickerRowHeight = float32(48)

// buildFormTableChoicePicker adapts one table select field to the shared Flutter-style anchored menu.
func (a *App) buildFormTableChoicePicker(snapshot *formTableChoicePickerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.options))
	for index, option := range snapshot.options {
		label := a.translate(option.Label)
		if label == "" {
			label = option.Value
		}
		choices[index] = launcherview.SettingsChoice{Value: option.Value, Label: label}
	}
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		Width: width, Height: height, Anchor: snapshot.anchor, Theme: palette.componentTheme(), Window: a.formTableNativeWindow(), Title: a.translate(snapshot.title),
		CurrentValue: snapshot.currentValue, Choices: choices, Selected: snapshot.selected, Scroll: snapshot.scroll,
		OnKey: a.onFormTableChoicePickerKeyEvent, OnSelect: a.selectFormTableChoice, OnChoose: a.chooseFormTableChoice, OnCancel: a.closeFormTableChoicePicker,
		OnScroll: a.scrollFormTableChoicePicker, OnSetViewport: a.setFormTableChoicePickerViewport,
	})
}

// openFocusedFormTableRowChoice resolves the focused field bounds for keyboard-opened menus.
func (a *App) openFocusedFormTableRowChoice(index int) {
	anchor := woxui.Rect{}
	a.mu.RLock()
	host := a.host
	if a.tableEditor != nil && a.formTableTargetUsesSettingsLocked(a.tableEditor.target) {
		host = a.settingsHost
	}
	a.mu.RUnlock()
	if host != nil {
		anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
	}
	a.openFormTableRowChoice(index, anchor)
}

// openFormTableRowChoice opens the menu at the exact field bounds captured by pointer hit testing.
func (a *App) openFormTableRowChoice(index int, anchor woxui.Rect) {
	a.stopHotkeyRecording()
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		a.mu.Unlock()
		return
	}
	definition := state.rowForm.definitions[index]
	if (definition.Type != "select" && definition.Type != "selectAIModel") || len(definition.Value.Options) == 0 {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	setFormFieldsFocusLocked(state.rowForm, index)
	selected := 0
	currentValue := state.rowForm.values[definition.Value.Key]
	for optionIndex, option := range definition.Value.Options {
		if option.Value == currentValue {
			selected = optionIndex
			break
		}
	}
	scroll := max(float32(0), float32(selected-4)*formTableChoicePickerRowHeight)
	state.appPicker = nil
	state.choicePicker = &formTableChoicePickerState{fieldIndex: index, anchor: anchor, selected: selected, scroll: scroll}
	state.status = ""
	a.mu.Unlock()
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// closeFormTableChoicePicker dismisses the menu and restores the row editor's input ownership.
func (a *App) closeFormTableChoicePicker() {
	a.mu.Lock()
	state := a.tableEditor
	textInput := false
	if state != nil && state.choicePicker != nil {
		state.choicePicker = nil
		state.status = ""
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	a.mu.Unlock()
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

// chooseFormTableChoice commits one option while preserving table-specific dependent defaults.
func (a *App) chooseFormTableChoice(index int) {
	a.mu.Lock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || state.choicePicker == nil || index < 0 {
		a.mu.Unlock()
		return
	}
	fieldIndex := state.choicePicker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		a.mu.Unlock()
		return
	}
	definition := state.rowForm.definitions[fieldIndex]
	if index >= len(definition.Value.Options) {
		a.mu.Unlock()
		return
	}
	state.rowForm.values[definition.Value.Key] = definition.Value.Options[index].Value
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	if definition.Value.Key == "Name" {
		applyAIProviderDefaultHostLocked(state, true, a.aiProviderCatalog)
	}
	state.choicePicker = nil
	state.status = ""
	a.mu.Unlock()
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// selectFormTableChoice follows pointer hover without committing the highlighted option.
func (a *App) selectFormTableChoice(index int) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.choicePicker != nil && state.rowForm != nil {
		fieldIndex := state.choicePicker.fieldIndex
		if fieldIndex >= 0 && fieldIndex < len(state.rowForm.definitions) && index >= 0 && index < len(state.rowForm.definitions[fieldIndex].Value.Options) {
			state.choicePicker.selected = index
			a.ensureFormTableChoiceVisibleLocked()
		}
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

// moveFormTableChoice changes the keyboard highlight and keeps it visible.
func (a *App) moveFormTableChoice(delta int) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.choicePicker != nil && state.rowForm != nil {
		fieldIndex := state.choicePicker.fieldIndex
		if fieldIndex >= 0 && fieldIndex < len(state.rowForm.definitions) {
			count := len(state.rowForm.definitions[fieldIndex].Value.Options)
			if count > 0 {
				state.choicePicker.selected = (state.choicePicker.selected + delta + count) % count
				a.ensureFormTableChoiceVisibleLocked()
			}
		}
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

// setFormTableChoicePickerViewport records the visible list height for scroll clamping.
func (a *App) setFormTableChoicePickerViewport(height float32) {
	a.mu.Lock()
	if state := a.tableEditor; state != nil && state.choicePicker != nil {
		state.choicePicker.viewport = max(float32(1), height)
		a.ensureFormTableChoiceVisibleLocked()
	}
	a.mu.Unlock()
}

// scrollFormTableChoicePicker applies pointer scrolling within the option list.
func (a *App) scrollFormTableChoicePicker(delta float32) {
	a.mu.Lock()
	state := a.tableEditor
	if state != nil && state.choicePicker != nil && state.rowForm != nil {
		fieldIndex := state.choicePicker.fieldIndex
		if fieldIndex >= 0 && fieldIndex < len(state.rowForm.definitions) {
			count := len(state.rowForm.definitions[fieldIndex].Value.Options)
			maximum := max(float32(0), float32(count)*formTableChoicePickerRowHeight-state.choicePicker.viewport)
			state.choicePicker.scroll = min(max(float32(0), state.choicePicker.scroll+delta), maximum)
		}
	}
	a.mu.Unlock()
	a.invalidateFormTableWindow()
}

// ensureFormTableChoiceVisibleLocked keeps the highlighted option within the current viewport.
func (a *App) ensureFormTableChoiceVisibleLocked() {
	state := a.tableEditor
	if state == nil || state.choicePicker == nil || state.rowForm == nil || state.choicePicker.selected < 0 {
		return
	}
	picker := state.choicePicker
	fieldIndex := picker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		return
	}
	viewport := max(float32(1), picker.viewport)
	top := float32(picker.selected) * formTableChoicePickerRowHeight
	bottom := top + formTableChoicePickerRowHeight
	if top < picker.scroll {
		picker.scroll = top
	} else if bottom > picker.scroll+viewport {
		picker.scroll = bottom - viewport
	}
	count := len(state.rowForm.definitions[fieldIndex].Value.Options)
	maximum := max(float32(0), float32(count)*formTableChoicePickerRowHeight-viewport)
	picker.scroll = min(max(float32(0), picker.scroll), maximum)
}

// onFormTableChoicePickerKey handles modal dropdown navigation before the row editor.
func (a *App) onFormTableChoicePickerKey(event woxui.KeyEvent, selected int) {
	switch event.Key {
	case woxui.KeyEscape:
		a.closeFormTableChoicePicker()
	case woxui.KeyArrowUp:
		a.moveFormTableChoice(-1)
	case woxui.KeyArrowDown:
		a.moveFormTableChoice(1)
	case woxui.KeyEnter, woxui.KeySpace:
		if selected >= 0 {
			a.chooseFormTableChoice(selected)
		}
	}
}

// onFormTableChoicePickerKeyEvent adapts the focused menu callback to controller state.
func (a *App) onFormTableChoicePickerKeyEvent(event woxui.KeyEvent) bool {
	a.mu.RLock()
	selected := -1
	if a.tableEditor != nil && a.tableEditor.choicePicker != nil {
		selected = a.tableEditor.choicePicker.selected
	}
	a.mu.RUnlock()
	a.onFormTableChoicePickerKey(event, selected)
	return true
}
