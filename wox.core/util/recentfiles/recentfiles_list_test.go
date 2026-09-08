package recentfiles

import (
	"context"
	"testing"
)

func TestListRecentDoesNotFailOnThisOS(t *testing.T) {
	files, err := List(context.Background(), ListOption{Limit: 5})
	if err != nil {
		t.Fatalf("list recent files: %v", err)
	}
	for _, file := range files {
		if file.Path == "" {
			t.Fatal("recent file path is empty")
		}
	}
}
