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
	if tails := fileSearchResultTails(filesearch.SearchResult{Path: "/tmp/name.txt"}); len(tails) != 0 {
		t.Fatalf("name matches should not get a content tail, got %#v", tails)
	}

	tails := fileSearchResultTails(filesearch.SearchResult{Path: "/tmp/content.txt", IsContentMatch: true})
	if len(tails) != 1 || tails[0].Text != "i18n:plugin_file_result_tail_content" {
		t.Fatalf("content matches should get a content tail, got %#v", tails)
	}
	if tails[0].Tooltip != "i18n:plugin_file_result_tail_content_tooltip" {
		t.Fatalf("content tail should explain the match source, got %#v", tails[0])
	}
}

func TestFileSearchResultLimitForContentUsesNameWindow(t *testing.T) {
	if got := fileSearchResultLimitFor(fileSearchTypeRefinementContent, fileSearchSortRefinementRelevance); got != fileSearchResultLimit {
		t.Fatalf("content filter should use the full result window, got %d", got)
	}
	if got := fileSearchResultLimitFor(fileSearchTypeRefinementAll, fileSearchSortRefinementRelevance); got != fileSearchResultLimit+fileSearchContentResultLimit {
		t.Fatalf("all+relevance should keep the extra content window, got %d", got)
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
