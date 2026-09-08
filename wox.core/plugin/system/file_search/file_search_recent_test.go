package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/plugin"
	"wox/util/filesearch"
	"wox/util/recentfiles"
)

func TestFileSearchEmptyTriggeredQueryShowsRecentFiles(t *testing.T) {
	filePath, folderPath := writeRecentFileFixtures(t)
	restoreRecentFilesFn(t, []recentfiles.File{{Path: filePath}, {Path: folderPath}})

	response := (&FileSearchPlugin{api: fileSearchToolbarTestAPI{}}).Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: "f",
	})
	if len(response.Results) != 2 {
		t.Fatalf("recent results = %d, want 2", len(response.Results))
	}
	if response.Results[0].Title != filepath.Base(filePath) || response.Results[0].SubTitle != filePath {
		t.Fatalf("first recent result = %#v", response.Results[0])
	}
	if len(response.Results[0].Tails) != 1 || response.Results[0].Tails[0].Text != "i18n:plugin_file_result_tail_recent" {
		t.Fatalf("recent tail = %#v", response.Results[0].Tails)
	}
	if len(response.Refinements) == 0 {
		t.Fatal("empty triggered query should still expose file refinements")
	}
}

func TestFileSearchEmptyGlobalQueryHidesRecentFiles(t *testing.T) {
	filePath, _ := writeRecentFileFixtures(t)
	restoreRecentFilesFn(t, []recentfiles.File{{Path: filePath}})

	response := (&FileSearchPlugin{api: fileSearchToolbarTestAPI{}}).Query(context.Background(), plugin.Query{
		Type: plugin.QueryTypeInput,
	})
	if len(response.Results) != 0 {
		t.Fatalf("global empty query results = %#v, want none", response.Results)
	}
}

func TestFileSearchRecentQueryHonorsFolderRefinement(t *testing.T) {
	filePath, folderPath := writeRecentFileFixtures(t)
	restoreRecentFilesFn(t, []recentfiles.File{{Path: filePath}, {Path: folderPath}})

	response := (&FileSearchPlugin{api: fileSearchToolbarTestAPI{}}).Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: "f",
		Refinements:    map[string]string{fileSearchTypeRefinementKey: fileSearchTypeRefinementFolder},
	})
	if len(response.Results) != 1 || response.Results[0].SubTitle != folderPath {
		t.Fatalf("folder-filtered recent results = %#v", response.Results)
	}
}

func TestFileSearchResultTailsMarksRecentFiles(t *testing.T) {
	tails := fileSearchResultTails(filesearch.SearchResult{Path: "/tmp/notes.txt"}, true)
	if len(tails) != 1 || tails[0].Text != "i18n:plugin_file_result_tail_recent" {
		t.Fatalf("recent tail = %#v", tails)
	}
}

func writeRecentFileFixtures(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("notes"), 0644); err != nil {
		t.Fatalf("write recent file: %v", err)
	}
	folderPath := filepath.Join(root, "Projects")
	if err := os.Mkdir(folderPath, 0755); err != nil {
		t.Fatalf("mkdir recent folder: %v", err)
	}
	return filePath, folderPath
}

func restoreRecentFilesFn(t *testing.T, files []recentfiles.File) {
	t.Helper()
	previous := listRecentFilesFn
	listRecentFilesFn = func(context.Context, recentfiles.ListOption) ([]recentfiles.File, error) {
		return files, nil
	}
	t.Cleanup(func() {
		listRecentFilesFn = previous
	})
}
