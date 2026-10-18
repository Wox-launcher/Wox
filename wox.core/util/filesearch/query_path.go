package filesearch

import (
	"context"
	"math/bits"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

// queryDirectory lives only while visiting one directory. Files share its path
// scores instead of reconstructing and rescoring the same parent for every row.
type queryDirectory struct {
	path    string
	matches []directoryQueryMatch
}

type directoryQueryMatch struct {
	plan *queryPlan
	fuzzyScore
}

// match preserves the exhaustive scorer's directory normalization and score.
func (d *queryDirectory) match(plan *queryPlan) fuzzyScore {
	for _, match := range d.matches {
		if match.plan == plan {
			return match.fuzzyScore
		}
	}
	target, query := d.path, plan.raw
	if plan.pathLike {
		target, query = normalizeIndexText(normalizePath(target)), plan.pathQuery
	}
	matched, score := scorePathMatch(target, query)
	result := fuzzyScore{matched: matched, score: score}
	d.matches = append(d.matches, directoryQueryMatch{plan: plan, fuzzyScore: result})
	return result
}

// requiredPathPattern selects a mandatory path clause whose separators cannot
// match a basename. Subsequence progress is only a rejection filter: final
// eligibility and ranking still use the shared scorer, including fuzzy gaps.
func requiredPathPattern(query SearchQuery) string {
	if query.wildcard != nil {
		return ""
	}
	plan := query.plan
	if len(plan.andTerms) > 0 {
		longest := ""
		for _, term := range plan.andTerms {
			if candidate := requiredPathPattern(term); len(candidate) > len(longest) {
				longest = candidate
			}
		}
		return longest
	}
	if strings.Contains(plan.raw, "/") || runtime.GOOS == "windows" && strings.Contains(plan.raw, `\`) {
		return plan.pathQuery
	}
	return ""
}

// columnPathFrame borrows child postings under the store lease. The stack and
// path buffer scale with directory depth, not the number of indexed paths.
type columnPathFrame struct {
	children  []uint32
	changes   []uint64
	pathLen   int
	progress  int
	directory *queryDirectory
}

type columnPathSeed struct {
	record   column.Record
	row      uint32
	path     string
	progress int
}

// pathPatternComponent follows path-query separator normalization. Backslashes
// cannot occur in Windows entry names, so that platform needs no replacement.
func pathPatternComponent(name string) string {
	if runtime.GOOS == "windows" {
		return name
	}
	return strings.ReplaceAll(name, `\`, "/")
}

type columnPathState struct {
	row           uint32
	before, after int
}

// columnPathStates memoizes subsequence progress, not full paths. The fixed-size
// table lets many name postings share their ancestor work without retaining a
// volume-sized cache; a collision simply causes that ancestry to be recomputed.
type columnPathStates struct {
	view    column.View
	pattern *fuzzymatch.PreparedPattern
	root    int
	entries [8192]columnPathState
}

// state follows the live parent relation and publishes progress from root to
// leaf. Base rows use scalar parent references; only changed edges need ID lookup.
func (c *columnPathStates) state(row uint32) (columnPathState, bool) {
	base := c.view.Base
	var local [32]struct {
		row  uint32
		name string
	}
	chain := local[:0]
	result := columnPathState{after: c.root}
	for row != 0 {
		if row == column.NoRow {
			return columnPathState{}, false
		}
		entry := c.entries[row%uint32(len(c.entries))]
		if entry.row == row+1 {
			result = entry
			break
		}
		parent, name := base.Parents[row], base.Name(base.NameIDs[row])
		if change, changed := c.view.Delta[base.IDs[row]]; changed {
			if change.Deleted || change.Flags&column.Directory == 0 {
				return columnPathState{}, false
			}
			for _, ancestor := range chain {
				if ancestor.row == row {
					return columnPathState{}, false
				}
			}
			name = change.Name
			var ok bool
			parent, ok = base.Row(change.ParentID)
			if !ok {
				path, exists := c.view.Path(change.ParentID)
				if !exists {
					return columnPathState{}, false
				}
				result.after = c.pattern.AdvanceSubsequence(normalizePathQuery(path), 0)
				parent = 0
			}
		}
		chain = append(chain, struct {
			row  uint32
			name string
		}{row: row, name: name})
		row = parent
	}
	for i := len(chain) - 1; i >= 0; i-- {
		ancestor := chain[i]
		progress := c.pattern.AdvanceSubsequence("/", result.after)
		progress = c.pattern.AdvanceSubsequence(pathPatternComponent(ancestor.name), progress)
		result = columnPathState{row: ancestor.row + 1, before: result.after, after: progress}
		c.entries[ancestor.row%uint32(len(c.entries))] = result
	}
	return result, true
}

// columnPathSeeds uses existing name bitmaps and postings to jump to directories
// that consume a selected character of the last path component. Only the first
// such directory on each ancestry chain becomes a seed, so subtrees never overlap.
func columnPathSeeds(ctx context.Context, view column.View, text string, pattern *fuzzymatch.PreparedPattern) ([]columnPathSeed, error) {
	base := view.Base
	rootProgress := pattern.AdvanceSubsequence(normalizePathQuery(base.Root), 0)
	root := []columnPathSeed{{record: base.Record(0), row: 0, path: base.Root, progress: rootProgress}}
	words := (base.Names() + 63) / 64
	var anchor []uint64
	position, bestCount := 0, int(^uint(0)>>1)
	runeIndex := 0
	lastSeparator := strings.LastIndexByte(text, '/')
	for byteIndex, r := range text {
		index := runeIndex
		runeIndex++
		if byteIndex <= lastSeparator {
			continue
		}
		candidate := base.NonASCIINameBitmap
		if r < 128 {
			mask, _ := column.CharacterMask(string(r))
			class := bits.TrailingZeros64(mask)
			candidate = base.NameBitmaps[class*words : (class+1)*words]
		} else {
			// A diacritic query can match an ASCII basename after normalization.
			for c, present := range fuzzymatch.PossibleASCIICharacters(string(r), false) {
				if present {
					mask, _ := column.CharacterMask(string(rune(c)))
					class := bits.TrailingZeros64(mask)
					candidate = base.NameBitmaps[class*words : (class+1)*words]
					break
				}
			}
		}
		count := 0
		for _, word := range candidate {
			count += bits.OnesCount64(word)
		}
		if count <= bestCount {
			anchor, position, bestCount = candidate, index, count
		}
	}
	if anchor == nil || rootProgress > position {
		return root, nil
	}
	plan := columnPathSeedPlan{view: view, pattern: pattern, anchor: anchor, position: position, rootProgress: rootProgress}
	workers := min(runtime.GOMAXPROCS(0), max(1, base.Names()/16384), 8)
	batches := make([]columnPathSeedBatch, workers)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			batches[worker] = plan.scan(ctx, worker, workers)
		}(worker)
	}
	wait.Wait()
	var seeds []columnPathSeed
	for _, batch := range batches {
		if batch.err != nil {
			return nil, batch.err
		}
		if batch.broad || len(seeds)+len(batch.seeds) > 4096 {
			return root, nil
		}
		seeds = append(seeds, batch.seeds...)
	}
	return seeds, nil
}

type columnPathSeedPlan struct {
	view         column.View
	pattern      *fuzzymatch.PreparedPattern
	anchor       []uint64
	position     int
	rootProgress int
}

type columnPathSeedBatch struct {
	seeds []columnPathSeed
	err   error
	broad bool
}

// scan partitions bitmap words just like filename retrieval. Each worker owns
// its ancestry cache and seed slice; mapped columns remain under one read lease.
func (p columnPathSeedPlan) scan(ctx context.Context, worker, workers int) columnPathSeedBatch {
	view, pattern, position := p.view, p.pattern, p.position
	base := view.Base
	var seeds []columnPathSeed
	states := &columnPathStates{view: view, pattern: pattern, root: p.rootProgress}
	consider := func(record column.Record, row uint32) {
		progress := p.rootProgress
		if row == column.NoRow {
			parent, ok := view.Path(record.ParentID)
			if !ok {
				return
			}
			progress = pattern.AdvanceSubsequence(normalizePathQuery(parent), 0)
		} else {
			state, ok := states.state(row)
			if !ok {
				return
			}
			if state.before > position || state.after <= position {
				return
			}
			progress = state.after
		}
		if row == column.NoRow {
			if progress > position {
				return
			}
			progress = pattern.AdvanceSubsequence("/", progress)
			progress = pattern.AdvanceSubsequence(pathPatternComponent(record.Name), progress)
			if progress <= position {
				return
			}
		}
		path, ok := view.Path(record.ID)
		if ok {
			seeds = append(seeds, columnPathSeed{record: record, row: row, path: path, progress: progress})
		}
	}
	for wordIndex := worker; wordIndex < len(p.anchor); wordIndex += workers {
		if ctx.Err() != nil {
			return columnPathSeedBatch{err: ctx.Err()}
		}
		word := p.anchor[wordIndex]
		for word != 0 {
			nameID := wordIndex*64 + bits.TrailingZeros64(word)
			word &= word - 1
			if nameID >= base.Names() {
				continue
			}
			kinds := base.NameFlags[nameID] & (column.NameHasDirectory | column.NameHasFile)
			if kinds != 0 && kinds&column.NameHasDirectory == 0 {
				continue
			}
			for i, row := range base.NameRows[base.NameStarts[nameID]:base.NameStarts[nameID+1]] {
				if i%256 == 0 && ctx.Err() != nil {
					return columnPathSeedBatch{err: ctx.Err()}
				}
				if base.Flags[row]&column.Directory == 0 {
					continue
				}
				if _, changed := view.Delta[base.IDs[row]]; !changed {
					consider(base.Record(row), row)
				}
				if len(seeds) > 4096 {
					return columnPathSeedBatch{broad: true}
				}
			}
		}
	}
	if worker == 0 {
		for _, change := range view.Delta {
			if !change.Deleted && change.Flags&column.Directory != 0 && change.ID != base.IDs[0] {
				row, ok := base.Row(change.ID)
				if !ok {
					row = column.NoRow
				}
				consider(change.Record, row)
				if len(seeds) > 4096 {
					return columnPathSeedBatch{broad: true}
				}
			}
		}
	}
	return columnPathSeedBatch{seeds: seeds}
}

// searchColumnPaths carries a cheap match state through the directory index.
// Files in impossible directories never construct paths or enter the scorer.
// The live overlay supplies moved/new edges; replaced base edges are skipped.
func searchColumnPaths(ctx context.Context, view column.View, query SearchQuery, limit int) ([]SearchResult, error) {
	base := view.Base
	results := &columnResultHeap{limit: limit}
	patternText := requiredPathPattern(query)
	pattern := fuzzymatch.PreparePattern(patternText)
	patternLen := utf8.RuneCountInString(patternText)
	seeds, err := columnPathSeeds(ctx, view, patternText, pattern)
	if err != nil {
		return nil, err
	}
	changes := make(map[uint64][]uint64)
	if len(seeds) > 0 {
		for _, change := range view.Delta {
			if !change.Deleted && change.ID != base.IDs[0] {
				changes[change.ParentID] = append(changes[change.ParentID], change.ID)
			}
		}
	}
	root := base.Record(0)
	root.Name = filepath.Base(base.Root)
	if columnExtensionEligible(query, root) {
		if matched, score := scoreDocAgainstQuery(query, docRecord{Path: base.Root, IsDir: true}); matched {
			addColumnPathResult(results, root, score, base.Root)
		}
	}
	// A simple path clause cannot match a basename. Its descendants are bounded
	// by the best possible substring/fuzzy directory score, so broad prefixes
	// such as C:\ do not enumerate a whole volume once the top K is established.
	bounded := limit > 0 && query.wildcard == nil && len(query.plan.andTerms) == 0 && len(query.plan.exactPhrases) == 0 && query.plan.patternASCII && patternText != ""
	pathQueryLength := utf8.RuneCountInString(query.plan.pathQuery)
	fuzzyUpper := int64(0)
	if bounded {
		fuzzyUpper = fuzzymatch.ASCIIScoreUpperBoundHint(fuzzymatch.PreparePattern(query.plan.pathQuery), pathQueryLength, ^uint64(0), ^uint64(0))
	}
	skipChildren := func(path string) bool {
		if !bounded || results.Len() < limit {
			return false
		}
		length := utf8.RuneCountInString(normalizePathQuery(path))
		upper := max(int64(8000+pathQueryLength*100-length), fuzzyUpper+2000) + 1500
		if query.plan.extension != "" {
			upper += 500
		}
		return upper < results.items[0].Score
	}
	var path []byte
	var stack []columnPathFrame
	visited := 0
	for len(stack) > 0 || len(seeds) > 0 {
		if len(stack) == 0 {
			seed := seeds[len(seeds)-1]
			seeds = seeds[:len(seeds)-1]
			path = append(path[:0], seed.path...)
			if seed.row != 0 && seed.progress == patternLen && columnExtensionEligible(query, seed.record) {
				if matched, score := scoreDocAgainstQuery(query, docRecord{Path: seed.path, IsDir: true}); matched {
					addColumnPathResult(results, seed.record, score, seed.path)
				}
			}
			if skipChildren(seed.path) {
				continue
			}
			var children []uint32
			if seed.row != column.NoRow {
				children = base.Children(seed.row)
			}
			stack = append(stack, columnPathFrame{children: children, changes: changes[seed.record.ID], pathLen: len(path), progress: seed.progress})
		}
		if visited%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		visited++
		frame := &stack[len(stack)-1]
		path = path[:frame.pathLen]
		var record column.Record
		row := column.NoRow
		if len(frame.children) > 0 {
			row = frame.children[0]
			frame.children = frame.children[1:]
			if _, changed := view.Delta[base.IDs[row]]; changed {
				continue
			}
			if base.Flags[row]&column.Directory == 0 && frame.progress != patternLen {
				continue
			}
			record = base.Record(row)
		} else if len(frame.changes) > 0 {
			record = view.Delta[frame.changes[0]].Record
			frame.changes = frame.changes[1:]
			if found, ok := base.Row(record.ID); ok {
				row = found
			}
		} else {
			stack = stack[:len(stack)-1]
			continue
		}
		if record.Flags&column.Directory == 0 {
			if frame.progress != patternLen || !columnExtensionEligible(query, record) {
				continue
			}
			if frame.directory == nil {
				frame.directory = &queryDirectory{path: string(path)}
			}
			doc := docRecord{nameText: record.Name, directory: frame.directory}
			if matched, score := scoreDocAgainstQuery(query, doc); matched && results.competitive(score, false, record.Name) {
				addColumnPathResult(results, record, score, doc.fullPath())
			}
			continue
		}
		if len(path) > 0 && path[len(path)-1] != filepath.Separator {
			path = append(path, filepath.Separator)
		}
		path = append(path, record.Name...)
		progress := pattern.AdvanceSubsequence("/", frame.progress)
		progress = pattern.AdvanceSubsequence(pathPatternComponent(record.Name), progress)
		var directory *queryDirectory
		if progress == patternLen {
			directory = &queryDirectory{path: string(path)}
			if columnExtensionEligible(query, record) {
				if matched, score := scoreDocAgainstQuery(query, docRecord{Path: directory.path, IsDir: true}); matched {
					addColumnPathResult(results, record, score, directory.path)
				}
			}
			if skipChildren(directory.path) {
				continue
			}
		}
		var children []uint32
		if row != column.NoRow {
			children = base.Children(row)
		}
		stack = append(stack, columnPathFrame{children: children, changes: changes[record.ID], pathLen: len(path), progress: progress, directory: directory})
	}
	return sortAndLimitResults(results.items, limit), ctx.Err()
}
