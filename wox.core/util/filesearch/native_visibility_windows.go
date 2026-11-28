//go:build windows

package filesearch

import (
	"bytes"
	"math/bits"
	"path/filepath"
	"slices"
	"sync/atomic"

	"wox/util/filesearch/column"
)

// The mapped facts remain authoritative for USN, content indexing and moves.
// Only search readers see this projection. Store leases serialize mutations
// between revisions and let queries on the same revision share all columns.
type nativeQueryVisibility struct {
	prepared          atomic.Pointer[nativeBaseVisibility]
	rules             FilenameIgnoreRules
	cacheDir          string
	source            *column.Snapshot
	base              column.Snapshot
	baseFlags         []byte
	visibleNames      []uint64
	prefixCounts      map[string]column.PrefixCounts
	frequentByInitial [128][]uint32
	frequentMatches   map[string][]column.ScoredName
	delta             map[uint64]column.Change
	overridden        []uint32
	movedPaths        []column.LiveDirectory
	pathsKnown        bool
}

// A generation under construction owns heap columns. Prepare its immutable
// mask before publication to avoid paging the same columns back in from disk.
type nativeBaseVisibility struct {
	generation      string
	flags           []byte
	names           []uint64
	frequentMatches map[string][]column.ScoredName
}

// PreparePublication computes visibility while the builder's columns are resident.
func (f *nativeQueryVisibility) PreparePublication(b *column.Snapshot, generation string) {
	mask := f.buildMask(b)
	mask.generation = generation
	f.saveCache(generation, mask)
	f.prepared.Store(mask)
}

func (f *nativeQueryVisibility) ignoresChild(root, parent, name string) bool {
	ignore, check := f.rules.ClassifyName(name)
	return ignore || check && f.rules.IgnorePath(root, filepath.Join(parent, name))
}

// ignoredDirectory checks each ancestor of a moved directory. A glob may
// match the directory itself without matching the full path of its children.
func (f *nativeQueryVisibility) ignoredDirectory(root, path string) bool {
	for path != root {
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		if f.ignoresChild(root, parent, filepath.Base(path)) {
			return true
		}
		path = parent
	}
	return false
}

// buildMask classifies interned names once. Directories are ordered by path
// length, so a child's parent already carries its inherited exclusion flag.
func (f *nativeQueryVisibility) buildMask(b *column.Snapshot) *nativeBaseVisibility {
	const (
		allowed byte = iota
		ignored
		needsPath
	)
	names := make([]byte, b.Names())
	for id := range names {
		drop, check := f.rules.ClassifyName(b.Name(uint32(id)))
		switch {
		case drop:
			names[id] = ignored
		case check:
			names[id] = needsPath
		}
	}
	flags := bytes.Clone(b.Flags)
	for dir, row := range b.DirectoryRows {
		parent := b.DirectoryPath(uint32(dir))
		if parent == "" {
			continue
		}
		hidden := row != 0 && flags[row]&column.Structural != 0
		for _, child := range b.Children(row) {
			name := b.NameIDs[child]
			if hidden || names[name] == ignored || names[name] == needsPath && f.rules.IgnorePath(b.Root, filepath.Join(parent, b.Name(name))) {
				flags[child] |= column.Structural
			}
		}
	}
	visibleNames := make([]uint64, (b.Names()+63)/64)
	for row, flag := range flags {
		if flag&column.Structural == 0 {
			id := b.NameIDs[row]
			visibleNames[id/64] |= 1 << (id % 64)
		}
	}
	return &nativeBaseVisibility{flags: flags, names: visibleNames, frequentMatches: frequentVisibleMatches(b, flags, visibleNames)}
}

// An existing index can acquire rules immediately, including an older index
// without a cache. Only a missing or invalid generation mask needs recomputing.
func (f *nativeQueryVisibility) prepareBase(v column.View) {
	mask := f.prepared.Swap(nil)
	if mask == nil || mask.generation != v.Generation {
		mask = f.loadCache(v)
		if mask == nil {
			mask = f.buildMask(v.Base)
			f.saveCache(v.Generation, mask)
		}
	}
	f.source, f.base = v.Base, *v.Base
	f.pathsKnown, f.movedPaths = false, nil
	f.baseFlags, f.visibleNames, f.frequentMatches, f.overridden = mask.flags, mask.names, mask.frequentMatches, nil
	f.base.Flags = mask.flags
	f.prepareFrequentNames(mask)
}

// An unrelated directory creation invalidates the raw overlay, but does not
// change visibility in previously moved base subtrees. Reuse the overlay's
// bounded path list to avoid walking those subtrees on each such event.
func (f *nativeQueryVisibility) refreshMovedRows(v column.View) {
	if v.Query.PathsCached {
		current := v.Query.LiveDirectories
		for i, dir := range current {
			if dir.Row == column.NoRow {
				current = current[:i]
				break
			}
		}
		if f.pathsKnown && slices.EqualFunc(current, f.movedPaths, func(a, b column.LiveDirectory) bool { return a.Row == b.Row && a.Path == b.Path }) {
			return
		}
		f.movedPaths = current
	} else {
		f.movedPaths = nil
	}
	f.pathsKnown = v.Query.PathsCached
	for _, row := range f.overridden {
		f.base.Flags[row] = f.baseFlags[row]
	}
	f.overridden = f.overridden[:0]
	for word, stale := range v.Query.StaleDirectories {
		for stale != 0 {
			dir := word*64 + bits.TrailingZeros64(stale)
			stale &= stale - 1
			row := v.Base.DirectoryRows[dir]
			path, ok := v.Path(v.Base.IDs[row])
			if !ok {
				continue
			}
			hidden := f.ignoredDirectory(v.Base.Root, path)
			for _, child := range v.Base.Children(row) {
				if _, changed := v.Delta[v.Base.IDs[child]]; changed {
					continue
				}
				flags := v.Base.Flags[child]
				if hidden || f.ignoresChild(v.Base.Root, path, v.Base.Name(v.Base.NameIDs[child])) {
					flags |= column.Structural
				}
				if flags == f.baseFlags[child] {
					continue
				}
				if &f.base.Flags[0] == &f.baseFlags[0] {
					f.base.Flags = bytes.Clone(f.baseFlags)
				}
				f.base.Flags[child] = flags
				f.overridden = append(f.overridden, child)
			}
		}
	}
}

// Project patches ordinary file changes by ID. Directory moves invalidate
// only old subtree paths, including descendants with no USN event of their own.
func (f *nativeQueryVisibility) Project(v column.View, pending []uint64, reset bool) column.View {
	if v.Base != f.source {
		f.prepareBase(v)
		reset = true
	}
	if reset {
		f.refreshMovedRows(v)
		f.delta = make(map[uint64]column.Change, len(v.Delta))
	}
	apply := func(change column.Change) {
		if !change.Deleted {
			parent, ok := v.Base.Row(change.ParentID)
			var path string
			hidden := false
			if ok && v.DirectoryUnchanged(parent) {
				path = v.Base.DirectoryPath(v.Base.RowDirectory[parent])
				hidden = parent != 0 && f.baseFlags[parent]&column.Structural != 0
			} else {
				path, _ = v.Path(change.ParentID)
				hidden = f.ignoredDirectory(v.Base.Root, path)
			}
			if hidden || f.ignoresChild(v.Base.Root, path, change.Name) {
				change.Flags |= column.Structural
			}
		}
		f.delta[change.ID] = change
	}
	if reset {
		for _, change := range v.Delta {
			apply(change)
		}
	} else {
		for _, id := range pending {
			apply(v.Delta[id])
		}
	}
	v.Base, v.Delta = &f.base, f.delta
	v.PruneStructuralDirectories = true
	if len(v.Delta) == 0 {
		v.PrefixCounts = f.prefixCounts
	}
	if len(f.overridden) == 0 {
		v.VisibleNames = f.visibleNames
		v.FrequentMatches = f.frequentMatches
		v.FrequentByInitial = &f.frequentByInitial
	}
	return v
}
