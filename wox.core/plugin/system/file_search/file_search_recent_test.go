package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

// Capture source diagnostics to verify that only unavailable macOS recents fall back.
type recentSourceTestAPI struct {
	fileSearchToolbarTestAPI
	messages []string
}

func (a *recentSourceTestAPI) Log(_ context.Context, _ plugin.LogLevel, message string) {
	a.messages = append(a.messages, message)
}

func TestFileSearchRecentFallbackPolicy(t *testing.T) {
	filePath, _ := writeRecentFileFixtures(t)
	for _, tc := range []struct {
		name     string
		items    []recentfiles.File
		err      error
		fallback bool
	}{
		{name: "system results", items: []recentfiles.File{{Path: filePath}}},
		{name: "empty", fallback: true},
		{name: "failure", err: errors.New("Spotlight unavailable"), fallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			previous := listRecentFilesFn
			listRecentFilesFn = func(context.Context, recentfiles.ListOption) ([]recentfiles.File, error) { return tc.items, tc.err }
			t.Cleanup(func() { listRecentFilesFn = previous })
			api := &recentSourceTestAPI{}
			response := (&FileSearchPlugin{api: api}).Query(context.Background(), plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "f"})
			usedFallback := strings.Contains(strings.Join(api.messages, "\n"), "source=index_modified")
			if usedFallback != (runtime.GOOS == "darwin" && tc.fallback) {
				t.Fatalf("unexpected fallback policy: %v", api.messages)
			}
			if !tc.fallback && len(response.Results) != 1 {
				t.Fatalf("system recents lost: %#v", response.Results)
			}
		})
	}
}

func TestFileSearchRecentUsesPrepopulatedMetadata(t *testing.T) {
	modTime := time.Unix(1700000123, 0)
	nonExistent := filepath.Join(t.TempDir(), "pre-stat-skipped.txt")
	results := recentFilesToSearchResults([]recentfiles.File{{
		Path:    nonExistent,
		HasMeta: true,
		IsDir:   true,
		ModTime: modTime,
		Size:    42,
	}}, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result from prepopulated metadata, got %d", len(results))
	}
	if !results[0].IsDir || results[0].Size != 42 || results[0].Mtime != modTime.UnixMilli() {
		t.Fatalf("prepopulated metadata mismatch: %+v", results[0])
	}
}

func TestFileSearchRecentQueryCachesBrieflyAndInvalidates(t *testing.T) {
	filePath, _ := writeRecentFileFixtures(t)
	calls := 0
	previous := listRecentFilesFn
	listRecentFilesFn = func(context.Context, recentfiles.ListOption) ([]recentfiles.File, error) {
		calls++
		return []recentfiles.File{{Path: filePath}}, nil
	}
	t.Cleanup(func() { listRecentFilesFn = previous })

	p := &FileSearchPlugin{api: fileSearchToolbarTestAPI{}}
	q := plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "f"}
	resp1 := p.Query(context.Background(), q)
	resp2 := p.Query(context.Background(), q)
	if len(resp1.Results) != 1 || len(resp2.Results) != 1 {
		t.Fatalf("unexpected result counts: %d, %d", len(resp1.Results), len(resp2.Results))
	}
	if calls != 1 {
		t.Fatalf("expected second recent query to hit TTL cache, got %d list calls", calls)
	}

	p.invalidateRecentFilesCache()
	resp3 := p.Query(context.Background(), q)
	if len(resp3.Results) != 1 {
		t.Fatalf("unexpected post-invalidation count: %d", len(resp3.Results))
	}
	if calls != 2 {
		t.Fatalf("expected cache invalidation to trigger fresh lookup, got %d list calls", calls)
	}
}
