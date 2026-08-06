package launcher

import (
	"context"
	"strings"
	"testing"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type skillAddTestServices struct {
	contract.Services
	cloned     []contract.AISkill
	cloneErr   error
	updated    map[string]string
	updatedErr error
}

func (s *skillAddTestServices) CloneAISkills(context.Context, string, string) ([]contract.AISkill, error) {
	return s.cloned, s.cloneErr
}

func (s *skillAddTestServices) UpdateGeneralSetting(_ context.Context, _ string, key, value string) error {
	if s.updated == nil {
		s.updated = map[string]string{}
	}
	s.updated[key] = value
	return s.updatedErr
}

func skillAddTestApp(t *testing.T, services *skillAddTestServices) *App {
	t.Helper()
	definition := formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "AISkills", SortColumnKey: "Name", InlineTable: true,
		Columns: []formTableColumn{
			{Key: "Name", Label: "Name", Width: 200, Type: "text", HideInUpdate: true},
			{Key: "Source", Label: "Source", Width: 100, Type: "aiSkillSource", HideInUpdate: true},
			{Key: "Description", Label: "Description", Width: 400, Type: "text", HideInUpdate: true},
			{Key: "Path", Label: "Path", Type: "dirPath", HideInTable: true},
			{Key: "SourceUrl", Label: "Source URL", Type: "text", HideInUpdate: true, HideInTable: true},
		},
	}}
	aiForm := newFormFieldsState([]formDefinition{definition}, map[string]string{"AISkills": `[]`}, true)
	deps := CommonDeps{}
	ai := newAISettingsController(deps)
	ai.SetForm(&aiForm)
	return &App{
		settingsOpen:   true,
		settingTab:     "ai",
		aiSettings:     ai,
		pluginSettings: newPluginSettingsController(deps),
		hotkeySettings: newHotkeySettingsController(deps),
		services:       services,
		lifecycleCtx:   context.Background(),
	}
}

func waitForSettingsUpdate(t *testing.T, services *skillAddTestServices, key string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := services.updated[key]; ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("settings key %q was not persisted", key)
}

func TestOpenFormTableSkillAddDialog(t *testing.T) {
	app := skillAddTestApp(t, &skillAddTestServices{})
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()

	state := app.settingsTableEditor
	if state == nil || state.skillAdd == nil {
		t.Fatal("add-skill dialog did not open over the skills table")
	}
	if state.skillAdd.tab != 0 || state.skillAdd.cloning {
		t.Fatalf("dialog state = tab %d cloning %v, want local and not cloning", state.skillAdd.tab, state.skillAdd.cloning)
	}
	if len(state.skillAdd.fields.definitions) != 2 {
		t.Fatalf("dialog fields = %d, want Path and SourceUrl", len(state.skillAdd.fields.definitions))
	}
	if state.skillAdd.fields.focused != 0 {
		t.Fatalf("initial focus = %d, want the local Path field", state.skillAdd.fields.focused)
	}
}

func TestSkillAddDialogRendersTabsFieldAndActions(t *testing.T) {
	app := skillAddTestApp(t, &skillAddTestServices{})
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()

	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return app.buildFormTableOverlay(snapshotFormTableEditorLocked(app.settingsTableEditor), uiPalette{}, 900, 700, 1)
	})
	host.AttachServices(formTableHostServices{})
	app.settingsHost = host
	displayList := woxui.DisplayList{}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 900, Height: 700}, PixelSize: woxui.PixelSize{Width: 900, Height: 700}, Scale: 1}
	host.Frame(&displayList, frame)

	for _, key := range []string{"form-table-skill-add-tab-0", "form-table-skill-add-tab-1", "form-table-skill-add-confirm", "form-table-skill-add-cancel", "form-table-row-field-0"} {
		if _, ok := host.BoundsForKey(woxwidget.Key(key)); !ok {
			t.Fatalf("missing dialog element %q", key)
		}
	}
	if _, ok := host.BoundsForKey("form-table-row-0"); ok {
		t.Fatal("skill add dialog must not render the row list")
	}
}

func TestSwitchSkillAddTabFocusesMatchingField(t *testing.T) {
	app := skillAddTestApp(t, &skillAddTestServices{})
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()
	app.switchFormTableSkillAddTab(1)

	state := app.settingsTableEditor
	if state.skillAdd.tab != 1 {
		t.Fatalf("tab = %d, want remote", state.skillAdd.tab)
	}
	if state.skillAdd.fields.focused != 1 {
		t.Fatalf("focused field = %d, want the SourceUrl field", state.skillAdd.fields.focused)
	}

	app.switchFormTableSkillAddTab(0)
	if app.settingsTableEditor.skillAdd.tab != 0 {
		t.Fatalf("switching back to local failed, tab = %d", app.settingsTableEditor.skillAdd.tab)
	}
}

func TestAddLocalSkillFromDialogAppendsAndSaves(t *testing.T) {
	services := &skillAddTestServices{}
	app := skillAddTestApp(t, services)
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()
	app.setFormTableSkillAddText(0, "/tmp/my-skill")
	app.addFormTableSkill()

	if app.settingsTableEditor != nil {
		t.Fatal("successful local skill add should close the dialog")
	}
	waitForSettingsUpdate(t, services, "AISkills")
	if value := services.updated["AISkills"]; !strings.Contains(value, "/tmp/my-skill") {
		t.Fatalf("persisted skills = %s, want the local path", value)
	}
}

func TestAddSkillDialogValidatesLocalPath(t *testing.T) {
	app := skillAddTestApp(t, &skillAddTestServices{})
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()
	app.addFormTableSkill()

	state := app.settingsTableEditor
	if state == nil || state.skillAdd == nil {
		t.Fatal("empty path must keep the dialog open")
	}
	if state.skillAdd.error == "" {
		t.Fatal("empty local path should surface the required-path validation error")
	}
	if len(state.rows) != 0 {
		t.Fatal("validation failure must not mutate the table")
	}
}

func TestAddRemoteSkillFromDialogClonesAndSaves(t *testing.T) {
	services := &skillAddTestServices{cloned: []contract.AISkill{
		{Name: "repo-skill-a", Source: "remote", SourceName: "Remote", Path: "/cache/a", Enabled: true},
		{Name: "repo-skill-b", Source: "remote", SourceName: "Remote", Path: "/cache/b", Enabled: true},
	}}
	app := skillAddTestApp(t, services)
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()
	app.switchFormTableSkillAddTab(1)
	app.setFormTableSkillAddText(1, "https://github.com/user/repo")
	app.addFormTableSkill()

	waitForSettingsUpdate(t, services, "AISkills")
	if app.settingsTableEditor != nil {
		t.Fatal("successful remote skill clone should close the dialog")
	}
	value := services.updated["AISkills"]
	if !strings.Contains(value, "repo-skill-a") || !strings.Contains(value, "repo-skill-b") {
		t.Fatalf("persisted skills = %s, want both cloned skills", value)
	}
}

func TestAddRemoteSkillFromDialogKeepsOpenOnCloneError(t *testing.T) {
	services := &skillAddTestServices{cloneErr: context.DeadlineExceeded}
	app := skillAddTestApp(t, services)
	app.openAISettingsTable(0)
	app.openFormTableSkillAdd()
	app.switchFormTableSkillAddTab(1)
	app.setFormTableSkillAddText(1, "https://github.com/user/repo")
	app.addFormTableSkill()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		state := app.settingsTableEditor
		if state != nil && state.skillAdd != nil && !state.skillAdd.cloning && state.skillAdd.error != "" {
			if len(state.rows) != 0 {
				t.Fatal("clone failure must not append rows")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("clone failure should surface in the dialog and keep it open, editor=%v", app.settingsTableEditor)
}
