package system

import (
	"context"
	"strings"
	"testing"
	"time"
	"wox/plugin"
	"wox/util/filesearch"
)

type fileSearchStatusCopyAPI struct {
	fileSearchToolbarTestAPI
	copiedText string
}

func (a *fileSearchStatusCopyAPI) Copy(ctx context.Context, params plugin.CopyParams) {
	if params.Type == plugin.CopyTypePlainText {
		a.copiedText = params.Text
	}
}

func TestFileSearchStatusMarkdownIncludesDiagnosticFields(t *testing.T) {
	diagnostics := testFileSearchDiagnosticSnapshot()
	markdown := formatFileSearchStatusMarkdown(diagnostics, nil)
	for _, expected := range []string{
		"File Search Status",
		"dynamic=1",
		"pending roots=1 paths=1",
		"direct_delta",
		"qianlifeng8.png",
		"entries=10 files=8",
		"/Users/qianlifeng/Desktop",
		"Content Search",
		"disabled",
	} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, markdown)
		}
	}
}

func TestFileSearchStatusResponseUsesFullscreenPreviewAndCopyAction(t *testing.T) {
	api := &fileSearchStatusCopyAPI{}
	fileSearchPlugin := &FileSearchPlugin{api: api}
	diagnostics := testFileSearchDiagnosticSnapshot()
	report := formatFileSearchStatusMarkdown(diagnostics, nil)

	response := fileSearchPlugin.buildStatusQueryResponse(diagnostics, nil)
	if response.Layout.ResultPreviewWidthRatio == nil {
		t.Fatal("expected status response to set preview ratio")
	}
	if *response.Layout.ResultPreviewWidthRatio != 0 {
		t.Fatalf("expected fullscreen preview ratio 0, got %f", *response.Layout.ResultPreviewWidthRatio)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one status result, got %d", len(response.Results))
	}
	if response.Results[0].Preview.PreviewData != report {
		t.Fatal("expected preview data to match full status report")
	}
	if len(response.Results[0].Actions) == 0 {
		t.Fatal("expected copy action")
	}

	response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	if api.copiedText != report {
		t.Fatal("expected copy action to copy full status report")
	}
}

func TestFileSearchStatusMarkdownWithContentStats(t *testing.T) {
	diagnostics := testFileSearchDiagnosticSnapshot()
	stats := &filesearch.ContentStats{
		DocCount:         1500,
		IndexedTextBytes: 1024 * 1024 * 50,
		CrawlComplete:    true,
	}
	markdown := formatFileSearchStatusMarkdown(diagnostics, stats)
	for _, expected := range []string{
		"Content Search",
		"crawl_state: `complete`",
		"docs: 1,500",
	} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("expected markdown to contain %q, got:\n%s", expected, markdown)
		}
	}
	if strings.Contains(markdown, "status: `disabled`") {
		t.Fatalf("markdown should not show 'disabled' when contentStats is non-nil")
	}
}

func testFileSearchDiagnosticSnapshot() filesearch.DiagnosticSnapshot {
	now := time.UnixMilli(1760000000000)
	return filesearch.DiagnosticSnapshot{
		CapturedAt:           now,
		RootCount:            2,
		UserVisibleRootCount: 1,
		DynamicRootCount:     1,
		RootKindCounts: map[filesearch.RootKind]int{
			filesearch.RootKindUser:    1,
			filesearch.RootKindDynamic: 1,
		},
		RootStatusCounts: map[filesearch.RootStatus]int{
			filesearch.RootStatusIdle:    1,
			filesearch.RootStatusSyncing: 1,
		},
		RootFeedStateCounts: map[filesearch.RootFeedState]int{
			filesearch.RootFeedStateReady:    1,
			filesearch.RootFeedStateDegraded: 1,
		},
		Status: filesearch.StatusSnapshot{
			IsIndexing:            true,
			ActiveRunKind:         filesearch.RunKindIncremental,
			ActiveRunStatus:       filesearch.RunStatusExecuting,
			ActiveJobKind:         filesearch.JobKindDirectDelta,
			ActiveScopePath:       "/Users/qianlifeng/Desktop/qianlifeng8.png",
			ActiveRunElapsedMs:    1200,
			PendingDirtyRootCount: 1,
			PendingDirtyPathCount: 1,
		},
		DirtyQueue: filesearch.DirtyQueueDiagnostics{
			PendingRootCount:       1,
			PendingPathCount:       1,
			PendingRootSignalCount: 0,
			CurrentDebounceWindow:  2 * time.Second,
			NextFlushIn:            900 * time.Millisecond,
			Config: filesearch.DirtyQueueConfig{
				DebounceWindow:       2 * time.Second,
				MaxPendingWaitWindow: 5 * time.Second,
			},
		},
		Index: filesearch.IndexDiagnosticSnapshot{
			EntryCount:     10,
			FileCount:      8,
			BigramRowCount: 0,
		},
		Roots: []filesearch.RootDiagnostic{
			{
				ID:        "root-user",
				Path:      "/Users/qianlifeng/Desktop",
				Kind:      filesearch.RootKindUser,
				Status:    filesearch.RootStatusIdle,
				FeedType:  filesearch.RootFeedTypeFSEvents,
				FeedState: filesearch.RootFeedStateReady,
			},
		},
	}
}
