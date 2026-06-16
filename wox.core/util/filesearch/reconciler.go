package filesearch

import (
	"context"
	"fmt"
	"wox/util"
)

type ReconcileResult struct {
	RootID       string
	Mode         ReconcileMode
	ReloadNeeded bool
}

type Reconciler struct {
	db       *FileSearchDB
	snapshot *SnapshotBuilder
}

type ReconcileProgress struct {
	Stage   ReplaceEntriesStage
	Current int64
	Total   int64
}

func NewReconciler(db *FileSearchDB, policy *policyState) *Reconciler {
	return newReconciler(db, NewSnapshotBuilder(policy))
}

func newReconciler(db *FileSearchDB, snapshot *SnapshotBuilder) *Reconciler {
	if snapshot == nil {
		snapshot = NewSnapshotBuilder(nil)
	}

	return &Reconciler{
		db:       db,
		snapshot: snapshot,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, batch ReconcileBatch) (ReconcileResult, error) {
	return r.ReconcileWithProgress(ctx, batch, nil)
}

func (r *Reconciler) ReconcileWithProgress(ctx context.Context, batch ReconcileBatch, onProgress func(ReconcileProgress)) (ReconcileResult, error) {
	result := ReconcileResult{
		RootID: batch.RootID,
		Mode:   batch.Mode,
	}

	root, err := r.db.FindRootByID(ctx, batch.RootID)
	if err != nil {
		return result, err
	}
	if root == nil {
		return result, fmt.Errorf("root %q not found", batch.RootID)
	}

	switch batch.Mode {
	case ReconcileModeDirectDelta:
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile direct delta started: root=%s paths=%d",
			root.ID,
			len(batch.DirectDeltas),
		))
		// Feature addition: callers that still use the legacy Reconciler API can
		// apply exact file deltas without widening them into subtree snapshots.
		// This keeps the old entry point behavior aligned with the planned-run
		// executor while preserving the same root state update semantics.
		if err := r.db.ApplyDirectDeltaJob(ctx, *root, Job{
			Kind:         JobKindDirectDelta,
			RootID:       root.ID,
			ScopePath:    root.Path,
			DirectDeltas: batch.DirectDeltas,
		}, r.snapshot.policy); err != nil {
			return result, err
		}
		if len(batch.DirectDeltas) > 0 {
			now := util.GetSystemTimestamp()
			root.LastReconcileAt = now
			root.FeedState = nextFeedStateAfterSuccessfulReconcile(*root)
			root.UpdatedAt = now
			if err := r.db.UpdateRootState(ctx, *root); err != nil {
				return result, err
			}
		}
		result.ReloadNeeded = len(batch.DirectDeltas) > 0
		return result, nil
	case ReconcileModeRoot:
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile snapshot build started: root=%s mode=%s scope=%s",
			root.ID,
			batch.Mode,
			root.Path,
		))
		snapshot, err := r.snapshot.BuildSubtreeSnapshot(ctx, *root, root.Path)
		if err != nil {
			return result, err
		}
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile snapshot built: root=%s mode=%s scope=%s directories=%d entries=%d",
			root.ID,
			batch.Mode,
			root.Path,
			len(snapshot.Directories),
			len(snapshot.Entries),
		))
		// Root reconcile used to hide all SQLite work behind a generic "syncing"
		// label, so the toolbar looked stuck even while ReplaceRootSnapshot was
		// writing hundreds of thousands of rows. Forward the DB progress so the UI
		// can switch from indeterminate syncing to real write progress.
		if err := r.db.ReplaceRootSnapshot(ctx, *root, snapshot.Directories, snapshot.Entries, func(progress ReplaceEntriesProgress) {
			if onProgress == nil {
				return
			}
			onProgress(ReconcileProgress{
				Stage:   progress.Stage,
				Current: progress.Current,
				Total:   progress.Total,
			})
		}); err != nil {
			return result, err
		}
		now := util.GetSystemTimestamp()
		root.LastReconcileAt = now
		root.FeedState = nextFeedStateAfterSuccessfulReconcile(*root)
		root.UpdatedAt = now
		if err := r.db.UpdateRootState(ctx, *root); err != nil {
			return result, err
		}
		result.ReloadNeeded = true
		return result, nil
	case ReconcileModeSubtree:
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile snapshot build started: root=%s mode=%s scopes=%s",
			root.ID,
			batch.Mode,
			summarizeLogPaths(batch.Paths),
		))
		snapshots := make([]SubtreeSnapshotBatch, 0, len(batch.Paths))
		totalDirectories := 0
		totalEntries := 0
		for _, scopePath := range batch.Paths {
			snapshot, err := r.snapshot.BuildSubtreeSnapshot(ctx, *root, scopePath)
			if err != nil {
				return result, err
			}
			snapshots = append(snapshots, snapshot)
			totalDirectories += len(snapshot.Directories)
			totalEntries += len(snapshot.Entries)
		}
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"filesearch reconcile snapshots built: root=%s mode=%s scopes=%d directories=%d entries=%d",
			root.ID,
			batch.Mode,
			len(batch.Paths),
			totalDirectories,
			totalEntries,
		))
		if err := r.db.ReplaceSubtreeSnapshots(ctx, snapshots); err != nil {
			return result, err
		}
		if len(batch.Paths) > 0 {
			now := util.GetSystemTimestamp()
			root.LastReconcileAt = now
			root.FeedState = nextFeedStateAfterSuccessfulReconcile(*root)
			root.UpdatedAt = now
			if err := r.db.UpdateRootState(ctx, *root); err != nil {
				return result, err
			}
		}
		result.ReloadNeeded = len(batch.Paths) > 0
		return result, nil
	default:
		return result, fmt.Errorf("unsupported reconcile mode %q", batch.Mode)
	}
}

func nextFeedStateAfterSuccessfulReconcile(root RootRecord) RootFeedState {
	if root.FeedState == RootFeedStateUnavailable {
		return RootFeedStateUnavailable
	}
	return RootFeedStateReady
}
