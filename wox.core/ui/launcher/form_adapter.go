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
	idPrefix          string
	labelWidth        float32
	settingsLayout    bool
	alignHotkeyRight  bool
	hotkeyError       string
	imageScale        float32
	focus             func(index int)
	change            func(index, delta int)
	setText           func(index int, value string)
	onKey             func(woxui.KeyEvent) bool
	openTable         func(index int)
	openChoice        func(index int, anchor woxui.Rect)
	openAIModelChoice func(index int, provider bool, anchor woxui.Rect)
	setAIModelName    func(index int, value string)
	finishAIModelEdit func(index int, value string)
	pickDir           func(index int)
	pickApp           func(index int)
	recordKey         func(index int)
	openModel         func(index int, anchor woxui.Rect)
	runServiceAction  func(actionID string)
	serviceBusy       bool
	serviceError      string
}

// buildFormPanel maps action form state into the shared form view.
func (a *App) buildFormPanel(snapshot viewSnapshot, windowWidth float32) (woxwidget.Widget, float32, float32) {
	form := snapshot.form
	labelWidth := a.measureFormLabelWidth(form.action.Form, a.window, 60, 0)
	panelPadding := woxwidget.Insets{
		Left: snapshot.densityMetrics.scaled(14), Top: snapshot.densityMetrics.scaled(10),
		Right: snapshot.densityMetrics.scaled(14), Bottom: snapshot.densityMetrics.scaled(10),
	}
	panelMaximumWidth := snapshot.densityMetrics.scaled(formContentMaximumWidth) + panelPadding.Left + panelPadding.Right
	panelMaximumHeight := snapshot.densityMetrics.scaled(formContentMaximumHeight) + panelPadding.Top + panelPadding.Bottom
	panelWidth := min(panelMaximumWidth, max(float32(320), windowWidth-28))
	contentWidth := panelWidth - panelPadding.Left - panelPadding.Right
	rows := make([]woxwidget.Widget, 0, len(form.action.Form))
	for index, definition := range form.action.Form {
		rows = append(rows, woxwidget.Keyed{Key: formFieldRowKey("action-form", index), Child: a.buildFormDefinition(snapshot, index, definition, contentWidth, labelWidth, 0)})
	}
	panel := launcherview.FormPanel(launcherview.FormPanelProps{
		Width: panelWidth, MaximumHeight: panelMaximumHeight, Padding: panelPadding, Rows: rows,
		KeepVisibleKey: formFieldsKeepVisibleKey("action-form", form.formFieldsSnapshot),
		CancelLabel:    fmt.Sprintf("%s (Esc)", a.translate("i18n:ui_cancel")),
		SaveLabel:      fmt.Sprintf("%s (%s)", a.translate("i18n:ui_save"), strings.Join(formatHotkeyLabels(primaryHotkey("enter")), "+")),
		Theme:          snapshot.palette.componentTheme(),
		OnCancel:       a.closeFormAction, OnSave: a.submitFormAction,
	})
	return panel, panelWidth, panelMaximumHeight
}

func (a *App) buildFormDefinition(snapshot viewSnapshot, index int, definition formDefinition, width, labelWidth, height float32) woxwidget.Widget {
	callbacks := formFieldCallbacks{idPrefix: "action-form", labelWidth: labelWidth, focus: a.focusFormField, change: a.changeFormChoice, setText: a.setFormText, onKey: a.onFormKey, openTable: a.openActionFormTable, pickDir: a.pickFormActionDirectory}
	return a.buildFormField(snapshot.form.formFieldsSnapshot, callbacks, snapshot.palette, index, definition, width, height)
}

// measureFormLabelWidth mirrors Flutter's measured label column while allowing each form surface to keep its own bounds.
func (a *App) measureFormLabelWidth(definitions []formDefinition, window *woxui.Window, minimum, maximum float32) float32 {
	width := minimum
	if window == nil {
		return width
	}
	style := woxui.TextStyle{Size: 13}
	for _, definition := range definitions {
		labelKey := definition.Value.Label
		if labelKey == "" {
			labelKey = definition.Value.Title
		}
		if definition.Type == "table" {
			labelKey = formTableTitle(definition)
		}
		label := strings.TrimSpace(a.translate(labelKey))
		if label == "" {
			continue
		}
		if metrics, err := window.MeasureText(label, style); err == nil {
			width = max(width, metrics.Size.Width+8)
		}
	}
	if maximum > 0 {
		width = min(width, maximum)
	}
	return width
}

// buildFormField translates one private form definition into a reusable field view.
func (a *App) buildFormField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	value := definition.Value
	switch definition.Type {
	case "stats":
		rows := make([]launcherview.FormStatsRow, 0, len(value.Rows))
		for _, row := range value.Rows {
			rows = append(rows, launcherview.FormStatsRow{Label: a.translate(row.Label), Value: row.Value})
		}
		return launcherview.FormStatsField(launcherview.FormStatsFieldProps{
			Width: width, Height: height, Title: a.translate(value.Title), Rows: rows, Theme: palette.componentTheme(),
		})
	case "fileIndexService":
		actions := make([]launcherview.FormServiceAction, 0, len(value.Actions))
		for _, action := range value.Actions {
			actionID := action.ID
			actions = append(actions, launcherview.FormServiceAction{
				ID: action.ID, Label: a.translate(action.Label), Primary: action.Primary, Danger: action.Danger, Enabled: action.Enabled && !callbacks.serviceBusy,
				OnTap: func() {
					if callbacks.runServiceAction != nil {
						callbacks.runServiceAction(actionID)
					}
				},
			})
		}
		return launcherview.FormServiceField(launcherview.FormServiceFieldProps{
			Width: width, Height: height, LabelWidth: callbacks.labelWidth, Title: a.translate(value.Title), Description: a.translate(value.Description),
			Status: a.translate(value.Status), Detail: value.Detail, Error: callbacks.serviceError, Actions: actions, Theme: palette.componentTheme(),
		})
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
	case "selectAIModel":
		if callbacks.openAIModelChoice != nil {
			return a.buildFormAIModelField(fields, callbacks, palette, index, definition, width, height)
		}
		fallthrough
	case "select":
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

// buildFormAIModelField maps the JSON-backed model value into Flutter's provider and model controls.
func (a *App) buildFormAIModelField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	models := aiModelsFromOptions(definition.Value.Options)
	selected := aiModel{}
	_ = json.Unmarshal([]byte(fields.values[definition.Value.Key]), &selected)
	providerLabel := selected.Provider
	if selected.ProviderAlias != "" {
		providerLabel = selected.ProviderAlias
	}
	if providerLabel == "" {
		providerLabel = a.translate("i18n:ui_ai_model_selector_not_selected")
	}
	modelLabel := selected.Name
	if modelLabel == "" {
		modelLabel = a.translate("i18n:ui_ai_model_selector_not_selected")
	}

	var providerIcon *woxui.Image
	for _, provider := range a.aiSettings.ProviderCatalog() {
		if provider.Name == selected.Provider {
			providerIcon = a.imageForSize(provider.Icon, physicalImageSize(18, callbacks.imageScale))
			break
		}
	}
	foreground := palette.resultTitle
	return launcherview.FormAIModelField(launcherview.FormAIModelFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip),
		Provider: providerLabel, Model: modelLabel, ProviderIcon: providerIcon, ModelIcon: providerIcon, ModelsAvailable: len(models) > 0,
		ModelNameHint: a.translate("i18n:ui_ai_model_selector_model_name"),
		Width:         width, Height: height, LabelWidth: callbacks.labelWidth, Focused: fields.active && fields.focused == index,
		EditIcon: a.imageForTint(settingControlIconSource("edit"), &foreground, physicalImageSize(18, callbacks.imageScale)),
		ListIcon: a.imageForTint(settingControlIconSource("list"), &foreground, physicalImageSize(18, callbacks.imageScale)),
		Window:   a.formFieldNativeWindow(callbacks.idPrefix), Theme: palette.componentTheme(),
		OnProviderTap:      func(anchor woxui.Rect) { callbacks.openAIModelChoice(index, true, anchor) },
		OnModelTap:         func(anchor woxui.Rect) { callbacks.openAIModelChoice(index, false, anchor) },
		OnModelNameChanged: func(value string) { callbacks.setAIModelName(index, value) },
		OnFinishEdit:       func(value string) { callbacks.finishAIModelEdit(index, value) },
		OnEditModeChanged:  func(bool) { callbacks.focus(index) },
	})
}

func aiModelsFromOptions(options []formOption) []aiModel {
	models := make([]aiModel, 0, len(options))
	for _, option := range options {
		var model aiModel
		if json.Unmarshal([]byte(option.Value), &model) == nil && strings.TrimSpace(model.Name) != "" && strings.TrimSpace(model.Provider) != "" {
			models = append(models, model)
		}
	}
	return models
}

func (a *App) buildFormModelField(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	selectedID := fields.values[definition.Value.Key]
	selectedLabel := selectedID
	for _, option := range definition.Value.Options {
		if modelOptionID(option) != selectedID {
			continue
		}
		selectedLabel = modelOptionLabel(option)
		break
	}
	if strings.TrimSpace(selectedLabel) == "" {
		selectedLabel = a.translate("i18n:plugin_dictation_model_select_hint")
	}
	return launcherview.FormModelField(launcherview.FormModelFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip), Value: selectedLabel,
		Width: width, Height: height, LabelWidth: callbacks.labelWidth, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func(anchor woxui.Rect) {
			callbacks.focus(index)
			if callbacks.openModel != nil {
				callbacks.openModel(index, anchor)
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
	presentation := a.hotkeyRecordingFieldStatus(callbacks.idPrefix, index)
	if !presentation.Active && callbacks.hotkeyError != "" {
		presentation.Status = callbacks.hotkeyError
		presentation.Error = true
	}
	if presentation.Active {
		value = presentation.Value
	}
	hold := strings.HasPrefix(strings.TrimSpace(value), "hold:")
	placeholder := a.translate("i18n:ui_hotkey_click_to_set")
	if presentation.Active {
		placeholder = a.translate("i18n:ui_hotkey_recording")
	}
	return launcherview.FormHotkeyField(launcherview.FormHotkeyFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip),
		Value: value, Labels: formatHotkeyLabels(value), Placeholder: placeholder, Status: presentation.Status, Recording: presentation.Active, Error: presentation.Error,
		Hold: hold, HoldPrefix: a.translate("i18n:ui_hotkey_hold_prefix"),
		Width: width, Height: height, LabelWidth: callbacks.labelWidth, SettingsLayout: callbacks.settingsLayout, AlignRecorderRight: callbacks.alignHotkeyRight,
		Window: a.formFieldNativeWindow(callbacks.idPrefix), Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			if callbacks.recordKey != nil {
				callbacks.recordKey(index)
			}
		},
		OnFocusChange: func(focused bool) {
			if focused {
				callbacks.focus(index)
				if callbacks.recordKey != nil {
					callbacks.recordKey(index)
				}
				return
			}
			if a.hotkeyRecordingFieldStatus(callbacks.idPrefix, index).Active {
				a.stopHotkeyRecording()
			}
		},
	})
}

func (a *App) buildFormChoice(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32, checked bool, selectedLabel string) woxwidget.Widget {
	if definition.Type == "checkbox" {
		return launcherview.FormSwitchField(launcherview.FormSwitchFieldProps{
			ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip),
			Width: width, Height: height, LabelWidth: callbacks.labelWidth, Checked: checked, Theme: palette.componentTheme(),
			OnChange: func(bool) {
				callbacks.focus(index)
				callbacks.change(index, 1)
			},
		})
	}
	props := launcherview.FormSelectFieldProps{
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip), Value: selectedLabel,
		Width: width, Height: height, LabelWidth: callbacks.labelWidth, Focused: fields.active && fields.focused == index, Theme: palette.componentTheme(),
		OnTap: func() {
			callbacks.focus(index)
			callbacks.change(index, 1)
		},
	}
	if callbacks.openChoice != nil {
		props.OnTap = nil
		props.OnChoiceTap = func(anchor woxui.Rect) { callbacks.openChoice(index, anchor) }
	}
	return launcherview.FormSelectField(props)
}

func (a *App) buildFormTextbox(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette uiPalette, index int, definition formDefinition, width, height float32) woxwidget.Widget {
	focused := fields.active && fields.focused == index
	state := fields.editing
	var controller *woxwidget.TextEditingController
	if !focused {
		state = woxui.TextEditingState{Text: fields.values[definition.Value.Key]}
	} else {
		controller = fields.editor
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
		ID: fmt.Sprintf("%s-field-%d", callbacks.idPrefix, index), Label: a.translate(definition.Value.Label), Description: a.translate(definition.Value.Tooltip), Suffix: a.translate(definition.Value.Suffix),
		Width: width, Height: height, LabelWidth: callbacks.labelWidth,
		State: state, Controller: controller, Focused: focused, Protected: definition.Type == "password", MaxLines: maxLines,
		Window: a.formFieldNativeWindow(callbacks.idPrefix), Theme: palette.componentTheme(), OnBrowse: onBrowse, BrowseLabel: a.translate("i18n:ui_runtime_browse"),
		OnFocus: func() { callbacks.focus(index) },
		OnChanged: func(value string) {
			if callbacks.setText != nil {
				callbacks.setText(index, value)
			}
		},
		OnKey: callbacks.onKey,
	})
}
