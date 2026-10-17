package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wox/util"
	"wox/util/filesearch/column"
)

// History recovery repairs a root after its change feed lost events, on every
// platform and for every feed. It replaces the old whole-root reconcile, which
// re-crawled the entire root (minutes for "/") and, because a burst could
// overflow the feed again during that crawl, could loop indefinitely.
//
// Creating, deleting or renaming an entry updates its parent directory's mtime
// on APFS, NTFS, ext4 and btrfs. Recovery therefore lstats every indexed
// directory in parallel and queues a one-level relist for each directory whose
// mtime differs from the indexed value, exactly like fsearch's relist_changed.
// The relist job diffs the listing against the indexed children, walks only
// directories new to the index, and prunes vanished ones. An in-place edit
// that changes no directory entry can stay stale until its next event, the
// same trade-off fsearch makes.
const (
	historyRecoveryCooldown    = time.Minute
	historyRecoveryStatWorkers = 16
)

// historyRecoveryState tracks the relist lifecycle of one root.
type historyRecoveryState struct {
	// DueAt is zero when no recovery is requested. Requests inside the cooldown
	// are delayed to its end rather than dropped, so a loss storm costs at most
	// one relist per cooldown window.
	DueAt     time.Time
	Reason    string
	LastRunAt time.Time
	// Queued means repair signals were queued. The root stays degraded until a
	// dirty flush covering it succeeds, so a crash before that point repeats the
	// recovery at startup.
	Queued bool
}

// requestHistoryRecovery schedules one relist of root. The persisted degraded
// feed state set by the caller is what survives a restart.
func (s *Scanner) requestHistoryRecovery(ctx context.Context, rootID string, reason string, at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	s.historyRecoveryMu.Lock()
	if s.historyRecovery == nil {
		s.historyRecovery = make(map[string]historyRecoveryState)
	}
	state := s.historyRecovery[rootID]
	if !state.DueAt.IsZero() {
		s.historyRecoveryMu.Unlock()
		return
	}
	state.DueAt = at
	if !state.LastRunAt.IsZero() && state.LastRunAt.Add(historyRecoveryCooldown).After(at) {
		state.DueAt = state.LastRunAt.Add(historyRecoveryCooldown)
	}
	state.Reason = reason
	s.historyRecovery[rootID] = state
	s.historyRecoveryMu.Unlock()

	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch history recovery requested: root=%s reason=%q due_at=%s",
		rootID,
		strings.TrimSpace(reason),
		state.DueAt.Format(time.RFC3339),
	))
	select {
	case s.dirtyCh <- struct{}{}:
	default:
	}
}

// nextHistoryRecoveryAt returns the earliest scheduled recovery, or zero. The
// scan loop uses it to wake up even when no further watcher event arrives.
func (s *Scanner) nextHistoryRecoveryAt() time.Time {
	s.historyRecoveryMu.Lock()
	defer s.historyRecoveryMu.Unlock()
	var next time.Time
	for _, state := range s.historyRecovery {
		if state.DueAt.IsZero() {
			continue
		}
		if next.IsZero() || state.DueAt.Before(next) {
			next = state.DueAt
		}
	}
	return next
}

// historyRecoveryOutstanding reports whether root still has a requested or
// queued-but-unapplied recovery.
func (s *Scanner) historyRecoveryOutstanding(rootID string) bool {
	s.historyRecoveryMu.Lock()
	defer s.historyRecoveryMu.Unlock()
	state := s.historyRecovery[rootID]
	return !state.DueAt.IsZero() || state.Queued
}

// runDueHistoryRecoveries relists every root whose recovery is due and queues
// the resulting repair signals for the dirty flush that follows.
func (s *Scanner) runDueHistoryRecoveries(ctx context.Context, now time.Time) {
	type dueRecovery struct {
		rootID string
		reason string
	}
	s.historyRecoveryMu.Lock()
	due := make([]dueRecovery, 0)
	for rootID, state := range s.historyRecovery {
		if state.DueAt.IsZero() || state.DueAt.After(now) {
			continue
		}
		due = append(due, dueRecovery{rootID: rootID, reason: state.Reason})
		state.DueAt = time.Time{}
		// Startup flushes pass a future now to skip the debounce window; the
		// cooldown must still start from the real run time.
		state.LastRunAt = time.Now()
		s.historyRecovery[rootID] = state
	}
	s.historyRecoveryMu.Unlock()

	for _, recovery := range due {
		queued, err := s.recoverRootHistory(ctx, recovery.rootID, recovery.reason)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch history recovery failed, retrying after cooldown: root=%s err=%s", recovery.rootID, err.Error()))
			s.requestHistoryRecovery(ctx, recovery.rootID, recovery.reason, now)
			continue
		}
		s.historyRecoveryMu.Lock()
		state := s.historyRecovery[recovery.rootID]
		state.Queued = queued
		s.historyRecovery[recovery.rootID] = state
		s.historyRecoveryMu.Unlock()
		if !queued {
			s.finishHistoryRecovery(ctx, recovery.rootID)
		}
	}
}

// completeHistoryRecoveries runs after a successful dirty flush. The dirty
// queue flushes every pending signal of a root together, so a flush that
// covers a root with queued repair signals has applied them.
func (s *Scanner) completeHistoryRecoveries(ctx context.Context, roots []RootRecord) {
	for _, root := range roots {
		s.historyRecoveryMu.Lock()
		state, ok := s.historyRecovery[root.ID]
		finished := ok && state.Queued && state.DueAt.IsZero()
		if finished {
			state.Queued = false
			s.historyRecovery[root.ID] = state
		}
		s.historyRecoveryMu.Unlock()
		if finished {
			s.finishHistoryRecovery(ctx, root.ID)
		}
	}
}

func (s *Scanner) finishHistoryRecovery(ctx context.Context, rootID string) {
	root, ok := s.findRootByID(ctx, rootID)
	if ok && root.FeedState == RootFeedStateDegraded {
		s.updateRootFeedState(ctx, rootID, RootFeedStateReady)
	}
}

// recoverRootHistory relists changed directories of one root and reports
// whether any repair signal was queued.
func (s *Scanner) recoverRootHistory(ctx context.Context, rootID string, reason string) (bool, error) {
	startedAt := time.Now()
	root, ok := s.findRootByID(ctx, rootID)
	if !ok {
		return false, nil
	}
	if root.LastFullScanAt <= 0 && root.LastReconcileAt <= 0 {
		// The first index build observes everything; there is no history to repair.
		return false, nil
	}
	if info, err := os.Stat(root.Path); err != nil || !info.IsDir() {
		// A missing or replaced root cannot be relisted. The root reconcile path
		// owns missing-root handling.
		s.enqueueRootReconcile(ctx, root)
		return true, nil
	}

	directories, err := s.db.directory.listIndexedDirectories(ctx, root.ID)
	if errors.Is(err, column.ErrNotReady) || (err == nil && len(directories) <= 1) {
		// Without a published snapshot there are no indexed facts to compare.
		s.enqueueRootReconcile(ctx, root)
		return true, nil
	}
	if err != nil {
		return false, err
	}

	changed, err := statChangedDirectories(ctx, directories)
	if err != nil {
		return false, err
	}
	now := time.Now()
	for _, directory := range changed {
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindPath,
			SemanticKind:  ChangeSemanticKindMetadata,
			RootID:        root.ID,
			Path:          directory,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            now,
		})
	}

	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch history recovery queued relists: root=%s path=%s reason=%q directories=%d changed_directories=%d elapsed=%s",
		root.ID,
		root.Path,
		strings.TrimSpace(reason),
		len(directories),
		len(changed),
		time.Since(startedAt).Round(time.Millisecond),
	))
	return len(changed) > 0, nil
}

func (s *Scanner) enqueueRootReconcile(ctx context.Context, root RootRecord) {
	s.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        root.ID,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
	})
}

// statChangedDirectories lstats indexed directories in parallel and returns
// those whose listing may differ from the index. Directories that vanished are
// skipped: their parent's mtime changed too, and its relist prunes them.
func statChangedDirectories(ctx context.Context, directories []indexedDirectory) ([]string, error) {
	changed := make([]bool, len(directories))
	var next atomic.Int64
	var wg sync.WaitGroup
	workers := min(historyRecoveryStatWorkers, len(directories))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= len(directories) {
					return
				}
				if index%1024 == 0 && ctx.Err() != nil {
					return
				}
				directory := directories[index]
				if !directory.ModifiedKnown {
					changed[index] = true
					continue
				}
				info, err := os.Lstat(directory.Path)
				if err != nil || !info.IsDir() {
					continue
				}
				changed[index] = info.ModTime().UnixMilli() != directory.Modified
			}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	paths := make([]string, 0)
	for index, isChanged := range changed {
		if isChanged {
			paths = append(paths, directories[index].Path)
		}
	}
	return paths, nil
}

// indexedDirectory is a directory row compared with disk by the recovery sweep.
type indexedDirectory struct {
	Path          string
	Modified      int64
	ModifiedKnown bool
}

// visitShardRecords yields every live record of one shard, applying the
// journal overlay over the published base.
func visitShardRecords(ctx context.Context, view column.View, yield func(column.Record) error) error {
	for row := 0; row < view.Base.Len(); row++ {
		if row%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if _, changed := view.Delta[view.Base.IDs[row]]; changed {
			continue
		}
		if err := yield(view.Base.Record(uint32(row))); err != nil {
			return err
		}
	}
	for _, change := range view.Delta {
		if change.Deleted {
			continue
		}
		if err := yield(change.Record); err != nil {
			return err
		}
	}
	return nil
}

// listIndexedDirectories returns the root directory plus every observed
// directory of a root. The root itself is always relisted because its own
// mtime is not tracked by the root descriptor row.
func (d *directoryIndex) listIndexedDirectories(ctx context.Context, rootID string) ([]indexedDirectory, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, errors.New("directory index is closed")
	}
	shard := d.shards[rootID]
	if shard == nil {
		return nil, column.ErrNotReady
	}

	directories := []indexedDirectory{{Path: filepath.Clean(shard.root.Path)}}
	err := shard.store.Read(func(view column.View) error {
		rootRecordID := view.Base.IDs[0]
		return visitShardRecords(ctx, view, func(record column.Record) error {
			if record.ID == rootRecordID || record.Flags&column.Directory == 0 || record.Flags&column.Structural != 0 {
				return nil
			}
			path, ok := view.Path(record.ID)
			if !ok {
				return nil
			}
			directories = append(directories, indexedDirectory{
				Path:          path,
				Modified:      record.Modified,
				ModifiedKnown: record.Flags&column.ModifiedKnown != 0,
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return directories, nil
}
