package launcher

import (
	"encoding/json"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildFormTableAppPicker resolves controller-owned image resources before delegating to the pure view.
func (a *App) buildFormTableAppPicker(snapshot *formTableAppPickerSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	apps := a.hotkeySettings.AppCandidates()
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
			Name: candidate.Name, Identity: candidate.Identity, Detail: detail, Icon: a.imageForSize(candidate.Icon, physicalImageSize(28, imageScale)),
		}
	}
	theme := palette.componentTheme()
	cancelLabel := a.translate("i18n:ui_cancel")
	confirmLabel := a.translate("i18n:ui_ok")
	appsError := a.hotkeySettings.AppsError()
	appsLoading := a.hotkeySettings.AppsLoading() || (!a.hotkeySettings.AppsLoaded() && appsError == "")
	return launcherview.FormAppPickerView(launcherview.FormAppPickerProps{
		OverlayWidth: width, OverlayHeight: height, Window: a.formTableNativeWindow(), Theme: theme,
		Title: a.translate("i18n:ui_hotkey_ignore_apps_dialog_title"), SearchPlaceholder: a.translate("i18n:ui_hotkey_ignore_apps_search"),
		LoadingLabel: a.translate("i18n:ui_hotkey_ignore_apps_loading"), EmptyLabel: a.translate("i18n:ui_hotkey_ignore_apps_empty"),
		CancelLabel: cancelLabel, ConfirmLabel: confirmLabel, CancelWidth: a.formTableButtonWidth(cancelLabel, 70), ConfirmWidth: a.formTableButtonWidth(confirmLabel, 70),
		Candidates: candidates, SelectedIdentity: snapshot.current.Identity, Loading: appsLoading, Error: appsError,
		OnConfirm: func(index int) {
			if index < 0 || index >= len(apps) {
				a.closeFormTableAppPicker()
				return
			}
			a.chooseFormTableAppCandidate(apps[index])
		},
		OnCancel: a.closeFormTableAppPicker,
	})
}

// openFormTableAppPicker opens the shared picker while the platform application catalog loads.
func (a *App) openFormTableAppPicker(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) || state.rowForm.definitions[index].Type != "app" {
		return
	}
	var current ignoredHotkeyApp
	_ = json.Unmarshal([]byte(state.rowForm.values[state.rowForm.definitions[index].Value.Key]), &current)
	state.appPicker = &formTableAppPickerState{fieldIndex: index, current: current}
	clearFormTableRowValidationLocked(state)
	a.updateFormTableTextInput(true)
	if !a.hotkeySettings.AppsLoaded() && !a.hotkeySettings.AppsLoading() {
		util.Go(a.lifecycleCtx, "load hotkey app candidates", a.loadHotkeyAppCandidates)
	}
	a.invalidateFormTableWindow()
}

func (a *App) closeFormTableAppPicker() {
	state := a.activeFormTableEditor()
	textInput := false
	if state != nil && state.appPicker != nil {
		state.appPicker = nil
		clearFormTableRowValidationLocked(state)
		textInput = state.rowForm != nil && state.rowForm.editor != nil
	}
	a.updateFormTableTextInput(textInput)
	a.invalidateFormTableWindow()
}

func (a *App) chooseFormTableAppCandidate(candidate ignoredHotkeyApp) {
	state := a.activeFormTableEditor()
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
		a.invalidateFormTableWindow()
		return
	}
	state.rowForm.values[state.rowForm.definitions[fieldIndex].Value.Key] = string(encoded)
	state.appPicker = nil
	clearFormTableRowValidationLocked(state)
	setFormFieldsFocusLocked(state.rowForm, fieldIndex)
	a.updateFormTableTextInput(false)
	a.invalidateFormTableWindow()
}
