package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func BenchmarkFileSearchFreshBulkLoadVariants(b *testing.B) {
	variants := []struct {
		name       string
		plainFacts bool
		indexes    []sqliteIndexDefinition
		indexMode  string
		schemaMode freshBulkSchemaMode
	}{
		{name: "A_current", indexes: entriesIndexDefinitions, indexMode: "full", schemaMode: freshBulkInlineUnique},
		{name: "B_plain_insert", plainFacts: true, indexes: entriesIndexDefinitions, indexMode: "full", schemaMode: freshBulkInlineUnique},
		{name: "E_deferred_unique_plain_insert", plainFacts: true, indexes: entriesIndexDefinitions, indexMode: "full", schemaMode: freshBulkDeferredUnique},
		{name: "D_minimal_indexes", indexes: foregroundEntriesIndexDefinitions, indexMode: "minimal", schemaMode: freshBulkInlineUnique},
		{name: "BD_plain_insert_minimal_indexes", plainFacts: true, indexes: foregroundEntriesIndexDefinitions, indexMode: "minimal", schemaMode: freshBulkInlineUnique},
		{name: "ED_deferred_unique_minimal_indexes", plainFacts: true, indexes: foregroundEntriesIndexDefinitions, indexMode: "minimal", schemaMode: freshBulkDeferredUnique},
	}

	for _, entryCount := range []int{50000, 200000} {
		entryCount := entryCount
		b.Run(fmt.Sprintf("entries_%d", entryCount), func(b *testing.B) {
			for _, variant := range variants {
				variant := variant
				b.Run(variant.name, func(b *testing.B) {
					var totals freshBulkBenchmarkTotals
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						b.StopTimer()
						db, ctx := openBenchmarkFileSearchDB(b)
						rootPath := filepath.Join(b.TempDir(), "fresh-root")
						root := RootRecord{
							ID:        fmt.Sprintf("fresh-root-%d", i),
							Path:      rootPath,
							Kind:      RootKindUser,
							Status:    RootStatusIdle,
							CreatedAt: 1,
							UpdatedAt: 1,
						}
						mustInsertBenchmarkRoot(b, ctx, db, root)
						if variant.schemaMode == freshBulkDeferredUnique {
							if err := recreateBenchmarkEntriesTable(ctx, db.db, false); err != nil {
								b.Fatalf("recreate entries without inline unique: %v", err)
							}
						}
						directories, entries := benchmarkFreshBulkFacts(root.ID, rootPath, entryCount)

						b.StartTimer()
						result := benchmarkFreshBulkLoadVariant(b, ctx, db, directories, entries, variant.plainFacts, variant.indexes, variant.schemaMode)
						b.StopTimer()
						totals.add(result)
						_ = db.Close()
					}
					totals.report(b)
					b.ReportMetric(float64(len(variant.indexes)), "indexes/op")
					if variant.plainFacts {
						b.ReportMetric(1, "plain_facts/op")
					} else {
						b.ReportMetric(0, "plain_facts/op")
					}
					if variant.indexMode == "minimal" {
						b.ReportMetric(1, "minimal_indexes/op")
					} else {
						b.ReportMetric(0, "minimal_indexes/op")
					}
					if variant.schemaMode == freshBulkDeferredUnique {
						b.ReportMetric(1, "deferred_unique/op")
					} else {
						b.ReportMetric(0, "deferred_unique/op")
					}
				})
			}
		})
	}
}

type freshBulkSchemaMode string

const (
	freshBulkInlineUnique   freshBulkSchemaMode = "inline_unique"
	freshBulkDeferredUnique freshBulkSchemaMode = "deferred_unique"
)

type freshBulkBenchmarkResult struct {
	DropIndexes       time.Duration
	Facts             time.Duration
	UniqueIndex       time.Duration
	RecreateIndexes   time.Duration
	Artifacts         time.Duration
	DirectoryRows     int
	EntryRows         int
	RecreatedIndexes  int
	PlainFacts        bool
	MinimalIndexCount int
}

type freshBulkBenchmarkTotals struct {
	DropIndexes      time.Duration
	Facts            time.Duration
	UniqueIndex      time.Duration
	RecreateIndexes  time.Duration
	Artifacts        time.Duration
	DirectoryRows    int
	EntryRows        int
	RecreatedIndexes int
}

func (t *freshBulkBenchmarkTotals) add(result freshBulkBenchmarkResult) {
	t.DropIndexes += result.DropIndexes
	t.Facts += result.Facts
	t.UniqueIndex += result.UniqueIndex
	t.RecreateIndexes += result.RecreateIndexes
	t.Artifacts += result.Artifacts
	t.DirectoryRows += result.DirectoryRows
	t.EntryRows += result.EntryRows
	t.RecreatedIndexes += result.RecreatedIndexes
}

func (t freshBulkBenchmarkTotals) report(b *testing.B) {
	if b.N == 0 {
		return
	}
	iterations := float64(b.N)
	b.ReportMetric(float64(t.DropIndexes.Milliseconds())/iterations, "drop_indexes_ms/op")
	b.ReportMetric(float64(t.Facts.Milliseconds())/iterations, "facts_ms/op")
	b.ReportMetric(float64(t.UniqueIndex.Milliseconds())/iterations, "unique_index_ms/op")
	b.ReportMetric(float64(t.RecreateIndexes.Milliseconds())/iterations, "recreate_indexes_ms/op")
	b.ReportMetric(float64(t.Artifacts.Milliseconds())/iterations, "artifacts_ms/op")
	b.ReportMetric(float64(t.DirectoryRows)/iterations, "directories/op")
	b.ReportMetric(float64(t.EntryRows)/iterations, "entries/op")
}

func benchmarkFreshBulkLoadVariant(b *testing.B, ctx context.Context, db *FileSearchDB, directories []DirectoryRecord, entries []EntryRecord, plainFacts bool, indexes []sqliteIndexDefinition, schemaMode freshBulkSchemaMode) freshBulkBenchmarkResult {
	b.Helper()

	result := freshBulkBenchmarkResult{
		DirectoryRows:     len(directories),
		EntryRows:         len(entries),
		RecreatedIndexes:  len(indexes),
		PlainFacts:        plainFacts,
		MinimalIndexCount: len(indexes),
	}

	startedAt := time.Now()
	if err := dropEntriesSecondaryIndexes(ctx, db.db); err != nil {
		b.Fatalf("drop secondary indexes: %v", err)
	}
	result.DropIndexes = time.Since(startedAt)

	startedAt = time.Now()
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatalf("begin facts tx: %v", err)
	}
	if plainFacts {
		err = insertDirectoryRecordsBatchTx(ctx, tx, directories)
	} else {
		err = upsertDirectoryRecordsBatchTx(ctx, tx, directories)
	}
	if err != nil {
		_ = tx.Rollback()
		b.Fatalf("write directories: %v", err)
	}
	if plainFacts {
		err = insertEntryFactsNoReturningBatchTx(ctx, tx, entries)
	} else {
		err = upsertEntryFactsNoReturningBatchTx(ctx, tx, entries)
	}
	if err != nil {
		_ = tx.Rollback()
		b.Fatalf("write entries: %v", err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit facts: %v", err)
	}
	result.Facts = time.Since(startedAt)

	if schemaMode == freshBulkDeferredUnique {
		startedAt = time.Now()
		if _, err := db.db.ExecContext(ctx, `CREATE UNIQUE INDEX idx_entries_path_unique ON entries(path)`); err != nil {
			b.Fatalf("create deferred path unique index: %v", err)
		}
		result.UniqueIndex = time.Since(startedAt)
	}

	if err := db.withBulkFinalizeConnection(ctx, func(conn *sql.Conn) error {
		startedAt = time.Now()
		if err := recreateEntryIndexesWithBeginner(ctx, conn, indexes); err != nil {
			return err
		}
		result.RecreateIndexes = time.Since(startedAt)

		startedAt = time.Now()
		if err := db.rebuildBulkSearchArtifactsWithBeginner(ctx, conn, false, true); err != nil {
			return err
		}
		result.Artifacts = time.Since(startedAt)
		return nil
	}); err != nil {
		b.Fatalf("finalize artifacts: %v", err)
	}

	return result
}

// recreateBenchmarkEntriesTable swaps only the entries fact table shape so the
// benchmark can isolate inline UNIQUE from deferred UNIQUE index creation.
func recreateBenchmarkEntriesTable(ctx context.Context, db *sql.DB, inlineUnique bool) error {
	if _, err := db.ExecContext(ctx, `DROP TABLE entries`); err != nil {
		return fmt.Errorf("drop benchmark entries: %w", err)
	}

	pathColumn := "path TEXT NOT NULL"
	if inlineUnique {
		pathColumn += " UNIQUE"
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE entries (
			entry_id INTEGER PRIMARY KEY,
			%s,
			root_id TEXT NOT NULL,
			parent_path TEXT NOT NULL,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			name_key TEXT NOT NULL DEFAULT '',
			normalized_path TEXT NOT NULL,
			pinyin_full TEXT NOT NULL DEFAULT '',
			pinyin_initials TEXT NOT NULL DEFAULT '',
			extension TEXT NOT NULL DEFAULT '',
			is_dir INTEGER NOT NULL,
			mtime INTEGER NOT NULL,
			size INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL
		)
	`, pathColumn)); err != nil {
		return fmt.Errorf("create benchmark entries: %w", err)
	}
	return nil
}

func benchmarkFreshBulkFacts(rootID string, rootPath string, entryCount int) ([]DirectoryRecord, []EntryRecord) {
	directoryCount := entryCount / 5
	if directoryCount < 1 {
		directoryCount = 1
	}
	fileCount := entryCount - directoryCount

	directories := make([]DirectoryRecord, 0, directoryCount)
	entries := make([]EntryRecord, 0, entryCount)
	for i := 0; i < directoryCount; i++ {
		dirName := fmt.Sprintf("dir-%06d", i)
		dirPath := filepath.Join(rootPath, dirName)
		directories = append(directories, DirectoryRecord{
			Path:         dirPath,
			RootID:       rootID,
			ParentPath:   rootPath,
			LastScanTime: int64(i + 1),
			Exists:       true,
		})
		entries = append(entries, EntryRecord{
			Path:           dirPath,
			RootID:         rootID,
			ParentPath:     rootPath,
			Name:           dirName,
			NormalizedName: dirName,
			NormalizedPath: normalizePath(dirPath),
			IsDir:          true,
			Mtime:          int64(i + 1),
			UpdatedAt:      int64(i + 1),
		})
	}

	for i := 0; i < fileCount; i++ {
		parent := directories[i%directoryCount].Path
		name := fmt.Sprintf("file-%06d-main.go", i)
		if i%4 == 0 {
			name = fmt.Sprintf("file-%06d-readme.md", i)
		}
		filePath := filepath.Join(parent, name)
		entries = append(entries, EntryRecord{
			Path:           filePath,
			RootID:         rootID,
			ParentPath:     parent,
			Name:           name,
			NormalizedName: name,
			NormalizedPath: normalizePath(filePath),
			IsDir:          false,
			Mtime:          int64(i + 1),
			Size:           128 + int64(i%4096),
			UpdatedAt:      int64(i + 1),
		})
	}
	return directories, entries
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
