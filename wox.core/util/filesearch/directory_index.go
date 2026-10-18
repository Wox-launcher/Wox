package filesearch

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"wox/util/filesearch/column"
)

// directoryIndex owns ordinary-user filename shards. Source adapters serialize
// writes per root; searches hold only map and column read leases.
type directoryIndex struct {
	mu     sync.RWMutex
	dir    string
	shards map[string]*directoryShard
	closed bool
}

type directoryShard struct {
	mu    sync.Mutex
	root  RootRecord
	store *column.Store
	build *directoryBuild
}

// directoryBuild is owned by the scanner's full-run lifecycle. Individual jobs
// append to it, but only a successful finalize publishes its complete generation.
type directoryBuild struct {
	builder    *column.Builder
	revision   uint64
	unreadable []string
}

func newDirectoryIndex(dir string) *directoryIndex {
	return &directoryIndex{dir: dir, shards: make(map[string]*directoryShard)}
}

// attach restores a configured shard without constructing a second filename map.
func (d *directoryIndex) attach(root RootRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return errors.New("directory index is closed")
	}
	if shard := d.shards[root.ID]; shard != nil {
		if normalizePath(shard.root.Path) != normalizePath(root.Path) {
			return errors.New("directory shard root identity changed")
		}
		shard.root = root
		return nil
	}
	store, err := column.OpenStore(d.shardDirectory(root.ID), prepareColumnNames)
	if err != nil {
		return err
	}
	err = store.Read(func(view column.View) error {
		if normalizePath(view.Base.Root) != normalizePath(root.Path) {
			return errors.New("directory shard path differs from configured root")
		}
		return nil
	})
	if err != nil && !errors.Is(err, column.ErrNotReady) {
		store.Close()
		return err
	}
	d.shards[root.ID] = &directoryShard{root: root, store: store}
	return nil
}

// search merges bounded per-shard results while suppressing overlapping-root paths.
func (d *directoryIndex) search(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, errors.New("directory index is closed")
	}
	shards := make([]*directoryShard, 0, len(d.shards))
	for _, shard := range d.shards {
		shards = append(shards, shard)
	}
	matches := make([][]SearchResult, len(shards))
	errs := make([]error, len(shards))
	jobs := make(chan int, len(shards))
	for i := range shards {
		jobs <- i
	}
	close(jobs)
	var wait sync.WaitGroup
	for worker := 0; worker < min(len(shards), runtime.GOMAXPROCS(0), 4); worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := range jobs {
				if err := ctx.Err(); err != nil {
					errs[i] = err
					continue
				}
				errs[i] = shards[i].store.ReadQuery(func(view column.View) error {
					var err error
					matches[i], err = searchColumnView(ctx, view, query, limit)
					return err
				})
			}
		}()
	}
	wait.Wait()
	results := &columnResultHeap{limit: limit}
	seen := make(map[string]bool)
	for i, batch := range matches {
		if errs[i] != nil && !errors.Is(errs[i], column.ErrNotReady) {
			return nil, errs[i]
		}
		for _, match := range batch {
			key := normalizePath(match.Path)
			if !seen[key] {
				seen[key] = true
				results.add(match)
			}
		}
	}

	return sortAndLimitResults(results.items, limit), nil
}

// applyEntries updates exact known paths; directory changes belong to subtree
// reconciliation so deleting a directory cannot accidentally preserve its children.
func (d *directoryIndex) applyEntries(ctx context.Context, rootID string, entries []EntryRecord, deleted []string, cursor string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	shard := d.shards[rootID]
	if d.closed || shard == nil {
		return errors.New("directory shard is not attached")
	}
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if err := shard.store.Read(func(column.View) error { return nil }); errors.Is(err, column.ErrNotReady) {
		base, buildErr := column.NewBuilder(shard.root.Path, directoryEntryID(shard.root.Path)).Finish()
		if buildErr != nil {
			return buildErr
		}
		if err := shard.store.Publish(base, "", "configured-root-v1", shard.store.Revision()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	changes := make([]column.Change, 0, len(entries)+len(deleted))
	// File events can precede the first parent scan. Supply only the ancestry
	// proven by each existing file, without recursively scanning its siblings.
	// A deleted path writes only its own tombstone, which hides any stored
	// descendants; reviving a dead directory here tombstones its stale children.
	err := shard.store.Read(func(view column.View) error {
		added := map[uint64]bool{}
		for _, entry := range entries {
			if entry.RootID != rootID || !pathWithinScope(shard.root.Path, entry.Path) {
				return fmt.Errorf("entry outside root")
			}
			if normalizePath(entry.Path) == normalizePath(shard.root.Path) {
				continue
			}
			for parent := filepath.Dir(entry.Path); normalizePath(parent) != normalizePath(shard.root.Path); parent = filepath.Dir(parent) {
				id := directoryEntryID(parent)
				if added[id] {
					break
				}
				if _, exists := view.Lookup(id); exists && view.Reachable(id) {
					break
				}
				added[id] = true
				changes = append(changes, column.Change{Record: column.Record{ID: id, ParentID: directoryEntryID(filepath.Dir(parent)), Name: filepath.Base(parent), Flags: column.Directory | column.Structural}})
			}
		}
		for _, entry := range entries {
			record, err := directoryEntryRecord(shard.root, entry)
			if err != nil {
				return err
			}
			changes = append(changes, column.Change{Record: record})
		}
		for _, path := range deleted {
			if !pathWithinScope(shard.root.Path, path) || normalizePath(shard.root.Path) == normalizePath(path) {
				return fmt.Errorf("invalid deleted path %q", path)
			}
			changes = append(changes, column.Change{Record: column.Record{ID: directoryEntryID(path)}, Deleted: true})
		}
		changes = appendRevivalDeletes(view, newScopeWalker(view), changes)
		return nil
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Durable once finishBuild syncs the run, before the feed cursor advances.
	return shard.store.ApplyUnsynced(changes, cursor)
}

// visit yields the common live directory view for content candidate enumeration,
// diagnostics and reconciliation. It never returns borrowed mmap strings.
func (d *directoryIndex) visit(ctx context.Context, rootID string, yield func(EntryRecord) error) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return errors.New("directory index is closed")
	}
	for id, shard := range d.shards {
		if rootID != "" && rootID != id {
			continue
		}
		err := shard.store.Read(func(view column.View) error {
			emit := func(record column.Record) error {
				if record.Flags&column.Structural != 0 {
					return nil
				}
				if err := ctx.Err(); err != nil {
					return err
				}
				path, ok := view.Path(record.ID)
				if !ok {
					return nil
				}
				return yield(EntryRecord{Path: path, RootID: id, ParentPath: filepath.Dir(path), Name: filepath.Base(path), IsDir: record.Flags&column.Directory != 0, Size: record.Size, Mtime: record.Modified})
			}
			for row := 0; row < view.Base.Len(); row++ {
				if _, changed := view.Delta[view.Base.IDs[row]]; changed {
					continue
				}
				if err := emit(view.Base.Record(uint32(row))); err != nil {
					return err
				}
			}
			for _, change := range view.Delta {
				if !change.Deleted && change.ID != view.Base.IDs[0] {
					if err := emit(change.Record); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return err
		}
	}
	return nil
}

// detach closes mapped files before removing only this root's private shard.
func (d *directoryIndex) detach(rootID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if shard := d.shards[rootID]; shard != nil {
		if err := shard.store.Close(); err != nil {
			return err
		}
		delete(d.shards, rootID)
	}
	return os.RemoveAll(d.shardDirectory(rootID))
}

func (d *directoryIndex) close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	var err error
	for _, shard := range d.shards {
		err = errors.Join(err, shard.store.Close())
	}
	d.shards = nil
	return err
}

// directoryEntryID identifies directory entries rather than inodes, preserving
// independent hard links in configured-root scans and deterministic replay IDs.
func directoryEntryID(path string) uint64 {
	digest := sha256.Sum256([]byte(normalizePath(path)))
	return binary.LittleEndian.Uint64(digest[:8])
}

func directoryEntryRecord(root RootRecord, entry EntryRecord) (column.Record, error) {
	if entry.RootID != root.ID || !pathWithinScope(root.Path, entry.Path) {
		return column.Record{}, fmt.Errorf("entry %q is outside root %q", entry.Path, root.Path)
	}
	return observedRecord(directoryEntryID(entry.Path), directoryEntryID(filepath.Dir(entry.Path)), filepath.Base(entry.Path), entry.IsDir, entry.Size, entry.Mtime), nil
}

// observedRecord is the single conversion from filesystem facts to a column
// row, shared by path-based entries and the scan streams' pre-resolved rows.
func observedRecord(id, parentID uint64, name string, isDir bool, size, modified int64) column.Record {
	flags := byte(column.SizeKnown | column.ModifiedKnown)
	if isDir {
		flags |= column.Directory
		// Directory st_size is filesystem bookkeeping, and bulk enumeration does
		// not report it. Storing zero keeps bulk-listed children and Lstat-ed
		// scope roots identical so reconciles do not rewrite unchanged folders.
		size = 0
	}
	return column.Record{ID: id, ParentID: parentID, Name: name, Flags: flags, Size: size, Modified: modified}
}

// observedInfoRecord converts a listed child whose parent ID is already known.
func observedInfoRecord(id, parentID uint64, name string, info os.FileInfo) column.Record {
	return observedRecord(id, parentID, name, info.IsDir(), info.Size(), info.ModTime().UnixMilli())
}

// shardDirectory isolates root storage from source-supplied path components.
func (d *directoryIndex) shardDirectory(rootID string) string {
	hash := sha256.Sum256([]byte(rootID))
	return filepath.Join(d.dir, hex.EncodeToString(hash[:]))
}
