package app

import (
	"context"
	"testing"
)

// TestMacDefaultAppDirectoriesUseChangeFeed prevents the default roots from
// silently returning to root-only kqueue watches while custom roots use FSEvents.
func TestMacDefaultAppDirectoriesUseChangeFeed(t *testing.T) {
	retriever := &MacRetriever{}
	for _, directory := range retriever.GetAppDirectories(context.Background()) {
		if !directory.trackChanges {
			t.Fatalf("default root bypasses FSEvents: %s", directory.Path)
		}
	}
}
