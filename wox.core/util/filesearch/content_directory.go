package filesearch

import (
	"context"
	"fmt"
	"strings"
	"wox/util/filesearch/column"
)

// contentDirectory supplies one authoritative directory view without exposing
// filename storage ownership to extraction or incremental content indexing.
type contentDirectory interface {
	ListContentIndexCandidates(context.Context, ContentCandidateCursor, map[string]bool, int) ([]ContentIndexCandidate, error)
	ListEntryPathsUnderScope(context.Context, string, string) ([]string, error)
}

type sharedContentDirectory struct {
	configured, native *directoryIndex
	nativeRoots        []string
}

// contentDirectoryLocked captures immutable source ownership for one hook or
// crawl. Store read leases handle shutdown without reacquiring the engine lock
// from a content worker that the engine may currently be joining.
func (e *Engine) contentDirectoryLocked() sharedContentDirectory {
	view := sharedContentDirectory{configured: e.db.directory, nativeRoots: nativeRootPaths(e.nativeIndex)}
	if e.nativeIndex != nil {
		view.native = e.nativeIndex.directoryView()
	}
	return view
}

// nativeRootPaths borrows the source's root list while the engine holds its
// lifecycle read lock. Callers use these paths to avoid duplicate configured facts.
func nativeRootPaths(index nativeNameIndex) []string {
	if index == nil {
		return nil
	}
	directory := index.directoryView()
	directory.mu.RLock()
	defer directory.mu.RUnlock()
	roots := make([]string, 0, len(directory.shards))
	for _, shard := range directory.shards {
		roots = append(roots, shard.root.Path)
	}
	return roots
}

func coveredByNativeRoot(roots []string, path string) bool {
	for _, root := range roots {
		if pathWithinScope(root, path) {
			return true
		}
	}
	return false
}

// ListContentIndexCandidates merges source namespaces with stable keyset pages.
// Each source supplies read leases; returned candidates contain owned paths.
func (d sharedContentDirectory) ListContentIndexCandidates(ctx context.Context, after ContentCandidateCursor, extensions map[string]bool, limit int) ([]ContentIndexCandidate, error) {
	if d.configured == nil {
		return nil, fmt.Errorf("filesearch directory unavailable")
	}
	if limit <= 0 {
		return nil, nil
	}
	if err := d.requireNativeReady(); err != nil {
		return nil, err
	}
	type source struct {
		name      string
		directory *directoryIndex
	}
	sources := []source{{"configured", d.configured}}
	if d.native != nil {
		sources = append(sources, source{"native", d.native})
	}
	afterSource, afterRoot, _ := strings.Cut(after.RootID, "\x00")
	result := make([]ContentIndexCandidate, 0, limit)
	for _, source := range sources {
		if source.name < afterSource {
			continue
		}
		cursor := ContentCandidateCursor{}
		if source.name == afterSource {
			cursor = ContentCandidateCursor{RootID: afterRoot, ID: after.ID}
		}
		for len(result) < limit {
			candidates, err := source.directory.contentCandidates(ctx, cursor, extensions, min(512, limit-len(result)))
			if err != nil {
				return nil, err
			}
			if len(candidates) == 0 {
				break
			}
			for _, candidate := range candidates {
				cursor = candidate.Cursor
				if source.name == "configured" && coveredByNativeRoot(d.nativeRoots, candidate.Path) {
					continue
				}
				candidate.Cursor.RootID = source.name + "\x00" + candidate.Cursor.RootID
				result = append(result, candidate)
			}
		}
		if len(result) == limit {
			break
		}
	}
	return result, nil
}

// ListEntryPathsUnderScope resolves content scopes across native/configured
// ownership, which need not share the content settings' root identifiers.
func (d sharedContentDirectory) ListEntryPathsUnderScope(ctx context.Context, _ string, scope string) ([]string, error) {
	if d.configured == nil {
		return nil, fmt.Errorf("filesearch directory unavailable")
	}
	if err := d.requireNativeReady(); err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	seen := make(map[string]bool)
	visit := func(directory *directoryIndex, configured bool) error {
		return directory.visit(ctx, "", func(entry EntryRecord) error {
			if entry.IsDir || !pathWithinScope(scope, entry.Path) || configured && coveredByNativeRoot(d.nativeRoots, entry.Path) {
				return nil
			}
			if !seen[entry.Path] {
				paths = append(paths, entry.Path)
				seen[entry.Path] = true
			}
			return nil
		})
	}
	if err := visit(d.configured, true); err != nil {
		return nil, err
	}
	if d.native != nil {
		if err := visit(d.native, false); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// requireNativeReady prevents incomplete startup or a closed source from
// looking like an empty directory during destructive stale-content cleanup.
func (d sharedContentDirectory) requireNativeReady() error {
	if d.native == nil {
		return nil
	}
	d.native.mu.RLock()
	defer d.native.mu.RUnlock()
	if d.native.closed {
		return fmt.Errorf("native directory closed")
	}
	for _, shard := range d.native.shards {
		if err := shard.store.Read(func(column.View) error { return nil }); err != nil {
			return err
		}
	}
	return nil
}
