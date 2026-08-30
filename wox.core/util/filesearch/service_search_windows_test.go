//go:build windows

package filesearch

import (
	"testing"

	"wox/util/filesearchservice"
)

func TestStatusSnapshotFromServiceIndexStatsUsesServiceProgress(t *testing.T) {
	status := statusSnapshotFromServiceIndexStats(filesearchservice.IndexStats{
		IsIndexing:    true,
		EntryCount:    328422,
		FileCount:     308591,
		VolumeIndex:   2,
		VolumeTotal:   3,
		CurrentVolume: `D:\`,
		ElapsedMs:     17101,
	})

	if !status.IsIndexing || status.ActiveRunKind != RunKindFull || status.ActiveRunStatus != RunStatusExecuting {
		t.Fatalf("service run state = %+v", status)
	}
	if status.ActiveRunFileCount != 308591 || status.ActiveRunEntryCount != 328422 || status.ActiveRunElapsedMs != 17101 {
		t.Fatalf("service progress = %+v", status)
	}
	if status.RootCount != 3 || status.ActiveRootIndex != 2 || status.ActiveRootPath != `D:\` {
		t.Fatalf("service volume progress = %+v", status)
	}
}
