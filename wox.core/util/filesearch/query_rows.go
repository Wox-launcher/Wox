package filesearch

import (
	"container/heap"
	"context"
	"path/filepath"
	"wox/util/filesearch/column"
)

// columnRowHeap compares borrowed path components and retains only the winning
// row IDs. Repeated names can have thousands of postings; materializing every
// full path merely to break a tie dominated common queries such as readme.
type columnRowHeap struct {
	view column.View
	rows []uint32
}

func (h columnRowHeap) Len() int { return len(h.rows) }
func (h columnRowHeap) Less(i, j int) bool {
	return compareColumnPaths(h.view, h.rows[i], h.rows[j]) > 0
}
func (h columnRowHeap) Swap(i, j int) { h.rows[i], h.rows[j] = h.rows[j], h.rows[i] }
func (h *columnRowHeap) Push(v any)   { h.rows = append(h.rows, v.(uint32)) }
func (h *columnRowHeap) Pop() any {
	i := len(h.rows) - 1
	v := h.rows[i]
	h.rows = h.rows[:i]
	return v
}

// selectColumnRows preserves path tie ordering before allocating result paths.
func selectColumnRows(ctx context.Context, view column.View, hit columnNameHit, limit int) ([]uint32, error) {
	h := columnRowHeap{view: view}
	base := view.Base
	// File edits and directory metadata updates do not disturb base path order.
	ordered := base.NameRowsPathSorted && (len(view.Delta) == 0 || view.Query != nil && len(view.Query.StaleDirectories) == 0)
	stableCount := 0
	parentStates := make(map[uint32]byte)
	var scratch [32]string
	for i, row := range base.NameRows[base.NameStarts[hit.id]:base.NameStarts[hit.id+1]] {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, changed := view.Delta[base.IDs[row]]; changed {
			continue
		}
		if base.Flags[row]&column.Structural != 0 || (base.Flags[row]&column.Directory != 0) != hit.directory {
			continue
		}
		stable := ordered
		if !ordered && base.NameRowsPathSorted {
			state := byte(1)
			if !view.DirectoryUnchanged(base.Parents[row]) {
				state = columnParentState(view, base.Parents[row], parentStates)
			}
			if state == 3 {
				continue
			}
			stable = state == 1
			if stable && limit > 0 && stableCount >= limit {
				continue
			}
		}
		if stable && view.DirectoryUnchanged(base.Parents[row]) {
			if base.DirectoryPath(base.RowDirectory[base.Parents[row]]) == "" {
				continue
			}
		} else if _, ok := columnPathParts(view, row, scratch[:0]); !ok {
			continue
		}
		if stable {
			stableCount++
		}
		if ordered {
			h.rows = append(h.rows, row)
			if limit > 0 && len(h.rows) >= limit {
				break
			}
			continue
		}
		if limit <= 0 || h.Len() < limit {
			h.rows = append(h.rows, row)
			heap.Fix(&h, len(h.rows)-1)
		} else if compareColumnPaths(view, row, h.rows[0]) < 0 {
			h.rows[0] = row
			heap.Fix(&h, 0)
		}
	}
	return h.rows, nil
}

// columnPathParts follows parent rows directly until an overlay changes ancestry.
// Components are borrowed under the caller's read lease, in leaf-to-root order.
func columnPathParts(view column.View, row uint32, parts []string) ([]string, bool) {
	base := view.Base
	id := base.IDs[row]
	for steps := 0; id != base.IDs[0]; steps++ {
		if steps > base.Len()+len(view.Delta) {
			return nil, false
		}
		if change, ok := view.Delta[id]; ok {
			if change.Deleted || steps > 0 && change.Flags&column.Directory == 0 {
				return nil, false
			}
			parts = append(parts, change.Name)
			id = change.ParentID
			row = column.NoRow
			continue
		}
		if row == column.NoRow {
			var ok bool
			row, ok = base.Row(id)
			if !ok {
				return nil, false
			}
		}
		if steps > 0 && base.Flags[row]&column.Directory == 0 {
			return nil, false
		}
		parts = append(parts, base.Name(base.NameIDs[row]))
		row = base.Parents[row]
		if row == column.NoRow {
			return nil, false
		}
		id = base.IDs[row]
	}
	return parts, true
}

// compareColumnPaths orders shorter joined paths first, then compares their
// bytes without joining. A component prefix is followed by a separator, whose
// ordering differs from an end-of-string (for example a/b versus a-b on Unix).
func compareColumnPaths(view column.View, left, right uint32) int {
	var leftLocal, rightLocal [32]string
	a, _ := columnPathParts(view, left, leftLocal[:0])
	b, _ := columnPathParts(view, right, rightLocal[:0])
	lenA, lenB := columnPathBytesLen(a), columnPathBytesLen(b)
	if lenA != lenB {
		if lenA < lenB {
			return -1
		}
		return 1
	}
	for i, j := len(a)-1, len(b)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		x, y := a[i], b[j]
		n := min(len(x), len(y))
		for k := 0; k < n; k++ {
			if x[k] < y[k] {
				return -1
			}
			if x[k] > y[k] {
				return 1
			}
		}
		if len(x) != len(y) {
			nextX, nextY := -1, -1
			if len(x) > n {
				nextX = int(x[n])
			} else if i > 0 {
				nextX = int(filepath.Separator)
			}
			if len(y) > n {
				nextY = int(y[n])
			} else if j > 0 {
				nextY = int(filepath.Separator)
			}
			if nextX < nextY {
				return -1
			}
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

func columnPathBytesLen(parts []string) int {
	if len(parts) == 0 {
		return 0
	}
	total := len(parts) - 1
	for _, part := range parts {
		total += len(part)
	}
	return total
}

// columnParentState memoizes whether a base path's ancestry survived unchanged.
// Sorted postings remain ordered among unaffected paths even when an unrelated
// directory changed; only affected paths need full tie-breaking against them.
func columnParentState(view column.View, row uint32, states map[uint32]byte) byte {
	var local [32]uint32
	chain := local[:0]
	state := byte(1)
	for row != 0 {
		if row == column.NoRow {
			state = 3
			break
		}
		if known := states[row]; known != 0 {
			state = known
			break
		}
		chain = append(chain, row)
		if change, changed := view.Delta[view.Base.IDs[row]]; changed {
			state = 2
			if change.Deleted {
				state = 3
			}
			break
		}
		row = view.Base.Parents[row]
	}
	for _, ancestor := range chain {
		states[ancestor] = state
	}
	return state
}
