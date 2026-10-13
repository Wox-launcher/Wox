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

// writeScope consumes bounded scanner batches. Full runs append compact rows;
// incremental scope replacement publishes atomically, including stale deletion.
func (d *directoryIndex) writeScope(ctx context.Context, rootID string, scope string, directFiles bool, stream func(func(SubtreeSnapshotBatch) error) error) error {
	return d.writeScopes(ctx, rootID, []string{scope}, directFiles, stream)
}

// writeScopes publishes all disjoint scopes in one root generation. Validating
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
	revision := shard.store.Revision()
	builder := column.NewBuilder(shard.root.Path, directoryEntryID(shard.root.Path))
	cursor := ""
	err := shard.store.Read(func(view column.View) error {
		cursor = view.Cursor
		if directFiles || !owns(shard.root.Path) {
			if err := builder.Add(view.Base.Record(0)); err != nil {
				return err
			}
		}
		// Scope membership is shared by all files in one directory. Avoid a full
		// path allocation for every unaffected file while copying immutable rows.
		parents := make(map[uint64]bool)
		copyRecord := func(record column.Record) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Tombstoned parents hide descendants. Copying those descendants into
			// a new generation could resurrect them when the same path is reused.
			if !view.Reachable(record.ID) {
				return nil
			}
			var owned bool
			if record.Flags&column.Directory != 0 {
				if !directFiles {
					path, reachable := view.Path(record.ID)
					owned = reachable && owns(path)
				}
			} else {
				var checked bool
				owned, checked = parents[record.ParentID]
				if !checked {
					parent, reachable := view.Path(record.ParentID)
					owned = reachable && owns(parent)
					parents[record.ParentID] = owned
				}
			}
			if owned {
				return nil
			}
			return builder.Add(record)
		}
		for row := 1; row < view.Base.Len(); row++ {
			if _, changed := view.Delta[view.Base.IDs[row]]; changed {
				continue
			}
			if err := copyRecord(view.Base.Record(uint32(row))); err != nil {
				return err
			}
		}
		for _, change := range view.Delta {
			if !change.Deleted && change.ID != view.Base.IDs[0] {
				if err := copyRecord(change.Record); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, column.ErrNotReady) {
		return err
	}
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
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := preserveUnreadable(ctx, shard.store, builder, unreadable); err != nil {
		return err
	}
	base, err := builder.Finish()
	if err != nil {
		return err
	}
	return shard.store.Publish(base, cursor, "configured-root-v1", revision)
}

// preserveUnreadable copies only still-reachable prior facts. An inaccessible
// directory is an unknown observation, whereas a confirmed missing path is a
// deletion. One pass covers all failed scopes without retaining full paths.
func preserveUnreadable(ctx context.Context, store *column.Store, builder *column.Builder, scopes []string) error {
	if len(scopes) == 0 {
		return nil
	}
	err := store.Read(func(view column.View) error {
		copyRecord := func(record column.Record) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			path, reachable := view.Path(record.ID)
			if !reachable {
				return nil
			}
			for _, scope := range scopes {
				if pathWithinScope(scope, path) {
					if err := addDirectoryParents(builder, view.Base.Root, path); err != nil {
						return err
					}
					return builder.Add(record)
				}
			}
			return nil
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
// carries them separately from searchable filename entries.
func appendDirectoryBatch(builder *column.Builder, root RootRecord, scope string, batch SubtreeSnapshotBatch) error {
	if batch.RootID != root.ID || !pathWithinScope(scope, batch.ScopePath) {
		return errors.New("snapshot batch belongs to another scope")
	}
	if err := validateSubtreeSnapshotBatch(batch); err != nil {
		return err
	}
	for _, directory := range batch.Directories {
		if !directory.Exists {
			continue
		}
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
