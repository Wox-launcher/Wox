package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// collectANDCandidateIDs intersects conditions in SQLite before limiting results.
// Limiting a common term such as .txt first can discard every matching rare name.
func (p *SQLiteSearchProvider) collectANDCandidateIDs(ctx context.Context, plan *queryPlan, limit int) ([]int64, error) {
	statement, args := buildANDCandidateSQL(plan, limit, true)
	ids, err := p.queryIDs(ctx, statement, args...)
	if !isMissingFTSContentRowError(err) {
		return ids, err
	}
	p.scheduleFTSRepair(ctx, "AND search", err)
	statement, args = buildANDCandidateSQL(plan, limit, false)
	return p.queryIDs(ctx, statement, args...)
}

// buildANDCandidateSQL reuses the name, directory and pinyin indexes for each
// token. Short tokens use contains recall within AND queries, where other
// conditions narrow the results; standalone short-query prefix rules stay intact.
func buildANDCandidateSQL(plan *queryPlan, limit int, useFTS bool) (string, []any) {
	var clauses []string
	var args []any
	for _, term := range plan.andTerms {
		token := term.plan
		if token.extensionOnly {
			clauses = append(clauses, "SELECT entry_id FROM entries WHERE extension = ? AND is_dir = 0")
			args = append(args, token.extension)
			continue
		}
		pathTerm := token.rawLower
		if token.pathLike {
			pathTerm = token.pathQuery
		}
		var alternatives []string
		if useFTS && utf8Len(token.rawLower) >= 3 {
			alternatives = append(alternatives, "SELECT rowid AS entry_id FROM entries_name_fts WHERE normalized_name LIKE ?")
			args = append(args, "%"+token.rawLower+"%")
			alternatives = append(alternatives, `
				SELECT e.entry_id FROM entries_path_fts f
				INNER JOIN entries d ON d.entry_id = f.rowid
				INNER JOIN entries e ON e.path = d.path OR (e.path >= d.path || ? AND e.path < d.path || ? || char(1114111))
				WHERE f.normalized_path LIKE ? AND d.is_dir = 1
			`)
			args = append(args, string(filepath.Separator), string(filepath.Separator), "%"+pathTerm+"%")
		} else {
			alternatives = append(alternatives, `SELECT entry_id FROM entries WHERE normalized_path LIKE ? ESCAPE '\'`)
			args = append(args, "%"+escapeLikePattern(pathTerm)+"%")
		}
		if token.asciiLettersDigits {
			alternatives = append(alternatives, "SELECT entry_id FROM entries WHERE name_key >= ? AND name_key < ?")
			args = append(args, token.rawLettersDigits, nextPrefixUpperBound(token.rawLettersDigits))
			if token.usePinyin && !token.pathLike {
				if useFTS && len(token.rawLettersDigits) >= 3 {
					alternatives = append(alternatives, "SELECT rowid AS entry_id FROM entries_pinyin_full_fts WHERE pinyin_full LIKE ?",
						"SELECT rowid AS entry_id FROM entries_initials_fts WHERE entries_initials_fts MATCH ?")
					args = append(args, "%"+token.rawLettersDigits+"%", token.rawLettersDigits+"*")
				} else {
					alternatives = append(alternatives, "SELECT entry_id FROM entries WHERE pinyin_full LIKE ? OR pinyin_initials LIKE ?")
					args = append(args, "%"+token.rawLettersDigits+"%", token.rawLettersDigits+"%")
				}
			}
		}
		clauses = append(clauses, "SELECT entry_id FROM ("+strings.Join(alternatives, " UNION ")+")")
	}
	for i := range plan.exactPhrases {
		clauses = append(clauses, `SELECT entry_id FROM entries WHERE normalized_name LIKE ? ESCAPE '\' OR normalized_path LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLikePattern(plan.exactNamePhrases[i])+"%", "%"+escapeLikePattern(plan.exactPathPhrases[i])+"%")
	}
	args = append(args, limit)
	return fmt.Sprintf("SELECT entry_id FROM (%s) ORDER BY entry_id LIMIT ?", strings.Join(clauses, " INTERSECT ")), args
}
