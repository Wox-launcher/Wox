package system

import (
	"os"
	"path/filepath"
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
