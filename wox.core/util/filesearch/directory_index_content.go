package filesearch

import (
	"context"
	"errors"
	"path/filepath"
	"sort"

	"wox/util/filesearch/column"
)

// ContentCandidateCursor is a keyset over shard identity and stable entry ID.
// It survives row reordering during compaction and does not retain mapped data.
type ContentCandidateCursor struct {
	RootID string
	ID     uint64
}

// contentCandidates merges sorted immutable IDs with the bounded live overlay.
// A page releases its read lease before content extraction performs filesystem IO.
func (d *directoryIndex) contentCandidates(ctx context.Context, after ContentCandidateCursor, extensions map[string]bool, limit int) ([]ContentIndexCandidate, error) {
	if limit <= 0 || len(extensions) == 0 {
		return nil, nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		return nil, errors.New("directory index is closed")
	}
	rootIDs := make([]string, 0, len(d.shards))
	for id := range d.shards {
		if id >= after.RootID {
			rootIDs = append(rootIDs, id)
		}
	}
	sort.Strings(rootIDs)
	candidates := make([]ContentIndexCandidate, 0, limit)
	for _, rootID := range rootIDs {
		err := d.shards[rootID].store.Read(func(view column.View) error {
			minimum := uint64(0)
			if rootID == after.RootID {
				minimum = after.ID
			}
			base := view.Base
			position := sort.Search(len(base.IDRows), func(i int) bool { return base.IDs[base.IDRows[i]] > minimum })
			deltaIDs := make([]uint64, 0, len(view.Delta))
			for id, change := range view.Delta {
				if id > minimum && !change.Deleted {
					deltaIDs = append(deltaIDs, id)
				}
			}
			sort.Slice(deltaIDs, func(i, j int) bool { return deltaIDs[i] < deltaIDs[j] })
			deltaPosition := 0
			for (position < len(base.IDRows) || deltaPosition < len(deltaIDs)) && len(candidates) < limit {
				if err := ctx.Err(); err != nil {
					return err
				}
				var record column.Record
				if deltaPosition < len(deltaIDs) && (position == len(base.IDRows) || deltaIDs[deltaPosition] <= base.IDs[base.IDRows[position]]) {
					id := deltaIDs[deltaPosition]
					deltaPosition++
					record = view.Delta[id].Record
					if position < len(base.IDRows) && id == base.IDs[base.IDRows[position]] {
						position++
					}
				} else {
					row := base.IDRows[position]
					position++
					if _, changed := view.Delta[base.IDs[row]]; changed {
						continue
					}
					record = base.Record(row)
				}
				if record.Flags&column.Directory != 0 {
					continue
				}
				extension := normalizeExtension(filepath.Ext(record.Name))
				if !extensions[extension] {
					continue
				}
				path, reachable := view.Path(record.ID)
				if !reachable {
					continue
				}
				candidates = append(candidates, ContentIndexCandidate{Cursor: ContentCandidateCursor{RootID: rootID, ID: record.ID}, Path: path, Mtime: record.Modified, Size: record.Size, Extension: extension, MetadataKnown: record.Flags&(column.SizeKnown|column.ModifiedKnown) == column.SizeKnown|column.ModifiedKnown})
			}
			return nil
		})
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return nil, err
		}
		if len(candidates) >= limit {
			break
		}
	}
	return candidates, nil
}
