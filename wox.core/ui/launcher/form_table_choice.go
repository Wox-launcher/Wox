package launcher

import (
	"fmt"
	woxcomponent "wox/ui/launcher/component"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildFormTableChoicePicker adapts one table select field to the shared Flutter-style anchored menu.
func (c *formTableController) buildFormTableChoicePicker(snapshot *formTableChoicePickerSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.options))
	for index, option := range snapshot.options {
		label := c.language.translate(option.Label)
		if label == "" {
			label = option.Value
		}
		choice := launcherview.SettingsChoice{Value: option.Value, Label: label}
		if option.Group != "" {
			choice.Group = c.language.translate(option.Group)
			choice.GroupTooltip = c.language.translate(option.GroupTooltip)
		}
		if option.Icon.ImageType != "" {
			choice.Leading = c.imageForSurface(option.Icon, physicalImageSize(18, imageScale), palette.Background)
		}
		choices[index] = choice
	}
	searchIcon := c.imageForTint(settingControlIconSource("search"), &palette.TextSecondary, physicalImageSize(16, imageScale))
	infoIcon := c.imageForTint(settingNavIconSource("about"), &palette.TextSecondary, physicalImageSize(14, imageScale))
	var onTooltip func(bool, string, woxui.Rect)
	if c.settingsEditor != nil {
		onTooltip = c.setSettingChoiceTooltip
	}
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		ID: "form-table-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Filterable: snapshot.filterable, Theme: palette, Window: c.formTableNativeWindow(), Title: c.language.translate(snapshot.title),
		FilterHint: c.language.translate("i18n:ui_filter_placeholder"), SearchIcon: searchIcon, InfoIcon: infoIcon, OnTooltip: onTooltip, CurrentValue: snapshot.currentValue, Choices: choices, OnChoose: c.chooseFormTableChoice, OnCancel: c.closeFormTableChoicePicker,
	})
}

// openFocusedFormTableRowChoice resolves the focused field bounds for keyboard-opened menus.
func (c *formTableController) openFocusedFormTableRowChoice(index int) {
	anchor := woxui.Rect{}
	host := c.formTableHost()
	if host != nil {
		anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", index)))
	}
	c.openFormTableRowChoice(index, anchor)
}

// openFormTableRowChoice opens the menu at the exact field bounds captured by pointer hit testing.
func (c *formTableController) openFormTableRowChoice(index int, anchor woxui.Rect) {
	c.stopHotkeyRecording()
	state := c.activeFormTableEditor()
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
	c.updateFormTableTextInput(false)
	c.invalidateFormTableWindow()
}

// formTableChoicePickerFilterable reports whether the open select menu owns a search field.
func formTableChoicePickerFilterable(state *formTableEditorState) bool {
	if state == nil || state.choicePicker == nil || state.rowForm == nil {
		return false
	}
	index := state.choicePicker.fieldIndex
	if index < 0 || index >= len(state.rowForm.definitions) {
		return false
	}
	return state.rowForm.definitions[index].Value.Filterable
}

// closeFormTableChoicePicker dismisses the menu and restores the row editor's input ownership.
func (c *formTableController) closeFormTableChoicePicker() {
	c.setSettingChoiceTooltip(false, "", woxui.Rect{})
	state := c.activeFormTableEditor()
	textInput := false
	if state != nil && state.choicePicker != nil {
		state.choicePicker = nil
		clearFormTableRowValidationLocked(state)
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	c.updateFormTableTextInput(textInput)
	c.invalidateFormTableWindow()
}

// chooseFormTableChoice commits one option while preserving table-specific dependent defaults.
func (c *formTableController) chooseFormTableChoice(index int) {
	state := c.activeFormTableEditor()
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
	c.setSettingChoiceTooltip(false, "", woxui.Rect{})
	state.rowForm.values[definition.Value.Key] = definition.Value.Options[index].Value
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	if definition.Value.Key == "Name" {
		applyAIProviderDefaultHostLocked(state, true, c.aiSettings().ProviderCatalog())
	}
	if formTableRowDependsOnField(state.definition, definition.Value.Key) {
		applyFormTableRowVisibleFieldsLocked(state)
	}
	state.choicePicker = nil
	clearFormTableRowValidationLocked(state)
	c.updateFormTableTextInput(false)
	c.invalidateFormTableWindow()
}
