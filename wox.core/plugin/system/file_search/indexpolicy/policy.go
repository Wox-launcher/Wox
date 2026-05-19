package indexpolicy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Policy owns the plugin-level filesystem ignore rules that sit above the
// policy-neutral filesearch engine.
type Policy struct {
	mu            sync.RWMutex
	patternsByDir map[string][]gitIgnorePattern
	ignoreRules   fileSearchIgnoreRules
	diagnostics   *Diagnostics
}

// TraversalContext is the ripgrep-style policy state carried by a directory
// walker. The old per-path callback rebuilt configured-rule candidates and the
// .gitignore ancestor chain for every entry; this context snapshots configured
// rules once and keeps the active .gitignore stack with the directory currently
// being read.
type TraversalContext struct {
	policy                    *Policy
	rootPath                  string
	policyRootPath            string
	matchRootPath             string
	dirPath                   string
	dirRelSlash               string
	hasDirRel                 bool
	dirSegmentsLower          []string
	ignoreRules               fileSearchIgnoreRules
	configuredAncestorIgnored bool
	gitIgnoreFrames           []traversalGitIgnoreFrame
	diagnostics               *Diagnostics
}

type traversalGitIgnoreFrame struct {
	dirRelSlash string
	patterns    []gitIgnorePattern
}

// Diagnostics accumulates policy costs for the opt-in real-index benchmark.
// The previous benchmark could report "scan is slow" without showing whether
// the cost came from configured globs, ancestor .gitignore checks, or uncached
// .gitignore reads, so these counters stay attached to the real plugin policy.
type Diagnostics struct {
	policyChecks                  atomic.Int64
	policyNanos                   atomic.Int64
	policyIgnored                 atomic.Int64
	configuredPatternChecks       atomic.Int64
	configuredPatternNanos        atomic.Int64
	configuredPatternIgnored      atomic.Int64
	gitIgnoreChecks               atomic.Int64
	gitIgnoreNanos                atomic.Int64
	gitIgnoreIgnored              atomic.Int64
	gitIgnoreAncestorDirectories  atomic.Int64
	gitIgnoreDirectoriesWithRules atomic.Int64
	gitIgnorePatternComparisons   atomic.Int64
	gitIgnorePatternLoads         atomic.Int64
	gitIgnorePatternsLoaded       atomic.Int64
	gitIgnorePatternLoadNanos     atomic.Int64
}

type DiagnosticsSnapshot struct {
	PolicyChecks                  int64 `json:"policy_checks"`
	PolicyMillis                  int64 `json:"policy_millis"`
	PolicyIgnored                 int64 `json:"policy_ignored"`
	ConfiguredPatternChecks       int64 `json:"configured_pattern_checks"`
	ConfiguredPatternMillis       int64 `json:"configured_pattern_millis"`
	ConfiguredPatternIgnored      int64 `json:"configured_pattern_ignored"`
	GitIgnoreChecks               int64 `json:"gitignore_checks"`
	GitIgnoreMillis               int64 `json:"gitignore_millis"`
	GitIgnoreIgnored              int64 `json:"gitignore_ignored"`
	GitIgnoreAncestorDirectories  int64 `json:"gitignore_ancestor_directories"`
	GitIgnoreDirectoriesWithRules int64 `json:"gitignore_directories_with_rules"`
	GitIgnorePatternComparisons   int64 `json:"gitignore_pattern_comparisons"`
	GitIgnorePatternLoads         int64 `json:"gitignore_pattern_loads"`
	GitIgnorePatternsLoaded       int64 `json:"gitignore_patterns_loaded"`
	GitIgnorePatternLoadMillis    int64 `json:"gitignore_pattern_load_millis"`
}

func NewDiagnostics() *Diagnostics {
	return &Diagnostics{}
}

func (p *Policy) SetDiagnostics(diagnostics *Diagnostics) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.diagnostics = diagnostics
	p.mu.Unlock()
}

func (p *Policy) ClearGitIgnoreCache() {
	if p == nil {
		return
	}
	p.mu.Lock()
	// Bug fix: file watcher events can report .gitignore edits after a directory
	// has already populated the cached pattern slice. Clearing the cache lets the
	// next change-signal policy check reload the current ignore file instead of
	// reusing stale ancestor rules.
	p.patternsByDir = map[string][]gitIgnorePattern{}
	p.mu.Unlock()
}

func (p *Policy) DiagnosticsSnapshot() DiagnosticsSnapshot {
	diagnostics := p.diagnosticsRef()
	if diagnostics == nil {
		return DiagnosticsSnapshot{}
	}
	return diagnostics.Snapshot()
}

func (p *Policy) diagnosticsRef() *Diagnostics {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	diagnostics := p.diagnostics
	p.mu.RUnlock()
	return diagnostics
}

func (d *Diagnostics) Snapshot() DiagnosticsSnapshot {
	if d == nil {
		return DiagnosticsSnapshot{}
	}
	return DiagnosticsSnapshot{
		PolicyChecks:                  d.policyChecks.Load(),
		PolicyMillis:                  diagnosticMillis(d.policyNanos.Load()),
		PolicyIgnored:                 d.policyIgnored.Load(),
		ConfiguredPatternChecks:       d.configuredPatternChecks.Load(),
		ConfiguredPatternMillis:       diagnosticMillis(d.configuredPatternNanos.Load()),
		ConfiguredPatternIgnored:      d.configuredPatternIgnored.Load(),
		GitIgnoreChecks:               d.gitIgnoreChecks.Load(),
		GitIgnoreMillis:               diagnosticMillis(d.gitIgnoreNanos.Load()),
		GitIgnoreIgnored:              d.gitIgnoreIgnored.Load(),
		GitIgnoreAncestorDirectories:  d.gitIgnoreAncestorDirectories.Load(),
		GitIgnoreDirectoriesWithRules: d.gitIgnoreDirectoriesWithRules.Load(),
		GitIgnorePatternComparisons:   d.gitIgnorePatternComparisons.Load(),
		GitIgnorePatternLoads:         d.gitIgnorePatternLoads.Load(),
		GitIgnorePatternsLoaded:       d.gitIgnorePatternsLoaded.Load(),
		GitIgnorePatternLoadMillis:    diagnosticMillis(d.gitIgnorePatternLoadNanos.Load()),
	}
}

func (d *Diagnostics) recordPolicyCheck(elapsed time.Duration, ignored bool) {
	d.policyChecks.Add(1)
	d.policyNanos.Add(elapsed.Nanoseconds())
	if ignored {
		d.policyIgnored.Add(1)
	}
}

func (d *Diagnostics) recordConfiguredPatternCheck(elapsed time.Duration, ignored bool) {
	d.configuredPatternChecks.Add(1)
	d.configuredPatternNanos.Add(elapsed.Nanoseconds())
	if ignored {
		d.configuredPatternIgnored.Add(1)
	}
}

func (d *Diagnostics) recordGitIgnoreCheck(elapsed time.Duration, ignored bool, ancestorDirectories int64, directoriesWithRules int64, patternComparisons int64) {
	d.gitIgnoreChecks.Add(1)
	d.gitIgnoreNanos.Add(elapsed.Nanoseconds())
	d.gitIgnoreAncestorDirectories.Add(ancestorDirectories)
	d.gitIgnoreDirectoriesWithRules.Add(directoriesWithRules)
	d.gitIgnorePatternComparisons.Add(patternComparisons)
	if ignored {
		d.gitIgnoreIgnored.Add(1)
	}
}

func (d *Diagnostics) recordGitIgnorePatternLoad(elapsed time.Duration, patternCount int) {
	d.gitIgnorePatternLoads.Add(1)
	d.gitIgnorePatternsLoaded.Add(int64(patternCount))
	d.gitIgnorePatternLoadNanos.Add(elapsed.Nanoseconds())
}

func diagnosticMillis(nanos int64) int64 {
	if nanos <= 0 {
		return 0
	}
	return (nanos + int64(time.Millisecond) - 1) / int64(time.Millisecond)
}

// WoxFileSearchStorageIgnorePattern is shared with settings loading so older
// serialized user preferences still receive the mandatory self-storage exclude.
const WoxFileSearchStorageIgnorePattern = "**/.wox/filesearch/**"

// Feature addition: seed the user-editable ignore table with generated and
// application-noise folders that are expensive to traverse and noisy as launcher
// results. Hidden-file skipping moved to its own checkbox, so this editable list
// no longer carries a broad `.*` rule that would make that checkbox ineffective.
var defaultIgnorePatterns = []string{
	"*.tmp",
	"*.temp",
	".DS_Store",
	".git",
	".hg",
	".svn",
	"node_modules",
	"build",
	"dist",
	".dart_tool",
	".gradle",
	".swiftpm",
	".build",
	"DerivedData",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	".ruff_cache",
	".venv",
	"venv",
	".cache",
	".umi",
	".umi-production",
	".next",
	".nuxt",
	".vite",
	".turbo",
	".parcel-cache",
	".output",
	"out",
	"output",
	"outputs",
	"coverage",
	"target",
	".idea",
	".vscode",
	".cursor",
	// Bug fix: Wox stores File Search's own SQLite files under ~/.wox/filesearch.
	// Indexing that directory makes each DB/WAL write emit another change event, so
	// exclude it by default before the scanner can feed its own storage back into
	// the incremental queue.
	WoxFileSearchStorageIgnorePattern,
	"**/tmp/**",
	"**/temp/**",
	"**/Cache/**",
	"**/Caches/**",
	"**/cache/**",
	"**/caches/**",
	"**/Library/Application Support/**",
	"**/Mobile Documents/**/PreferenceSync/**",
	"**/Mobile Documents/**/Application Support/**",
	"*.photoslibrary",
	"*.lrlibrary",
	"*.lrdata",
	"**/_work/**",
	"**/externals.*/**",
}

// DefaultIgnorePatterns returns a copy so callers can expose or sort the
// defaults without mutating the shared plugin policy baseline.
func DefaultIgnorePatterns() []string {
	return append([]string(nil), defaultIgnorePatterns...)
}

func New() *Policy {
	return &Policy{
		patternsByDir: map[string][]gitIgnorePattern{},
		ignoreRules:   compileFileSearchIgnoreRules(defaultIgnorePatterns),
	}
}

func (p *Policy) NewTraversalContext(rootPath string, policyRootPath string, scopePath string) *TraversalContext {
	if p == nil {
		return nil
	}

	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	policyRootPath = strings.TrimSpace(policyRootPath)
	if policyRootPath == "" {
		policyRootPath = rootPath
	}
	policyRootPath = filepath.Clean(policyRootPath)
	scopePath = filepath.Clean(strings.TrimSpace(scopePath))
	if scopePath == "" || scopePath == "." {
		scopePath = rootPath
	}

	rules := p.ignoreRulesSnapshot()
	dirRelSlash, hasDirRel := relativePathForGitIgnoreMatch(policyRootPath, scopePath)
	if dirRelSlash == "." {
		dirRelSlash = ""
	}
	dirSegmentsLower := []string{}
	if hasDirRel && dirRelSlash != "" {
		dirSegmentsLower = lowerSlashPathSegments(dirRelSlash)
	}
	// Optimization: traversal starts from a sealed scope, not always from the
	// policy root. A scope inside an ignored configured path must keep the old
	// behavior where every descendant is ignored, but normal children only need
	// to test their own basename because accepted ancestors were already checked.
	configuredAncestorIgnored := configuredPatternMatchesPath(rules, policyRootPath, scopePath)

	diagnostics := p.diagnosticsRef()
	return &TraversalContext{
		policy:                    p,
		rootPath:                  rootPath,
		policyRootPath:            policyRootPath,
		matchRootPath:             policyRootPath,
		dirPath:                   scopePath,
		dirRelSlash:               dirRelSlash,
		hasDirRel:                 hasDirRel,
		dirSegmentsLower:          dirSegmentsLower,
		ignoreRules:               rules,
		configuredAncestorIgnored: configuredAncestorIgnored,
		gitIgnoreFrames:           p.gitIgnoreFramesForDirectory(policyRootPath, scopePath, diagnostics),
		diagnostics:               diagnostics,
	}
}

func (p *Policy) SetIgnorePatterns(patterns []string) {
	// Feature addition: ignore rules moved from a fixed code list into user
	// settings. Compile them once on setting changes so every visited path pays
	// only cheap matcher checks during large file-index runs.
	compiled := compileFileSearchIgnoreRules(patterns)

	p.mu.Lock()
	p.ignoreRules = compiled
	p.mu.Unlock()
}

func (p *Policy) ignoreRulesSnapshot() fileSearchIgnoreRules {
	if p == nil {
		return fileSearchIgnoreRules{segmentLiterals: map[string]struct{}{}}
	}

	p.mu.RLock()
	rules := p.ignoreRules
	p.mu.RUnlock()
	return rules
}

func (c *TraversalContext) ShouldIndexPath(path string, isDir bool) bool {
	if c == nil || c.policy == nil {
		return true
	}

	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" || cleanPath == "." {
		return true
	}
	if cleanPath == c.dirPath {
		return true
	}
	if filepath.Dir(cleanPath) != c.dirPath {
		// Safety fallback without resurrecting the old per-path matcher: traversal
		// contexts are scoped to direct children, so unexpected callers are
		// re-rooted at the path's parent and still evaluated through the same
		// incremental matcher used by full indexing.
		context := c.policy.NewTraversalContext(c.rootPath, c.policyRootPath, filepath.Dir(cleanPath))
		if context == nil {
			return true
		}
		return context.ShouldIndexPath(cleanPath, isDir)
	}

	diagnostics := c.diagnostics
	startedAt := time.Now()
	ignored := false
	defer func() {
		if diagnostics != nil {
			diagnostics.recordPolicyCheck(time.Since(startedAt), ignored)
		}
	}()

	name := filepath.Base(cleanPath)
	if c.shouldIgnoreByConfiguredPattern(cleanPath, name, diagnostics) {
		ignored = true
		return false
	}

	ignored = c.shouldIgnoreByGitIgnore(name, isDir, diagnostics)
	return !ignored
}

func (c *TraversalContext) Descend(directoryPath string) *TraversalContext {
	if c == nil || c.policy == nil {
		return c
	}

	cleanPath := filepath.Clean(strings.TrimSpace(directoryPath))
	if cleanPath == "" || cleanPath == "." {
		return c
	}

	name := filepath.Base(cleanPath)
	childRelSlash, hasChildRel := c.childRelPath(name)
	if filepath.Dir(cleanPath) != c.dirPath {
		// This keeps manual callers correct even if they construct a child context
		// from a non-direct descendant. The hot traversal path uses direct children
		// and avoids this filepath.Rel fallback.
		childRelSlash, hasChildRel = relativePathForGitIgnoreMatch(c.matchRootPath, cleanPath)
		if childRelSlash == "." {
			childRelSlash = ""
		}
	}
	childSegmentsLower := append(append([]string(nil), c.dirSegmentsLower...), strings.ToLower(name))
	if filepath.Dir(cleanPath) != c.dirPath {
		if hasChildRel && childRelSlash != "" {
			childSegmentsLower = lowerSlashPathSegments(childRelSlash)
		} else {
			childSegmentsLower = nil
		}
	}

	child := &TraversalContext{
		policy:                    c.policy,
		rootPath:                  c.rootPath,
		policyRootPath:            c.policyRootPath,
		matchRootPath:             c.matchRootPath,
		dirPath:                   cleanPath,
		dirRelSlash:               childRelSlash,
		hasDirRel:                 hasChildRel,
		dirSegmentsLower:          childSegmentsLower,
		ignoreRules:               c.ignoreRules,
		configuredAncestorIgnored: c.configuredAncestorIgnored || c.configuredChildPathIgnored(cleanPath, name),
		gitIgnoreFrames:           append([]traversalGitIgnoreFrame(nil), c.gitIgnoreFrames...),
		diagnostics:               c.diagnostics,
	}
	// Optimization: Descend only moves traversal state now. The scanner can pass
	// the child directory's actual ReadDir entries through WithDirectoryEntries
	// when that directory is read, so most directories no longer pay a failed
	// child/.gitignore ReadFile before we know such a file exists.
	return child
}

func (c *TraversalContext) WithDirectoryEntries(directoryPath string, entries []os.DirEntry) *TraversalContext {
	if c == nil || c.policy == nil {
		return c
	}

	cleanPath := filepath.Clean(strings.TrimSpace(directoryPath))
	if cleanPath == "" || cleanPath == "." {
		return c
	}
	if cleanPath != c.dirPath {
		context := c.policy.NewTraversalContext(c.rootPath, c.policyRootPath, cleanPath)
		if context == nil {
			return c
		}
		return context.WithDirectoryEntries(cleanPath, entries)
	}
	if !c.hasDirRel || !directoryEntriesContain(entries, ".gitignore") || c.hasGitIgnoreFrameForCurrentDirectory() {
		return c
	}

	patterns := c.policy.patternsForDirectory(cleanPath, c.diagnostics)
	if len(patterns) == 0 {
		return c
	}

	updated := *c
	updated.gitIgnoreFrames = append(append([]traversalGitIgnoreFrame(nil), c.gitIgnoreFrames...), traversalGitIgnoreFrame{
		dirRelSlash: c.dirRelSlash,
		patterns:    patterns,
	})
	// Optimization: directory-local .gitignore loading is now tied to the
	// directory listing that proved the file exists. This preserves the existing
	// carried-frame matcher while avoiding a failed os.ReadFile for the many
	// directories that do not contain .gitignore.
	return &updated
}

func (c *TraversalContext) hasGitIgnoreFrameForCurrentDirectory() bool {
	if c == nil {
		return false
	}
	for _, frame := range c.gitIgnoreFrames {
		if frame.dirRelSlash == c.dirRelSlash {
			return true
		}
	}
	return false
}

func (c *TraversalContext) shouldIgnoreByConfiguredPattern(fullPath string, name string, diagnostics *Diagnostics) bool {
	startedAt := time.Now()
	ignored := c.configuredAncestorIgnored
	defer func() {
		if diagnostics != nil {
			diagnostics.recordConfiguredPatternCheck(time.Since(startedAt), ignored)
		}
	}()
	if ignored {
		return true
	}

	if c.ignoreRules.matchesTraversalChildSegments(name, c.dirSegmentsLower) {
		ignored = true
		return true
	}
	if c.ignoreRules.hasPathCandidateRules() {
		childRelSlash, hasChildRel := c.childRelPath(name)
		pathCandidate := filepath.ToSlash(fullPath)
		if hasChildRel {
			pathCandidate = childRelSlash
			childRelSlash = ""
			hasChildRel = false
		}
		if c.ignoreRules.matchesTraversalChildPathCandidates(pathCandidate, childRelSlash, hasChildRel) {
			ignored = true
			return true
		}
	}
	return false
}

func (c *TraversalContext) configuredChildPathIgnored(cleanPath string, name string) bool {
	if c == nil {
		return false
	}
	if c.ignoreRules.matchesTraversalChildSegments(name, c.dirSegmentsLower) {
		return true
	}
	if !c.ignoreRules.hasPathCandidateRules() {
		return false
	}
	childRelSlash, hasChildRel := c.childRelPath(name)
	if filepath.Dir(cleanPath) != c.dirPath {
		childRelSlash, hasChildRel = relativePathForGitIgnoreMatch(c.matchRootPath, cleanPath)
		if childRelSlash == "." {
			childRelSlash = ""
		}
	}
	pathCandidate := filepath.ToSlash(cleanPath)
	if hasChildRel {
		pathCandidate = childRelSlash
		childRelSlash = ""
		hasChildRel = false
	}
	return c.ignoreRules.matchesTraversalChildPathCandidates(pathCandidate, childRelSlash, hasChildRel)
}

func (c *TraversalContext) shouldIgnoreByGitIgnore(name string, isDir bool, diagnostics *Diagnostics) bool {
	startedAt := time.Now()
	directoriesWithPatterns := int64(0)
	patternComparisons := int64(0)
	ignored := false
	defer func() {
		if diagnostics != nil {
			diagnostics.recordGitIgnoreCheck(time.Since(startedAt), ignored, int64(len(c.gitIgnoreFrames)), directoriesWithPatterns, patternComparisons)
		}
	}()

	childRelSlash, hasChildRel := c.childRelPath(name)
	if !hasChildRel {
		return false
	}

	for _, frame := range c.gitIgnoreFrames {
		relPath, ok := traversalRelPathFromFrame(frame.dirRelSlash, childRelSlash)
		if !ok {
			continue
		}
		directoriesWithPatterns++
		patternComparisons += int64(len(frame.patterns))
		for _, pattern := range frame.patterns {
			if pattern.matchesRelPath(relPath, isDir) {
				ignored = !pattern.negate
			}
		}
	}

	return ignored
}

func (c *TraversalContext) childRelPath(name string) (string, bool) {
	if !c.hasDirRel {
		return "", false
	}
	name = filepath.ToSlash(name)
	if c.dirRelSlash == "" {
		return name, true
	}
	return c.dirRelSlash + "/" + name, true
}

func traversalRelPathFromFrame(frameDirRelSlash string, childRelSlash string) (string, bool) {
	if frameDirRelSlash == "" {
		return childRelSlash, true
	}
	prefix := frameDirRelSlash + "/"
	if !strings.HasPrefix(childRelSlash, prefix) {
		return "", false
	}
	return strings.TrimPrefix(childRelSlash, prefix), true
}

func directoryEntriesContain(entries []os.DirEntry, name string) bool {
	for _, entry := range entries {
		if entry.Name() == name {
			return true
		}
	}
	return false
}

func splitFileSearchPathSegments(fullPath string) []string {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(fullPath)))
	if normalized == "." || normalized == "" {
		return nil
	}

	// Bug fix: absolute paths start with "/", so strings.Split would emit an
	// empty first segment. Segment ignore rules should only see real path
	// components, otherwise generated empty segments add work to the hot path and
	// make user-visible glob behavior harder to reason about.
	rawSegments := strings.Split(normalized, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
}

type fileSearchIgnoreRule struct {
	hasSlash          bool
	segmentLiteral    string
	segmentPattern    string
	segmentSimpleGlob bool
	segmentParts      []string
	pathSegmentParts  []string
	leadingStar       bool
	trailingStar      bool
	hasQuestion       bool
	pathRegex         *regexp.Regexp
	segmentRe         *regexp.Regexp
}

type fileSearchIgnoreRules struct {
	pathRules       []fileSearchIgnoreRule
	segmentLiterals map[string]struct{}
	segmentRules    []fileSearchIgnoreRule
}

func compileFileSearchIgnoreRules(patterns []string) fileSearchIgnoreRules {
	compiled := fileSearchIgnoreRules{segmentLiterals: map[string]struct{}{}}
	seen := make(map[string]struct{}, len(patterns))
	for _, pattern := range patterns {
		raw := strings.TrimSpace(pattern)
		if raw == "" {
			continue
		}

		normalized := filepath.ToSlash(raw)
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		rule, ok := compileFileSearchIgnoreRule(normalized)
		if ok {
			if rule.hasSlash {
				compiled.pathRules = append(compiled.pathRules, rule)
				continue
			}
			if rule.segmentLiteral != "" {
				compiled.segmentLiterals[rule.segmentLiteral] = struct{}{}
				continue
			}
			compiled.segmentRules = append(compiled.segmentRules, rule)
		}
	}

	return compiled
}

func compileFileSearchIgnoreRule(pattern string) (fileSearchIgnoreRule, bool) {
	pattern = strings.TrimSpace(filepath.ToSlash(pattern))
	if pattern == "" {
		return fileSearchIgnoreRule{}, false
	}

	// Optimization: the default list contains many recursive single-segment
	// paths such as "**/cache/**". Treating them as path regexes made every
	// visited entry run expensive regexp checks even though traversal only needs
	// to reject the matching segment once and then prune the subtree.
	if segmentPattern, ok := recursiveSingleSegmentPattern(pattern); ok {
		pattern = segmentPattern
	}

	// Ignore patterns are user-facing, so they intentionally use Raycast-style
	// path globs instead of Go regexes. Segment-only patterns such as
	// "node_modules" match any path segment, while path patterns such as
	// "**/Library/Application Support/**" can prune a whole subtree before the
	// scanner descends into it.
	hasSlash := strings.Contains(pattern, "/")
	if !hasSlash && !strings.ContainsAny(pattern, "*?[") {
		return fileSearchIgnoreRule{
			segmentLiteral: strings.ToLower(pattern),
		}, true
	}
	if !hasSlash && isSimpleGitIgnoreGlob(pattern) {
		normalized := strings.ToLower(pattern)
		return fileSearchIgnoreRule{
			segmentPattern:    normalized,
			segmentSimpleGlob: true,
			segmentParts:      strings.Split(normalized, "*"),
			leadingStar:       strings.HasPrefix(normalized, "*"),
			trailingStar:      strings.HasSuffix(normalized, "*"),
			hasQuestion:       strings.Contains(normalized, "?"),
		}, true
	}
	if hasSlash {
		if parts, ok := recursivePathSegmentPattern(pattern); ok {
			return fileSearchIgnoreRule{
				hasSlash:         true,
				pathSegmentParts: parts,
			}, true
		}
	}
	expr := globPatternToRegex(pattern, !hasSlash)
	if expr == "" {
		return fileSearchIgnoreRule{}, false
	}

	compiled, err := regexp.Compile("(?i)^" + expr + "$")
	if err != nil {
		return fileSearchIgnoreRule{}, false
	}

	rule := fileSearchIgnoreRule{
		hasSlash: hasSlash,
	}
	if hasSlash {
		rule.pathRegex = compiled
	} else {
		rule.segmentRe = compiled
	}
	return rule, true
}

func recursiveSingleSegmentPattern(pattern string) (string, bool) {
	if !strings.HasPrefix(pattern, "**/") || !strings.HasSuffix(pattern, "/**") {
		return "", false
	}
	segment := strings.TrimSuffix(strings.TrimPrefix(pattern, "**/"), "/**")
	if segment == "" || strings.Contains(segment, "/") {
		return "", false
	}
	return segment, true
}

func recursivePathSegmentPattern(pattern string) ([]string, bool) {
	if !strings.HasSuffix(pattern, "/**") {
		return nil, false
	}
	base := strings.TrimSuffix(pattern, "/**")
	if strings.HasPrefix(base, "**/") {
		base = strings.TrimPrefix(base, "**/")
	}
	if base == "" || !strings.Contains(base, "/") {
		return nil, false
	}

	rawParts := strings.Split(base, "/")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		if part == "" {
			return nil, false
		}
		if part == "**" {
			parts = append(parts, part)
			continue
		}
		if strings.ContainsAny(part, "*?[") {
			return nil, false
		}
		parts = append(parts, strings.ToLower(part))
	}
	return parts, true
}

func lowerSlashPathSegments(path string) []string {
	rawSegments := strings.Split(path, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		segments = append(segments, strings.ToLower(segment))
	}
	return segments
}

func matchPathSegmentSequence(patternParts []string, pathSegments []string) bool {
	for start := 0; start < len(pathSegments); start++ {
		if matchPathSegmentSequenceFrom(patternParts, pathSegments[start:]) {
			return true
		}
	}
	return false
}

func matchPathSegmentSequenceFrom(patternParts []string, pathSegments []string) bool {
	if len(patternParts) == 0 {
		return true
	}
	if patternParts[0] == "**" {
		if len(patternParts) == 1 {
			return true
		}
		for offset := 0; offset <= len(pathSegments); offset++ {
			if matchPathSegmentSequenceFrom(patternParts[1:], pathSegments[offset:]) {
				return true
			}
		}
		return false
	}
	if len(pathSegments) == 0 || patternParts[0] != pathSegments[0] {
		return false
	}
	return matchPathSegmentSequenceFrom(patternParts[1:], pathSegments[1:])
}

func matchPathSegmentSequenceWithChild(patternParts []string, dirSegments []string, childSegment string) bool {
	totalSegments := len(dirSegments) + 1
	for start := 0; start < totalSegments; start++ {
		if matchPathSegmentSequenceWithChildFrom(patternParts, dirSegments, childSegment, start) {
			return true
		}
	}
	return false
}

func matchPathSegmentSequenceWithChildFrom(patternParts []string, dirSegments []string, childSegment string, segmentIndex int) bool {
	if len(patternParts) == 0 {
		return true
	}
	totalSegments := len(dirSegments) + 1
	if patternParts[0] == "**" {
		if len(patternParts) == 1 {
			return true
		}
		for offset := segmentIndex; offset <= totalSegments; offset++ {
			if matchPathSegmentSequenceWithChildFrom(patternParts[1:], dirSegments, childSegment, offset) {
				return true
			}
		}
		return false
	}
	if segmentIndex >= totalSegments || patternParts[0] != traversalSegmentAt(dirSegments, childSegment, segmentIndex) {
		return false
	}
	return matchPathSegmentSequenceWithChildFrom(patternParts[1:], dirSegments, childSegment, segmentIndex+1)
}

func traversalSegmentAt(dirSegments []string, childSegment string, index int) string {
	if index < len(dirSegments) {
		return dirSegments[index]
	}
	return childSegment
}

func globPatternToRegex(pattern string, segmentOnly bool) string {
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		if base == "" {
			return ".*"
		}
		return globPatternToRegex(base, segmentOnly) + "(?:/.*)?"
	}

	var builder strings.Builder
	for index := 0; index < len(pattern); {
		if strings.HasPrefix(pattern[index:], "**/") {
			builder.WriteString("(?:.*/)?")
			index += 3
			continue
		}
		if strings.HasPrefix(pattern[index:], "**") {
			builder.WriteString(".*")
			index += 2
			continue
		}

		character := pattern[index]
		switch character {
		case '*':
			if segmentOnly {
				builder.WriteString(".*")
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			if segmentOnly {
				builder.WriteByte('.')
			} else {
				builder.WriteString("[^/]")
			}
		case '[':
			end := strings.IndexByte(pattern[index+1:], ']')
			if end < 0 {
				builder.WriteString(regexp.QuoteMeta(string(character)))
			} else {
				class := pattern[index : index+end+2]
				builder.WriteString(class)
				index += end + 2
				continue
			}
		default:
			builder.WriteString(regexp.QuoteMeta(string(character)))
		}
		index++
	}

	return builder.String()
}

func configuredPatternMatchesPath(rules fileSearchIgnoreRules, matchRootPath string, fullPath string) bool {
	relPath, hasRelPath := relativePathForGitIgnoreMatch(filepath.Clean(matchRootPath), fullPath)
	if hasRelPath {
		if relPath == "." || relPath == "" {
			return false
		}
		// Bug fix: configured ignore rules are scoped to the indexed policy root.
		// Matching segment rules against the absolute Windows path made any root
		// under %TEMP% look ignored by the default **/temp/** rule. Use the
		// root-relative path when available so defaults describe content inside the
		// configured root, not every ancestor chosen by the OS or test harness.
		return rules.matches([]string{relPath}, splitFileSearchPathSegments(relPath))
	}

	fullSlash := filepath.ToSlash(filepath.Clean(fullPath))
	candidates := []string{fullSlash}
	segments := splitFileSearchPathSegments(fullSlash)
	return rules.matches(candidates, segments)
}

func (rules fileSearchIgnoreRules) matches(pathCandidates []string, segments []string) bool {
	for _, rule := range rules.pathRules {
		if rule.matchesPath(pathCandidates) {
			return true
		}
	}

	for _, segment := range segments {
		normalizedSegment := strings.ToLower(segment)
		if _, ok := rules.segmentLiterals[normalizedSegment]; ok {
			return true
		}
		for _, rule := range rules.segmentRules {
			if rule.matchesSegment(segment) {
				return true
			}
		}
	}

	return false
}

func (rules fileSearchIgnoreRules) matchesTraversalChildSegments(segment string, dirSegmentsLower []string) bool {
	normalizedSegment := strings.ToLower(segment)
	for _, rule := range rules.pathRules {
		if rule.matchesTraversalPathSegments(dirSegmentsLower, normalizedSegment) {
			return true
		}
	}

	if _, ok := rules.segmentLiterals[normalizedSegment]; ok {
		return true
	}
	for _, rule := range rules.segmentRules {
		if rule.matchesSegment(segment) {
			return true
		}
	}

	return false
}

func (rules fileSearchIgnoreRules) hasPathCandidateRules() bool {
	for _, rule := range rules.pathRules {
		if len(rule.pathSegmentParts) == 0 {
			return true
		}
	}
	return false
}

func (rules fileSearchIgnoreRules) matchesTraversalChildPathCandidates(fullSlash string, relSlash string, hasRelSlash bool) bool {
	for _, rule := range rules.pathRules {
		if len(rule.pathSegmentParts) > 0 {
			continue
		}
		if rule.matchesPathCandidate(fullSlash) {
			return true
		}
		if hasRelSlash && rule.matchesPathCandidate(relSlash) {
			return true
		}
	}
	return false
}

func (r fileSearchIgnoreRule) matchesPath(pathCandidates []string) bool {
	if r.pathRegex == nil && len(r.pathSegmentParts) == 0 {
		return false
	}
	for _, candidate := range pathCandidates {
		if r.matchesPathCandidate(candidate) {
			return true
		}
	}
	return false
}

func (r fileSearchIgnoreRule) matchesPathCandidate(candidate string) bool {
	if len(r.pathSegmentParts) > 0 {
		return matchPathSegmentSequence(r.pathSegmentParts, lowerSlashPathSegments(candidate))
	}
	if r.pathRegex == nil || candidate == "." || candidate == "" {
		return false
	}
	return r.pathRegex.MatchString(strings.TrimPrefix(candidate, "/")) || r.pathRegex.MatchString(candidate)
}

func (r fileSearchIgnoreRule) matchesTraversalPathSegments(dirSegmentsLower []string, childSegmentLower string) bool {
	if len(r.pathSegmentParts) == 0 {
		return false
	}
	return matchPathSegmentSequenceWithChild(r.pathSegmentParts, dirSegmentsLower, childSegmentLower)
}

func (r fileSearchIgnoreRule) matchesSegment(segment string) bool {
	if r.segmentSimpleGlob {
		// Optimization: simple configured globs such as "*.tmp" are checked for
		// every file. Lowercase string matching preserves the previous
		// case-insensitive regexp semantics without paying the regexp engine cost
		// on the full-index hot path.
		normalized := strings.ToLower(segment)
		if !r.hasQuestion {
			return matchSimpleGitIgnoreLiteralGlob(r.segmentParts, r.leadingStar, r.trailingStar, normalized)
		}
		return matchSimpleGitIgnoreGlob(r.segmentPattern, normalized)
	}
	return r.segmentRe != nil && r.segmentRe.MatchString(segment)
}

func (p *Policy) patternsForDirectory(directory string, diagnostics *Diagnostics) []gitIgnorePattern {
	directory = filepath.Clean(strings.TrimSpace(directory))
	if directory == "" {
		return nil
	}

	p.mu.RLock()
	patterns, ok := p.patternsByDir[directory]
	p.mu.RUnlock()
	if ok {
		return patterns
	}

	startedAt := time.Now()
	loaded := loadGitIgnorePatterns(directory)
	if diagnostics != nil {
		diagnostics.recordGitIgnorePatternLoad(time.Since(startedAt), len(loaded))
	}

	p.mu.Lock()
	if existing, ok := p.patternsByDir[directory]; ok {
		p.mu.Unlock()
		return existing
	}
	p.patternsByDir[directory] = loaded
	p.mu.Unlock()

	return loaded
}

func (p *Policy) gitIgnoreFramesForDirectory(rootPath string, directory string, diagnostics *Diagnostics) []traversalGitIgnoreFrame {
	directories := directoriesFromRootToDirectory(rootPath, directory)
	if len(directories) == 0 {
		return nil
	}

	frames := make([]traversalGitIgnoreFrame, 0, len(directories))
	for _, current := range directories {
		patterns := p.patternsForDirectory(current, diagnostics)
		if len(patterns) == 0 {
			continue
		}
		relPath, ok := relativePathForGitIgnoreMatch(rootPath, current)
		if !ok {
			continue
		}
		if relPath == "." {
			relPath = ""
		}
		frames = append(frames, traversalGitIgnoreFrame{
			dirRelSlash: relPath,
			patterns:    patterns,
		})
	}
	return frames
}

func directoriesFromRootToDirectory(rootPath string, directory string) []string {
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	directory = filepath.Clean(strings.TrimSpace(directory))
	if rootPath == "" || directory == "" || !pathWithinRoot(rootPath, directory) {
		return nil
	}

	reversed := make([]string, 0, 8)
	for current := directory; ; current = filepath.Dir(current) {
		reversed = append(reversed, current)
		if current == rootPath {
			break
		}
		next := filepath.Dir(current)
		if next == current || !pathWithinRoot(rootPath, next) {
			break
		}
	}

	directories := make([]string, 0, len(reversed))
	for index := len(reversed) - 1; index >= 0; index-- {
		directories = append(directories, reversed[index])
	}
	return directories
}

func pathWithinRoot(rootPath string, candidatePath string) bool {
	rel, err := filepath.Rel(filepath.Clean(rootPath), filepath.Clean(candidatePath))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}

	parentPrefix := ".." + string(filepath.Separator)
	return rel != ".." && !strings.HasPrefix(rel, parentPrefix)
}

type gitIgnorePattern struct {
	patternSlash string
	negate       bool
	dirOnly      bool
	rooted       bool
	hasSlash     bool
	hasMeta      bool
	simpleGlob   bool
	simpleParts  []string
	leadingStar  bool
	trailingStar bool
	hasQuestion  bool
}

func loadGitIgnorePatterns(directory string) []gitIgnorePattern {
	gitIgnorePath := filepath.Join(directory, ".gitignore")
	data, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	patterns := make([]gitIgnorePattern, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		pattern := gitIgnorePattern{}
		if strings.HasPrefix(line, "!") {
			pattern.negate = true
			line = strings.TrimPrefix(line, "!")
		}
		if strings.HasPrefix(line, "/") {
			pattern.rooted = true
			line = strings.TrimPrefix(line, "/")
		}
		if strings.HasSuffix(line, "/") {
			pattern.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		pattern.patternSlash = filepath.ToSlash(line)
		pattern.hasSlash = strings.Contains(pattern.patternSlash, "/")
		pattern.hasMeta = hasGitIgnoreGlobMeta(pattern.patternSlash)
		pattern.simpleGlob = isSimpleGitIgnoreGlob(pattern.patternSlash)
		if pattern.simpleGlob {
			pattern.simpleParts = strings.Split(pattern.patternSlash, "*")
			pattern.leadingStar = strings.HasPrefix(pattern.patternSlash, "*")
			pattern.trailingStar = strings.HasSuffix(pattern.patternSlash, "*")
			pattern.hasQuestion = strings.Contains(pattern.patternSlash, "?")
		}
		if pattern.patternSlash != "" {
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

func relativePathForGitIgnoreMatch(baseDir string, fullPath string) (string, bool) {
	relPath, err := filepath.Rel(baseDir, fullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", false
	}
	return filepath.ToSlash(relPath), true
}

func (p gitIgnorePattern) matchesRelPath(relPath string, isDir bool) bool {
	if p.dirOnly && !isDir {
		return false
	}

	pattern := p.patternSlash

	if p.rooted || p.hasSlash {
		// Most ignore patterns are literals such as "target" or "coverage/". The
		// old matcher sent those through filepath.Match for every visited file,
		// which dominated filesearch CPU during startup restore. Literal patterns
		// can be compared directly; glob patterns keep the previous matcher path.
		if !p.hasMeta && relPath == pattern {
			return true
		}
		if p.hasMeta {
			if p.matchesCandidate(pattern, relPath) {
				return true
			}
		}
		return strings.HasPrefix(relPath, pattern+"/")
	}

	if !p.hasMeta {
		// For unrooted literal patterns, scan segments without strings.Split so
		// large trees do not allocate a segment slice for every path/pattern pair.
		return containsGitIgnorePathSegment(relPath, pattern)
	}

	return containsGitIgnoreMatchingSegment(relPath, p)
}

func (p gitIgnorePattern) matchesCandidate(pattern string, candidate string) bool {
	if p.simpleGlob {
		// CPU profiles showed simple patterns such as "*.ext" still dominating
		// startup restore because filepath.Match uses a general parser for every
		// path segment. Patterns without '?' are pre-split once at .gitignore load
		// time so common suffix/prefix globs use string searches instead of
		// per-candidate backtracking; anything more complex keeps the safe matcher.
		if !p.hasQuestion {
			return matchSimpleGitIgnoreLiteralGlob(p.simpleParts, p.leadingStar, p.trailingStar, candidate)
		}
		return matchSimpleGitIgnoreGlob(pattern, candidate)
	}

	ok, _ := filepath.Match(pattern, candidate)
	return ok
}

func containsGitIgnoreMatchingSegment(relPath string, pattern gitIgnorePattern) bool {
	for start := 0; start <= len(relPath); {
		end := strings.IndexByte(relPath[start:], '/')
		if end < 0 {
			return pattern.matchesCandidate(pattern.patternSlash, relPath[start:])
		}
		if pattern.matchesCandidate(pattern.patternSlash, relPath[start:start+end]) {
			return true
		}
		start += end + 1
	}

	return false
}

func hasGitIgnoreGlobMeta(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

func isSimpleGitIgnoreGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?") && !strings.ContainsAny(pattern, "[\\")
}

func matchSimpleGitIgnoreLiteralGlob(parts []string, leadingStar bool, trailingStar bool, candidate string) bool {
	if len(parts) == 0 {
		return candidate == ""
	}

	position := 0
	firstPart := 0
	if !leadingStar {
		prefix := parts[0]
		if !strings.HasPrefix(candidate, prefix) {
			return false
		}
		position = len(prefix)
		firstPart = 1
	}

	lastPart := len(parts) - 1
	searchLimit := len(candidate)
	if !trailingStar {
		suffix := parts[lastPart]
		if !strings.HasSuffix(candidate, suffix) {
			return false
		}
		searchLimit = len(candidate) - len(suffix)
		lastPart--
	}

	for index := firstPart; index <= lastPart; index++ {
		part := parts[index]
		if part == "" {
			continue
		}
		if position > searchLimit {
			return false
		}
		offset := strings.Index(candidate[position:searchLimit], part)
		if offset < 0 {
			return false
		}
		position += offset + len(part)
	}

	return position <= searchLimit
}

func matchSimpleGitIgnoreGlob(pattern string, candidate string) bool {
	patternIndex := 0
	candidateIndex := 0
	starIndex := -1
	starCandidateIndex := 0

	for candidateIndex < len(candidate) {
		if patternIndex < len(pattern) && (pattern[patternIndex] == '?' || pattern[patternIndex] == candidate[candidateIndex]) {
			patternIndex++
			candidateIndex++
			continue
		}
		if patternIndex < len(pattern) && pattern[patternIndex] == '*' {
			starIndex = patternIndex
			starCandidateIndex = candidateIndex
			patternIndex++
			continue
		}
		if starIndex >= 0 {
			patternIndex = starIndex + 1
			starCandidateIndex++
			candidateIndex = starCandidateIndex
			continue
		}
		return false
	}

	for patternIndex < len(pattern) && pattern[patternIndex] == '*' {
		patternIndex++
	}

	return patternIndex == len(pattern)
}

func containsGitIgnorePathSegment(relPath string, pattern string) bool {
	for start := 0; start <= len(relPath); {
		end := strings.IndexByte(relPath[start:], '/')
		if end < 0 {
			return relPath[start:] == pattern
		}
		if relPath[start:start+end] == pattern {
			return true
		}
		start += end + 1
	}

	return false
}
