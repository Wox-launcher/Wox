package launcher

import (
	"encoding/json"
	"strings"
	woxcomponent "wox/ui/launcher/component"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildFormTableAppPicker resolves controller-owned image resources before delegating to the pure view.
func (c *formTableController) buildFormTableAppPicker(snapshot *formTableAppPickerSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	apps := c.hotkeySettings().AppCandidates()
	if identity := strings.TrimSpace(snapshot.current.Identity); identity != "" {
		found := false
		for _, candidate := range apps {
			if strings.EqualFold(strings.TrimSpace(candidate.Identity), identity) {
				found = true
				break
			}
		}
		if !found {
			apps = append([]ignoredHotkeyApp{snapshot.current}, apps...)
		}
	}
	candidates := make([]launcherview.FormAppCandidate, len(apps))
	for index, candidate := range apps {
		detail := strings.TrimSpace(candidate.Path)
		if detail == "" {
			detail = candidate.Identity
		}
		candidates[index] = launcherview.FormAppCandidate{
			Name: candidate.Name, Identity: candidate.Identity, Detail: detail, Icon: c.imageForSurface(candidate.Icon, physicalImageSize(28, imageScale), palette.Background),
		}
	}
	theme := palette
	cancelLabel := c.language.translate("i18n:ui_cancel")
	confirmLabel := c.language.translate("i18n:ui_ok")
	appsError := c.hotkeySettings().AppsError()
	appsLoading := c.hotkeySettings().AppsLoading() || (!c.hotkeySettings().AppsLoaded() && appsError == "")
	return launcherview.FormAppPickerView(launcherview.FormAppPickerProps{
		OverlayWidth: width, OverlayHeight: height, Window: c.formTableNativeWindow(), Theme: theme,
		Title: c.language.translate("i18n:ui_hotkey_ignore_apps_dialog_title"), SearchPlaceholder: c.language.translate("i18n:ui_hotkey_ignore_apps_search"),
		LoadingLabel: c.language.translate("i18n:ui_hotkey_ignore_apps_loading"), EmptyLabel: c.language.translate("i18n:ui_hotkey_ignore_apps_empty"),
		CancelLabel: cancelLabel, ConfirmLabel: confirmLabel, CancelWidth: c.formTableButtonWidth(cancelLabel, 70), ConfirmWidth: c.formTableButtonWidth(confirmLabel, 70),
		Candidates: candidates, SelectedIdentity: snapshot.current.Identity, Loading: appsLoading, Error: appsError,
		OnConfirm: func(index int) {
			if index < 0 || index >= len(apps) {
				c.closeFormTableAppPicker()
				return
			}
			c.chooseFormTableAppCandidate(apps[index])
		},
		OnCancel: c.closeFormTableAppPicker,
	})
}

// openFormTableAppPicker opens the shared picker while the platform application catalog loads.
func (c *formTableController) openFormTableAppPicker(index int) {
	state := c.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "app" {
		return
	}
	var current ignoredHotkeyApp
	_ = json.Unmarshal([]byte(state.rowForm.values[state.rowForm.definitions[index].Value.Key]), &current)
	state.appPicker = &formTableAppPickerState{fieldIndex: index, current: current}
	clearFormTableRowValidationLocked(state)
	c.updateFormTableTextInput(true)
	if !c.hotkeySettings().AppsLoaded() && !c.hotkeySettings().AppsLoading() {
		util.Go(c.lifecycleCtx, "load hotkey app candidates", c.loadHotkeyAppCandidates)
	}
	c.invalidateFormTableWindow()
}

func (c *formTableController) closeFormTableAppPicker() {
	state := c.activeFormTableEditor()
	textInput := false
	if state != nil && state.appPicker != nil {
		state.appPicker = nil
		clearFormTableRowValidationLocked(state)
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	c.updateFormTableTextInput(textInput)
	c.invalidateFormTableWindow()
}

func (c *formTableController) chooseFormTableAppCandidate(candidate ignoredHotkeyApp) {
	state := c.activeFormTableEditor()
	if state == nil || state.rowForm == nil || state.appPicker == nil {
		return
	}
	fieldIndex := state.appPicker.fieldIndex
	if fieldIndex < 0 || fieldIndex >= len(state.rowForm.definitions) {
		return
	}
	encoded, err := json.Marshal(candidate)
	if err != nil {
		state.status = err.Error()
		c.invalidateFormTableWindow()
		return
	}
	state.rowForm.values[state.rowForm.definitions[fieldIndex].Value.Key] = string(encoded)
	state.appPicker = nil
	clearFormTableRowValidationLocked(state)
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	c.updateFormTableTextInput(false)
	c.invalidateFormTableWindow()
}
