package system

import (
	"os"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/util/filesearch"
)

// TestFileSearchGlobalBrowsePathsBypassIndex keeps Folder browsing and completion free of duplicate file hits.
func TestFileSearchGlobalBrowsePathsBypassIndex(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes with spaces.txt")
	if err := os.WriteFile(file, []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "another folder test"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WOX_FILE_QUERY_ROOT", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	for _, search := range []string{
		root, root + string(os.PathSeparator), file, "  " + file + "  ",
		"%WOX_FILE_QUERY_ROOT%", "%WOX_FILE_QUERY_ROOT%/notes with spaces.txt",
		"~", "~/notes with spaces.txt",
		filepath.Join(root, "an"), filepath.Join(root, "notes with"),
		"~/an", "%WOX_FILE_QUERY_ROOT%/an",
	} {
		t.Run(search, func(t *testing.T) {
			// An uninitialized engine/API would fail if the query reached indexed search.
			p := &FileSearchPlugin{engine: &filesearch.Engine{}}
			response := p.Query(t.Context(), plugin.Query{Type: plugin.QueryTypeInput, Search: search})
			if len(response.Results) != 0 || len(response.Refinements) != 0 {
				t.Fatalf("global browse path response = %#v, want empty", response)
			}
		})
	}
}

// TestFileSearchPathSuppressionOnlyAppliesToGlobalBrowsePaths preserves explicit search routing.
func TestFileSearchPathSuppressionOnlyAppliesToGlobalBrowsePaths(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name  string
		query plugin.Query
	}{
		{"triggered", plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "f", Search: root}},
		{"scoped", plugin.Query{Type: plugin.QueryTypeInput, Search: root, Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: PluginID}}}}},
		{"triggered completion", plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "f", Search: filepath.Join(root, "an")}},
		{"missing parent", plugin.Query{Type: plugin.QueryTypeInput, Search: filepath.Join(root, "missing", "child")}},
		{"ordinary search", plugin.Query{Type: plugin.QueryTypeInput, Search: "notes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Disabled content search exercises normal query completion without a live index or database.
			tc.query.Refinements = map[string]string{fileSearchTypeRefinementKey: fileSearchTypeRefinementContent}
			p := &FileSearchPlugin{api: fileSearchSettingAPI{}, engine: &filesearch.Engine{}}
			response := p.Query(t.Context(), tc.query)
			if len(response.Refinements) == 0 {
				t.Fatalf("query was suppressed: %#v", tc.query)
			}
		})
	}
}
