package sqlitememory

import (
	"context"
	"testing"
)

func TestReleaseIdleMemoryIsSafeWithoutRegisteredDatabases(t *testing.T) {
	ReleaseIdleMemory(context.Background())
}
