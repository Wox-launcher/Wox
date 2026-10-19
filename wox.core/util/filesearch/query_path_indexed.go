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

// scorePreparedDirectory preserves substring and fuzzy scores while borrowing
// the normalized directory key and compiled pattern instead of allocating both.
func scorePreparedDirectory(key string, plan *queryPlan) fuzzyScore {
	if key == "" || plan.pathQuery == "" {
		return fuzzyScore{}
	}
	if strings.Contains(key, plan.pathQuery) {
		return fuzzyScore{true, int64(8000 + utf8.RuneCountInString(plan.pathQuery)*100 - utf8.RuneCountInString(key))}
	}
	ascii := true
	for i := range len(key) {
		if key[i] >= 128 {
			ascii = false
			break
		}
	}
	result := fuzzymatch.FuzzyMatchPattern(key, plan.pathPattern, ascii, false)
	return fuzzyScore{result.IsMatch, result.Score + 2000}
}

// searchIndexedColumnPaths intersects persisted directory filters, then scores
// only eligible directories and their direct files. Every directory is checked
// independently so fuzzy matches may still span any number of path components.
func searchIndexedColumnPaths(ctx context.Context, view column.View, query SearchQuery, text string, limit int) ([]SearchResult, error) {
	view = view.PrepareQueryView()
	base := view.Base
	pattern := fuzzymatch.PreparePattern(text)
	length := utf8.RuneCountInString(text)
	queryLength := utf8.RuneCountInString(query.plan.pathQuery)
	fuzzyUpper := fuzzymatch.ASCIIScoreUpperBoundHint(query.plan.pathPattern, queryLength, ^uint64(0), ^uint64(0)) + 2000
	mask := column.PathCharacterMask(text)
	pairs := column.PathPairClasses(text)
	workers := min(runtime.GOMAXPROCS(0), max(1, len(base.DirectoryRows)/8192), 8)
	batches := make([][]SearchResult, workers)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			results := &columnResultHeap{limit: limit}
			words := (len(base.DirectoryRows) + 63) / 64
			filters := make([][]uint64, 0, bits.OnesCount64(mask)+len(pairs))
			for remaining := mask; remaining != 0; remaining &= remaining - 1 {
				class := bits.TrailingZeros64(remaining)
				filters = append(filters, base.DirectoryBitmaps[class*words:(class+1)*words])
			}
			for _, pair := range pairs {
				filters = append(filters, base.DirectoryPairBitmaps[pair*words:(pair+1)*words])
			}
			directory := &queryDirectory{}
			simple := query.wildcard == nil && len(query.plan.andTerms) == 0 && len(query.plan.exactPhrases) == 0
			consider := func(record column.Record, row uint32, path, key string, runes int) {
				matched := false
				if runes == len(key) {
					matched = pattern.MatchesNormalizedASCII(key)
				} else {
					matched = pattern.AdvanceSubsequence(key, 0) == length
				}
				if key == "" || !matched {
					return
				}
				if simple && limit > 0 && results.Len() >= limit {
					// Both substring and fuzzy scores are bounded independently. A
					// short prefix must not force a whole-volume file enumeration.
					upper := max(int64(8000+queryLength*100-runes), fuzzyUpper) + 1500
					if !query.plan.patternASCII {
						upper = 1 << 60
					}
					if query.plan.extension != "" {
						upper += 500
					}
					if upper < results.items[0].Score {
						return
					}
				}
				if row != column.NoRow {
					record = base.Record(row)
					if change, exists := view.Delta[record.ID]; exists {
						record = change.Record
					}
				}
				directory.path, directory.key = path, key
				directory.matches = directory.matches[:0]

				if row == 0 {
					record.Name = filepath.Base(base.Root)
				}
				if columnExtensionEligible(query, record) {
					doc := docRecord{Path: path, IsDir: true}
					if len(query.plan.exactPhrases) == 0 {
						doc.pathKeyText = key
					}
					if matched, score := scoreDocAgainstQuery(query, doc); matched {
						addColumnPathResult(results, record, score, path)
					}
				}

				var shared fuzzyScore
				if simple {
					shared = scorePreparedDirectory(key, query.plan)
				}
				file := func(child column.Record) {
					if child.Flags&column.Directory != 0 || !columnExtensionEligible(query, child) {
						return
					}
					matched, score := shared.matched, shared.score+1500
					if simple {
						if matched && query.plan.extension != "" && normalizeExtension(filepath.Ext(child.Name)) == query.plan.extension {
							score += 500
						}
					} else {
						matched, score = scoreDocAgainstQuery(query, docRecord{nameText: child.Name, directory: directory})
					}
					if matched && results.competitive(score, false, child.Name) {
						addColumnPathResult(results, child, score, filepath.Join(path, child.Name))
					}
				}
				if row != column.NoRow {
					for i, child := range base.Children(row) {
						if i%256 == 0 && ctx.Err() != nil {
							return
						}
						if _, changed := view.Delta[base.IDs[child]]; !changed {
							file(base.Record(child))
						}
					}
				}
				for _, id := range view.Query.Children[record.ID] {
					file(view.Delta[id].Record)
				}
			}
			for word := worker; word < words; word += workers {
				if ctx.Err() != nil {
					return
				}
				candidates := ^uint64(0)
				if len(view.Query.StaleDirectories) > 0 {
					candidates &^= view.Query.StaleDirectories[word]
				}
				for _, filter := range filters {
					candidates &= filter[word]
					if candidates == 0 {
						break
					}
				}
				for candidates != 0 {
					id := word*64 + bits.TrailingZeros64(candidates)
					candidates &= candidates - 1
					if id >= len(base.DirectoryRows) {
						continue
					}
					row := base.DirectoryRows[id]
					consider(column.Record{}, row, base.DirectoryPath(uint32(id)), base.DirectoryKey(uint32(id)), int(base.DirectoryRuneCounts[id]))
				}
			}
			// Changed ancestry cannot use old path filters. Only this small part
			// resolves live paths; new overlay directories share the same scorer.
			if view.Query.PathsCached {
				for id := worker; id < len(view.Query.LiveDirectories); id += workers {
					if ctx.Err() != nil {
						return
					}
					directory := view.Query.LiveDirectories[id]
					consider(directory.Record, directory.Row, directory.Path, directory.Key, directory.Runes)
				}
			} else {
				for word := worker; word < len(view.Query.StaleDirectories); word += workers {
					if ctx.Err() != nil {
						return
					}
					stale := view.Query.StaleDirectories[word]
					for stale != 0 {
						id := word*64 + bits.TrailingZeros64(stale)
						stale &= stale - 1
						row := base.DirectoryRows[id]
						path, ok := view.Path(base.IDs[row])
						if !ok {
							continue
						}
						record, ok := view.Lookup(base.IDs[row])
						if !ok || record.Flags&column.Directory == 0 {
							continue
						}
						key := normalizePathQuery(path)
						consider(record, row, path, key, utf8.RuneCountInString(key))
					}
				}
				if worker == 0 {
					for _, change := range view.Delta {
						if change.Deleted || change.Flags&column.Directory == 0 {
							continue
						}
						if row, exists := base.Row(change.ID); exists && base.Flags[row]&column.Directory != 0 {
							continue
						}
						path, ok := view.Path(change.ID)
						if ok {
							key := normalizePathQuery(path)
							consider(change.Record, column.NoRow, path, key, utf8.RuneCountInString(key))
						}
					}
				}
			}
			batches[worker] = results.items
		}(worker)
	}
	wait.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := &columnResultHeap{limit: limit}
	for _, batch := range batches {
		for _, result := range batch {
			results.add(result)
		}
	}
	return sortAndLimitResults(results.items, limit), nil
}
