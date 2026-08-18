package system

import (
	"testing"

	"wox/plugin"
)

func TestFolderActionsExposeStableIDs(t *testing.T) {
	folderPlugin := &FolderPlugin{}

	pathActions := folderPlugin.buildPathActions("folder", true, nil)
	assertFolderActionIDs(t, pathActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		"add_folder_favorite",
		folderToggleHiddenFilesActionID,
	})

	favoriteActions := folderPlugin.buildFavoriteActions("favorite", "folder", 0)
	assertFolderActionIDs(t, favoriteActions, []string{
		folderOpenActionID,
		folderEnterActionID,
		"edit_folder_favorite",
		"delete_folder_favorite",
		folderToggleHiddenFilesActionID,
	})
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
