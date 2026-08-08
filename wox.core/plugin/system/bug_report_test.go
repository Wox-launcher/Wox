package system

import (
	"context"
	"testing"
	"wox/diagnostic"
	"wox/plugin"
)

type bugReportTestAPI struct {
	plugin.API
}

func (bugReportTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

// TestBuildCrashIncidentResultPreservesNewestFirstOrdering verifies the score used by the launcher cache.
func TestBuildCrashIncidentResultPreservesNewestFirstOrdering(t *testing.T) {
	pluginInstance := &BugReportPlugin{api: bugReportTestAPI{}}

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
