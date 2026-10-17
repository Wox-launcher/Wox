package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"wox/util/filesearch/column"
)

// columnSearchProvider keeps the engine contract independent of the source and
// of the SQLite database used for root lifecycle metadata.
type columnSearchProvider struct{ db *FileSearchDB }

func (p *columnSearchProvider) Search(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, error) {
	return p.db.directory.search(ctx, query, limit)
}

// restoreDirectory opens existing mappings without rebuilding on the startup
// path. A missing generation leaves the root unready for its next scan.
func (d *FileSearchDB) restoreDirectory(ctx context.Context) error {
	d.directory = newDirectoryIndex(filepath.Join(filepath.Dir(d.dbPath), "columns"))
	roots, err := d.ListRoots(ctx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if err := d.directory.attach(root); err != nil {
			return err
		}
	}
	return nil
}

// writeColumnScope publishes one scope. Feed cursors only advance in
// FinalizeRootRun, so a crash after this write replays safely.
func (d *FileSearchDB) writeColumnScope(ctx context.Context, rootID, scope string, direct bool, stream func(func(SubtreeSnapshotBatch) error) error) error {
	return d.writeColumnScopes(ctx, rootID, []string{scope}, direct, stream)
}

// writeColumnScopes shares one column publication across the scanner's
// coalesced subtree batches. Directory facts live only in the column store;
// SQLite keeps root lifecycle state.
func (d *FileSearchDB) writeColumnScopes(ctx context.Context, rootID string, scopes []string, direct bool, stream func(func(SubtreeSnapshotBatch) error) error) error {
	root, err := d.FindRootByID(ctx, rootID)
	if err != nil {
		return err
	}
	if root == nil {
		return fmt.Errorf("root %q not found", rootID)
	}
	for _, scope := range scopes {
		if !pathWithinScope(root.Path, scope) {
			return fmt.Errorf("scope %q outside root %q", scope, root.Path)
		}
	}
	return d.directory.writeScopes(ctx, rootID, scopes, direct, stream)
}

// applyColumnJob streams traversal batches directly into the root builder. No
// full-path filename staging table or second filename index is created.
func (d *FileSearchDB) applyColumnJob(ctx context.Context, root RootRecord, job Job, snapshot *SnapshotBuilder, onProgress func(JobApplyStats)) (JobApplyStats, error) {
	if snapshot == nil {
		return JobApplyStats{}, fmt.Errorf("invalid %s scan job", job.Kind)
	}
	stats := JobApplyStats{}
	counted := func(yield func(SubtreeSnapshotBatch) error) func(SubtreeSnapshotBatch) error {
		return func(batch SubtreeSnapshotBatch) error {
			if err := validateJobSnapshotBatch(job, batch); err != nil {
				return err
			}
			if err := yield(batch); err != nil {
				return err
			}
			stats.add(jobApplyStatsFromBatch(batch))
			if onProgress != nil {
				onProgress(stats)
			}
			return nil
		}
	}
	var err error
	switch job.Kind {
	case JobKindDirectFiles:
		err = d.writeColumnScope(ctx, root.ID, job.ScopePath, true, func(yield func(SubtreeSnapshotBatch) error) error {
			return snapshot.StreamDirectFilesJobBatches(ctx, root, job, counted(yield))
		})
	case JobKindSubtree:
		err = d.writeColumnScope(ctx, root.ID, job.ScopePath, false, func(yield func(SubtreeSnapshotBatch) error) error {
			return snapshot.StreamSubtreeJobBatches(ctx, root, job, counted(yield))
		})
	case JobKindDirectory:
		err = d.directory.writeDirectory(ctx, root.ID, job.ScopePath, func(knownDirectory func(uint64) bool, yield func(SubtreeSnapshotBatch) error) error {
			return snapshot.StreamDirectoryJobBatches(ctx, root, job, knownDirectory, counted(yield))
		})
	default:
		return JobApplyStats{}, fmt.Errorf("invalid %s scan job", job.Kind)
	}
	return stats, err
}

// applyColumnDelta preserves exact-path watcher semantics. In particular, Lstat
// indexes a symlink itself and rename events resolve whether the path still exists.
func (d *FileSearchDB) applyColumnDelta(ctx context.Context, root RootRecord, job Job, policy *policyState) error {
	if job.Kind != JobKindDirectDelta {
		return fmt.Errorf("invalid direct delta job %s", job.Kind)
	}
	if policy == nil {
		policy = newPolicyState(Policy{})
	}
	updates := make(map[string]*EntryRecord, len(job.DirectDeltas))
	for _, delta := range job.DirectDeltas {
		path := filepath.Clean(delta.Path)
		if path == "." || path == "" {
			continue
		}
		if !pathWithinScope(root.Path, path) {
			return fmt.Errorf("delta %q outside root %q", path, root.Path)
		}
		if isDeleteOnlyDelta(delta.SemanticKind) {
			updates[path] = nil
			continue
		}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			updates[path] = nil
			continue
		}
		if err != nil {
			return err
		}
		if info.IsDir() || !policy.newTraversalContext(root, filepath.Dir(path)).ShouldIndexPath(path, false) {
			updates[path] = nil
			continue
		}
		entry := newEntryRecord(root, path, info)
		updates[path] = &entry
	}
	entries := make([]EntryRecord, 0, len(updates))
	deleted := make([]string, 0, len(updates))
	for path, entry := range updates {
		if entry == nil {
			deleted = append(deleted, path)
		} else {
			entries = append(entries, *entry)
		}
	}
	// Only FinalizeRootRun acknowledges feed progress after every job succeeds.
	return d.directory.applyEntries(ctx, root.ID, entries, deleted, root.FeedCursor)
}

// columnEntries exposes owned paths only for callers that explicitly request a
// materialized list. Queries and scanner writes use bounded streaming paths.
func (d *FileSearchDB) columnEntries(ctx context.Context, rootID string) ([]EntryRecord, error) {
	var entries []EntryRecord
	err := d.directory.visit(ctx, rootID, func(entry EntryRecord) error { entries = append(entries, entry); return nil })
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}

// compactColumns is called by the scanner's serialized maintenance lifecycle.
// Store publication checks revisions so a concurrent mutation is never dropped.
func (d *FileSearchDB) compactColumns(ctx context.Context) error {
	d.directory.mu.RLock()
	defer d.directory.mu.RUnlock()
	for _, shard := range d.directory.shards {
		if !shard.store.NeedsCompaction() {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		shard.mu.Lock()
		err := shard.store.Compact(ctx)
		shard.mu.Unlock()
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return err
		}
	}
	return nil
}

// columnDiskBytes includes every immutable generation and its replay journal.
func (d *FileSearchDB) columnDiskBytes() int64 {
	total := fileSizeOrZero(d.dbPath) + fileSizeOrZero(d.dbPath+"-wal") + fileSizeOrZero(d.dbPath+"-shm")
	_ = filepath.WalkDir(d.directory.dir, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			total += fileSizeOrZero(path)
		}
		return nil
	})
	return total
}

// columnSnapshot retains the diagnostic API while measuring the common store.
func (d *FileSearchDB) columnSnapshot(ctx context.Context) (columnIndexSnapshot, error) {
	roots, err := d.ListRoots(ctx)
	if err != nil {
		return columnIndexSnapshot{}, err
	}
	snapshot := columnIndexSnapshot{RootCount: len(roots), DiskBytes: d.columnDiskBytes(), MetadataBytes: fileSizeOrZero(d.dbPath) + fileSizeOrZero(d.dbPath+"-wal") + fileSizeOrZero(d.dbPath+"-shm")}
	for _, root := range roots {
		rootStats := columnRootSnapshot{RootID: root.ID, Path: root.Path}
		d.directory.mu.RLock()
		shard := d.directory.shards[root.ID]
		if shard != nil {
			files, entries, countErr := shard.store.Counts(ctx)
			if countErr != nil && !errors.Is(countErr, column.ErrNotReady) {
				d.directory.mu.RUnlock()
				return snapshot, countErr
			}
			rootStats.Docs = entries
			snapshot.EntryCount += entries
			snapshot.FileCount += files
		}
		d.directory.mu.RUnlock()
		_ = filepath.WalkDir(d.directory.shardDirectory(root.ID), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				rootStats.DiskBytes += fileSizeOrZero(path)
			}
			return nil
		})
		snapshot.TopRoots = append(snapshot.TopRoots, rootStats)
	}
	sort.Slice(snapshot.TopRoots, func(i, j int) bool { return snapshot.TopRoots[i].Docs > snapshot.TopRoots[j].Docs })
	return snapshot, nil
}
