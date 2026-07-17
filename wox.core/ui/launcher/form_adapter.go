package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type formFieldCallbacks struct {
	idPrefix   string
	focus      func(index int)
	change     func(index, delta int)
	setText    func(index int, value string)
	onKey      func(woxui.KeyEvent) bool
	openTable  func(index int)
	openChoice func(index int, anchor woxui.Rect)
	pickDir    func(index int)
	pickApp    func(index int)
	recordKey  func(index int)
	openModel  func(index int)
}

// buildFormPanel maps action form state into the shared form view.
func (a *App) buildFormPanel(snapshot viewSnapshot, windowWidth float32) (woxwidget.Widget, float32, float32) {
	form := snapshot.form
	panelWidth := min(float32(520), max(float32(320), windowWidth-28))
	panelHeight := float32(formDefinitionsPanelHeight(form.action.Form, form.values))
	rows := make([]woxwidget.Widget, 0, len(form.action.Form))
	for index, definition := range form.action.Form {
		rowHeight := formDefinitionHeight(definition, form.values)
		rows = append(rows, a.buildFormDefinition(snapshot, index, definition, panelWidth-28, rowHeight))
	}
	panel := launcherview.FormPanel(launcherview.FormPanelProps{
		Width: panelWidth, Height: panelHeight, Title: a.translate(form.action.Name), Rows: rows,
		ContentHeight: formDefinitionsContentHeight(form.action.Form, form.values), KeepVisible: formFieldsKeepVisible(form.formFieldsSnapshot),
		CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: a.translate("i18n:ui_save"), Theme: snapshot.palette.componentTheme(),
		OnCancel: a.closeFormAction, OnSave: a.submitFormAction,
	})
	return panel, panelWidth, panelHeight
}

func formDefinitionHeight(definition formDefinition, valueMaps ...map[string]string) float32 {
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
		value := definition.Value.DefaultValue
		if len(valueMaps) > 0 && valueMaps[0] != nil {
			value = valueMaps[0][definition.Value.Key]
		}
		rows, err := decodeFormTableRows(value)
		if err != nil {
			rows = nil
		}
		return launcherview.FormTableFieldHeight(definition.Value.InlineTable, definition.Value.Tooltip, len(rows), definition.Value.MaxHeight)
	case "dictationModel", "ocrModel":
		return 70
	default:
		return 56
	}
}

func (a *App) buildFormDefinition(snapshot viewSnapshot, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	callbacks := formFieldCallbacks{idPrefix: "action-form", focus: a.focusFormField, change: a.changeFormChoice, setText: a.setFormText, onKey: a.onFormKey, openTable: a.openActionFormTable}
	return a.buildFormField(snapshot.form.formFieldsSnapshot, callbacks, snapshot.palette, index, definition, width, height)
}

// buildFormField translates one private form definition into a reusable field view.
func (a *App) buildFormField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	value := definition.Value
	switch definition.Type {
	case "head", "label", "newline":
		return launcherview.FormStaticField(launcherview.FormStaticFieldProps{Width: width, Height: height, Value: a.translate(value.Content), Kind: definition.Type, Theme: palette.componentTheme()})
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
		return launcherview.FormStaticField(launcherview.FormStaticFieldProps{Width: width, Height: height, Value: fmt.Sprintf("Unsupported form field: %s", definition.Type), Kind: "unsupported", Theme: palette.componentTheme()})
	}
}

func (a *App) buildFormModelField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
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
	return launcherview.FormModelField(launcherview.FormModelFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Value: selectedLabel, Status: status,
		Width: width, Height: height, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			if callbacks.openModel != nil {
				callbacks.openModel(index)
			}
		},
	})
}

func (a *App) buildFormApp(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
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
	return launcherview.FormAppField(launcherview.FormAppFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Name: name, Detail: compactFormTableText(detail, 64),
		Width: width, Height: height, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			if callbacks.pickApp != nil {
				callbacks.pickApp(index)
			}
		},
	})
}

func (a *App) buildFormHotkey(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	value := fields.values[definition.Value.Key]
	recording, status := a.hotkeyRecordingFieldStatus(callbacks.idPrefix, index)
	placeholder := a.translate("i18n:ui_hotkey_click_to_set")
	if recording {
		placeholder = a.translate("i18n:ui_hotkey_recording")
	}
	return launcherview.FormHotkeyField(launcherview.FormHotkeyFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip),
		Labels: formatHotkeyLabels(value), Placeholder: placeholder, Status: status, Recording: recording,
		Width: width, Height: height, Focused: fields.active && fields.focused == index, Window: a.formFieldNativeWindow(callbacks.idPrefix), Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			if callbacks.recordKey != nil {
				callbacks.recordKey(index)
			}
		},
	})
}

func (a *App) buildFormChoice(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32, checked bool, selectedLabel string) woxwidget.Widget {
	valueText := selectedLabel
	if definition.Type == "checkbox" {
		valueText = "Off"
		if checked {
			valueText = "On"
		}
	} else {
		valueText = "‹  " + valueText + "  ›"
	}
	return launcherview.FormValueField(launcherview.FormValueFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Value: valueText,
		Width: width, Height: height, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			callbacks.change(index, 1)
		},
	})
}

func (a *App) buildFormTextbox(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	state := fields.editing
	if !focused {
		state = woxui.TextEditingState{Text: fields.values[definition.Value.Key]}
	}
	maxLines := min(8, max(1, definition.Value.MaxLines))
	var onBrowse func()
	if definition.Type == "dirPath" {
		onBrowse = func() {
			if callbacks.pickDir != nil {
				callbacks.pickDir(index)
			}
		}
	}
	return launcherview.FormTextField(launcherview.FormTextFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Width: width, Height: height,
		State: state, Focused: focused, Protected: definition.Type == "password", MaxLines: maxLines,
		Window: a.formFieldNativeWindow(callbacks.idPrefix), Theme: palette.componentTheme(), OnBrowse: onBrowse,
		OnFocus: func() { callbacks.focus(index) },
		OnChanged: func(value string) {
			if callbacks.setText != nil {
				callbacks.setText(index, value)
			}
		},
		OnKey: callbacks.onKey,
	})
}
