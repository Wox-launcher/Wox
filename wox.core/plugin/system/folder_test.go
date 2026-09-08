package system

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"wox/plugin"
)

func TestFolderActionsExposeStableIDs(t *testing.T) {
	folderPlugin := &FolderPlugin{}

	pathActions := folderPlugin.buildPathActions("folder", true, nil)
	assertFolderActionIDs(t, pathActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		folderExecuteCommandHereActionID,
		"add_folder_favorite",
		folderToggleHiddenFilesActionID,
	})

	fileActions := folderPlugin.buildPathActions("file.txt", false, nil)
	assertFolderActionIDs(t, fileActions, []string{
		folderOpenActionID,
		folderExecuteCommandHereActionID,
		folderToggleHiddenFilesActionID,
	})

	favoriteActions := folderPlugin.buildFavoriteActions("favorite", "folder", 0)
	assertFolderActionIDs(t, favoriteActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		folderExecuteCommandHereActionID,
		"edit_folder_favorite",
		"delete_folder_favorite",
		folderToggleHiddenFilesActionID,
	})
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
	result := (&FolderPlugin{}).handlePluginCommand(t.Context(), plugin.PluginCommandRequest{Command: "unknown"})
	if result.Handled {
		t.Fatalf("unknown command should not be handled: %#v", result)
	}

	empty := (&FolderPlugin{}).handlePluginCommand(t.Context(), plugin.PluginCommandRequest{Command: PluginCommandBrowsePath})
	if !empty.Handled || empty.Message == "" {
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

func TestParseFolderQueryPathExpandsWindowsEnv(t *testing.T) {
	root := t.TempDir()
	cursorPath := filepath.Join(root, "Programs", "cursor")
	if err := os.MkdirAll(cursorPath, 0o755); err != nil {
		t.Fatalf("create cursor folder: %v", err)
	}
	t.Setenv("LOCALAPPDATA", root)

	inputs := []string{`%LOCALAPPDATA%/Programs/cursor/`}
	if runtime.GOOS == "windows" {
		inputs = append(inputs, `%LOCALAPPDATA%\Programs\cursor\`)
	}

	for _, input := range inputs {
		path, shouldListChildren, ok := parseFolderQueryPath(input)
		if !ok || !shouldListChildren {
			t.Fatalf("parse %q: ok=%v shouldListChildren=%v", input, ok, shouldListChildren)
		}
		if path != filepath.Clean(cursorPath) {
			t.Fatalf("parse %q = %q, want %q", input, path, cursorPath)
		}
	}
}

func TestParseFolderQueryPathExpandsEnvInsideAbsolutePath(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "tester", "docs")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("create docs folder: %v", err)
	}
	t.Setenv("WOX_FOLDER_TEST_USER", "tester")

	path, shouldListChildren, ok := parseFolderQueryPath(filepath.Join(root, "%WOX_FOLDER_TEST_USER%", "docs"))
	if !ok || shouldListChildren {
		t.Fatalf("ok=%v shouldListChildren=%v", ok, shouldListChildren)
	}
	if path != filepath.Clean(target) {
		t.Fatalf("path = %q, want %q", path, target)
	}
}

func TestParseFolderQueryPathRejectsUnsetEnv(t *testing.T) {
	_ = os.Unsetenv("WOX_FOLDER_MISSING_ENV")

	if _, _, ok := parseFolderQueryPath(`%WOX_FOLDER_MISSING_ENV%/foo`); ok {
		t.Fatal("unset environment variable should not parse as a folder path")
	}
}

func TestParseFolderQueryPathAcceptsProgramFilesX86(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ProgramFiles(x86)", root)

	path, shouldListChildren, ok := parseFolderQueryPath(`%ProgramFiles(x86)%`)
	if !ok || shouldListChildren {
		t.Fatalf("ok=%v shouldListChildren=%v", ok, shouldListChildren)
	}
	if path != filepath.Clean(root) {
		t.Fatalf("path = %q, want %q", path, root)
	}
}

func TestParseFolderQueryPathRejectsPartialPercentToken(t *testing.T) {
	if _, _, ok := parseFolderQueryPath(`%LOCALAPPDATA`); ok {
		t.Fatal("unclosed environment variable should not parse as a folder path")
	}
	if _, _, ok := parseFolderQueryPath(`50% complete`); ok {
		t.Fatal("percent text should not parse as a folder path")
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

	if len(response.Results) != 1 || response.Results[0].Title != "resources" || response.Results[0].SubTitle != resourcesPath {
		t.Fatalf("results = %#v, want resources at %q", response.Results, resourcesPath)
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
