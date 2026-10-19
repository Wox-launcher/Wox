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
	view = view.PrepareQueryView()
	base := view.Base
	results := &columnResultHeap{limit: limit}
	if query.plan.pathLike {
		return searchColumnPaths(ctx, view, query, limit)
	}
	// A query-local directory cache avoids allocating each repeated parent path.
	// It is not retained by the index or shared with subsequent queries.
	parents := make(map[uint64]string)
	resolve := func(record column.Record) (string, bool) {
		parent, found := parents[record.ParentID]
		if !found {
			var ok bool
			if row, exists := base.Row(record.ParentID); exists && view.DirectoryUnchanged(row) {
				parent = base.DirectoryPath(base.RowDirectory[row])
				ok = parent != ""
			} else {
				parent, ok = view.Path(record.ParentID)
			}
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
		// An unpenalized miss cannot become competitive after the recycle-bin
		// penalty, so path resolution stays limited to names that can still win.
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
		addColumnPathResult(results, record, score, path)
	}
	root := base.Record(0)
	if root.Flags&column.Structural == 0 && columnExtensionEligible(query, root) {
		if matched, score := scoreDocAgainstQuery(query, docRecord{Path: base.Root, IsDir: true}); matched {
			root.Name = filepath.Base(base.Root)
			add(root, score, base.Root)
		}
	}
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
	for _, live := range view.Query.Names {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if requiresNonASCII && !live.NonASCII || live.Mask&mask != mask {
			continue
		}
		if live.BaseName != column.NoRow && len(base.NamePairBitmaps) != 0 {
			words := (base.Names() + 63) / 64
			eligible := true
			for _, pair := range query.plan.namePairs {
				if base.NamePairBitmaps[pair*words+int(live.BaseName)/64]&(1<<(live.BaseName%64)) == 0 {
					eligible = false
					break
				}
			}
			if !eligible {
				continue
			}
		}
		change := view.Delta[live.ID]
		if !columnExtensionEligible(query, change.Record) {
			continue
		}
		minimum := int64(0)
		if limit > 0 && results.Len() >= limit {
			minimum = results.items[0].Score
			if query.wildcard == nil {
				minimum -= 1000000
			}
		}
		if matched, score := scoreColumnName(query, change.Name, !live.NonASCII, minimum, nil); matched {
			if query.wildcard == nil {
				score = 1000000 + min(max(score, 0), 999999)
			}
			add(change.Record, score, "")
		}
	}

	return sortAndLimitResults(results.items, limit), ctx.Err()
}

// addColumnPathResult materializes only a competitive result and applies the
// same recycle-bin penalty regardless of the candidate retrieval strategy.
func addColumnPathResult(results *columnResultHeap, record column.Record, score int64, path string) {
	isDir := record.Flags&column.Directory != 0
	if pathInWindowsRecycleBin(path) {
		score -= recycleBinRankPenalty
	}
	if !results.competitive(score, isDir, record.Name) {
		return
	}
	path = strings.Clone(path)
	result := SearchResult{Path: path, Name: strings.Clone(record.Name), ParentPath: filepath.Dir(path), IsDir: isDir, Score: score}
	if record.Flags&column.SizeKnown != 0 {
		result.Size = record.Size
	}
	if record.Flags&column.ModifiedKnown != 0 {
		result.Mtime = record.Modified
	}
	results.add(result)
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
	if len(name) != len(worst.Name) {
		return len(name) < len(worst.Name)
	}
	return name <= worst.Name
}

func (h *columnResultHeap) add(result SearchResult) {
	if h.limit <= 0 || h.Len() < h.limit {
		h.items = appendColumnHeap(h.items, result, h.limit)
		heap.Fix(h, len(h.items)-1)
	} else if compareSearchResults(result, h.items[0]) < 0 {
		h.items[0] = result
		heap.Fix(h, 0)
	}
}

type columnNameHit struct {
	id          uint32
	name        string
	score       int64
	directory   bool
	recycleOnly bool
}

// rankScore drops recycle-only names below every ordinary match. The raw score
// stays intact so result emission can penalize individual paths once.
func (h columnNameHit) rankScore() int64 {
	if h.recycleOnly {
		return h.score - recycleBinRankPenalty
	}
	return h.score
}

// columnNameHeap prunes whole posting lists by their final score/name order.
// Path ordering is resolved only within the final retained groups.
type columnNameHeap struct {
	items []columnNameHit
	limit int
}

func compareColumnNames(a, b columnNameHit) int {
	return compareSearchResults(SearchResult{Score: a.rankScore(), IsDir: a.directory, Name: a.name}, SearchResult{Score: b.rankScore(), IsDir: b.directory, Name: b.name})
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
		h.items = appendColumnHeap(h.items, hit, h.limit)
		heap.Fix(h, len(h.items)-1)
	} else if compareColumnNames(hit, h.items[0]) < 0 {
		h.items[0] = hit
		heap.Fix(h, 0)
	}
}

// appendColumnHeap keeps sparse worker heaps small, then reserves a bounded
// top-K buffer once a query proves broad enough to need it. This avoids both
// repeated growth copies and allocating K entries for a worker with one hit.
func appendColumnHeap[T any](items []T, value T, limit int) []T {
	if len(items) == cap(items) && limit > 0 {
		capacity := min(limit, 8)
		if len(items) > 0 {
			capacity = min(limit, max(cap(items)*2, 128))
		}
		grown := make([]T, len(items), capacity)
		copy(grown, items)
		items = grown
	}
	return append(items, value)
}

// scanColumnNames scores disjoint name ranges with private top-K heaps. A local
// loser cannot enter the global top K, so merging needs no recall cutoff.
func scanColumnNames(ctx context.Context, view column.View, query SearchQuery, limit, start, end, stride int, minimumScore int64) *columnNameHeap {
	base := view.Base
	mask, requiresNonASCII := columnNameFilter(query)
	groups := &columnNameHeap{limit: limit}
	reachableParents := make(map[uint64]bool)
	recycleParents := make(map[uint64]bool)
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
		if candidates != 0 && len(base.NamePairBitmaps) != 0 && len(query.plan.namePairs) != 0 {
			for _, pair := range query.plan.namePairs {
				candidates &= base.NamePairBitmaps[pair*words+word]
				if candidates == 0 {
					break
				}
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
				// Use the penalized score. A heap full of short recycle-bin names
				// must not raise the floor enough to skip ordinary filenames.
				floor := groups.items[0].rankScore()
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
			// A name that loses at its full score cannot win from a recycle-bin row.
			// Rows are still scanned when a normal copy of that name may exist.
			fileCandidate := kinds&column.NameHasFile != 0 && groups.competitive(columnNameHit{name: name, score: score})
			dirCandidate := kinds&column.NameHasDirectory != 0 && groups.competitive(columnNameHit{name: name, score: score, directory: true})
			if !fileCandidate && !dirCandidate {
				continue
			}
			fileSeen, dirSeen := false, false
			fileNormal, dirNormal := false, false
			for _, row := range base.NameRows[base.NameStarts[nameID]:base.NameStarts[nameID+1]] {
				if (!fileCandidate || fileNormal) && (!dirCandidate || dirNormal) {
					break
				}
				if _, changed := view.Delta[base.IDs[row]]; changed {
					continue
				}
				record := base.Record(row)
				if record.Flags&column.Structural != 0 {
					continue
				}
				directory := record.Flags&column.Directory != 0
				if directory {
					if !dirCandidate || dirNormal {
						continue
					}
				} else if !fileCandidate || fileNormal {
					continue
				}
				if !columnExtensionEligible(query, record) {
					if directory {
						dirCandidate = false
					} else {
						fileCandidate = false
					}
					continue
				}
				parentRow := base.Parents[row]
				stable := view.DirectoryUnchanged(parentRow)
				reachable, checked := reachableParents[record.ParentID]
				if stable {
					reachable, checked = base.DirectoryPath(base.RowDirectory[parentRow]) != "", true
				}
				if !checked {
					reachable = view.Reachable(record.ParentID)
					reachableParents[record.ParentID] = reachable
				}
				if !reachable {
					continue
				}
				recycled := false
				if stable {
					recycled = pathInWindowsRecycleBin(base.DirectoryPath(base.RowDirectory[parentRow])) || parentRow == 0 && strings.EqualFold(record.Name, "$Recycle.Bin") && runtime.GOOS == "windows"
				} else {
					recycled = recordInWindowsRecycleBin(view, record, recycleParents)
				}
				if directory {
					dirSeen = true
					dirNormal = dirNormal || !recycled
				} else {
					fileSeen = true
					fileNormal = fileNormal || !recycled
				}
			}
			if fileSeen {
				hit := columnNameHit{id: uint32(nameID), name: name, score: score, recycleOnly: !fileNormal}
				if groups.competitive(hit) {
					groups.add(hit)
				}
			}
			if dirSeen {
				hit := columnNameHit{id: uint32(nameID), name: name, score: score, directory: true, recycleOnly: !dirNormal}
				if groups.competitive(hit) {
					groups.add(hit)
				}
			}
		}
	}
	return groups
}

// recycleBinRankPenalty sits above the filename score band (1_000_000 plus up to
// 999_999) so every ordinary match outranks a Recycle Bin path. The same gap
// also covers wildcard and explicit path scores, which are much smaller.
const recycleBinRankPenalty int64 = 2_000_000

// pathInWindowsRecycleBin reports paths whose first segment under a drive root
// is $Recycle.Bin. A same-named folder anywhere else stays a normal directory.
func pathInWindowsRecycleBin(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return false
	}
	relative := strings.TrimPrefix(clean, volume)
	relative = strings.TrimPrefix(relative, string(filepath.Separator))
	if relative == "" {
		return false
	}
	first, _, _ := strings.Cut(relative, string(filepath.Separator))
	return strings.EqualFold(first, "$Recycle.Bin")
}

// recordInWindowsRecycleBin is the posting-list form of pathInWindowsRecycleBin.
// Name ranking decides the top K before paths are built, so ancestry is checked
// from parent ids and remembered for the many files that share one SID folder.
func recordInWindowsRecycleBin(view column.View, record column.Record, cache map[uint64]bool) bool {
	if runtime.GOOS != "windows" || view.Base == nil {
		return false
	}
	if strings.EqualFold(record.Name, "$Recycle.Bin") && record.ParentID == view.Base.IDs[0] {
		return true
	}
	return directoryInWindowsRecycleBin(view, record.ParentID, cache)
}

// directoryInWindowsRecycleBin walks parent ids until the drive root. The cache
// stores the answer for every directory visited on the way.
func directoryInWindowsRecycleBin(view column.View, id uint64, cache map[uint64]bool) bool {
	if view.Base == nil {
		return false
	}
	var chain []uint64
	for steps := 0; ; steps++ {
		if steps > view.Base.Len()+len(view.Delta) {
			return false
		}
		if id == view.Base.IDs[0] {
			inside := pathInWindowsRecycleBin(view.Base.Root)
			for _, cached := range chain {
				cache[cached] = inside
			}
			return inside
		}
		if known, ok := cache[id]; ok {
			for _, cached := range chain {
				cache[cached] = known
			}
			return known
		}
		chain = append(chain, id)
		record, ok := view.Lookup(id)
		if !ok {
			for _, cached := range chain {
				cache[cached] = false
			}
			return false
		}
		if record.ParentID == view.Base.IDs[0] && strings.EqualFold(record.Name, "$Recycle.Bin") {
			for _, cached := range chain {
				cache[cached] = true
			}
			return true
		}
		id = record.ParentID
	}
}
