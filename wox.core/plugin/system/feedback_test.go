package system

import (
	"context"
	"testing"
	"wox/diagnostic"
	"wox/plugin"
	"wox/util"
)

type feedbackTestAPI struct {
	plugin.API
}

func (feedbackTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

func TestQueryShowsCommonOperationsByDefault(t *testing.T) {
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}

	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{})
	if len(response.Results) != 3 {
		t.Fatalf("default feedback query should show 3 operations, got %d", len(response.Results))
	}

	wantTitles := []string{
		"i18n:plugin_feedback_bug_title",
		"i18n:plugin_feedback_feature_title",
		"i18n:plugin_feedback_clear_logs_title",
	}
	for i, want := range wantTitles {
		if response.Results[i].Title != want {
			t.Fatalf("default result %d title = %q, want %q", i, response.Results[i].Title, want)
		}
		if response.Results[i].Preview.PreviewData != "" {
			t.Fatalf("default result %d should not have a preview", i)
		}
	}
}

func TestQueryUnknownCommandReturnsNoResults(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{Command: "unknown"})
	if len(response.Results) != 0 {
		t.Fatalf("unknown command should return no results, got %d", len(response.Results))
	}
}

func TestQueryCrashCommandListsCrashOrEmptyState(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{Command: feedbackCommandCrash})
	if len(response.Results) == 0 {
		t.Fatal("crash command should return at least one result")
	}
	if response.Results[0].Title == "i18n:plugin_feedback_bug_title" {
		t.Fatal("crash command should not show the default bug operation")
	}
}

// TestBuildCrashIncidentResultPreservesNewestFirstOrdering verifies the score used by the launcher cache.
func TestBuildCrashIncidentResultPreservesNewestFirstOrdering(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	older := pluginInstance.buildCrashIncidentResult(context.Background(), diagnostic.CrashIncident{
		ID:         "older",
		DetectedAt: 100,
	})
	newer := pluginInstance.buildCrashIncidentResult(context.Background(), diagnostic.CrashIncident{
		ID:         "newer",
		DetectedAt: 200,
	})

	if newer.Score <= older.Score {
		t.Fatalf("newer crash should have a higher result score, newer=%d older=%d", newer.Score, older.Score)
	}
}
