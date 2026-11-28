//go:build windows

package filesearch

import (
	"math/bits"
	"sort"
	"strings"

	"wox/util/filesearch/column"
)

// Each persisted score stores three 32-bit fields and two kind summaries.
const (
	frequentTermLimit   = 32
	frequentScoreBudget = (4 << 20) / 16
)

// When exclusions remove a large exact-name group, ordinary fuzzy search loses
// its exact-score floor. Prepare prefix-quality candidates for the most affected
// short terms. The ordinary scorer still decides all rankings and pinyin behavior.
func frequentVisibleMatches(b *column.Snapshot, flags []byte, visible []uint64) map[string][]column.ScoredName {
	type counts struct{ lost, kept int }
	frequency := map[string]counts{}
	for _, id := range b.FrequentNameIDs {
		name := b.Name(id)
		if len(name) < 3 || len(name) > 4 || b.NameFlags[id]&column.NameNonASCII != 0 {
			continue
		}
		name = strings.ToLower(name)
		if strings.Trim(name, "abcdefghijklmnopqrstuvwxyz") != "" {
			continue
		}
		c := frequency[name]
		for _, row := range b.NameRows[b.NameStarts[id]:b.NameStarts[id+1]] {
			if flags[row]&column.Structural == 0 {
				c.kept++
			} else {
				c.lost++
			}
		}
		frequency[name] = c
	}
	var terms []string
	for name, c := range frequency {
		if c.lost >= 16 && c.lost > c.kept {
			terms = append(terms, name)
		}
	}
	sort.Slice(terms, func(i, j int) bool {
		a, b := frequency[terms[i]].lost, frequency[terms[j]].lost
		if a == b {
			return terms[i] < terms[j]
		}
		return a > b
	})
	// Bound both build work and retained scores independently of volume size.
	matches := make(map[string][]column.ScoredName)
	remainingBudget := frequentScoreBudget
	for _, term := range terms[:min(len(terms), frequentTermLimit)] {
		query := normalizeSearchQuery(SearchQuery{Raw: term})
		plain := normalizeSearchQuery(SearchQuery{Raw: term, DisablePinyin: true})
		minimum := columnPrefixScore(len(term))
		var local [128][]uint64
		mask, needsUnicode := columnNameFilter(query)
		columns := columnNameCandidateBitmaps(b, query, mask, needsUnicode, local[:0])
		var candidates []column.ScoredName
		for word, remaining := range visible {
			for _, bitmap := range columns {
				remaining &= bitmap[word]
				if remaining == 0 {
					break
				}
			}
			for remaining != 0 {
				offset := bits.TrailingZeros64(remaining)
				remaining &= remaining - 1
				id := uint32(word*64 + offset)
				ascii := b.NameFlags[id]&column.NameNonASCII == 0
				name := b.Name(id)
				if ascii && columnNameUpperBound(query, len(name), b.NameScoreHints[id], b.NameSecondScoreHints[id]) < minimum {
					continue
				}
				ok, score := scoreColumnName(query, name, ascii, minimum, nil)
				if !ok || score < minimum {
					continue
				}
				without := score
				if !ascii {
					if found, value := scoreColumnName(plain, name, false, minimum, nil); found && value >= minimum {
						without = value
					} else {
						without = 0
					}
				}
				hit := column.ScoredName{ID: id, WithPinyin: int32(score), WithoutPinyin: int32(without)}
				// Summarize only the cached candidates so a heavily excluded group
				// never has to walk all its postings to prove which kinds survive.
				kinds := b.NameFlags[id] & (column.NameHasFile | column.NameHasDirectory)
				for _, row := range b.NameRows[b.NameStarts[id]:b.NameStarts[id+1]] {
					parent := b.Parents[row]
					if flags[row]&column.Structural != 0 || parent == column.NoRow {
						continue
					}
					path := b.DirectoryPath(b.RowDirectory[parent])
					if path == "" {
						continue
					}
					kind := byte(column.NameHasFile)
					if flags[row]&column.Directory != 0 {
						kind = column.NameHasDirectory
					}
					hit.Kinds |= kind
					if !pathInWindowsRecycleBin(path) && !(parent == 0 && strings.EqualFold(name, "$Recycle.Bin")) {
						hit.NormalKinds |= kind
						if hit.NormalKinds == kinds {
							break
						}
					}
				}
				candidates = append(candidates, hit)
				if len(candidates) > remainingBudget {
					return matches
				}
			}
		}
		matches[term] = candidates
		remainingBudget -= len(candidates)
	}
	return matches
}

// prepareFrequentNames narrows score-floor checks to visible ASCII postings.
func (f *nativeQueryVisibility) prepareFrequentNames(mask *nativeBaseVisibility) {
	f.frequentByInitial = [128][]uint32{}
	for _, id := range f.base.FrequentNameIDs {
		if mask.names[id/64]&(1<<(id%64)) == 0 || f.base.NameFlags[id]&column.NameNonASCII != 0 {
			continue
		}
		name := f.base.Name(id)
		if len(name) == 0 || name[0] >= 128 {
			continue
		}
		initial := name[0]
		if initial >= 'A' && initial <= 'Z' {
			initial += 'a' - 'A'
		}
		f.frequentByInitial[initial] = append(f.frequentByInitial[initial], id)
	}
	// Each reachable non-recycled name proves at least one competitive row.
	// These counts provide a safe floor even when most exact copies were hidden.
	f.prefixCounts = make(map[string]column.PrefixCounts, len(mask.frequentMatches))
	for term, candidates := range mask.frequentMatches {
		var counts column.PrefixCounts
		for _, hit := range candidates {
			if hit.NormalKinds == 0 {
				continue
			}
			if hit.WithPinyin != 0 {
				counts.WithPinyin++
			}
			if hit.WithoutPinyin != 0 {
				counts.WithoutPinyin++
			}
		}
		f.prefixCounts[term] = counts
	}
}
