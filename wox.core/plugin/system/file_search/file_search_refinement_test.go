package system

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/util/filesearch"
)

type fileSearchSettingAPI struct {
	fileSearchToolbarTestAPI
	settings map[string]string
}

func (a fileSearchSettingAPI) GetSetting(ctx context.Context, key string) string {
	if a.settings == nil {
		return ""
	}
	return a.settings[key]
}

func TestFileSearchTypeRefinementOptionsIncludeContentWhenEnabled(t *testing.T) {
	withoutContent := fileSearchTypeRefinementValues(fileSearchTypeRefinementOptions(false))
	if containsString(withoutContent, fileSearchTypeRefinementContent) {
		t.Fatal("content type should stay hidden when content search is disabled")
	}

	withContent := fileSearchTypeRefinementValues(fileSearchTypeRefinementOptions(true))
	if !containsString(withContent, fileSearchTypeRefinementAll) ||
		!containsString(withContent, fileSearchTypeRefinementFile) ||
		!containsString(withContent, fileSearchTypeRefinementFolder) ||
		!containsString(withContent, fileSearchTypeRefinementContent) {
		t.Fatalf("enabled content search should add the content type, got %#v", withContent)
	}
}

func TestBuildFileSearchTypeRefinementUsesContentSearchSetting(t *testing.T) {
	ctx := context.Background()
	disabled := &FileSearchPlugin{api: fileSearchSettingAPI{}}
	if containsString(fileSearchTypeRefinementValues(disabled.buildFileSearchTypeRefinement(ctx).Options), fileSearchTypeRefinementContent) {
		t.Fatal("disabled content search should omit the content type option")
	}

	enabled := &FileSearchPlugin{api: fileSearchSettingAPI{settings: map[string]string{contentSearchEnabledKey: "true"}}}
	if !containsString(fileSearchTypeRefinementValues(enabled.buildFileSearchTypeRefinement(ctx).Options), fileSearchTypeRefinementContent) {
		t.Fatal("enabled content search should offer the content type option")
	}
}

func TestSelectedFileSearchTypeAcceptsContent(t *testing.T) {
	query := plugin.Query{Refinements: map[string]string{fileSearchTypeRefinementKey: fileSearchTypeRefinementContent}}
	if selected := selectedFileSearchType(query); selected != fileSearchTypeRefinementContent {
		t.Fatalf("expected content type, got %q", selected)
	}
}

func TestFileSearchResultTailsMarksContentMatches(t *testing.T) {
	if tails := fileSearchResultTails(filesearch.SearchResult{Path: "/tmp/name.txt"}, false); len(tails) != 0 {
		t.Fatalf("name matches should not get a content tail, got %#v", tails)
	}

	tails := fileSearchResultTails(filesearch.SearchResult{Path: "/tmp/content.txt", IsContentMatch: true}, false)
	if len(tails) != 1 || tails[0].Text != "i18n:plugin_file_result_tail_content" {
		t.Fatalf("content matches should get a content tail, got %#v", tails)
	}
	if tails[0].Tooltip != "i18n:plugin_file_result_tail_content_tooltip" {
		t.Fatalf("content tail should explain the match source, got %#v", tails[0])
	}
}

func TestFileSearchResultLimitForContentUsesNameWindow(t *testing.T) {
	ctx := context.Background()
	search := &FileSearchPlugin{}
	triggered := plugin.Query{Type: plugin.QueryTypeInput, TriggerKeyword: "f", Search: "report"}
	if got := search.fileSearchResultLimitFor(ctx, triggered, fileSearchTypeRefinementContent); got != fileSearchResultLimit {
		t.Fatalf("content filter should use the full result window, got %d", got)
	}
	if got := search.fileSearchResultLimitFor(ctx, triggered, fileSearchTypeRefinementAll); got != fileSearchResultLimit+fileSearchContentResultLimit {
		t.Fatalf("all types should keep the extra content window, got %d", got)
	}
	global := plugin.Query{Type: plugin.QueryTypeInput, Search: "report"}
	if got := search.fileSearchResultLimitFor(ctx, global, fileSearchTypeRefinementAll); got != fileSearchGlobalResultLimit {
		t.Fatalf("global query should cap results at %d, got %d", fileSearchGlobalResultLimit, got)
	}
	if got := search.fileSearchCandidateLimit(ctx, global, fileSearchTypeRefinementAll); got != fileSearchGlobalResultLimit {
		t.Fatalf("global query candidate limit should be %d, got %d", fileSearchGlobalResultLimit, got)
	}

	custom := &FileSearchPlugin{api: fileSearchSettingAPI{settings: map[string]string{fileGlobalResultLimitSettingKey: "25"}}}
	if got := custom.fileSearchResultLimitFor(ctx, global, fileSearchTypeRefinementAll); got != 25 {
		t.Fatalf("configured global result limit should be 25, got %d", got)
	}
	if got := custom.fileSearchCandidateLimit(ctx, global, fileSearchTypeRefinementAll); got != 25 {
		t.Fatalf("configured global candidate limit should be 25, got %d", got)
	}
	if got := custom.fileSearchResultLimitFor(ctx, triggered, fileSearchTypeRefinementAll); got != fileSearchResultLimit+fileSearchContentResultLimit {
		t.Fatalf("triggered search should ignore the global result limit, got %d", got)
	}

	invalid := &FileSearchPlugin{api: fileSearchSettingAPI{settings: map[string]string{fileGlobalResultLimitSettingKey: "0"}}}
	if got := invalid.getConfiguredGlobalResultLimit(ctx); got != fileSearchGlobalResultLimit {
		t.Fatalf("non-positive global result limit should fall back to %d, got %d", fileSearchGlobalResultLimit, got)
	}
	capped := &FileSearchPlugin{api: fileSearchSettingAPI{settings: map[string]string{fileGlobalResultLimitSettingKey: "500"}}}
	if got := capped.getConfiguredGlobalResultLimit(ctx); got != fileSearchResultLimit {
		t.Fatalf("global result limit above %d should clamp, got %d", fileSearchResultLimit, got)
	}
}

func fileSearchTypeRefinementValues(options []plugin.QueryRefinementOption) []string {
	values := make([]string, 0, len(options))
	for _, option := range options {
		values = append(values, option.Value)
	}
	return values
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
