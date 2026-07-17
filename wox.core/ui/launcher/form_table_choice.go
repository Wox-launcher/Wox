package launcher

import (
	"fmt"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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
		ID: "form-table-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Theme: palette.componentTheme(), Window: a.formTableNativeWindow(), Title: a.translate(snapshot.title),
		CurrentValue: snapshot.currentValue, Choices: choices, OnChoose: a.chooseFormTableChoice, OnCancel: a.closeFormTableChoicePicker,
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
	state.appPicker = nil
	state.choicePicker = &formTableChoicePickerState{fieldIndex: index, anchor: anchor}
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
