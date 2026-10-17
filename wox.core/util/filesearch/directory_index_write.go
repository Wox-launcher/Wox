package filesearch

import (
	"context"
	"errors"
	"path/filepath"

	"wox/util/filesearch/column"
)

// beginBuild starts one root generation while keeping its prior snapshot visible.
func (d *directoryIndex) beginBuild(rootID string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[rootID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.build = &directoryBuild{builder: column.NewBuilder(shard.root.Path, directoryEntryID(shard.root.Path)), revision: shard.store.Revision()}
	return nil
}

// abortBuilds drops unfinished, unpublished work after an interrupted full run.
func (d *directoryIndex) abortBuilds() {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, shard := range d.shards {
		shard.mu.Lock()
		shard.build = nil
		shard.mu.Unlock()
	}
}

// finishBuild commits filename data before its source cursor can be persisted by
// the root metadata owner. Repeating after a crash may replay events, never skip them.
// For an incremental run its synced frame also commits every unsynced scoped
// write the run made, so a run costs one fsync however many jobs it has.
func (d *directoryIndex) finishBuild(ctx context.Context, root RootRecord) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[root.ID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if shard.build == nil {
		return shard.store.Apply(nil, root.FeedCursor)
	}
	build := shard.build
	shard.build = nil
	if err := preserveUnreadable(ctx, shard.store, build.builder, build.unreadable); err != nil {
		return err
	}
	base, err := build.builder.Finish()
	if err != nil {
		return err
	}
	return shard.store.Publish(base, root.FeedCursor, "configured-root-v1", build.revision)
}

// writeScopes publishes all disjoint scopes in one atomic update. Validating
// and building them together prevents a later failed batch exposing a partial run.
func (d *directoryIndex) writeScopes(ctx context.Context, rootID string, scopes []string, directFiles bool, stream func(func(SubtreeSnapshotBatch) error) error) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[rootID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	for _, scope := range scopes {
		if !pathWithinScope(shard.root.Path, scope) {
			return errors.New("directory scope lies outside root")
		}
	}
	owns := func(path string) bool {
		for _, scope := range scopes {
			if directFiles {
				if normalizePath(scope) == normalizePath(path) {
					return true
				}
			} else if pathWithinScope(scope, path) {
				return true
			}
		}
		return false
	}
	appendBatch := func(builder *column.Builder, batch SubtreeSnapshotBatch) error {
		if !owns(batch.ScopePath) {
			return errors.New("snapshot batch outside owned scopes")
		}
		for _, path := range batch.UnreadableScopes {
			if !pathWithinScope(batch.ScopePath, path) {
				return errors.New("unreadable path outside owned scope")
			}
		}
		return appendDirectoryBatch(builder, shard.root, shard.root.Path, batch)
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if shard.build != nil {
		return stream(func(batch SubtreeSnapshotBatch) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := appendBatch(shard.build.builder, batch); err != nil {
				return err
			}
			shard.build.unreadable = append(shard.build.unreadable, batch.UnreadableScopes...)
			return nil
		})
	}
	// Stage only the observed scope. A failed stream must leave both the live
	// overlay and its journal untouched.
	builder := column.NewBuilder(shard.root.Path, directoryEntryID(shard.root.Path))
	var unreadable []string
	if err := stream(func(batch SubtreeSnapshotBatch) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := appendBatch(builder, batch); err != nil {
			return err
		}
		unreadable = append(unreadable, batch.UnreadableScopes...)
		return nil
	}); err != nil {
		return err
	}
	// An explicit whole-root replacement owns every row, including the root
	// descriptor (which journal mutations intentionally cannot replace).
	if !directFiles && owns(shard.root.Path) {
		if err := preserveUnreadable(ctx, shard.store, builder, unreadable); err != nil {
			return err
		}
		base, err := builder.Finish()
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		cursor := ""
		if err := shard.store.Read(func(view column.View) error { cursor = view.Cursor; return nil }); err != nil && !errors.Is(err, column.ErrNotReady) {
			return err
		}
		return shard.store.Publish(base, cursor, "configured-root-v1", shard.store.Revision())
	}
	observed, err := builder.Finish()
	if err != nil {
		return err
	}
	var changes []column.Change
	cursor := ""
	err = shard.store.Read(func(view column.View) error {
		cursor = view.Cursor
		walker := newScopeWalker(view)
		// Deletion candidates are only the rows already stored under the owned
		// scopes. Walking each scope's subtree through the child index keeps a
		// dirty-directory refresh proportional to that subtree; the previous sweep
		// materialized paths for every row in the shard on every scoped write.
		// A direct-files scope owns only its immediate non-directory children.
		deleteUnobserved := unobservedDeleter(observed, directoryScopeProtection(view, unreadable), &changes)
		for _, scope := range scopes {
			scopeID := directoryEntryID(scope)
			if directFiles {
				walker.children(scopeID, func(id uint64) {
					if record, live := view.Lookup(id); live && record.Flags&column.Directory == 0 {
						deleteUnobserved(record)
					}
				})
				continue
			}
			if err := walker.subtree(ctx, scopeID, deleteUnobserved); err != nil {
				return err
			}
		}
		changes = appendObservedChanges(view, walker, observed, changes)
		return nil
	})
	return applyObservedChanges(ctx, shard.store, observed, changes, cursor, err)
}

// writeDirectory relists one directory: its own row and its immediate
// children. Child directories the index already holds are trusted, because
// their contents report their own changes; the stream walks only child
// directories new to the index. Stored subtrees are deleted only beneath
// children that vanished or stopped being directories, so the diff stays
// local to this directory however large its subtree is.
func (d *directoryIndex) writeDirectory(ctx context.Context, rootID string, scope string, stream func(knownDirectory func(uint64) bool, yield func(SubtreeSnapshotBatch) error) error) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[rootID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	if !pathWithinScope(shard.root.Path, scope) {
		return errors.New("directory scope lies outside root")
	}
	scopeID := directoryEntryID(scope)
	appendBatch := func(builder *column.Builder, batch SubtreeSnapshotBatch) error {
		if normalizePath(batch.ScopePath) != normalizePath(scope) {
			return errors.New("snapshot batch outside owned directory")
		}
		for _, path := range batch.UnreadableScopes {
			if !pathWithinScope(scope, path) {
				return errors.New("unreadable path outside owned directory")
			}
		}
		return appendDirectoryBatch(builder, shard.root, shard.root.Path, batch)
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if shard.build != nil {
		// A full build replaces everything, so this relist simply contributes a
		// complete observation of the directory.
		return stream(func(uint64) bool { return false }, func(batch SubtreeSnapshotBatch) error {
			if err := appendBatch(shard.build.builder, batch); err != nil {
				return err
			}
			shard.build.unreadable = append(shard.build.unreadable, batch.UnreadableScopes...)
			return nil
		})
	}

	known := make(map[uint64]bool)
	err := shard.store.Read(func(view column.View) error {
		newScopeWalker(view).children(scopeID, func(id uint64) {
			if record, live := view.Lookup(id); live && record.Flags&column.Directory != 0 && record.Flags&column.Structural == 0 {
				known[id] = true
			}
		})
		return nil
	})
	if err != nil && !errors.Is(err, column.ErrNotReady) {
		return err
	}
	builder := column.NewBuilder(shard.root.Path, directoryEntryID(shard.root.Path))
	var unreadable []string
	if err := stream(func(id uint64) bool { return known[id] }, func(batch SubtreeSnapshotBatch) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := appendBatch(builder, batch); err != nil {
			return err
		}
		unreadable = append(unreadable, batch.UnreadableScopes...)
		return nil
	}); err != nil {
		return err
	}
	observed, err := builder.Finish()
	if err != nil {
		return err
	}

	var changes []column.Change
	cursor := ""
	err = shard.store.Read(func(view column.View) error {
		cursor = view.Cursor
		walker := newScopeWalker(view)
		// owned lists the stored subtrees this relist replaces: the directory or
		// a child that vanished, a directory now replaced by a file, and child
		// directories the stream walked as new.
		var owned []uint64
		consider := func(id uint64) {
			record, live := view.Lookup(id)
			if !live {
				return
			}
			row, observedNow := observed.Row(id)
			if !observedNow || (record.Flags&column.Directory != 0 && observed.Flags[row]&column.Directory == 0) {
				owned = append(owned, id)
			}
		}
		consider(scopeID)
		walker.children(scopeID, consider)
		for row := 1; row < observed.Len(); row++ {
			record := observed.Record(uint32(row))
			if record.ParentID == scopeID && record.Flags&column.Directory != 0 && !known[record.ID] {
				owned = append(owned, record.ID)
			}
		}

		deleteUnobserved := unobservedDeleter(observed, directoryScopeProtection(view, unreadable), &changes)
		for _, id := range owned {
			if err := walker.subtree(ctx, id, deleteUnobserved); err != nil {
				return err
			}
		}
		changes = appendObservedChanges(view, walker, observed, changes)
		return nil
	})
	return applyObservedChanges(ctx, shard.store, observed, changes, cursor, err)
}

// unobservedDeleter returns a subtree visitor that tombstones stored rows the
// observation no longer contains. Visits arrive parents first, and only the
// topmost vanished row is written: its tombstone already hides everything
// beneath it, so deleting a large tree stays one journal record instead of
// overflowing the journal frame and forcing a full republish.
func unobservedDeleter(observed *column.Snapshot, protected func(column.Record) bool, changes *[]column.Change) func(column.Record) {
	deleted := make(map[uint64]bool)
	return func(record column.Record) {
		if deleted[record.ID] || protected(record) {
			return
		}
		if _, exists := observed.Row(record.ID); exists {
			return
		}
		deleted[record.ID] = true
		if !deleted[record.ParentID] {
			*changes = append(*changes, column.Change{Record: column.Record{ID: record.ID}, Deleted: true})
		}
	}
}

// appendObservedChanges adds upserts for observed rows that differ from the
// stored view, plus the deletes that keep revived directories from exposing
// their stale stored children.
func appendObservedChanges(view column.View, walker *scopeWalker, observed *column.Snapshot, changes []column.Change) []column.Change {
	for row := 1; row < observed.Len(); row++ {
		record := observed.Record(uint32(row))
		old, exists := view.Lookup(record.ID)
		// Synthetic ancestry must not replace an observed directory's flags
		// or metadata, including directories outside this scan's ownership.
		if exists && view.Reachable(record.ID) && (record.Flags&column.Structural != 0 || old == record) {
			continue
		}
		changes = append(changes, column.Change{Record: record})
	}
	return appendRevivalDeletes(view, walker, changes)
}

// applyObservedChanges commits a scoped diff computed under readErr. A store
// without a published generation adopts the observation as its first one.
func applyObservedChanges(ctx context.Context, store *column.Store, observed *column.Snapshot, changes []column.Change, cursor string, readErr error) error {
	if errors.Is(readErr, column.ErrNotReady) {
		if err := ctx.Err(); err != nil {
			return err
		}
		return store.Publish(observed, "", "configured-root-v1", store.Revision())
	}
	if readErr != nil {
		return readErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	// One journal frame publishes the entire diff atomically. It is synced
	// with the rest of the run by finishBuild, before the feed cursor advances.
	// The scanner's threshold-based maintenance owns full generation compaction.
	return store.ApplyUnsynced(changes, cursor)
}

// preserveUnreadable copies only still-reachable prior facts. An inaccessible
// directory is an unknown observation, whereas a confirmed missing path is a
// deletion. One pass covers all failed scopes without retaining full paths.
func preserveUnreadable(ctx context.Context, store *column.Store, builder *column.Builder, scopes []string) error {
	if len(scopes) == 0 {
		return nil
	}
	err := store.Read(func(view column.View) error {
		protected := directoryScopeProtection(view, scopes)
		copyRecord := func(record column.Record) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !protected(record) {
				return nil
			}
			path, reachable := view.Path(record.ID)
			if !reachable {
				return nil
			}
			if err := addDirectoryParents(builder, view.Base.Root, path); err != nil {
				return err
			}
			return builder.Add(record)
		}
		for row := 0; row < view.Base.Len(); row++ {
			if _, changed := view.Delta[view.Base.IDs[row]]; !changed {
				if err := copyRecord(view.Base.Record(uint32(row))); err != nil {
					return err
				}
			}
		}
		for _, change := range view.Delta {
			if !change.Deleted {
				if err := copyRecord(change.Record); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if errors.Is(err, column.ErrNotReady) {
		return nil
	}
	return err
}

// appendDirectoryBatch includes directory relationships even when a scanner job
// carries them separately from searchable filename entries. Path-based entries
// are validated and resolved here; pre-resolved stream records are inserted as is.
func appendDirectoryBatch(builder *column.Builder, root RootRecord, scope string, batch SubtreeSnapshotBatch) error {
	if batch.RootID != root.ID || !pathWithinScope(scope, batch.ScopePath) {
		return errors.New("snapshot batch belongs to another scope")
	}
	if err := validateSubtreeSnapshotBatch(batch); err != nil {
		return err
	}
	for _, directory := range batch.Directories {
		if err := addDirectoryParents(builder, root.Path, directory.Path); err != nil {
			return err
		}
		if builder.Contains(directoryEntryID(directory.Path)) {
			continue
		}
		if err := builder.Add(column.Record{ID: directoryEntryID(directory.Path), ParentID: directoryEntryID(filepath.Dir(directory.Path)), Name: filepath.Base(directory.Path), Flags: column.Directory | column.Structural}); err != nil {
			return err
		}
	}
	for _, entry := range batch.Entries {
		record, err := directoryEntryRecord(root, entry)
		if err != nil {
			return err
		}
		if err := addDirectoryParents(builder, root.Path, entry.Path); err != nil {
			return err
		}
		if err := builder.Add(record); err != nil {
			return err
		}
	}
	if len(batch.Records) > 0 {
		// Stream records link to each other by parent ID; only the scope itself
		// may need synthetic ancestry when it is scanned before its parent.
		if err := addDirectoryParents(builder, root.Path, batch.ScopePath); err != nil {
			return err
		}
		for _, record := range batch.Records {
			if err := builder.Add(record); err != nil {
				return err
			}
		}
	}
	return nil
}

// addDirectoryParents supplies ancestry for an incremental scope received before
// its parent's first scan. Known rows retain their observed metadata.
func addDirectoryParents(builder *column.Builder, root, path string) error {
	if normalizePath(root) == normalizePath(path) {
		return nil
	}
	for parent := filepath.Dir(path); normalizePath(parent) != normalizePath(root); parent = filepath.Dir(parent) {
		if !pathWithinScope(root, parent) {
			return errors.New("directory ancestor outside root")
		}
		id := directoryEntryID(parent)
		if builder.Contains(id) {
			break
		}
		if err := builder.Add(column.Record{ID: id, ParentID: directoryEntryID(filepath.Dir(parent)), Name: filepath.Base(parent), Flags: column.Directory | column.Structural}); err != nil {
			return err
		}
	}
	return nil
}

// scopeWalker enumerates stored children through the base generation's child
// index plus the rows the journal overlay added since that generation.
type scopeWalker struct {
	view  column.View
	added map[uint64][]uint64
}

func newScopeWalker(view column.View) *scopeWalker {
	// Base rows changed in the overlay keep their base parent, because entry
	// IDs derive from paths and a path's parent never changes.
	added := make(map[uint64][]uint64)
	for id, change := range view.Delta {
		if change.Deleted {
			continue
		}
		if _, inBase := view.Base.Row(id); !inBase {
			added[change.ParentID] = append(added[change.ParentID], id)
		}
	}
	return &scopeWalker{view: view, added: added}
}

// children yields every stored child ID of id, including deleted ones.
func (w *scopeWalker) children(id uint64, yield func(uint64)) {
	if row, ok := w.view.Base.Row(id); ok {
		for _, child := range w.view.Base.Children(row) {
			yield(w.view.Base.IDs[child])
		}
	}
	for _, child := range w.added[id] {
		yield(child)
	}
}

// subtree visits id and its live descendants, parents before children. A
// deleted row already hides everything beneath it, so the walk does not
// descend through it. The start is descended even when deleted: a scan of a
// recreated directory must see the stale rows it is about to replace. The
// root descriptor row is never visited.
func (w *scopeWalker) subtree(ctx context.Context, id uint64, visit func(column.Record)) error {
	rootID := w.view.Base.IDs[0]
	stack := []uint64{id}
	for visited := 0; len(stack) > 0; visited++ {
		if visited%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		record, live := w.view.Lookup(current)
		if !live && current != id {
			continue
		}
		if live && current != rootID {
			visit(record)
		}
		w.children(current, func(child uint64) { stack = append(stack, child) })
	}
	return nil
}

// appendRevivalDeletes keeps a recreated directory from resurrecting its
// stale contents. Deleting a directory only tombstones its own row, which
// hides the whole stored subtree (as fsearch's dead range does) without one
// journal record per descendant. When a change set brings a dead directory
// back (deleted, under a dead ancestor, or previously a file), its stored
// children that are not re-observed are tombstoned here;
// re-observed child directories are revived too and get the same treatment.
func appendRevivalDeletes(view column.View, walker *scopeWalker, changes []column.Change) []column.Change {
	written := make(map[uint64]bool, len(changes))
	for _, change := range changes {
		written[change.ID] = true
	}
	for i, count := 0, len(changes); i < count; i++ {
		change := changes[i]
		if change.Deleted || change.Flags&column.Directory == 0 {
			continue
		}
		// A directory whose path was a file also hid its old children.
		if old, live := view.Lookup(change.ID); live && old.Flags&column.Directory != 0 && view.Reachable(change.ID) {
			continue
		}
		walker.children(change.ID, func(child uint64) {
			if written[child] {
				return
			}
			if _, live := view.Lookup(child); live {
				written[child] = true
				changes = append(changes, column.Change{Record: column.Record{ID: child}, Deleted: true})
			}
		})
	}
	return changes
}

// directoryScopeProtection matches unreadable scopes by stable path IDs. Memoizing
// directory ancestry avoids comparing every old file against every failed path;
// whole-volume scans can report thousands of inaccessible system directories.
func directoryScopeProtection(view column.View, scopes []string) func(column.Record) bool {
	if len(scopes) == 0 {
		return func(column.Record) bool { return false }
	}
	protectedIDs := make(map[uint64]bool, len(scopes))
	for _, scope := range scopes {
		protectedIDs[directoryEntryID(scope)] = true
	}
	memo := make(map[uint64]bool)
	var parentProtected func(uint64) bool
	parentProtected = func(id uint64) bool {
		if protectedIDs[id] {
			return true
		}
		if value, known := memo[id]; known {
			return value
		}
		// Seed before following parents so corrupt ancestry cannot recurse forever.
		memo[id] = false
		if id == view.Base.IDs[0] {
			return false
		}
		record, found := view.Lookup(id)
		if !found {
			return false
		}
		value := parentProtected(record.ParentID)
		memo[id] = value
		return value
	}
	return func(record column.Record) bool {
		return protectedIDs[record.ID] || parentProtected(record.ParentID)
	}
}
