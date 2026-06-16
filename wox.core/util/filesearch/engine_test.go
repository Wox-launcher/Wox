package filesearch

import (
	"testing"
	"time"
)

func TestEngineGetStatusKeepsTransientRunIndexingState(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	errorMessage := "access is denied"

	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-error",
		Path:      `C:\Windows`,
		Kind:      RootKindUser,
		Status:    RootStatusError,
		LastError: &errorMessage,
		CreatedAt: now,
		UpdatedAt: now,
	})
	mustInsertRoot(t, ctx, db, RootRecord{
		ID:        "root-active",
		Path:      `C:\dev`,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	})

	scanner := NewScanner(db)
	// The toolbar bug happened when a planner-owned run was active but the root
	// counters still looked idle/error-only. This regression test locks in that
	// active run state must keep GetStatus() in indexing mode until the run ends.
	scanner.setTransientRunState(StatusSnapshot{
		ActiveRootStatus:      RootStatusScanning,
		ActiveRunStatus:       RunStatusPlanning,
		ActiveStage:           RunStagePlanning,
		ActiveRootPath:        `C:\dev`,
		ActiveProgressCurrent: 3,
		ActiveProgressTotal:   7,
		IsIndexing:            true,
	})

	engine := &Engine{
		db:      db,
		scanner: scanner,
	}

	status, err := engine.GetStatus(ctx)
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if !status.IsIndexing {
		t.Fatal("expected active transient run to keep indexing state true")
	}
	if status.ActiveRootPath != `C:\dev` {
		t.Fatalf("expected active root path to stay on transient run root, got %q", status.ActiveRootPath)
	}
	if status.ErrorRootPath != `C:\Windows` {
		t.Fatalf("expected error root path to stay available for idle/error banners, got %q", status.ErrorRootPath)
	}
}
