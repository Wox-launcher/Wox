package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestClosingSettingsDropsOnlyTheSettingsTableEditor(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	settingsEditor := &formTableEditorState{deletePending: -1}
	launcherEditor := &formTableEditorState{deletePending: -1}
	app.formTables.settingsEditor = settingsEditor
	app.formTables.launcherEditor = launcherEditor
	if app.formTables.activeFormTableEditor() != settingsEditor {
		t.Fatal("the Settings editor must stay active while both editors exist")
	}

	app.onSettingsWindowClosed()

	if app.formTables.settingsEditor != nil {
		t.Fatal("closing Settings must drop its table editor")
	}
	if app.formTables.activeFormTableEditor() != launcherEditor {
		t.Fatal("closing Settings must leave the launcher table editor active")
	}
}
