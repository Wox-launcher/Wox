package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/filesearch"
)

type fileSearchNavigationAPI struct {
	fileSearchToolbarTestAPI
	query common.PlainQuery
}

func (a *fileSearchNavigationAPI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	a.query = query
}

// TestFileSearchShiftEnterBrowsesFilesAndFolders checks the path queries handed to Folder.
func TestFileSearchShiftEnterBrowsesFilesAndFolders(t *testing.T) {
	root := filepath.Join(t.TempDir(), "项目 files")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "report.txt")
	if err := os.WriteFile(file, []byte("report"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, item := range []filesearch.SearchResult{{Path: root, IsDir: true}, {Path: file}} {
		t.Run(filepath.Base(item.Path), func(t *testing.T) {
			api := &fileSearchNavigationAPI{}
			search := &FileSearchPlugin{api: api}
			actions := search.buildFileSearchResultActions(t.Context(), item)
			if actions[0].Name != "i18n:plugin_file_open" || actions[0].PreventHideAfterAction {
				t.Fatal("Enter must keep opening the result")
			}
			browseCount := 0
			revealFound := false
			for _, action := range actions {
				if action.Name == "i18n:plugin_file_open_containing_folder" {
					revealFound = action.Hotkey == util.PrimaryHotkey("enter")
				}
				if action.Hotkey == "shift+enter" {
					browseCount++
					if !action.PreventHideAfterAction || action.IsDefault {
						t.Fatal("browsing must keep Wox visible without becoming the default action")
					}
					action.Action(t.Context(), plugin.ActionContext{})
				}
			}
			if browseCount != 1 || api.query.QueryType != plugin.QueryTypeInput || api.query.QueryText != root+string(os.PathSeparator) {
				t.Fatalf("browse count=%d query=%+v, want containing folder", browseCount, api.query)
			}
			if !item.IsDir && !revealFound {
				t.Fatal("file results must retain the system file manager shortcut")
			}
		})
	}
}
