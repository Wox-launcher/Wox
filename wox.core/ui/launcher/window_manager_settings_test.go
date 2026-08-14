package launcher

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWindowManagerGroupDisplayValueUsesComputedColumns(t *testing.T) {
	row := map[string]any{
		"Id":   "group-1",
		"Name": "Coding",
		"Screens": []any{
			map[string]any{"DisplayId": "1", "DisplayIndex": 0, "Layout": "full", "Assignments": []any{
				map[string]any{"Slot": "full", "App": map[string]any{"Identity": "code.exe", "Name": "Code"}, "Urls": []any{}},
			}},
			map[string]any{"DisplayId": "2", "DisplayIndex": 1, "Layout": "full", "Assignments": []any{}},
		},
	}
	if got := windowManagerGroupDisplayValue(formTableColumn{Key: "Name"}, row); got != "Coding" {
		t.Fatalf("name = %q, want Coding", got)
	}
	if got := windowManagerGroupDisplayValue(formTableColumn{Key: "AppCount"}, row); got != "1" {
		t.Fatalf("app count = %q, want 1", got)
	}
	if got := windowManagerGroupDisplayValue(formTableColumn{Key: "DisplayCount"}, row); got != "2" {
		t.Fatalf("display count = %q, want 2", got)
	}
}

func TestWindowManagerGroupsTableUsesInlineCloneHiddenContract(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: windowManagerGroupsSettingKey, InlineTable: true}}
	if !isWindowManagerGroupsTable(definition) {
		t.Fatal("window manager groups setting should be recognized as a table")
	}
}

func TestWindowGroupEditorBuildsDialog(t *testing.T) {
	dialog, ok := launcherview.WindowGroupEditor(launcherview.WindowGroupEditorProps{
		Width: 1200, Height: 800, Title: "Create workspace", GroupName: "Coding", NamePlaceholder: "Name",
		CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.Theme{}, OnCancel: func() {},
	}).(woxwidget.Stateful)
	if !ok {
		t.Fatal("window group editor should use shared dialog shell")
	}
	props := dialog.Widget.(woxcomponent.DialogProps)
	if props.OnEscape == nil {
		t.Fatal("workspace editor should expose its cancel action to Escape")
	}
	if props.Height != 722 {
		t.Fatalf("workspace editor height = %.0f, want 722", props.Height)
	}
}

func TestWindowGroupEditorAcceptsNameInput(t *testing.T) {
	name := ""
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return launcherview.WindowGroupEditor(launcherview.WindowGroupEditorProps{
			Width: 1200, Height: 800, Title: "Create workspace", GroupName: name, NamePlaceholder: "Name",
			CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.Theme{}, OnNameChanged: func(value string) { name = value },
		})
	})
	host.AttachServices(formTableHostServices{})
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 1200, Height: 800}, PixelSize: woxui.PixelSize{Width: 1200, Height: 800}, Scale: 1})

	if !host.HasFocus("window-group-name") {
		t.Fatal("workspace editor did not focus the name field")
	}
	if !host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "Coding"}) {
		t.Fatal("workspace name field did not handle text input")
	}
	if name != "Coding" {
		t.Fatalf("workspace name = %q, want Coding", name)
	}
}

func TestWindowGroupEditorLeavesPrintableKeysToNativeTextInput(t *testing.T) {
	definition := formDefinition{Type: "table", Value: formDefinitionValue{Key: windowManagerGroupsSettingKey}}
	target := newFormFieldsState([]formDefinition{definition}, map[string]string{windowManagerGroupsSettingKey: "[]"}, true)
	deps := CommonDeps{}
	plugins := newPluginSettingsController(deps)
	plugins.SetForm(&pluginSettingsFormState{pluginID: windowManagerPluginID, formFieldsState: target})
	app := &App{
		settingsOpen: true, settingTab: "plugins", pluginSettings: plugins,
		aiSettings: newAISettingsController(deps), hotkeySettings: newHotkeySettingsController(deps),
	}
	app.settingsTableEditor = &formTableEditorState{
		target: &plugins.Form().formFieldsState, definition: definition,
		windowGroupEditor: &windowGroupEditorState{group: windowManagerGroupUI{}},
	}

	if app.onSettingsWindowKey(woxui.KeyEvent{Key: "c", Down: true}) {
		t.Fatal("workspace name key should continue to native text input")
	}
	if !app.onSettingsWindowKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("Escape should cancel the workspace editor")
	}
	if app.settingsTableEditor.windowGroupEditor != nil {
		t.Fatal("Escape did not cancel the workspace editor")
	}
}
