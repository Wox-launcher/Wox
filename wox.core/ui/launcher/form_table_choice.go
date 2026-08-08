package launcher

import (
	"fmt"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildFormTableChoicePicker adapts one table select field to the shared Flutter-style anchored menu.
func (a *App) buildFormTableChoicePicker(snapshot *formTableChoicePickerSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.options))
	for index, option := range snapshot.options {
		label := a.translate(option.Label)
		if label == "" {
			label = option.Value
		}
		choice := launcherview.SettingsChoice{Value: option.Value, Label: label}
		if option.Icon.ImageType != "" {
			choice.Leading = a.imageForSize(option.Icon, physicalImageSize(18, imageScale))
		}
		choices[index] = choice
	}
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		ID: "form-table-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Theme: palette.componentTheme(), Window: a.formTableNativeWindow(), Title: a.translate(snapshot.title),
		CurrentValue: snapshot.currentValue, Choices: choices, OnChoose: a.chooseFormTableChoice, OnCancel: a.closeFormTableChoicePicker,
	})
}

// openFocusedFormTableRowChoice resolves the focused field bounds for keyboard-opened menus.
func (a *App) openFocusedFormTableRowChoice(index int) {
	anchor := woxui.Rect{}
	host := a.host
	if a.settingsTableEditor != nil {
		host = a.settingsHost
	}
	if host != nil {
		anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
	}
	a.openFormTableRowChoice(index, anchor)
}

// openFormTableRowChoice opens the menu at the exact field bounds captured by pointer hit testing.
func (a *App) openFormTableRowChoice(index int, anchor woxui.Rect) {
	a.stopHotkeyRecording()
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		return
	}
	definition := state.rowForm.definitions[index]
	if (definition.Type != "select" && definition.Type != "selectAIModel") || len(definition.Value.Options) == 0 {
		return
	}
	syncFormFieldsEditorLocked(state.rowForm)
	setFormFieldsFocusLocked(state.rowForm, index)
	state.appPicker = nil
	state.choicePicker = &formTableChoicePickerState{fieldIndex: index, anchor: anchor}
	clearFormTableRowValidationLocked(state)
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}

// closeFormTableChoicePicker dismisses the menu and restores the row editor's input ownership.
func (a *App) closeFormTableChoicePicker() {
	state := a.activeFormTableEditor()
	textInput := false
	if state != nil && state.choicePicker != nil {
		state.choicePicker = nil
		clearFormTableRowValidationLocked(state)
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

// chooseFormTableChoice commits one option while preserving table-specific dependent defaults.
func (a *App) chooseFormTableChoice(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.choicePicker == nil || index < 0 {
		return
	}
	fieldIndex := state.choicePicker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		return
	}
	definition := state.rowForm.definitions[fieldIndex]
	if index >= len(definition.Value.Options) {
		return
	}
	state.rowForm.values[definition.Value.Key] = definition.Value.Options[index].Value
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	if definition.Value.Key == "Name" {
		applyAIProviderDefaultHostLocked(state, true, a.aiSettings.ProviderCatalog())
	}
	state.choicePicker = nil
	clearFormTableRowValidationLocked(state)
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}
