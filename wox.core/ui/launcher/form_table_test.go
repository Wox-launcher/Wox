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
		form:           form,
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: newHotkeySettingsController(deps),
		tableEditor:    &formTableEditorState{target: &form.formFieldsState, rowForm: &rowForm, deletePending: -1},
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
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: hotkeys,
		tableEditor:    &formTableEditorState{target: &target, definition: definition, rows: []map[string]any{{"Name": "Clipboard", "Query": "cb"}}, selected: 0, rowIndex: -1},
		settingsSearch: newSettingsSearchController(deps),
		themeSettings:  newThemeSettingsController(deps),
		sharedEdit:     newSharedEditState(),
	}

	app.beginCloneFormTableRowDirect()

	if app.tableEditor.rowForm == nil {
		t.Fatal("clone action did not open a row editor")
	}
	if app.tableEditor.rowIndex != -1 {
		t.Fatalf("clone row index = %d, want -1 so saving appends a new row", app.tableEditor.rowIndex)
	}
	if !app.tableEditor.rowEditorOnly {
		t.Fatal("inline clone should close the direct row editor after saving")
	}
	if value := app.tableEditor.rowForm.values["Name"]; value != "Clipboard" {
		t.Fatalf("cloned Name = %q, want source value", value)
	}
	if value := app.tableEditor.rowForm.values["Query"]; value != "cb" {
		t.Fatalf("cloned Query = %q, want source value", value)
	}
	app.tableEditor.rowBase["Name"] = "Changed"
	if source := app.tableEditor.rows[0]["Name"]; source != "Clipboard" {
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
		tableEditor: &formTableEditorState{
			target: &form.formFieldsState, definition: definition,
			rows: []map[string]any{{"Name": "First"}, {"Name": "Second"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}

	app.deleteFormTableRow()
	if app.tableEditor.deletePending != 0 {
		t.Fatalf("pending delete row = %d, want selected row 0", app.tableEditor.deletePending)
	}
	if len(app.tableEditor.rows) != 2 {
		t.Fatal("opening the confirmation dialog must not delete a row")
	}

	app.cancelFormTableRowDelete()
	if app.tableEditor.deletePending != -1 || len(app.tableEditor.rows) != 2 {
		t.Fatal("canceling deletion must preserve all rows")
	}

	app.deleteFormTableRow()
	app.confirmFormTableRowDelete()
	if len(app.tableEditor.rows) != 1 || app.tableEditor.rows[0]["Name"] != "Second" {
		t.Fatalf("confirmed rows = %#v, want only Second", app.tableEditor.rows)
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
		tableEditor: &formTableEditorState{
			target: &target, definition: definition,
			rows: []map[string]any{{"Name": "First"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}

	app.beginDeleteFormTableRowDirect()

	if app.tableEditor.deletePending != 0 || !app.tableEditor.deleteDirect {
		t.Fatalf("direct delete state = pending %d, direct %v", app.tableEditor.deletePending, app.tableEditor.deleteDirect)
	}
	if len(app.tableEditor.rows) != 1 {
		t.Fatal("direct delete must wait for confirmation")
	}

	app.cancelFormTableRowDelete()
	if app.tableEditor != nil {
		t.Fatal("canceling a direct delete should return to the settings page")
	}
	if value := target.values["Commands"]; value != `[{"Name":"First"}]` {
		t.Fatalf("canceling direct delete changed the table to %s", value)
	}
}
