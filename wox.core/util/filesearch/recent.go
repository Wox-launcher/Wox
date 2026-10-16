package filesearch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"wox/util/filesearch/column"
)

// RecentModified returns bounded, deduplicated directory-index results. Sources
// without modification metadata are omitted; accept filters before the limit.
func (e *Engine) RecentModified(ctx context.Context, limit int, accept func(SearchResult) bool) ([]SearchResult, error) {
	if e == nil || limit <= 0 {
		return nil, nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.db == nil {
		return nil, fmt.Errorf("filesearch engine closed")
	}
	d := e.db.directory
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, fmt.Errorf("directory index is closed")
	}
	results := make([]SearchResult, 0, limit+1)
	for _, shard := range d.shards {
		err := shard.store.Read(func(view column.View) error {
			consider := func(record column.Record) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if record.Flags&column.Structural != 0 || record.Modified <= 0 {
					return nil
				}
				// Only resolve paths for competitive rows, avoiding a whole-index path allocation.
				if len(results) == limit && record.Modified < results[len(results)-1].Mtime {
					return nil
				}
				path, ok := view.Path(record.ID)
				if !ok {
					return nil
				}
				item := SearchResult{Path: path, Name: filepath.Base(path), ParentPath: filepath.Dir(path), IsDir: record.Flags&column.Directory != 0, Mtime: record.Modified, Size: record.Size}
				for i, existing := range results {
					if existing.Path == path {
						if existing.Mtime >= item.Mtime {
							return nil
						}
						results = append(results[:i], results[i+1:]...)
						break
					}
				}
				if accept != nil && !accept(item) {
					return nil
				}
				position := sort.Search(len(results), func(i int) bool {
					return results[i].Mtime < item.Mtime || (results[i].Mtime == item.Mtime && results[i].Path >= item.Path)
				})
				if position >= limit {
					return nil
				}
				results = append(results, SearchResult{})
				copy(results[position+1:], results[position:])
				results[position] = item
				if len(results) > limit {
					results = results[:limit]
				}
				return nil
			}
			for row := 0; row < view.Base.Len(); row++ {
				if _, changed := view.Delta[view.Base.IDs[row]]; changed {
					continue
				}
				if err := consider(view.Base.Record(uint32(row))); err != nil {
					return err
				}
			}
			for _, change := range view.Delta {
				if !change.Deleted {
					if err := consider(change.Record); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return nil, err
		}
	}
	for i := range results {
		results[i].Score = int64(len(results) - i)
	}
	return results, nil
}
