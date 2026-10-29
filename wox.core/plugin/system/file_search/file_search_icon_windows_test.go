package system

import (
	"context"
	"testing"

	"wox/common"
	"wox/util/filesearch"
)

func TestExecutableResultUsesLazyPerFileIcon(t *testing.T) {
	path := `C:\Downloads\OfficeSetup.EXE`
	icon := resolveFileSearchResultIcon(context.Background(), filesearch.SearchResult{Path: path}, map[string]common.WoxImage{}, &fileSearchQueryDiagnostics{})
	payload, err := common.ParseWoxLazyLoadImagePayload(icon)
	if err != nil || payload.Source == nil || *payload.Source != common.NewWoxImageFileIcon(path) || payload.TargetSize != common.ResultListIconSize {
		t.Fatalf("expected a lazy per-file icon for %s, got %+v (error: %v)", path, payload, err)
	}
}
