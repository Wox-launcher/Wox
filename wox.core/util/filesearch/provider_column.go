package filesearch

import (
	"container/heap"
	"context"
	"math/bits"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"wox/util/filesearch/column"
)

// searchColumnView shares the query grammar and final scorer with every source.
// Filename matches visit interned names first; paths are materialized only for
// competitive results or queries that actually need directory context.
func searchColumnView(ctx context.Context, view column.View, input SearchQuery, limit int) ([]SearchResult, error) {
	query := normalizeSearchQuery(input)
	if query.plan == nil || view.Base == nil {
		return nil, nil
	}
	base := view.Base
	results := &columnResultHeap{limit: limit}
	namePriority := !query.plan.pathLike
	// A query-local directory cache avoids allocating each repeated parent path.
	// It is not retained by the index or shared with subsequent queries.
	parents := make(map[uint64]string)
	resolve := func(record column.Record) (string, bool) {
		parent, found := parents[record.ParentID]
		if !found {
			var ok bool
			parent, ok = view.Path(record.ParentID)
			if !ok {
				return "", false
			}
			parents[record.ParentID] = parent
		}
		return filepath.Join(parent, record.Name), true
	}
	add := func(record column.Record, score int64, path string) {
		if record.Flags&column.Structural != 0 {
			return
		}
		isDir := record.Flags&column.Directory != 0
		if !results.competitive(score, isDir, record.Name) {
			return
		}
		if path == "" {
			var ok bool
			path, ok = resolve(record)
			if !ok {
				return
			}
		}
		result := SearchResult{Path: path, Name: strings.Clone(record.Name), ParentPath: filepath.Dir(path), IsDir: isDir, Score: score}
		if record.Flags&column.SizeKnown != 0 {
			result.Size = record.Size
		}
		if record.Flags&column.ModifiedKnown != 0 {
			result.Mtime = record.Modified
		}
		results.add(result)
	}
	root := base.Record(0)
	if root.Flags&column.Structural == 0 && columnExtensionEligible(query, root) {
		if matched, score := scoreDocAgainstQuery(query, docRecord{Path: base.Root, IsDir: true}); matched {
			root.Name = filepath.Base(base.Root)
			add(root, score, base.Root)
		}
	}
	if namePriority {
		groups := &columnNameHeap{limit: limit}
		minimumScore := exactColumnNameThreshold(ctx, view, query, limit)
		workers := min(runtime.GOMAXPROCS(0), max(1, base.Names()/16384), 8)
		partitions := make([]*columnNameHeap, workers)
		var wait sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wait.Add(1)
			go func(worker int) {
				defer wait.Done()
				partitions[worker] = scanColumnNames(ctx, view, query, limit, worker*64, base.Names(), workers, minimumScore)
			}(worker)
		}
		wait.Wait()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		for _, partition := range partitions {
			for _, hit := range partition.items {
				groups.add(hit)
			}
		}
		// Each retained group is known to contain a live row. The best K groups
		// therefore contain the global best K base rows, without a recall cutoff.
		sort.Slice(groups.items, func(i, j int) bool { return compareColumnNames(groups.items[i], groups.items[j]) < 0 })
		for _, hit := range groups.items {
			rows, err := selectColumnRows(ctx, view, hit, limit)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				add(base.Record(row), hit.score, "")
			}
			if limit > 0 && results.Len() >= limit {
				break
			}
		}

		mask, requiresNonASCII := columnNameFilter(query)
		for _, change := range view.Delta {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if change.Deleted || change.ID == base.IDs[0] || !columnExtensionEligible(query, change.Record) {
				continue
			}
			nameMask, nonASCII := column.CharacterMask(change.Name)
			if !nonASCII && (requiresNonASCII || nameMask&mask != mask) {
				continue
			}
			minimum := int64(0)
			if limit > 0 && results.Len() >= limit {
				minimum = results.items[0].Score
				if query.wildcard == nil {
					minimum -= 1000000
				}
			}
			if matched, score := scoreColumnName(query, change.Name, !nonASCII, minimum, nil); matched {
				if query.wildcard == nil {
					score = 1000000 + min(max(score, 0), 999999)
				}
				add(change.Record, score, "")
			}
		}

		return sortAndLimitResults(results.items, limit), ctx.Err()
	}
	// Only explicit path queries materialize paths for matching. Default name
	// queries return their actual hit count regardless of the requested limit.
	visit := func(record column.Record) {
		if !columnExtensionEligible(query, record) {
			return
		}
		path, ok := resolve(record)
		if !ok {
			return
		}
		if matched, score := scoreDocAgainstQuery(query, docRecord{Path: path, IsDir: record.Flags&column.Directory != 0}); matched {
			add(record, score, path)
		}
	}
	for row := 1; row < base.Len(); row++ {
		if row%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if _, changed := view.Delta[base.IDs[row]]; !changed {
			visit(base.Record(uint32(row)))
		}
	}
	for _, change := range view.Delta {
		if !change.Deleted && change.ID != base.IDs[0] {
			visit(change.Record)
		}
	}
	return sortAndLimitResults(results.items, limit), ctx.Err()
}

// columnExtensionEligible makes an extension-only clause a filter, including in
// AND queries; fuzzy matches in a basename must not admit a different extension.
func columnExtensionEligible(query SearchQuery, record column.Record) bool {
	if record.Flags&column.Structural != 0 {
		return false
	}
	for _, term := range query.plan.andTerms {
		if !columnExtensionEligible(term, record) {
			return false
		}
	}
	if len(query.plan.andTerms) == 0 && query.plan.extensionOnly {
		return record.Flags&column.Directory == 0 && normalizeExtension(filepath.Ext(record.Name)) == query.plan.extension
	}
	return true
}

// columnResultHeap retains the worst result at the root, bounding result memory
// even when a one-character query matches most of a volume.
type columnResultHeap struct {
	items []SearchResult
	limit int
}

func (h columnResultHeap) Len() int { return len(h.items) }
func (h columnResultHeap) Less(i, j int) bool {
	return compareSearchResults(h.items[i], h.items[j]) > 0
}
func (h columnResultHeap) Swap(i, j int)   { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *columnResultHeap) Push(value any) { h.items = append(h.items, value.(SearchResult)) }
func (h *columnResultHeap) Pop() any {
	last := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return last
}

// competitive avoids path construction when score, kind or filename already
// determines that a candidate cannot enter the retained results.
func (h *columnResultHeap) competitive(score int64, directory bool, name string) bool {
	if h.limit <= 0 || h.Len() < h.limit {
		return true
	}
	worst := h.items[0]
	if score != worst.Score {
		return score > worst.Score
	}
	if directory != worst.IsDir {
		return directory
	}
	return name <= worst.Name
}

func (h *columnResultHeap) add(result SearchResult) {
	if h.limit <= 0 || h.Len() < h.limit {
		heap.Push(h, result)
	} else if compareSearchResults(result, h.items[0]) < 0 {
		h.items[0] = result
		heap.Fix(h, 0)
	}
}

type columnNameHit struct {
	id        uint32
	name      string
	score     int64
	directory bool
}

// columnNameHeap prunes whole posting lists by their final score/name order.
// Path ordering is resolved only within the final retained groups.
type columnNameHeap struct {
	items []columnNameHit
	limit int
}

func compareColumnNames(a, b columnNameHit) int {
	return compareSearchResults(SearchResult{Score: a.score, IsDir: a.directory, Name: a.name}, SearchResult{Score: b.score, IsDir: b.directory, Name: b.name})
}

func (h columnNameHeap) Len() int           { return len(h.items) }
func (h columnNameHeap) Less(i, j int) bool { return compareColumnNames(h.items[i], h.items[j]) > 0 }
func (h columnNameHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *columnNameHeap) Push(value any)    { h.items = append(h.items, value.(columnNameHit)) }
func (h *columnNameHeap) Pop() any {
	last := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return last
}

func (h *columnNameHeap) competitive(hit columnNameHit) bool {
	return h.limit <= 0 || h.Len() < h.limit || compareColumnNames(hit, h.items[0]) < 0
}

func (h *columnNameHeap) add(hit columnNameHit) {
	if h.limit <= 0 || h.Len() < h.limit {
		heap.Push(h, hit)
	} else if compareColumnNames(hit, h.items[0]) < 0 {
		h.items[0] = hit
		heap.Fix(h, 0)
	}
}

// scanColumnNames scores disjoint name ranges with private top-K heaps. A local
// loser cannot enter the global top K, so merging needs no recall cutoff.
func scanColumnNames(ctx context.Context, view column.View, query SearchQuery, limit, start, end, stride int, minimumScore int64) *columnNameHeap {
	base := view.Base
	mask, requiresNonASCII := columnNameFilter(query)
	groups := &columnNameHeap{limit: limit}
	reachableParents := make(map[uint64]bool)
	words := (base.Names() + 63) / 64
	var columns [91][]uint64
	count := 0
	for remaining := mask; remaining != 0; remaining &= remaining - 1 {
		class := bits.TrailingZeros64(remaining)
		columns[count] = base.NameBitmaps[class*words : (class+1)*words]
		count++
	}
	for remaining := columnRepeatedLetters(query); remaining != 0; remaining &= remaining - 1 {
		class := bits.TrailingZeros32(remaining)
		columns[count] = base.NameRepeatBitmaps[class*words : (class+1)*words]
		count++
	}
	if requiresNonASCII {
		columns[count] = base.NonASCIINameBitmap
		count++
	}
	for word := start / 64; word < (end+63)/64; word += stride {
		if ctx.Err() != nil {
			return groups
		}
		candidates := ^uint64(0)
		for _, bitmap := range columns[:count] {
			candidates &= bitmap[word]
			if candidates == 0 {
				break
			}
		}
		if word == start/64 {
			candidates &= ^uint64(0) << uint(start%64)
		}
		if word == (end-1)/64 && end%64 != 0 {
			candidates &= (uint64(1) << uint(end%64)) - 1
		}
		for candidates != 0 {
			nameID := word*64 + bits.TrailingZeros64(candidates)
			candidates &= candidates - 1

			if nameID%256 == 0 && ctx.Err() != nil {
				return groups
			}
			flags := base.NameFlags[nameID]
			if requiresNonASCII && flags&column.NameNonASCII == 0 {
				continue
			}
			if (flags&column.NameNonASCII == 0 || flags&column.NameMaskComplete != 0) && base.NameMasks[nameID]&mask != mask {
				continue
			}
			minimum := minimumScore
			if limit > 0 && groups.Len() >= limit {
				floor := groups.items[0].score
				if query.wildcard == nil {
					floor -= 1000000
				}
				minimum = max(minimum, floor)
			}
			if flags&column.NameNonASCII == 0 && len(base.NameScoreHints) != 0 {
				upper := columnNameUpperBound(query, int(base.NameOffsets[nameID+1]-base.NameOffsets[nameID]), base.NameScoreHints[nameID], base.NameSecondScoreHints[nameID])
				if upper < minimum {
					continue
				}
			}
			name := base.Name(uint32(nameID))
			bounds := columnNameBounds{length: len(name), first: base.NameScoreHints[nameID], second: base.NameSecondScoreHints[nameID]}
			matched, score := scoreColumnName(query, name, flags&column.NameNonASCII == 0, minimum, &bounds)
			if !matched {
				continue
			}
			if query.wildcard == nil {
				score = 1000000 + min(max(score, 0), 999999)
			}
			kinds := base.NameFlags[nameID] & (column.NameHasDirectory | column.NameHasFile)
			if kinds == 0 {
				// Early development snapshots did not record kind summaries.
				kinds = column.NameHasDirectory | column.NameHasFile
			}
			for _, row := range base.NameRows[base.NameStarts[nameID]:base.NameStarts[nameID+1]] {
				if kinds == 0 {
					break
				}
				if _, changed := view.Delta[base.IDs[row]]; changed {
					continue
				}
				record := base.Record(row)
				if record.Flags&column.Structural != 0 {
					continue
				}
				kind := byte(column.NameHasFile)
				if record.Flags&column.Directory != 0 {
					kind = column.NameHasDirectory
				}
				if kinds&kind == 0 {
					continue
				}
				if !columnExtensionEligible(query, record) {
					kinds &^= kind
					continue
				}
				hit := columnNameHit{id: uint32(nameID), name: name, score: score, directory: kind == column.NameHasDirectory}
				if !groups.competitive(hit) {
					kinds &^= kind
					continue
				}
				reachable, checked := reachableParents[record.ParentID]
				if !checked {
					reachable = view.Reachable(record.ParentID)
					reachableParents[record.ParentID] = reachable
				}
				if reachable {
					groups.add(hit)
					kinds &^= kind
				}
				if kinds == 0 {
					break
				}
			}
		}
	}
	return groups
}
