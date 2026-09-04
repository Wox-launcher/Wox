package ui

import (
	"context"
	"testing"
	"wox/common"
)

func TestRefreshActiveWindowSnapshotDetailsKeepsExistingName(t *testing.T) {
	manager := &Manager{
		activeWindowSnapshotSeq: 1,
		activeWindowSnapshot: common.ActiveWindowSnapshot{
			Pid:  1,
			Name: "Visual Studio Code",
		},
	}

	manager.refreshActiveWindowSnapshotDetails(1, 1)

	snapshot := manager.GetActiveWindowSnapshot(context.Background())
	if snapshot.Name == "" {
		t.Fatal("async detail refresh cleared the window name captured at activation")
	}
}
