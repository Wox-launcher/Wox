package launcher

import (
	"context"
	"fmt"
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type formTableHostServices struct{}

func (formTableHostServices) MeasureText(string, woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{}, nil
}
func (formTableHostServices) Invalidate() error               { return nil }
func (formTableHostServices) InvalidateRect(woxui.Rect) error { return nil }
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

func TestFormTableWoxImageCellDoesNotRepeatEmojiAsText(t *testing.T) {
	icon := woxImage{ImageType: "emoji", ImageData: "🤖"}
	cacheKey := imageKey(icon) + "-svg-24"
	app := &App{
		images:         map[string]*woxui.Image{cacheKey: {Width: 1, Height: 1}},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}

	cell := app.formTableViewCell(formTableColumn{Key: "Icon", Type: "woxImage"}, map[string]any{"Icon": icon}, woxcomponent.Theme{}, 1)

	if cell.Icon == nil {
		t.Fatal("woxImage cell should render its image")
	}
	if cell.Text != "" {
		t.Fatalf("woxImage cell text = %q, want empty so emoji is not rendered twice", cell.Text)
	}
	if cell.IconSize != 24 {
		t.Fatalf("woxImage cell icon size = %v, want Flutter's 24", cell.IconSize)
	}
}

func TestFormTableCheckboxCellShowsLocalizedDisabledText(t *testing.T) {
	app := &App{
		translations: map[string]string{"ui_disabled": "Disabled"},
	}
	column := formTableColumn{Key: "Disabled", Type: "checkbox"}

	unchecked := app.formTableViewCell(column, map[string]any{"Disabled": false}, woxcomponent.Theme{}, 1)
	if unchecked.Text != "" || unchecked.Icon != nil {
		t.Fatalf("unchecked cell = %#v, want no text or icon", unchecked)
	}

	checked := app.formTableViewCell(column, map[string]any{"Disabled": true}, woxcomponent.Theme{}, 1)
	if checked.Text != "Disabled" || checked.Icon != nil {
		t.Fatalf("checked cell = %#v, want localized text without an icon", checked)
	}
}

func TestFormTableMultilineFieldUsesRowFormEditingController(t *testing.T) {
	definition := formDefinition{Type: "textbox", Value: formDefinitionValue{Key: "InjectCss", Label: "Inject CSS", MaxLines: 12}}
	fields := newFormFieldsState([]formDefinition{definition}, map[string]string{"InjectCss": "header {\n  display: none;\n}"}, true)
	snapshot := snapshotFormFieldsLocked(&fields)
	app := &App{}

	row := app.buildFormTableRowField(snapshot, formFieldCallbacks{}, uiPalette{}, 0, definition, 600, 120, "")
	rowContainer := row.(woxwidget.Container)
	columns := rowContainer.Child.(woxwidget.Flex)
	rightColumn := columns.Children[1].(woxwidget.Flex)
	field := rightColumn.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)

	if snapshot.editor == nil || field.Controller != snapshot.editor || field.Controller != fields.editor {
		t.Fatal("focused multiline table field must share the row form editing controller")
	}
	field.Controller.SelectAll()
	selection := fields.editor.State().Selection
	if selection.Anchor != 0 || selection.Focus != len([]rune(fields.values["InjectCss"])) {
		t.Fatalf("shared Ctrl+A selection = %#v, want complete InjectCss value", selection)
	}

	field.Controller.SetCaret(len([]rune(field.Controller.Text())))
	field.Controller.InsertText("\nfooter { display: none; }")
	setFormFieldsTextLocked(&fields, 0, field.Controller.Text())
	if !field.Controller.Undo() {
		t.Fatal("shared row form controller should retain undo history after value synchronization")
	}
	setFormFieldsTextLocked(&fields, 0, field.Controller.Text())
	if !field.Controller.Redo() {
		t.Fatal("shared row form controller should retain redo history after undo synchronization")
	}
}

func TestFormTableAppPickerLeavesSearchInputToHost(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "Apps"}}
	form := &formState{formFieldsState: newFormFieldsState([]formDefinition{definition}, map[string]string{"Apps": "[]"}, true)}
	deps := CommonDeps{}
	app := &App{
		form: form, aiSettings: newAISettingsController(deps), pluginSettings: newPluginSettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
		launcherTableEditor: &formTableEditorState{target: &form.formFieldsState, appPicker: &formTableAppPickerState{}, deletePending: -1},
	}
	key := woxui.KeyEvent{Key: "a", Down: true}
	if app.onFormTableKey(key) {
		t.Fatal("printable keys should reach the picker search field")
	}
	if app.onFormTableTextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "app"}) {
		t.Fatal("committed text should reach the picker search field")
	}
	if !app.onFormTableKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.launcherTableEditor.appPicker != nil {
		t.Fatal("Escape should close the app picker")
	}
}

func TestFormTableEmojiPickerLeavesSearchInputToHost(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "TrayQueries"}}
	form := &formState{formFieldsState: newFormFieldsState([]formDefinition{definition}, map[string]string{"TrayQueries": "[]"}, true)}
	deps := CommonDeps{}
	app := &App{
		form: form, aiSettings: newAISettingsController(deps), pluginSettings: newPluginSettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
		launcherTableEditor: &formTableEditorState{target: &form.formFieldsState, emojiPicker: &formTableEmojiPickerState{}, deletePending: -1},
	}
	key := woxui.KeyEvent{Key: "a", Down: true}
	if app.onFormTableKey(key) {
		t.Fatal("printable keys should reach the emoji picker search field")
	}
	if app.onFormTableTextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "emoji"}) {
		t.Fatal("committed text should reach the emoji picker search field")
	}
	if !app.onFormTableKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.launcherTableEditor.emojiPicker != nil {
		t.Fatal("Escape should close the emoji picker")
	}
}

func TestQueryHotkeyPresetsMatchFlutterDefaults(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{
		{Type: "textbox", Value: formDefinitionValue{Key: "Name"}},
		{Type: "textbox", Value: formDefinitionValue{Key: "Width"}},
	}, map[string]string{"Position": "system_default", "Width": "", "MaxResultCount": ""}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}

	app.applyQueryHotkeyPreset(string(queryHotkeyPresetWebPanel))
	if fields.values["Position"] != "center" || fields.values["Width"] != "500" || fields.values["MaxResultCount"] != "12" || fields.values["HideQueryBox"] != "true" || fields.values["HideToolbar"] != "true" {
		t.Fatalf("web panel values = %#v", fields.values)
	}
	app.applyQueryHotkeyPreset(string(queryHotkeyPresetNormal))
	if fields.values["Position"] != "system_default" || fields.values["Width"] != "" || fields.values["MaxResultCount"] != "" || fields.values["HideQueryBox"] != "false" || fields.values["HideToolbar"] != "false" {
		t.Fatalf("normal values = %#v", fields.values)
	}
}

func TestQueryHotkeyPresetVisibilityMatchesFlutter(t *testing.T) {
	if !queryHotkeyFieldVisible(queryHotkeyPresetNormal, "Query", false) || queryHotkeyFieldVisible(queryHotkeyPresetNormal, "Width", false) {
		t.Fatal("normal preset should show core fields only")
	}
	if !queryHotkeyFieldVisible(queryHotkeyPresetWebPanel, "Position", false) || queryHotkeyFieldVisible(queryHotkeyPresetWebPanel, "HideToolbar", false) {
		t.Fatal("preview preset should show display sizing without custom chrome toggles")
	}
	if !queryHotkeyFieldVisible(queryHotkeyPresetCustom, "HideToolbar", false) || queryHotkeyFieldVisible(queryHotkeyPresetCustom, "Disabled", false) || !queryHotkeyFieldVisible(queryHotkeyPresetCustom, "Disabled", true) {
		t.Fatal("custom preset should expose chrome fields and edit-only disabled state")
	}
}

func TestQueryHotkeyVariablePickerTriggersAndReplacesText(t *testing.T) {
	if got := queryVariableTriggerStart("x {sel tail", 6); got != 2 {
		t.Fatalf("middle-caret trigger = %d", got)
	}
	fields := newFormFieldsState([]formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "Query"}}}, map[string]string{"Query": ""}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}

	app.setFormTableRowText(0, "open {sel")
	if app.launcherTableEditor.queryVariable == nil || app.launcherTableEditor.queryVariable.triggerStart != 5 {
		t.Fatal("typing an unfinished variable should open the picker at its trigger")
	}
	app.chooseFormTableQueryVariable(0)
	if got := fields.values["Query"]; got != "open {wox:selected_text}" {
		t.Fatalf("typed variable replacement = %q", got)
	}

	fields.editor.SetSelection(0, 4)
	app.openFormTableQueryVariablePicker(0, woxui.Rect{})
	app.chooseFormTableQueryVariable(1)
	if got := fields.values["Query"]; got != "{wox:selected_file} {wox:selected_text}" {
		t.Fatalf("button variable replacement = %q", got)
	}
}

func TestAICommandPromptVariablePickerTriggersAndReplacesText(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "prompt", Tooltip: "i18n:plugin_ai_command_prompt_tooltip"},
	}}, map[string]string{"prompt": ""}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "commands"}}, rowForm: &fields,
	}}

	app.setFormTableRowText(0, "Summarize {inp")
	if app.launcherTableEditor.queryVariable == nil || app.launcherTableEditor.queryVariable.triggerStart != 10 {
		t.Fatal("typing an unfinished AI command variable should open the picker at its trigger")
	}
	app.chooseFormTableQueryVariable(0)
	if got := fields.values["prompt"]; got != "Summarize {wox:input_text}" {
		t.Fatalf("typed AI command variable replacement = %q", got)
	}

	fields.editor.SetText("Extract facts:\n", false)
	fields.editor.SetCaret(len([]rune("Extract facts:\n")))
	app.openFormTableQueryVariablePicker(0, woxui.Rect{})
	app.chooseFormTableQueryVariable(0)
	if got := fields.values["prompt"]; got != "Extract facts:\n{wox:input_text}" {
		t.Fatalf("button AI command variable replacement = %q", got)
	}
}

func TestDictationPromptVariablePickerInsertsDictationText(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "prompt", Tooltip: "i18n:plugin_dictation_action_prompt_tooltip"},
	}}, map[string]string{"prompt": "Rewrite "}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "actions"}}, rowForm: &fields,
	}}

	fields.editor.SetCaret(len([]rune(fields.values["prompt"])))
	app.openFormTableQueryVariablePicker(0, woxui.Rect{})
	app.chooseFormTableQueryVariable(0)
	if got := fields.values["prompt"]; got != "Rewrite {wox:dictation_text}" {
		t.Fatalf("dictation variable replacement = %q", got)
	}
}

func TestQueryVariableTokensIgnoreIncompletePlaceholders(t *testing.T) {
	value := "open {wox:selected_ and {wox:selected_text} done"
	tokens := queryVariableTokens(value)
	closed := strings.Index(value, "{wox:selected_text}")
	if len(tokens) != 1 || tokens[0].start != closed || tokens[0].end != closed+len("{wox:selected_text}") {
		t.Fatalf("tokens = %#v, want only the closed selected_text placeholder at %d", tokens, closed)
	}
}

func TestQueryVariableBackspaceDeletesWholePlaceholder(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "Query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip"},
	}}, map[string]string{"Query": "ai translate {wox:selected_text} now"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}
	fields.editor.SetCaret(len([]rune("ai translate {wox:selected_text}")))
	if !app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}) {
		t.Fatal("backspace after a complete placeholder should be handled")
	}
	if got := fields.values["Query"]; got != "ai translate  now" {
		t.Fatalf("backspace text = %q, want the whole placeholder removed", got)
	}
}

func TestQueryVariableDeleteRemovesWholePlaceholder(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "prompt", Tooltip: "i18n:plugin_ai_command_prompt_tooltip"},
	}}, map[string]string{"prompt": "Summarize {wox:input_text} please"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "commands"}}, rowForm: &fields,
	}}
	fields.editor.SetCaret(len([]rune("Summarize ")))
	if !app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyDelete, Down: true}) {
		t.Fatal("delete before a complete placeholder should be handled")
	}
	if got := fields.values["prompt"]; got != "Summarize  please" {
		t.Fatalf("delete text = %q, want the whole placeholder removed", got)
	}
}

func TestQueryVariableArrowsJumpPlaceholder(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "Query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip"},
	}}, map[string]string{"Query": "{wox:selected_file}"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}
	fields.editor.SetCaret(len([]rune("{wox:selected_file}")))
	if !app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyArrowLeft, Down: true}) {
		t.Fatal("left arrow should jump to the start of the placeholder")
	}
	if got := fields.editor.State().Selection.Focus; got != 0 {
		t.Fatalf("caret after left = %d, want 0", got)
	}
	if !app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true}) {
		t.Fatal("right arrow should jump to the end of the placeholder")
	}
	if got := fields.editor.State().Selection.Focus; got != len([]rune("{wox:selected_file}")) {
		t.Fatalf("caret after right = %d, want placeholder end", got)
	}
}

func TestQueryVariableCaretSnapsOutOfPlaceholder(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "Query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip"},
	}}, map[string]string{"Query": "x{wox:selected_text}y"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}
	fields.editor.SetCaret(3)
	app.snapFormTableQueryVariableSelection()
	if got := fields.editor.State().Selection.Focus; got != 1 {
		t.Fatalf("snapped caret = %d, want placeholder start", got)
	}
}

func TestQueryVariablePartialSelectionDeletesWholePlaceholder(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "Query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip"},
	}}, map[string]string{"Query": "keep {wox:selected_text} tail"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}
	start := strings.Index(fields.values["Query"], "selected")
	fields.editor.SetSelection(start, start+8)
	if !app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}) {
		t.Fatal("deleting a partial placeholder should be handled")
	}
	if got := fields.values["Query"]; got != "keep  tail" {
		t.Fatalf("partial delete text = %q, want the whole placeholder removed", got)
	}
}

func TestQueryVariableIncompletePlaceholderStillDeletesByCharacter(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{
		Type: "textbox", Value: formDefinitionValue{Key: "Query", Tooltip: "i18n:ui_query_hotkeys_query_tooltip"},
	}}, map[string]string{"Query": "open {wox:selected_"}, true)
	app := &App{launcherTableEditor: &formTableEditorState{
		definition: formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys"}}, rowForm: &fields,
	}}
	fields.editor.SetCaret(len([]rune("open {wox:selected_")))
	if app.handleFormTableQueryVariableEditorKey(woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true}) {
		t.Fatal("incomplete placeholder should keep character-by-character editing")
	}
}

func TestFormTableColumnDefinitionKeepsAICommandPromptType(t *testing.T) {
	field, editable := formTableColumnDefinition(formTableColumn{
		Key: "prompt", Type: "aiCommandPrompt", Tooltip: "translated prompt tip", TextMaxLines: 10,
	}, nil)
	if !editable || field.Type != "textbox" || field.Value.ColumnType != "aiCommandPrompt" {
		t.Fatalf("ai command prompt field = type %q column %q editable %v", field.Type, field.Value.ColumnType, editable)
	}
	if formTableQueryVariableKind(field) != formTableQueryVariableKindAICommand {
		t.Fatal("translated AI command prompt tooltip should still offer input_text variables")
	}
}

func TestFormTableQueryVariableKindMatchesFieldTooltips(t *testing.T) {
	if got := formTableQueryVariableKind(formDefinition{Value: formDefinitionValue{ColumnType: "aiCommandPrompt", Tooltip: "translated prompt tip"}}); got != formTableQueryVariableKindAICommand {
		t.Fatalf("ai command column type = %q", got)
	}
	if got := formTableQueryVariableKind(formDefinition{Value: formDefinitionValue{Tooltip: "i18n:plugin_ai_command_prompt_tooltip"}}); got != formTableQueryVariableKindAICommand {
		t.Fatalf("ai command kind = %q", got)
	}
	if got := formTableQueryVariableKind(formDefinition{Value: formDefinitionValue{Tooltip: "i18n:ui_query_hotkeys_query_tooltip"}}); got != formTableQueryVariableKindQueryHotkey {
		t.Fatalf("query hotkey kind = %q", got)
	}
	if got := formTableQueryVariableKind(formDefinition{Value: formDefinitionValue{Tooltip: "i18n:plugin_ai_command_name_tooltip"}}); got != "" {
		t.Fatalf("unrelated field should not offer variables, got %q", got)
	}
}

func TestReplaceQueryHotkeyVariablesForTestUsesSampleValues(t *testing.T) {
	query := "ai translate {wox:selected_text} from {wox:active_browser_url} in {wox:file_explorer_path} file {wox:selected_file}"
	got := replaceQueryHotkeyVariablesForTest(query)
	want := "ai translate test text from https://example.com in /path/to/folder file /path/to/test.txt"
	if got != want {
		t.Fatalf("test query = %q, want %q", got, want)
	}
	if got := replaceQueryHotkeyVariablesForTest("webview x "); got != "webview x " {
		t.Fatalf("trailing spaces must stay intact for query tests, got %q", got)
	}
}

func TestQueryHotkeyVariablePickerEnterUsesFocusedHost(t *testing.T) {
	target := newHotkeySettingsForm(settingsData{})
	definition := formDefinition{}
	for _, candidate := range target.definitions {
		if candidate.Value.Key == "QueryHotkeys" {
			definition = candidate
			break
		}
	}
	fields, _ := formTableRowFields(definition, nil)
	queryIndex := -1
	for index, field := range fields.definitions {
		if field.Value.Key == "Query" {
			queryIndex = index
			break
		}
	}
	deps := CommonDeps{}
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&target)
	app := &App{
		settingsOpen: true, settingTab: "general", hotkeySettings: hotkeys,
		aiSettings: newAISettingsController(deps), pluginSettings: newPluginSettingsController(deps),
		settingsTableEditor: &formTableEditorState{target: &target, definition: definition, rowForm: &fields, rowIndex: -1, deletePending: -1, queryPreset: queryHotkeyPresetNormal},
		lifecycleCtx:        context.Background(), images: map[string]*woxui.Image{}, imageRequested: map[string]string{}, imageLastUsed: map[string]uint64{}, imageErrors: map[string]string{},
	}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return app.buildFormTableOverlay(snapshotFormTableEditorLocked(app.settingsTableEditor), uiPalette{}, 900, 700, 1)
	})
	host.AttachServices(formTableHostServices{})
	app.settingsHost = host
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 900, Height: 700}, PixelSize: woxui.PixelSize{Width: 900, Height: 700}, Scale: 1}
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, frame)
	queryKey := woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", queryIndex))
	host.RequestFocus(queryKey)
	host.Frame(&displayList, frame)
	if !host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "{"}) {
		t.Fatal("query field should accept the variable trigger")
	}
	host.Frame(&displayList, frame)
	if !host.HasFocus(queryKey) {
		t.Fatal("opening the variable picker after an already-focused frame should preserve query-field focus")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true, Composing: true}) {
		t.Fatal("Enter should be handled by the variable picker")
	}
	if got := fields.values["Query"]; got != "{wox:selected_text}" {
		t.Fatalf("query after Enter = %q", got)
	}
	host.Frame(&displayList, frame)
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) {
		t.Fatal("Tab should move focus from the query field")
	}
	host.Frame(&displayList, frame)
	if host.HasFocus(queryKey) {
		t.Fatal("Tab should leave the query field")
	}
	if fields.active {
		t.Fatal("query editor should stop drawing its caret after Host focus leaves the field")
	}
	click := func(key woxwidget.Key) {
		bounds, ok := host.BoundsForKey(key)
		if !ok {
			t.Fatalf("missing bounds for %q", key)
		}
		position := woxui.Point{X: bounds.X + min(float32(20), bounds.Width/2), Y: bounds.Y + bounds.Height/2}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: position})
		host.Frame(&displayList, frame)
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: position})
		host.Frame(&displayList, frame)
	}
	click("query-hotkey-preset-web-panel")
	if app.settingsTableEditor.queryPreset != queryHotkeyPresetWebPanel {
		t.Fatalf("web-panel preset click selected %q", app.settingsTableEditor.queryPreset)
	}
	click("query-hotkey-preset-normal")
	if app.settingsTableEditor.queryPreset != queryHotkeyPresetNormal {
		t.Fatalf("normal preset click selected %q", app.settingsTableEditor.queryPreset)
	}
	if !host.HasFocus("query-hotkey-preset-normal") {
		t.Fatalf("normal preset should own Host focus before query click, query=%v", host.HasFocus(queryKey))
	}
	bounds, ok := host.BoundsForKey(queryKey)
	if !ok {
		t.Fatal("missing query bounds after switching presets")
	}
	position := woxui.Point{X: bounds.X + 20, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: position})
	if !host.HasFocus(queryKey) || !fields.active {
		t.Fatalf("query pointer down focus: host=%v active=%v", host.HasFocus(queryKey), fields.active)
	}
	host.Frame(&displayList, frame)
	if !fields.active {
		t.Fatal("query focus was cleared while reconciling the pointer-down frame")
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: position})
	host.Frame(&displayList, frame)
	if !host.HasFocus(queryKey) || !fields.active {
		t.Fatalf("clicking the query field after switching presets should restore its single Host focus: host=%v active=%v focused=%d query=%d", host.HasFocus(queryKey), fields.active, fields.focused, queryIndex)
	}
	if !host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "x"}) {
		t.Fatal("query field should accept text after switching presets")
	}
	if got := fields.values["Query"]; got != "{wox:selected_text}x" {
		t.Fatalf("query after preset switch = %q", got)
	}
}

func TestQueryHotkeyRowNormalizesNumericFieldsForCore(t *testing.T) {
	definition := formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys", Columns: []formTableColumn{
		{Key: "Width", Type: "text", EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
		{Key: "MaxResultCount", Type: "text", EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
	}}}
	fields, _ := formTableRowFields(definition, map[string]any{"Width": 500, "MaxResultCount": 12})
	row := formTableRowFromFields(definition, &fields, nil)
	if row["Width"] != 500 || row["MaxResultCount"] != 12 {
		t.Fatalf("numeric query hotkey row = %#v", row)
	}
	fields.values["MaxResultCount"] = "16"
	if got := validateFormTableRow(definition, &fields, nil, -1); got["MaxResultCount"] != "i18n:ui_query_hotkeys_max_result_count_range_error" {
		t.Fatalf("range validation = %#v", got)
	}
}

func TestQueryHotkeyOptionalMaxResultCountZeroIsValid(t *testing.T) {
	definition := formDefinition{Value: formDefinitionValue{Key: "QueryHotkeys", Columns: []formTableColumn{
		{Key: "Name", Type: "text"},
		{Key: "Hotkey", Type: "hotkey"},
		{Key: "Query", Type: "queryHotkeyQuery"},
		{Key: "Width", Type: "text", EmptyAsZero: true, Validators: optionalIntegerValidators(false, 0, 0, "")},
		{Key: "MaxResultCount", Type: "text", EmptyAsZero: true, Validators: optionalIntegerValidators(true, 5, 15, "i18n:ui_query_hotkeys_max_result_count_range_error")},
		{Key: "IsSilentExecution", Type: "checkbox"},
	}}}
	fields, _ := formTableRowFields(definition, map[string]any{
		"Name": "翻译并显示", "Hotkey": "CapsLock+K", "Query": "ai translate-display {wox:selected_text}",
		"Width": 0, "MaxResultCount": 0, "IsSilentExecution": true,
	})
	if fields.values["Width"] != "" || fields.values["MaxResultCount"] != "" {
		t.Fatalf("optional sizing zeros should load blank, got width=%q max=%q", fields.values["Width"], fields.values["MaxResultCount"])
	}
	if got := inferQueryHotkeyPreset(fields.values); got != queryHotkeyPresetSilent {
		t.Fatalf("preset = %q, want silent", got)
	}
	if got := validateFormTableRow(definition, &fields, nil, 0); len(got) != 0 {
		t.Fatalf("zero max results should mean global default, got %#v", got)
	}
	fields.values["MaxResultCount"] = "0"
	if got := validateFormTableRow(definition, &fields, nil, 0); len(got) != 0 {
		t.Fatalf("explicit zero should still mean global default, got %#v", got)
	}
	row := formTableRowFromFields(definition, &fields, nil)
	if row["Width"] != 0 || row["MaxResultCount"] != 0 {
		t.Fatalf("empty optional ints should persist as 0, got %#v", row)
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
	app.settingsTableEditor.fieldErrors = map[string]string{"keyword": "duplicate keyword"}
	app.focusFormTableRowField(0)
	if got := app.settingsTableEditor.fieldErrors["keyword"]; got != "duplicate keyword" {
		t.Fatalf("validation field error = %q after refocus, want it preserved", got)
	}
	app.setFormTableRowText(0, "chat2")
	if got := app.settingsTableEditor.fieldErrors["keyword"]; got != "" {
		t.Fatalf("validation field error = %q after changing text, want it cleared", got)
	}
}

func TestSaveFormTableRowEditSurfacesFieldErrorsInline(t *testing.T) {
	definition := formDefinition{
		Type: "table",
		Value: formDefinitionValue{
			Key: "WebViews",
			Columns: []formTableColumn{
				{Key: "Keyword", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
				{Key: "Url", Type: "text", Validators: []formValidator{{Type: "not_empty"}}},
			},
		},
	}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"WebViews": "[]"}, true)
	deps := CommonDeps{}
	plugins := newPluginSettingsController(deps)
	plugins.SetForm(&pluginSettingsFormState{formFieldsState: target})
	rowForm := newFormFieldsState([]formDefinition{
		{Type: "textbox", Value: formDefinitionValue{Key: "Keyword", Validators: []formValidator{{Type: "not_empty"}}}},
		{Type: "textbox", Value: formDefinitionValue{Key: "Url", Validators: []formValidator{{Type: "not_empty"}}}},
	}, map[string]string{"Keyword": "", "Url": "https://example.com"}, true)
	app := &App{
		settingsOpen: true, settingTab: "plugins", pluginSettings: plugins,
		aiSettings: newAISettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
		settingsTableEditor: &formTableEditorState{
			target: &plugins.Form().formFieldsState, definition: definition, rows: nil, rowForm: &rowForm, rowIndex: -1, deletePending: -1,
		},
		translations: map[string]string{"ui_validator_value_can_not_be_empty": "Value cannot be empty"},
	}

	app.saveFormTableRowEdit()

	if got := app.settingsTableEditor.status; got != "" {
		t.Fatalf("dialog status = %q, want field-level errors only", got)
	}
	if got := app.settingsTableEditor.fieldErrors["Keyword"]; got != "Value cannot be empty" {
		t.Fatalf("Keyword field error = %q", got)
	}
	if _, exists := app.settingsTableEditor.fieldErrors["Url"]; exists {
		t.Fatalf("Url should not have a field error, got %#v", app.settingsTableEditor.fieldErrors)
	}
	if app.settingsTableEditor.rowForm == nil {
		t.Fatal("invalid save should keep the row editor open")
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

func TestDirectFormTableDeleteRemovesRowImmediately(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: "Commands"}}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{"Commands": `[{"Name":"First"},{"Name":"Second"}]`}, true)
	deps := CommonDeps{}
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&target)
	app := &App{
		settingsOpen:   true,
		settingTab:     "general",
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: hotkeys,
		services:       &skillAddTestServices{},
		lifecycleCtx:   context.Background(),
		settingsTableEditor: &formTableEditorState{
			target: &target, definition: definition,
			rows: []map[string]any{{"Name": "First"}, {"Name": "Second"}}, selected: 0, rowIndex: -1, deletePending: -1,
		},
	}

	app.beginDeleteFormTableRowDirect()

	if app.settingsTableEditor != nil {
		t.Fatal("direct delete should close the overlay after removing the row")
	}
	if value := target.values["Commands"]; value != `[{"Name":"Second"}]` {
		t.Fatalf("direct delete persisted %s, want only Second", value)
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

func TestAISkillsDirectDeleteAllowsReadOnlyDiscoveredSkill(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "AISkills", SortColumnKey: "Name", InlineTable: true,
		Columns: []formTableColumn{{Key: "Name", Label: "Name", Width: 200, Type: "text"}, {Key: "Source", Label: "Source", Width: 100, Type: "aiSkillSource"}},
	}}
	aiForm := newFormFieldsState([]formDefinition{definition}, map[string]string{
		"AISkills": `[{"Name":"DiscoveredSkill","Source":"local","ReadOnly":true}]`,
	}, true)
	deps := CommonDeps{}
	ai := newAISettingsController(deps)
	ai.SetForm(&aiForm)
	app := &App{
		settingsOpen:   true,
		settingTab:     "ai",
		aiSettings:     ai,
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: newHotkeySettingsController(deps),
		services:       &skillAddTestServices{},
		lifecycleCtx:   context.Background(),
	}

	app.openFormTableLocked(&aiForm, 0)
	app.selectFormTableRow(0)
	app.beginDeleteFormTableRowDirect()

	if app.settingsTableEditor != nil {
		t.Fatal("discovered skill delete should close the overlay after removal")
	}
	if value := aiForm.values["AISkills"]; value != `[]` {
		t.Fatalf("discovered skill table = %s, want an empty table after delete", value)
	}
}

func TestAISkillsDirectDeleteBlocksBuiltinRow(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "AISkills", SortColumnKey: "Name", InlineTable: true,
		Columns: []formTableColumn{{Key: "Name", Label: "Name", Width: 200, Type: "text"}, {Key: "Source", Label: "Source", Width: 100, Type: "aiSkillSource"}},
	}}
	aiForm := newFormFieldsState([]formDefinition{definition}, map[string]string{
		"AISkills": `[{"Name":"BuiltinSkill","Source":"local","ReadOnly":true,"Builtin":true}]`,
	}, true)
	deps := CommonDeps{}
	ai := newAISettingsController(deps)
	ai.SetForm(&aiForm)
	app := &App{
		settingsOpen:   true,
		settingTab:     "ai",
		aiSettings:     ai,
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: newHotkeySettingsController(deps),
	}

	app.openFormTableLocked(&aiForm, 0)
	app.selectFormTableRow(0)
	app.beginDeleteFormTableRowDirect()

	// A built-in skill must not start a delete. The inline path guards these
	// rows before mutating the table.
	if app.settingsTableEditor == nil {
		t.Fatal("expected the table editor to be open")
	}
	if app.settingsTableEditor.deletePending >= 0 || app.settingsTableEditor.deleteDirect {
		t.Fatalf("built-in skill direct delete = pending %d, direct %v, want no delete started", app.settingsTableEditor.deletePending, app.settingsTableEditor.deleteDirect)
	}
	if value := aiForm.values["AISkills"]; value != `[{"Name":"BuiltinSkill","Source":"local","ReadOnly":true,"Builtin":true}]` {
		t.Fatalf("built-in skill table = %s, want the row preserved", value)
	}
}

func TestAISkillsDirectDeleteDoesNotRenderOverlay(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "AISkills", SortColumnKey: "Name", InlineTable: true,
		Columns: []formTableColumn{{Key: "Name", Label: "Name", Width: 200, Type: "text"}, {Key: "Source", Label: "Source", Width: 100, Type: "aiSkillSource"}},
	}}
	aiForm := newFormFieldsState([]formDefinition{definition}, map[string]string{
		"AISkills": `[{"Name":"DiscoveredSkill","Source":"local","ReadOnly":true}]`,
	}, true)
	deps := CommonDeps{}
	ai := newAISettingsController(deps)
	ai.SetForm(&aiForm)
	app := &App{
		settingsOpen: true, settingTab: "ai", aiSettings: ai,
		pluginSettings: newPluginSettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
		services: &skillAddTestServices{}, lifecycleCtx: context.Background(),
		images: map[string]*woxui.Image{}, imageRequested: map[string]string{}, imageLastUsed: map[string]uint64{}, imageErrors: map[string]string{},
	}

	app.openFormTableLocked(&aiForm, 0)
	app.selectFormTableRow(0)
	app.beginDeleteFormTableRowDirect()

	if app.settingsTableEditor != nil {
		t.Fatal("direct delete must close the overlay instead of showing a confirmation dialog")
	}
	if value := aiForm.values["AISkills"]; value != `[]` {
		t.Fatalf("discovered skill table = %s, want an empty table after delete", value)
	}
}
