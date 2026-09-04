package launcher

import (
	"context"
	"testing"
	"time"

	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
)

func TestFormTableIgnoreRuleSaveDoesNotRestoreUncheckedSavedApp(t *testing.T) {
	first := ignoredHotkeyApp{Name: "First", Path: "/apps/first"}
	second := ignoredHotkeyApp{Name: "Second", Path: "/apps/second"}
	preview := &formTablePatternPreviewState{
		savedApps: []ignoredHotkeyApp{first, second},
		unchecked: map[string]bool{formTableAppMatchKey(first.Path, first.Identity): true},
	}
	row, errKey := formTableIgnoreRuleSaveRow("*", false, preview, []ignoredHotkeyApp{first, second})
	apps := formTableIgnoreRuleApps(row)
	if errKey != "" || len(apps) != 1 || apps[0].Path != second.Path {
		t.Fatalf("saved selection = %+v, error = %q", apps, errKey)
	}
	preview.unchecked[formTableAppMatchKey(second.Path, second.Identity)] = true
	if _, errKey = formTableIgnoreRuleSaveRow("*", false, preview, []ignoredHotkeyApp{first, second}); errKey == "" {
		t.Fatal("deselecting every saved app must require a selection")
	}
}

func TestFormTableOrdinaryAppsColumnKeepsItsText(t *testing.T) {
	app := &App{}
	column := formTableColumn{Key: "Apps", Type: "text"}
	cell := app.formTableViewCell(column, map[string]any{"Apps": "ordinary plugin value"}, woxcomponent.Theme{}, 1)
	if cell.Text != "ordinary plugin value" || len(cell.Icons) != 0 {
		t.Fatalf("ordinary Apps column was specialized: %+v", cell)
	}
}

type ignorePreviewService struct {
	contract.Services
}

func (s *ignorePreviewService) IndexedApps(_ context.Context, _ string, pattern string) ([]contract.HotkeyApp, error) {
	return []contract.HotkeyApp{{Name: pattern, Path: "/apps/" + pattern}}, nil
}

// TestFormTablePreviewRequestLifetime delivers results on a controlled UI queue.
func TestFormTablePreviewRequestLifetime(t *testing.T) {
	queue := make(chan func(), 4)
	app := &App{lifecycleCtx: context.Background(), services: &ignorePreviewService{}, uiCall: func(fn func()) error { queue <- fn; return nil }}
	newEditor := func(pattern string) *formTableEditorState {
		return &formTableEditorState{rowForm: &formFieldsState{values: map[string]string{"Pattern": pattern, "IncludeFuture": "true"}}}
	}
	result := func() func() {
		t.Helper()
		select {
		case fn := <-queue:
			return fn
		case <-time.After(5 * time.Second):
			t.Fatal("preview request did not complete")
			return nil
		}
	}
	first := newEditor("old")
	app.launcherTableEditor = first
	app.initFormTablePatternPreview(first)
	oldResult := result()
	first.rowForm.values["Pattern"] = "new"
	app.refreshFormTablePatternPreview(first)
	newResult := result()
	newResult()
	oldResult()
	if len(first.patternPreview.apps) != 1 || first.patternPreview.apps[0].Name != "new" {
		t.Fatalf("stale result replaced current preview: %+v", first.patternPreview.apps)
	}
	second := newEditor("new")
	app.launcherTableEditor = second
	app.initFormTablePatternPreview(second)
	if !second.patternPreview.loading {
		t.Fatal("reopening reused the previous editor's cache")
	}
	result()()
	if len(second.patternPreview.apps) != 1 {
		t.Fatal("reopened preview was not loaded")
	}
}

func TestFormTableIgnoreRuleSaveKeepsDynamicPattern(t *testing.T) {
	row, errKey := formTableIgnoreRuleSaveRow("notepad", true, &formTablePatternPreviewState{}, []ignoredHotkeyApp{
		{Name: "notepad", Path: `C:\Windows\notepad.exe`},
	})
	if errKey != "" || row["IncludeFuture"] != true {
		t.Fatalf("save = %+v err=%q", row, errKey)
	}
	if _, ok := row["Apps"]; ok {
		t.Fatalf("dynamic save must omit Apps: %+v", row)
	}
}

func TestFormTableIgnoreRuleSaveWritesCheckedApps(t *testing.T) {
	chrome := ignoredHotkeyApp{Name: "Chrome", Path: `C:\Desktop\Chrome.lnk`, Identity: "chrome.exe"}
	notes := ignoredHotkeyApp{Name: "Notes", Path: `C:\Apps\Notes.exe`, Identity: "notes.exe"}
	preview := &formTablePatternPreviewState{unchecked: map[string]bool{
		formTableAppMatchKey(notes.Path, notes.Identity): true,
	}}
	row, errKey := formTableIgnoreRuleSaveRow("Chrome", false, preview, []ignoredHotkeyApp{chrome, notes})
	if errKey != "" || row["IncludeFuture"] != false {
		t.Fatalf("save = %+v err=%q", row, errKey)
	}
	apps := formTableIgnoreRuleApps(row)
	if len(apps) != 1 || apps[0].Path != chrome.Path {
		t.Fatalf("saved apps = %+v", apps)
	}
}

func TestFormTableIgnoreRuleSaveRequiresOneCheckedApp(t *testing.T) {
	notes := ignoredHotkeyApp{Name: "Notes", Path: `C:\Apps\Notes.exe`, Identity: "notes.exe"}
	preview := &formTablePatternPreviewState{unchecked: map[string]bool{
		formTableAppMatchKey(notes.Path, notes.Identity): true,
	}}
	_, errKey := formTableIgnoreRuleSaveRow("Notes", false, preview, []ignoredHotkeyApp{notes})
	if errKey != "i18n:plugin_app_ignore_rule_preview_select_one" {
		t.Fatalf("err = %q", errKey)
	}
}

func TestFormTableIgnoreRuleSaveKeepsUnseenSavedApps(t *testing.T) {
	visible := ignoredHotkeyApp{Name: "Notes", Path: `C:\Apps\Notes.exe`, Identity: "notes.exe"}
	uninstalled := ignoredHotkeyApp{Name: "Old", Path: `C:\Apps\Old.exe`, Identity: "old.exe"}
	preview := &formTablePatternPreviewState{savedApps: []ignoredHotkeyApp{uninstalled}}
	row, errKey := formTableIgnoreRuleSaveRow("Notes", false, preview, []ignoredHotkeyApp{visible})
	if errKey != "" {
		t.Fatalf("err = %q", errKey)
	}
	apps := formTableIgnoreRuleApps(row)
	if len(apps) != 2 {
		t.Fatalf("saved apps = %+v", apps)
	}
}

func TestFormTablePatternPreviewToggleUncheckTurnsOffIncludeFuture(t *testing.T) {
	state := &formTableEditorState{
		rowForm:        &formFieldsState{values: map[string]string{"IncludeFuture": "true"}},
		patternPreview: &formTablePatternPreviewState{unchecked: map[string]bool{}},
	}
	if !formTablePatternPreviewToggle(state, "path:notes", false) {
		t.Fatal("expected toggle to apply")
	}
	if state.rowForm.values["IncludeFuture"] != "false" {
		t.Fatalf("IncludeFuture = %q", state.rowForm.values["IncludeFuture"])
	}
	if !state.patternPreview.unchecked["path:notes"] {
		t.Fatal("expected the unchecked app to be recorded")
	}
}

func TestFormTableIgnoreRuleAppPathTooltipPrefersPath(t *testing.T) {
	if got := formTableIgnoreRuleAppPathTooltip(ignoredHotkeyApp{
		Name: "记事本", Identity: "notepad.exe", Path: `C:\Windows\System32\notepad.exe`,
	}); got != `C:\Windows\System32\notepad.exe` {
		t.Fatalf("tooltip = %q", got)
	}
	if got := formTableIgnoreRuleAppPathTooltip(ignoredHotkeyApp{Name: "Settings", Identity: "ms-settings:display"}); got != "ms-settings:display" {
		t.Fatalf("identity tooltip = %q", got)
	}
}

func TestFormTableIgnoreRuleAppsCellShowsPathOnHover(t *testing.T) {
	cell := (&App{}).formTableIgnoreRuleAppsCell(map[string]any{
		"IncludeFuture": false,
		"Apps": []map[string]any{
			{"Name": "notepad", "Path": `C:\Windows\System32\notepad.exe`},
			{"Name": "WeChat", "Path": `C:\Program Files\Tencent\WeChat\WeChat.exe`},
		},
	}, 1)
	if len(cell.Icons) != 2 || cell.Icons[0].Tooltip != `C:\Windows\System32\notepad.exe` || cell.Icons[1].Tooltip != `C:\Program Files\Tencent\WeChat\WeChat.exe` {
		t.Fatalf("app icon props = %+v", cell.Icons)
	}
}

func TestFormTableIgnoreRuleIncludeFutureAndOverflow(t *testing.T) {
	if !formTableIgnoreRuleIncludeFuture(map[string]any{"IncludeFuture": true}) {
		t.Fatal("expected include future")
	}
	if formTableIgnoreRuleIncludeFuture(map[string]any{"IncludeFuture": false}) {
		t.Fatal("expected fixed list")
	}
	if formTableIgnoreRuleAppIconOverflow(6, 6) != 0 {
		t.Fatal("expected no overflow at the limit")
	}
	if formTableIgnoreRuleAppIconOverflow(8, 6) != 2 {
		t.Fatal("expected +2 overflow")
	}
}
