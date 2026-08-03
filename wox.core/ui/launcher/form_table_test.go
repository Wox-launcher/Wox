package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type formTableHostServices struct{}

func (formTableHostServices) MeasureText(string, woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{}, nil
}
func (formTableHostServices) Invalidate() error { return nil }
func (formTableHostServices) SetTextInputState(woxui.TextInputState) error {
	return nil
}
func (formTableHostServices) SetPointerCursor(woxui.PointerCursor) error { return nil }
func (formTableHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestFormTableTabIncludesFooterButtons(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "Commands"}}
	form := &formState{formFieldsState: newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": "[]"}, true)}
	deps := CommonDeps{}
	rowForm := newFormFieldsState([]formDefinition{{Type: "checkbox", Value: formDefinitionValue{Key: "Disabled"}}}, nil, true)
	app := &App{
		form:                form,
		aiSettings:          newAISettingsController(deps),
		pluginSettings:      newPluginSettingsController(deps),
		hotkeySettings:      newHotkeySettingsController(deps),
		launcherTableEditor: &formTableEditorState{target: &form.formFieldsState, rowForm: &rowForm, deletePending: -1},
	}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Focusable{Key: "form-table-row-field-0", OnKey: app.onFormTableKey, Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "form-table-row-cancel", Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "form-table-row-save", Child: woxwidget.Container{Width: 100, Height: 30}},
		}}
	})
	host.AttachServices(formTableHostServices{})
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 100}, PixelSize: woxui.PixelSize{Width: 200, Height: 100}, Scale: 1})
	host.RequestFocus("form-table-row-field-0")
	app.host = host

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("form-table-row-cancel") {
		t.Fatal("Tab from the last field should focus Cancel")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("form-table-row-save") {
		t.Fatal("Tab from Cancel should focus Save")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("form-table-row-field-0") {
		t.Fatal("Tab from Save should wrap to the first field")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift, Down: true}) || !host.HasFocus("form-table-row-save") {
		t.Fatal("Shift+Tab from the first field should focus Save")
	}
}

func TestPluginTriggerKeywordRowAcceptsTextInput(t *testing.T) {
	definition := pluginTriggerKeywordDefinition()
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"TriggerKeywords": "[]"}, true)
	deps := CommonDeps{}
	plugins := newPluginSettingsController(deps)
	plugins.SetForm(&pluginSettingsFormState{formFieldsState: target})
	app := &App{
		settingsOpen: true, settingTab: "plugins", pluginSettings: plugins,
		aiSettings: newAISettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
	}
	app.openFormTableLocked(&plugins.Form().formFieldsState, 0)
	app.beginAddFormTableRowDirect()
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return app.buildFormTableOverlay(snapshotFormTableEditorLocked(app.settingsTableEditor), uiPalette{}, 800, 600, 1)
	})
	host.AttachServices(formTableHostServices{})
	app.settingsHost = host
	displayList := woxui.DisplayList{}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 600}, PixelSize: woxui.PixelSize{Width: 800, Height: 600}, Scale: 1}
	host.Frame(&displayList, frame)
	host.Frame(&displayList, frame)

	key := woxui.KeyEvent{Key: "c", Down: true}
	if host.Key(key) {
		t.Fatal("printable key should continue to native text input")
	}
	if app.onSettingsWindowKey(key) {
		t.Fatal("settings window should not route a table editor key to the underlying plugin form")
	}
	if !host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "chat"}) {
		t.Fatal("trigger keyword input did not handle committed text")
	}
	if got := app.settingsTableEditor.rowForm.values["keyword"]; got != "chat" {
		t.Fatalf("trigger keyword = %q, want committed input", got)
	}
	app.settingsTableEditor.status = "duplicate keyword"
	app.focusFormTableRowField(0)
	if got := app.settingsTableEditor.status; got != "duplicate keyword" {
		t.Fatalf("validation status = %q after refocus, want it preserved", got)
	}
	app.setFormTableRowText(0, "chat2")
	if got := app.settingsTableEditor.status; got != "" {
		t.Fatalf("validation status = %q after changing text, want it cleared", got)
	}
}

func TestBeginCloneFormTableRowDirectPrefillsNewRow(t *testing.T) {
	definition := formDefinition{
		Type: "table",
		Value: formDefinitionValue{
			Key: "Commands",
			Columns: []formTableColumn{
				{Key: "Name", Type: "text"},
				{Key: "Query", Type: "text"},
			},
		},
	}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": "[]"}, true)
	deps := CommonDeps{}
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&target)
	app := &App{
		aiSettings:          newAISettingsController(deps),
		pluginSettings:      newPluginSettingsController(deps),
		hotkeySettings:      hotkeys,
		settingsTableEditor: &formTableEditorState{target: &target, definition: definition, rows: []map[string]any{{"Name": "Clipboard", "Query": "cb"}}, selected: 0, rowIndex: -1},
		settingsSearch:      newSettingsSearchController(deps),
		themeSettings:       newThemeSettingsController(deps),
		sharedEdit:          newSharedEditState(),
	}

	app.beginCloneFormTableRowDirect()

	if app.settingsTableEditor.rowForm == nil {
		t.Fatal("clone action did not open a row editor")
	}
	if app.settingsTableEditor.rowIndex != -1 {
		t.Fatalf("clone row index = %d, want -1 so saving appends a new row", app.settingsTableEditor.rowIndex)
	}
	if !app.settingsTableEditor.rowEditorOnly {
		t.Fatal("inline clone should close the direct row editor after saving")
	}
	if value := app.settingsTableEditor.rowForm.values["Name"]; value != "Clipboard" {
		t.Fatalf("cloned Name = %q, want source value", value)
	}
	if value := app.settingsTableEditor.rowForm.values["Query"]; value != "cb" {
		t.Fatalf("cloned Query = %q, want source value", value)
	}
	app.settingsTableEditor.rowBase["Name"] = "Changed"
	if source := app.settingsTableEditor.rows[0]["Name"]; source != "Clipboard" {
		t.Fatalf("cloned row shares source map; source Name changed to %v", source)
	}
}

func TestFormTableDeleteRequiresDialogConfirmation(t *testing.T) {
	definition := formDefinition{
		Type: "table",
		Value: formDefinitionValue{
			Key:     "Commands",
			Columns: []formTableColumn{{Key: "Name", Type: "text"}},
		},
	}
	rowsJSON := `[{"Name":"First"},{"Name":"Second"}]`
	form := &formState{formFieldsState: newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": rowsJSON}, true)}
	deps := CommonDeps{}
	app := &App{
		form:           form,
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: newHotkeySettingsController(deps),
		launcherTableEditor: &formTableEditorState{
			target: &form.formFieldsState, definition: definition,
			rows: []map[string]any{{"Name": "First"}, {"Name": "Second"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}

	app.deleteFormTableRow()
	if app.launcherTableEditor.deletePending != 0 {
		t.Fatalf("pending delete row = %d, want selected row 0", app.launcherTableEditor.deletePending)
	}
	if len(app.launcherTableEditor.rows) != 2 {
		t.Fatal("opening the confirmation dialog must not delete a row")
	}

	app.cancelFormTableRowDelete()
	if app.launcherTableEditor.deletePending != -1 || len(app.launcherTableEditor.rows) != 2 {
		t.Fatal("canceling deletion must preserve all rows")
	}

	app.deleteFormTableRow()
	app.confirmFormTableRowDelete()
	if len(app.launcherTableEditor.rows) != 1 || app.launcherTableEditor.rows[0]["Name"] != "Second" {
		t.Fatalf("confirmed rows = %#v, want only Second", app.launcherTableEditor.rows)
	}
	if value := form.values["Commands"]; value != `[{"Name":"Second"}]` {
		t.Fatalf("persisted table value = %s, want the confirmed deletion only", value)
	}
}

func TestDirectFormTableDeleteMarksDialogWithoutMutatingRows(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "Commands"}}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": `[{"Name":"First"}]`}, true)
	deps := CommonDeps{}
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&target)
	app := &App{
		settingsOpen:   true,
		settingTab:     "general",
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: hotkeys,
		settingsTableEditor: &formTableEditorState{
			target: &target, definition: definition,
			rows: []map[string]any{{"Name": "First"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}

	app.beginDeleteFormTableRowDirect()

	if app.settingsTableEditor.deletePending != 0 || !app.settingsTableEditor.deleteDirect {
		t.Fatalf("direct delete state = pending %d, direct %v", app.settingsTableEditor.deletePending, app.settingsTableEditor.deleteDirect)
	}
	if len(app.settingsTableEditor.rows) != 1 {
		t.Fatal("direct delete must wait for confirmation")
	}

	app.cancelFormTableRowDelete()
	if app.settingsTableEditor != nil {
		t.Fatal("canceling a direct delete should return to the settings page")
	}
	if value := target.values["Commands"]; value != `[{"Name":"First"}]` {
		t.Fatalf("canceling direct delete changed the table to %s", value)
	}
}

func TestOpenFormTableKeepsWindowOwnedEditorStateSeparate(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "Commands"}}
	launcherForm := &formState{formFieldsState: newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": "[]"}, true)}
	settingsForm := newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": "[]"}, true)
	deps := CommonDeps{}
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&settingsForm)
	app := &App{
		form:            launcherForm,
		editor:          woxui.NewTextEditor(""),
		generalSettings: newGeneralSettingsController(deps, newSharedEditState()),
		themeSettings:   newThemeSettingsController(deps),
		aiSettings:      newAISettingsController(deps),
		pluginSettings:  newPluginSettingsController(deps),
		hotkeySettings:  hotkeys,
	}

	app.openFormTableLocked(&settingsForm, 0)
	if app.settingsTableEditor == nil || app.launcherTableEditor != nil {
		t.Fatal("settings table editor leaked into launcher-owned state")
	}
	if app.snapshot().tableEditor != nil {
		t.Fatal("launcher snapshot rendered the settings-owned table editor")
	}

	app.settingsTableEditor = nil
	app.openFormTableLocked(&launcherForm.formFieldsState, 0)
	if app.launcherTableEditor == nil || app.settingsTableEditor != nil {
		t.Fatal("launcher table editor leaked into settings-owned state")
	}
}
