package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"wox/util"
)

// util.GetLocation() is process-global, so do not use this helper from parallel benchmarks.
func openBenchmarkFileSearchDB(b *testing.B) (*FileSearchDB, context.Context) {
	b.Helper()

	dataDir, err := os.MkdirTemp("", "wox-filesearch-bench-")
	if err != nil {
		b.Fatalf("create benchmark data dir: %v", err)
	}
	b.Setenv(util.TestWoxDataDirEnv, dataDir)
	b.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))

	if err := util.GetLocation().Init(); err != nil {
		b.Fatalf("init location: %v", err)
	}

	ctx := context.Background()
	db, err := NewFileSearchDB(ctx)
	if err != nil {
		b.Fatalf("open filesearch db: %v", err)
	}

	b.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(dataDir)
	})

	return db, ctx
}

func mustInsertBenchmarkRoot(b *testing.B, ctx context.Context, db *FileSearchDB, root RootRecord) {
	b.Helper()

	if err := db.UpsertRoot(ctx, root); err != nil {
		b.Fatalf("upsert root: %v", err)
	}
}

func benchmarkBatch(rootID, scopePath string, dirCount, fileCount int) SubtreeSnapshotBatch {
	directories := make([]DirectoryRecord, 0, dirCount)
	entries := make([]EntryRecord, 0, fileCount)

	for i := 0; i < dirCount; i++ {
		dirPath := filepath.Join(scopePath, fmt.Sprintf("dir-%04d", i))
		directories = append(directories, DirectoryRecord{
			Path:         dirPath,
			RootID:       rootID,
			ParentPath:   scopePath,
			LastScanTime: int64(i + 1),
			Exists:       true,
		})
	}

	for i := 0; i < fileCount; i++ {
		filePath := filepath.Join(scopePath, fmt.Sprintf("file-%04d.txt", i))
		entries = append(entries, EntryRecord{
			Path:           filePath,
			RootID:         rootID,
			ParentPath:     scopePath,
			Name:           filepath.Base(filePath),
			NormalizedName: filepath.Base(filePath),
			NormalizedPath: filePath,
			IsDir:          false,
			Mtime:          int64(i + 1),
			Size:           128,
			UpdatedAt:      int64(i + 1),
		})
	}

	return SubtreeSnapshotBatch{
		RootID:      rootID,
		ScopePath:   scopePath,
		Directories: directories,
		Entries:     entries,
	}
}

func BenchmarkFileSearchDBReplaceSubtreeSnapshot(b *testing.B) {
	db, ctx := openBenchmarkFileSearchDB(b)
	rootPath := filepath.Join(b.TempDir(), "bench-root")
	root := RootRecord{
		ID:        "bench-root",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	mustInsertBenchmarkRoot(b, ctx, db, root)

	b.ReportAllocs()

	b.Run("small-subtree", func(b *testing.B) {
		batch := benchmarkBatch(root.ID, filepath.Join(rootPath, "small"), 64, 256)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := db.ReplaceSubtreeSnapshot(context.Background(), batch); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("large-subtree", func(b *testing.B) {
		batch := benchmarkBatch(root.ID, filepath.Join(rootPath, "large"), 1024, 4096)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := db.ReplaceSubtreeSnapshot(context.Background(), batch); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkFileSearchRunPlanner(b *testing.B) {
	rootPath := filepath.Join(b.TempDir(), "planner-root")
	createBenchmarkTree(b, rootPath, 8, 32)
	root := RootRecord{
		ID:        "planner-root",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: 1,
		UpdatedAt: 1,
	}

	planner := NewRunPlanner(newPolicyState(Policy{}))
	planner.budget = splitBudget{
		LeafEntryBudget:     24,
		LeafWriteBudget:     24,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 16,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := planner.PlanFullRun(context.Background(), []RootRecord{root}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileSearchJobExecutor(b *testing.B) {
	db, ctx := openBenchmarkFileSearchDB(b)
	rootPath := filepath.Join(b.TempDir(), "executor-root")
	createBenchmarkTree(b, rootPath, 8, 32)

	root := RootRecord{
		ID:        "executor-root",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: 1,
		UpdatedAt: 1,
	}
	mustInsertBenchmarkRoot(b, ctx, db, root)

	scanner := NewScanner(db)
	scanner.plannerBudgetOverride = &splitBudget{
		LeafEntryBudget:     24,
		LeafWriteBudget:     24,
		LeafMemoryBudget:    1 << 20,
		DirectFileBatchSize: 16,
	}

	planner := NewRunPlanner(scanner.policy)
	planner.budget = *scanner.plannerBudgetOverride
	plan, err := planner.PlanFullRun(ctx, []RootRecord{root})
	if err != nil {
		b.Fatalf("plan benchmark run: %v", err)
	}

	executor := NewJobExecutor(NewSnapshotBuilder(scanner.policy))
	executor.SetApplyFunc(func(runCtx context.Context, currentRoot RootRecord, job Job, batch *SubtreeSnapshotBatch) error {
		return scanner.applyRunJob(runCtx, RunKindFull, currentRoot, job, batch)
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.BeginBulkSync()
		if _, _, err := executor.ExecuteRun(context.Background(), plan, []RootRecord{root}, nil); err != nil {
			b.Fatal(err)
		}
		if err := db.EndBulkSync(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

func createBenchmarkTree(b *testing.B, rootPath string, childDirCount int, filesPerChild int) {
	b.Helper()

	for dirIndex := 0; dirIndex < childDirCount; dirIndex++ {
		childDir := filepath.Join(rootPath, fmt.Sprintf("scope-%02d", dirIndex))
		if err := os.MkdirAll(childDir, 0o755); err != nil {
			b.Fatalf("mkdir benchmark child dir %q: %v", childDir, err)
		}
		for fileIndex := 0; fileIndex < filesPerChild; fileIndex++ {
			filePath := filepath.Join(childDir, fmt.Sprintf("file-%03d.txt", fileIndex))
			if err := os.WriteFile(filePath, []byte("benchmark"), 0o644); err != nil {
				b.Fatalf("write benchmark file %q: %v", filePath, err)
			}
		}
	}
}
