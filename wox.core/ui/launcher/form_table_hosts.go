package launcher

import (
	"context"
	"strings"
	"time"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
)

// formTableTargetCurrentLocked reports whether a table target still belongs to a form
// that is on screen. The table editor uses it to drop work for forms that went away.
func (a *App) formTableTargetCurrentLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return a.formTableTargetCurrentWithFormsLocked(target, pluginForm, a.aiSettings.Form(), a.hotkeySettings.Form(), a.generalQuerySettingsForm())
}

// formTableTargetCurrentWithFormsLocked compares one table target using controller pointers captured for the UI-thread snapshot.
func (a *App) formTableTargetCurrentWithFormsLocked(target *formFieldsState, pluginForm *pluginSettingsFormState, aiForm *formFieldsState, hotkeyForm *formFieldsState, generalForm *formFieldsState) bool {
	return target != nil && ((a.forms.form != nil && target == &a.forms.form.formFieldsState) ||
		(a.requirements.requirementForm != nil && target == &a.requirements.requirementForm.formFieldsState) ||
		(pluginForm != nil && target == &pluginForm.formFieldsState) ||
		(a.settingsWindow.open && a.settingsWindow.tab == "ai" && target == aiForm) ||
		(a.settingsWindow.open && a.settingsWindow.tab == "hotkey" && target == hotkeyForm) ||
		(a.settingsWindow.open && a.settingsWindow.tab == "general" && target == generalForm))
}

// formTableTargetUsesSettingsLocked reports whether a table opened from this form renders in the Settings window.
func (a *App) formTableTargetUsesSettingsLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return target != nil && ((pluginForm != nil && target == &pluginForm.formFieldsState) || target == a.aiSettings.Form() || target == a.hotkeySettings.Form() || target == a.generalQuerySettingsForm())
}

// settingsOwnedTableTarget reports whether a table belongs to a built-in settings form that persists immediately.
func (a *App) settingsOwnedTableTarget(target *formFieldsState) bool {
	if target == nil {
		return false
	}
	if a.aiSettings != nil && target == a.aiSettings.Form() {
		return true
	}
	if a.hotkeySettings != nil && target == a.hotkeySettings.Form() {
		return true
	}
	return target == a.generalQuerySettingsForm()
}

// generalQuerySettingsForm returns the General query-tables form when that controller exists.
func (a *App) generalQuerySettingsForm() *formFieldsState {
	if a.generalSettings == nil {
		return nil
	}
	return a.generalSettings.Form()
}

func (a *App) openActionFormTable(index int) {
	if a.forms.form != nil {
		a.formTables.openFormTableLocked(&a.forms.form.formFieldsState, index)
	}
	a.formTables.finishOpeningFormTable()
}

func (a *App) openRequirementFormTable(index int) {
	if a.requirements.requirementForm != nil {
		a.formTables.openFormTableLocked(&a.requirements.requirementForm.formFieldsState, index)
	}
	a.formTables.finishOpeningFormTable()
}

func (a *App) openPluginFormTable(index int) {
	a.pluginSettings.SetSearchFocused(false)
	pluginForm := a.pluginSettings.Form()
	if pluginForm != nil {
		a.formTables.openFormTableLocked(&pluginForm.formFieldsState, index)
	}
	a.formTables.finishOpeningFormTable()
}

// runLauncherQueryTest shows the launcher with the supplied query so the user can
// see the live result of a settings-side test action.
func (a *App) runLauncherQueryTest(query plainQuery) {
	if strings.TrimSpace(query.QueryText) == "" && len(query.QueryScope.Plugins) == 0 {
		return
	}
	a.settingsWindow.setSettingChoiceTooltip(false, "", woxui.Rect{})

	params := a.show
	params.SelectAll = false
	params.HideQueryBox = false
	params.HideToolbar = false
	params.ShowSource = string(common.ShowSourceDefault)
	if params.Position.Type == "" && a.window != nil {
		if bounds, err := a.window.Bounds(); err == nil {
			params.Position = position{Type: "last_location", X: int(bounds.X), Y: int(bounds.Y)}
		}
	}
	a.setQuery(query)
	util.Go(a.lifecycleCtx, "show launcher for query test", func() {
		if err := a.showWindow(params); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "show launcher for query test: "+err.Error())
			return
		}
		if err := a.sendCurrentQuery(); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "send query test: "+err.Error())
		}
	})
}

// saveSettingsTable persists one settings-owned table and rolls the editor back if core rejects it.
func (a *App) saveSettingsTable(state *formTableEditorState, key, value, previousValue string) {
	coreValue := value
	if key == "IgnoredHotkeyApps" {
		var err error
		coreValue, err = settingsIgnoredHotkeyAppsCoreJSON(value)
		if err != nil {
			_ = a.runOnUI("apply invalid settings table value", func() {
				a.settingsWindow.saving = false
				state.target.values[key] = previousValue
				if a.formTables.settingsEditor == state {
					state.saving = false
					state.status = "Could not save: " + err.Error()
				}
				a.settingsWindow.invalidateSettingsWindow()
			})
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	err := a.services.UpdateGeneralSetting(ctx, a.sessionID, key, coreValue)
	cancel()

	_ = a.runOnUI("apply settings table save", func() {
		a.settingsWindow.saving = false
		if err != nil {
			state.target.values[key] = previousValue
			if a.formTables.settingsEditor == state {
				if rows, decodeErr := decodeFormTableRows(previousValue); decodeErr == nil {
					state.rows = rows
					state.selected = min(state.selected, len(rows)-1)
				}
				state.saving = false
				state.status = "Could not save: " + err.Error()
			}
		} else {
			if state.target == a.aiSettings.Form() {
				a.aiSettings.applyAISettingsRawLocked(key, value)
			} else if state.target == a.hotkeySettings.Form() || state.target == a.generalQuerySettingsForm() {
				a.hotkeySettings.applyHotkeySettingsRawLocked(key, coreValue)
			}
			if a.formTables.settingsEditor == state {
				state.saving = false
				state.status = ""
			}
		}
		a.settingsWindow.invalidateSettingsWindow()
	})
}
