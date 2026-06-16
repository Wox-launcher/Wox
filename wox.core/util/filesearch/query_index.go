package filesearch

import (
	"path/filepath"
	"strings"

	"wox/util"
)

// queryPlan is the shared query normalization contract for the single SQLite
// search provider. The old in-memory query index kept many provider-specific
// recall structures in this file; keeping only the plan and scorer here makes
// the runtime flow easier to follow: SQLite recalls candidates, then this code
// reranks them with the same wildcard, path, extension, and pinyin semantics.
type queryPlan struct {
	raw                   string
	rawLower              string
	rawLettersDigits      string
	pathLike              bool
	asciiLettersDigits    bool
	extension             string
	extensionOnly         bool
	pathQuery             string
	pathSegments          []string
	wildcardLiterals      []string
	nameTerm              string
	usePinyin             bool
	perClauseLimit        int
	postIntersectionLimit int
	preRerankLimit        int
	shortQueryLength      int
}

type docRecord struct {
	Path           string
	IsDir          bool
	PinyinFull     string
	PinyinInitials string
}

const (
	defaultPerClauseLimit        = 20000
	defaultPostIntersectionLimit = 10000
	defaultPreRerankLimit        = 4000
)

func buildQueryPlan(query SearchQuery) *queryPlan {
	raw := normalizeQuery(query.Raw)
	if raw == "" {
		return nil
	}

	rawLower := normalizeIndexText(raw)
	lettersDigits := keepLettersAndDigits(rawLower)
	pathLike := strings.ContainsAny(raw, `/\`)
	pathQuery := normalizePathQuery(raw)
	pathSegments := splitDirectorySegments(pathQuery)

	nameTerm := rawLower
	wildcardLiterals := buildWildcardLiterals(raw)
	if query.wildcard != nil && len(wildcardLiterals) > 0 {
		nameTerm = longestString(wildcardLiterals)
	}

	extension := extractQueryExtension(raw)
	extensionOnly := extension != "" && !pathLike && strings.TrimSpace(strings.ReplaceAll(rawLower, "*", "")) == "."+extension

	plan := &queryPlan{
		raw:                raw,
		rawLower:           rawLower,
		rawLettersDigits:   lettersDigits,
		pathLike:           pathLike,
		asciiLettersDigits: isASCIIAlphaNumeric(lettersDigits) && lettersDigits != "",
		extension:          extension,
		extensionOnly:      extensionOnly,
		pathQuery:          pathQuery,
		pathSegments:       pathSegments,
		wildcardLiterals:   wildcardLiterals,
		nameTerm:           nameTerm,
		// Pinyin recall used to be unconditional inside filesearch, bypassing
		// Wox's global UsePinYin setting. Keeping the flag in the plan makes every
		// recall and rerank path use one normalized decision for this query.
		usePinyin:             !query.DisablePinyin,
		perClauseLimit:        defaultPerClauseLimit,
		postIntersectionLimit: defaultPostIntersectionLimit,
		preRerankLimit:        defaultPreRerankLimit,
		shortQueryLength:      utf8Len(rawLower),
	}

	if plan.shortQueryLength <= 2 {
		plan.preRerankLimit = min(defaultPreRerankLimit, 2000)
	}

	if plan.extensionOnly {
		plan.preRerankLimit = 0
	}

	return plan
}

func scoreDocAgainstQuery(query SearchQuery, record docRecord) (bool, int64) {
	if query.Raw == "" {
		return false, 0
	}

	if query.wildcard != nil {
		return query.wildcard.match(record.name(), record.Path)
	}

	plan := query.plan
	if plan == nil {
		return false, 0
	}

	var (
		matched   bool
		bestScore int64
	)

	updateBest := func(ok bool, score int64) {
		if !ok {
			return
		}
		if !matched || score > bestScore {
			matched = true
			bestScore = score
		}
	}

	nameScore := maybeScoreFuzzy(record.name(), plan.raw, plan.usePinyin)
	updateBest(nameScore.matched, nameScore.score+4000)

	pathTarget := record.Path
	if !record.IsDir {
		pathTarget = record.parentPath()
	}
	pathQuery := plan.raw
	if plan.pathLike {
		pathTarget = record.directoryPath()
		pathQuery = plan.pathQuery
	}
	pathMatched, pathScore := scorePathMatch(pathTarget, pathQuery)
	updateBest(pathMatched, pathScore+1500)

	if plan.usePinyin && !plan.pathLike && plan.rawLettersDigits != "" {
		fullScore := maybeScoreFuzzy(record.PinyinFull, plan.rawLettersDigits, false)
		updateBest(fullScore.matched, fullScore.score+2500)

		initialsScore := maybeScoreFuzzy(record.PinyinInitials, plan.rawLettersDigits, false)
		updateBest(initialsScore.matched, initialsScore.score+2500)
	}

	if !matched && plan.extensionOnly && normalizeExtension(filepath.Ext(record.name())) == plan.extension {
		return true, 500
	}

	if matched && plan.extension != "" && normalizeExtension(filepath.Ext(record.name())) == plan.extension {
		bestScore += 500
	}

	return matched, bestScore
}

type fuzzyScore struct {
	matched bool
	score   int64
}

func maybeScoreFuzzy(term string, query string, usePinyin bool) fuzzyScore {
	query = strings.TrimSpace(query)
	term = strings.TrimSpace(term)
	if query == "" || term == "" {
		return fuzzyScore{}
	}

	ok, score := util.IsStringMatchScore(term, query, usePinyin)
	return fuzzyScore{matched: ok, score: score}
}

func (record docRecord) directoryPath() string {
	if record.IsDir {
		return record.pathKey()
	}
	return normalizeIndexText(normalizePath(record.parentPath()))
}

func (record docRecord) name() string {
	return filepath.Base(record.Path)
}

func (record docRecord) parentPath() string {
	return filepath.Dir(record.Path)
}

func (record docRecord) pathKey() string {
	return normalizeIndexText(normalizePath(record.Path))
}

func normalizeIndexText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(value))
}

func normalizeEntryPathKey(entry EntryRecord) string {
	return normalizeIndexText(entry.NormalizedPath)
}

func shouldDropRedundantPinyinPayload(normalizedName string, pinyinFull string, pinyinInitials string) bool {
	if normalizedName == "" || pinyinFull == "" || pinyinInitials == "" {
		return false
	}
	if keepLettersAndDigits(normalizedName) != normalizedName {
		return false
	}
	return pinyinFull == normalizedName && pinyinInitials == normalizedName
}

func normalizeExtension(ext string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	ext = strings.TrimPrefix(ext, ".")
	return ext
}

func uniqueNgrams(value string, size int) []string {
	value = strings.TrimSpace(value)
	if value == "" || size <= 0 {
		return nil
	}

	runes := []rune(value)
	if len(runes) < size {
		return nil
	}

	seen := map[string]struct{}{}
	grams := make([]string, 0, len(runes)-size+1)
	for i := 0; i+size <= len(runes); i++ {
		gram := string(runes[i : i+size])
		if _, ok := seen[gram]; ok {
			continue
		}
		seen[gram] = struct{}{}
		grams = append(grams, gram)
	}
	return grams
}

func splitDirectorySegments(path string) []string {
	path = normalizePathQuery(path)
	if path == "" {
		return nil
	}

	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		segment = strings.TrimSpace(segment)
		if segment == "" || strings.HasSuffix(segment, ":") {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
}

func normalizePathQuery(value string) string {
	value = normalizeIndexText(value)
	value = strings.ReplaceAll(value, `\`, "/")
	return strings.Trim(value, "/")
}

func keepLettersAndDigits(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func buildWildcardLiterals(raw string) []string {
	if !strings.Contains(raw, "*") {
		return nil
	}

	parts := strings.Split(raw, "*")
	literals := make([]string, 0, len(parts))
	for _, part := range parts {
		part = keepLettersAndDigits(normalizeIndexText(part))
		if part == "" {
			continue
		}
		literals = append(literals, part)
	}
	return literals
}

func extractQueryExtension(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, ".") && !strings.ContainsAny(raw[1:], `/\*`) {
		return normalizeExtension(raw)
	}

	if strings.Contains(raw, "*") {
		index := strings.LastIndex(raw, ".")
		if index >= 0 && index < len(raw)-1 && !strings.ContainsAny(raw[index+1:], `/*\`) {
			return normalizeExtension(raw[index:])
		}
	}

	return ""
}

func longestString(values []string) string {
	longest := ""
	for _, value := range values {
		if len(value) > len(longest) {
			longest = value
		}
	}
	return longest
}

func isASCIIAlphaNumeric(value string) bool {
	if value == "" {
		return false
	}

	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			continue
		}
		return false
	}
	return true
}

func utf8Len(value string) int {
	return len([]rune(value))
}
