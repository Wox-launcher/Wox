package filesearch

import (
	"context"
	"fmt"
	"time"

	"wox/util"
)

func (s *Scanner) startupRestore(ctx context.Context) {
	persistedEntryCount := int64(0)

	persistedFileCount, persistedEntryCount, err := s.db.SearchIndexCounts(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch startup restore failed to load persisted sqlite counts: "+err.Error())
		s.scanAllRootsWithReason(ctx, "startup_restore_fallback")
		s.refreshChangeFeed(ctx)
		return
	}
	// Startup restore now trusts the persisted SQLite search state directly.
	// The previous local-provider reload branch duplicated the same data in
	// memory and left startup with two result stores to keep consistent.
	// Optimization: restore decisions only need cheap entry/file counts. The old
	// full snapshot also sampled FTS vocab tables, which made production startup
	// pay for diagnostic-only index statistics before the diagnostic guard ran.
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch startup restore loaded persisted sqlite search state: entries=%d files=%d",
		persistedEntryCount,
		persistedFileCount,
	))

	roots, err := s.listPolicyAllowedRoots(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch startup restore failed to load roots: "+err.Error())
		s.refreshChangeFeed(ctx)
		return
	}
	roots = s.clearStartupTransientMissingPathErrors(ctx, roots)

	s.refreshChangeFeed(ctx)
	if startupNeedsInitialFullScan(roots, persistedEntryCount) {
		// Startup restore used to treat an empty persisted index as good enough and
		// enqueue root-dirty incremental reconcile for every never-scanned root.
		// That sent the very first index build down the incremental path, which
		// bypassed full-run bulk sync and subtree grouping. When persisted search
		// state is empty and at least one root has never completed a full scan, we
		// force the real full-run path so initial indexing uses the intended heavy-
		// scan execution strategy instead of replaying root-level dirty batches.
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch startup restore escalating to full scan: roots=%d persisted_entries=%d",
			len(roots),
			persistedEntryCount,
		))
		s.scanAllRootsWithReason(ctx, "startup_restore_initial_full")
		return
	}

	reconcileRoots := startupReconcileRoots(roots, time.Now())
	if len(reconcileRoots) == 0 {
		util.GetLogger().Info(ctx, "filesearch startup restore completed without reconcile")
		if shouldCollectFileSearchDiagnosticSnapshot() {
			// Optimization: this second startup snapshot is diagnostic-only. The
			// initial persisted-state read above already decided restore behavior, so
			// production should not rescan SQLite just to feed disabled snapshot logs.
			snapshot, err := s.db.SearchIndexSnapshot(ctx)
			if err == nil {
				logSQLiteIndexSnapshot(ctx, "startup_restore_complete", snapshot, true)
			}
		}
		return
	}

	for _, root := range reconcileRoots {
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindRoot,
			RootID:        root.ID,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            time.Now(),
		})
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("filesearch startup restore queued selective reconcile: roots=%d", len(reconcileRoots)))
	if err := s.processDirtyQueue(ctx, time.Now().Add(2*s.dirtyDebounceWindow())); err != nil {
		util.GetLogger().Warn(ctx, "filesearch startup restore failed to process selective reconcile: "+err.Error())
	}
	if shouldCollectFileSearchDiagnosticSnapshot() {
		// Optimization: selective reconcile already calls the real dirty flush path.
		// Keep the completion snapshot available for dev triage, but avoid another
		// expensive fts5vocab pass when diagnostic logging is disabled.
		if snapshot, err := s.db.SearchIndexSnapshot(ctx); err == nil {
			logSQLiteIndexSnapshot(ctx, "startup_restore_complete", snapshot, true)
		}
	}
}

func (s *Scanner) clearStartupTransientMissingPathErrors(ctx context.Context, roots []RootRecord) []RootRecord {
	for index := range roots {
		root := roots[index]
		if root.Status != RootStatusError || root.LastError == nil || !isMissingPathErrorMessage(*root.LastError) {
			continue
		}
		// Older builds persisted missing temp/build paths as root errors. Once the
		// process restarts those paths are already gone, so clear the stale banner
		// instead of forcing a broad fallback reconcile just to remove the message.
		root.Status = RootStatusIdle
		root.LastError = nil
		root.ProgressCurrent = RootProgressScale
		root.ProgressTotal = RootProgressScale
		if root.FeedState == RootFeedStateDegraded {
			root.FeedState = RootFeedStateReady
		}
		root.UpdatedAt = util.GetSystemTimestamp()
		if err := s.db.UpdateRootState(ctx, root); err != nil {
			util.GetLogger().Warn(ctx, "filesearch startup restore failed to clear transient root error: "+err.Error())
			continue
		}
		roots[index] = root
		util.GetLogger().Info(ctx, fmt.Sprintf("filesearch startup restore cleared transient root error: root=%s path=%s", root.ID, root.Path))
	}
	return roots
}

func startupNeedsInitialFullScan(roots []RootRecord, persistedEntryCount int64) bool {
	if persistedEntryCount > 0 {
		return false
	}

	for _, root := range roots {
		if root.LastFullScanAt <= 0 {
			return true
		}
	}

	return false
}

func startupReconcileRoots(roots []RootRecord, now time.Time) []RootRecord {
	selected := make([]RootRecord, 0, len(roots))
	for _, root := range roots {
		if !rootNeedsStartupReconcile(root, now) {
			continue
		}
		selected = append(selected, root)
	}
	return selected
}

func rootNeedsStartupReconcile(root RootRecord, now time.Time) bool {
	// Older fallback roots may have a successful reconcile timestamp without a
	// full-scan timestamp because startup restore previously drove them through
	// the incremental root path. Treat that reconcile as enough history for the
	// feed-specific freshness check below; otherwise a migrated large root keeps
	// rescanning on every launch despite having just completed a reconcile.
	if root.LastFullScanAt <= 0 && root.LastReconcileAt <= 0 {
		return true
	}

	if root.FeedType == RootFeedTypeFallback || root.FeedType == "" {
		// Fallback degraded can be left behind by a transient dirty subtree that
		// disappeared during reconcile. Use the freshness window for fallback
		// roots so every debug restart does not immediately rescan the whole tree.
		return fallbackRootNeedsStartupReconcile(root, now)
	}

	if root.FeedState == RootFeedStateDegraded || root.FeedState == RootFeedStateUnavailable {
		return true
	}

	switch root.FeedType {
	case RootFeedTypeFSEvents:
		cursor, ok := decodeFeedCursor(root.FeedCursor, RootFeedTypeFSEvents)
		return !ok || !feedCursorFresh(cursor, now, defaultFeedCursorSafeWindow)
	case RootFeedTypeUSN:
		return usnRootNeedsStartupReconcile(root, now)
	default:
		return true
	}
}

func fallbackRootNeedsStartupReconcile(root RootRecord, now time.Time) bool {
	if root.LastReconcileAt <= 0 {
		return true
	}

	lastReconcile := time.UnixMilli(root.LastReconcileAt)
	if lastReconcile.After(now) {
		return false
	}

	// Fallback feeds cannot replay offline changes, but reconciling every large
	// root on every debug restart made startup restore run an unbounded tree scan
	// even when the previous reconcile finished minutes ago. Reuse the same safe
	// window as cursor-based feeds: recent fallback roots rely on the live
	// fsnotify watcher, while stale roots still get a periodic full reconcile to
	// repair missed offline changes.
	return now.Sub(lastReconcile) > defaultFeedCursorSafeWindow
}
