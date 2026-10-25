package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/plugin"
)

func TestFolderActionsExposeStableIDs(t *testing.T) {
	folderPlugin := &FolderPlugin{}

	pathActions := folderPlugin.buildPathActions("folder", true, nil)
	assertFolderActionIDs(t, pathActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		folderCopyPathActionID,
		folderCopyNameActionID,
		folderExecuteCommandHereActionID,
		"add_folder_favorite",
		folderToggleHiddenFilesActionID,
	})

	fileActions := folderPlugin.buildPathActions("file.txt", false, nil)
	assertFolderActionIDs(t, fileActions, []string{
		folderOpenActionID,
		folderBrowseContainingFolderActionID,
		folderOpenContainingFolderActionID,
		folderCopyPathActionID,
		folderCopyNameActionID,
		folderExecuteCommandHereActionID,
		folderToggleHiddenFilesActionID,
	})

	favoriteActions := folderPlugin.buildFavoriteActions("favorite", "folder", true, 0)
	assertFolderActionIDs(t, favoriteActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		folderCopyPathActionID,
		folderCopyNameActionID,
		folderExecuteCommandHereActionID,
		"edit_folder_favorite",
		"delete_folder_favorite",
		folderToggleHiddenFilesActionID,
	})

	fileFavoriteActions := folderPlugin.buildFavoriteActions("main", "main.go", false, 0)
	assertFolderActionIDs(t, fileFavoriteActions, []string{
		folderOpenActionID,
		folderBrowseContainingFolderActionID,
		folderOpenContainingFolderActionID,
		folderCopyPathActionID,
		folderCopyNameActionID,
		folderExecuteCommandHereActionID,
		"edit_folder_favorite",
		"delete_folder_favorite",
		folderToggleHiddenFilesActionID,
	})
}

func TestFolderCopyNamePrefersTitleAndVolumeRoot(t *testing.T) {
	if got := folderCopyName("Droppy", `C:\Users\qianl\Droppy`); got != "Droppy" {
		t.Fatalf("folder name = %q, want Droppy", got)
	}
	if got := folderCopyName("", `C:\Users\qianl\notes.txt`); got != "notes.txt" {
		t.Fatalf("file name = %q, want notes.txt", got)
	}
	if got := folderCopyName("Projects", `D:\dev\Wox`); got != "Projects" {
		t.Fatalf("favorite name = %q, want Projects", got)
	}
}

// TestFolderShiftEnterNavigation keeps file, folder, favorite, and volume-root actions consistent.
func TestFolderShiftEnterNavigation(t *testing.T) {
	root := t.TempDir()
	volumeRoot := filepath.VolumeName(root) + string(os.PathSeparator)
	api := &chatTestAPI{}
	p := &FolderPlugin{api: api}
	for _, test := range []struct {
		name    string
		actions []plugin.QueryResultAction
		want    string
	}{
		{"folder", p.buildPathActions(root, true, nil), root + string(os.PathSeparator)},
		{"file", p.buildPathActions(filepath.Join(root, "report.txt"), false, nil), root + string(os.PathSeparator)},
		{"favorite", p.buildFavoriteActions("Projects", root, true, 0), root + string(os.PathSeparator)},
		{"file favorite", p.buildFavoriteActions("main", filepath.Join(root, "main.go"), false, 0), root + string(os.PathSeparator)},
		{"volume root", p.buildPathActions(volumeRoot, true, nil), volumeRoot},
	} {
		t.Run(test.name, func(t *testing.T) {
			api.changed = common.PlainQuery{}
			if test.actions[0].Id != folderOpenActionID || !test.actions[0].IsDefault {
				t.Fatal("Enter must keep opening the result")
			}
			count := 0
			for _, action := range test.actions {
				if action.Hotkey != "shift+enter" {
					continue
				}
				count++
				if !action.PreventHideAfterAction || action.IsDefault {
					t.Fatal("browsing must keep Wox visible without becoming the default action")
				}
				action.Action(t.Context(), plugin.ActionContext{})
			}
			if count != 1 || api.changed.QueryType != plugin.QueryTypeInput || api.changed.QueryText != test.want {
				t.Fatalf("browse count=%d query=%+v, want %q", count, api.changed, test.want)
			}
		})
	}
}

func TestResolveFolderBrowsePathUsesDirectoryOrParent(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	folder, err := resolveFolderBrowsePath(root)
	if err != nil || folder != root {
		t.Fatalf("folder path = %q, err=%v", folder, err)
	}

	parent, err := resolveFolderBrowsePath(filePath)
	if err != nil || parent != root {
		t.Fatalf("file parent = %q, err=%v, want %q", parent, err, root)
	}

	if _, err := resolveFolderBrowsePath(""); err == nil {
		t.Fatal("empty path should fail")
	}
	if _, err := resolveFolderBrowsePath(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing path should fail")
	}

	t.Setenv("WOX_FOLDER_BROWSE_ROOT", root)
	envFolder, err := resolveFolderBrowsePath(`%WOX_FOLDER_BROWSE_ROOT%`)
	if err != nil || envFolder != root {
		t.Fatalf("env folder path = %q, err=%v, want %q", envFolder, err, root)
	}
}

func TestFolderBrowseCommandRejectsUnknownCommand(t *testing.T) {
	empty := (&FolderPlugin{}).browsePathTool(t.Context(), plugin.InvokePluginToolHandlerOption{})
	if empty.Error == nil {
		t.Fatalf("empty browse path = %#v", empty)
	}
}

func TestFolderQueryCompletesHomePathPrefix(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	projectsPath := filepath.Join(homeDir, "Projects")
	if err := os.Mkdir(projectsPath, 0o755); err != nil {
		t.Fatalf("create Projects folder: %v", err)
	}
	if err := os.Mkdir(filepath.Join(homeDir, "Documents"), 0o755); err != nil {
		t.Fatalf("create Documents folder: %v", err)
	}

	response := (&FolderPlugin{}).Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: "~/Proj",
	})

	if len(response.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(response.Results))
	}
	if response.Results[0].Title != "Projects" || response.Results[0].SubTitle != projectsPath {
		t.Fatalf("result = %#v, want Projects at %q", response.Results[0], projectsPath)
	}
	if response.Results[0].Score != folderResultScore {
		t.Fatalf("result score = %d, want %d", response.Results[0].Score, folderResultScore)
	}
}

func TestFolderQueryListsWindowsEnvPathChildren(t *testing.T) {
	root := t.TempDir()
	resourcesPath := filepath.Join(root, "Programs", "cursor", "resources")
	if err := os.MkdirAll(resourcesPath, 0o755); err != nil {
		t.Fatalf("create resources folder: %v", err)
	}
	t.Setenv("LOCALAPPDATA", root)

	response := (&FolderPlugin{}).Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: `%LOCALAPPDATA%/Programs/cursor/`,
	})

	parentPath := filepath.Dir(filepath.Dir(resourcesPath))
	if len(response.Results) != 2 || response.Results[0].Title != ".." || response.Results[0].SubTitle != parentPath {
		t.Fatalf("results = %#v, want parent %q then resources", response.Results, parentPath)
	}
	if response.Results[1].Title != "resources" || response.Results[1].SubTitle != resourcesPath {
		t.Fatalf("child = %#v, want resources at %q", response.Results[1], resourcesPath)
	}
}

func TestFolderQueryListsParentDirectoryFirst(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "projects")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("create child folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(child, "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	api := &chatTestAPI{}
	folderPlugin := &FolderPlugin{api: api}
	response := folderPlugin.Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: child + string(os.PathSeparator),
	})

	if len(response.Results) != 2 {
		t.Fatalf("result count = %d, want parent and notes.txt", len(response.Results))
	}
	parent := response.Results[0]
	if parent.Title != ".." || parent.SubTitle != root || parent.Score != folderResultScore || !parent.RankAboveUsage {
		t.Fatalf("parent result = %#v, want .. at %q", parent, root)
	}
	if parent.Actions[0].Id != folderGoToParentActionID || !parent.Actions[0].IsDefault || !parent.Actions[0].PreventHideAfterAction {
		t.Fatalf("parent default action = %#v", parent.Actions[0])
	}
	parent.Actions[0].Action(t.Context(), plugin.ActionContext{})
	if api.changed.QueryType != plugin.QueryTypeInput || api.changed.QueryText != root+string(os.PathSeparator) {
		t.Fatalf("parent query = %+v, want %q", api.changed, root+string(os.PathSeparator))
	}
	if response.Results[1].Title != "notes.txt" {
		t.Fatalf("child = %#v, want notes.txt", response.Results[1])
	}

	filtered := folderPlugin.Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: filepath.Join(child, "notes"),
	})
	if len(filtered.Results) != 1 || filtered.Results[0].Title != "notes.txt" {
		t.Fatalf("filtered results = %#v, want only notes.txt", filtered.Results)
	}
}

func TestFolderResultsKeepFilePreviewHidden(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(child, []byte("notes"), 0o644); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	p := &FolderPlugin{}
	listed := p.Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: root + string(os.PathSeparator),
	})
	parentPath, ok := folderParentPath(root)
	if !ok || len(listed.Results) < 2 || listed.Results[0].Title != ".." {
		t.Fatalf("results = %#v, want parent %q then notes.txt", listed.Results, parentPath)
	}
	assertFolderPreviewHidden(t, listed.Results[0], parentPath)
	var notes plugin.QueryResult
	for _, result := range listed.Results[1:] {
		if result.Title == "notes.txt" {
			notes = result
			break
		}
	}
	if notes.Title == "" {
		t.Fatalf("results = %#v, want notes.txt", listed.Results)
	}
	assertFolderPreviewHidden(t, notes, child)
	assertFolderPreviewHidden(t, p.buildPathResult(root, "root", true, 0, nil), root)
	assertFolderPreviewHidden(t, p.buildFavoriteResult("Root", root, 0, 0), root)
}

func TestFolderBrowseActionsUseSharedTriggerQuery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	api := &chatTestAPI{}
	p := &FolderPlugin{api: api}
	p.buildBrowseAction(path, true).Action(t.Context(), plugin.ActionContext{})
	if api.changed.QueryType != plugin.FolderBrowseQuery(path).QueryType || api.changed.QueryText != plugin.FolderBrowseQuery(path).QueryText {
		t.Fatalf("browse query = %+v, want %+v", api.changed, plugin.FolderBrowseQuery(path))
	}

	parent, ok := folderParentPath(path)
	if !ok {
		t.Fatal("browse path should have a parent")
	}
	api.changed = common.PlainQuery{}
	p.buildParentDirectoryActions(parent, nil)[0].Action(t.Context(), plugin.ActionContext{})
	if api.changed.QueryType != plugin.FolderBrowseQuery(parent).QueryType || api.changed.QueryText != plugin.FolderBrowseQuery(parent).QueryText {
		t.Fatalf("parent query = %+v, want %+v", api.changed, plugin.FolderBrowseQuery(parent))
	}

	api.changed = common.PlainQuery{}
	toolResult := p.browsePathTool(t.Context(), plugin.InvokePluginToolHandlerOption{Arguments: map[string]any{"path": path}})
	if toolResult.Error != nil || api.changed.QueryText != plugin.FolderBrowseQuery(path).QueryText {
		t.Fatalf("tool query = %+v, result = %+v", api.changed, toolResult)
	}

	response := p.Query(t.Context(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: "dir",
		Search:         path + string(os.PathSeparator),
	})
	if len(response.Results) == 0 || response.Results[0].Title != ".." {
		t.Fatalf("triggered listing = %#v", response.Results)
	}
	assertFolderPreviewHidden(t, response.Results[0], parent)
}

func TestFolderParentPathStopsAtRoot(t *testing.T) {
	if _, ok := folderParentPath(string(os.PathSeparator)); ok {
		t.Fatal("filesystem root should not have a parent row")
	}
	root := t.TempDir()
	parent, ok := folderParentPath(filepath.Join(root, "child"))
	if !ok || parent != root {
		t.Fatalf("parent = %q ok=%v, want %q", parent, ok, root)
	}
	volume := filepath.VolumeName(root)
	if volume == "" {
		return
	}
	if _, ok := folderParentPath(volume + string(os.PathSeparator)); ok {
		t.Fatalf("volume root %q should not have a parent row", volume)
	}
}

func TestFolderQueryFuzzyMatchesChildName(t *testing.T) {
	root := t.TempDir()
	woxVideoPath := filepath.Join(root, "wox.video")
	if err := os.Mkdir(woxVideoPath, 0o755); err != nil {
		t.Fatalf("create wox.video folder: %v", err)
	}

	response := (&FolderPlugin{}).Query(t.Context(), plugin.Query{
		Type:   plugin.QueryTypeInput,
		Search: filepath.Join(root, "video"),
	})

	if len(response.Results) != 1 || response.Results[0].SubTitle != woxVideoPath {
		t.Fatalf("results = %#v, want wox.video at %q", response.Results, woxVideoPath)
	}
}

type folderFavoriteSettingsAPI struct {
	plugin.API
	settings map[string]string
	logs     []string
}

func (a *folderFavoriteSettingsAPI) GetSetting(_ context.Context, key string) string {
	return a.settings[key]
}

func (a *folderFavoriteSettingsAPI) Log(_ context.Context, _ plugin.LogLevel, message string) {
	a.logs = append(a.logs, message)
}

func (a *folderFavoriteSettingsAPI) SaveSetting(_ context.Context, key string, value string, _ bool) {
	if a.settings == nil {
		a.settings = map[string]string{}
	}
	a.settings[key] = value
}

func TestFolderQueryFindsFileAndFolderFavorites(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	folderPath := filepath.Join(root, "src")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	raw, err := json.Marshal([]folderFavorite{
		{Name: "qlfqlf", Path: filePath},
		{Name: "source", Path: folderPath},
		{Name: "gone", Path: filepath.Join(root, "missing.go")},
	})
	if err != nil {
		t.Fatalf("marshal favorites: %v", err)
	}
	api := &folderFavoriteSettingsAPI{settings: map[string]string{
		folderFavoritesSettingKey: string(raw),
	}}
	folderPlugin := &FolderPlugin{api: api}

	fileResult := folderPlugin.Query(t.Context(), plugin.Query{Type: plugin.QueryTypeInput, Search: "qlfqlf"})
	if len(fileResult.Results) != 1 || fileResult.Results[0].Title != "qlfqlf" || fileResult.Results[0].SubTitle != filePath {
		t.Fatalf("file favorite = %#v logs=%v", fileResult.Results, api.logs)
	}
	if fileResult.Results[0].Icon.ImageType != common.WoxImageTypeFileIcon {
		t.Fatalf("file favorite icon = %s", fileResult.Results[0].Icon.ImageType)
	}
	assertFolderActionIDs(t, fileResult.Results[0].Actions, []string{
		folderOpenActionID,
		folderBrowseContainingFolderActionID,
		folderOpenContainingFolderActionID,
		folderCopyPathActionID,
		folderCopyNameActionID,
		folderExecuteCommandHereActionID,
		"edit_folder_favorite",
		"delete_folder_favorite",
		folderToggleHiddenFilesActionID,
	})

	folderResult := folderPlugin.Query(t.Context(), plugin.Query{Type: plugin.QueryTypeInput, Search: "source"})
	if len(folderResult.Results) != 1 || folderResult.Results[0].Title != "source" || folderResult.Results[0].SubTitle != folderPath {
		t.Fatalf("folder favorite = %#v", folderResult.Results)
	}
	if folderResult.Results[0].Actions[1].Id != folderEnterActionID {
		t.Fatalf("folder favorite browse action = %s", folderResult.Results[0].Actions[1].Id)
	}

	if missing := folderPlugin.Query(t.Context(), plugin.Query{Type: plugin.QueryTypeInput, Search: "gone"}); len(missing.Results) != 0 {
		t.Fatalf("missing favorite = %#v", missing.Results)
	}
}

func TestImportFavoritesAppendsFilesAndFolders(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	folderPath := filepath.Join(root, "src")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	api := &folderFavoriteSettingsAPI{settings: map[string]string{}}
	folderPlugin := &FolderPlugin{api: api}
	results := folderPlugin.ImportFavorites(t.Context(), []ImportedFavorite{
		{Name: "qlfqlf", Path: filePath},
		{Name: "source", Path: folderPath},
		{Name: "gone", Path: filepath.Join(root, "missing.go")},
	})
	if results[0].Status != favoriteImportImported || results[1].Status != favoriteImportImported || results[2].Status != favoriteImportFailed {
		t.Fatalf("import = %+v logs=%v", results, api.logs)
	}
	again := folderPlugin.ImportFavorites(t.Context(), []ImportedFavorite{
		{Name: "qlfqlf", Path: filePath},
		{Name: "QLFQLF", Path: folderPath},
	})
	if again[0].Status != favoriteImportExists || again[1].Status != favoriteImportConflict {
		t.Fatalf("repeat = %+v", again)
	}
	stored := folderPlugin.loadFavorites(t.Context())
	if len(stored) != 2 || stored[0].Name != "qlfqlf" || stored[0].Path != filePath || stored[1].Path != folderPath {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestFolderMetadataEnablesMRU(t *testing.T) {
	metadata := (&FolderPlugin{}).GetMetadata()
	if !metadata.IsSupportFeature(plugin.MetadataFeatureMRU) {
		t.Fatal("folder plugin must declare the MRU feature")
	}
	if len(metadata.TriggerKeywords) != 2 || metadata.TriggerKeywords[0] != "*" || metadata.TriggerKeywords[1] != "cd" {
		t.Fatalf("trigger keywords = %#v, want * and cd", metadata.TriggerKeywords)
	}
}

func TestFolderMRURestoreRebuildsExistingPath(t *testing.T) {
	dir := t.TempDir()
	p := &FolderPlugin{}
	restored, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{folderMRUPathKey: dir},
	})
	if err != nil {
		t.Fatalf("restore folder: %v", err)
	}
	if restored.SubTitle != dir || restored.IdentityKey != dir {
		t.Fatalf("restored folder = %#v", restored)
	}
	if restored.Actions[0].ContextData[folderMRUPathKey] != dir {
		t.Fatalf("restored context = %#v", restored.Actions[0].ContextData)
	}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{}); err == nil {
		t.Fatal("empty context should fail restore")
	}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{folderMRUPathKey: filepath.Join(dir, "missing")},
	}); err == nil {
		t.Fatal("missing path should fail restore")
	}
}

func assertFolderPreviewHidden(t *testing.T, result plugin.QueryResult, path string) {
	t.Helper()
	preview := result.Preview
	if preview.PreviewType != plugin.WoxPreviewTypeFile || preview.PreviewData != path || !preview.DefaultHidden {
		t.Fatalf("preview = %#v, want hidden file preview for %q", preview, path)
	}
}

// assertFolderActionIDs verifies the ordered action contract exposed to the launcher.
func assertFolderActionIDs(t *testing.T, actions []plugin.QueryResultAction, expected []string) {
	t.Helper()
	if len(actions) != len(expected) {
		t.Fatalf("action count = %d, want %d", len(actions), len(expected))
	}
	for index, action := range actions {
		if action.Id != expected[index] {
			t.Fatalf("action %d ID = %q, want %q", index, action.Id, expected[index])
		}
	}
}
