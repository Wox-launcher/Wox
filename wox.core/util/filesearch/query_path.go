package filesearch

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"wox/util/filesearch/column"
)

// queryDirectory lives only while visiting one directory. Files share its path
// scores instead of reconstructing and rescoring the same parent for every row.
type queryDirectory struct {
	path    string
	key     string
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
	if d.key != "" {
		result := scorePreparedDirectory(d.key, plan)
		d.matches = append(d.matches, directoryQueryMatch{plan: plan, fuzzyScore: result})
		return result
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

// columnPathFrame lends child postings for unanchored wildcard/phrase queries.
// Only the active ancestry owns directory scores and path buffers.
type columnPathFrame struct {
	children  []uint32
	changes   []uint64
	pathLen   int
	directory *queryDirectory
}

// searchColumnPaths uses the directory index when a mandatory path clause can
// reject candidates. Unanchored phrases and wildcards keep exhaustive traversal.
func searchColumnPaths(ctx context.Context, view column.View, query SearchQuery, limit int) ([]SearchResult, error) {
	if text := requiredPathPattern(query); text != "" {
		return searchIndexedColumnPaths(ctx, view, query, text, limit)
	}
	base := view.Base
	results := &columnResultHeap{limit: limit}
	root := base.Record(0)
	root.Name = filepath.Base(base.Root)
	if columnExtensionEligible(query, root) {
		if matched, score := scoreDocAgainstQuery(query, docRecord{Path: base.Root, IsDir: true}); matched {
			addColumnPathResult(results, root, score, base.Root)
		}
	}
	path := []byte(base.Root)
	stack := []columnPathFrame{{children: base.Children(0), changes: view.Query.Children[root.ID], pathLen: len(path)}}
	for visited := 0; len(stack) > 0; visited++ {
		if visited%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		frame := &stack[len(stack)-1]
		path = path[:frame.pathLen]
		row := column.NoRow
		var record column.Record
		if len(frame.children) > 0 {
			row = frame.children[0]
			frame.children = frame.children[1:]
			if _, changed := view.Delta[base.IDs[row]]; changed {
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
			if !columnExtensionEligible(query, record) {
				continue
			}
			if frame.directory == nil {
				frame.directory = &queryDirectory{path: string(path), key: normalizePathQuery(string(path))}
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
		directoryPath := ""
		if view.DirectoryUnchanged(row) {
			directoryPath = base.DirectoryPath(base.RowDirectory[row])
		} else {
			directoryPath = string(path)
		}
		if columnExtensionEligible(query, record) {
			if matched, score := scoreDocAgainstQuery(query, docRecord{Path: directoryPath, IsDir: true}); matched {
				addColumnPathResult(results, record, score, directoryPath)
			}
		}
		var children []uint32
		if row != column.NoRow {
			children = base.Children(row)
		}
		stack = append(stack, columnPathFrame{children: children, changes: view.Query.Children[record.ID], pathLen: len(path)})
	}
	return sortAndLimitResults(results.items, limit), ctx.Err()
}
