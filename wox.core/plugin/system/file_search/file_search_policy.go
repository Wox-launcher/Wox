package system

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"wox/plugin/system/file_search/indexpolicy"
	"wox/util/filesearch"
)

const maxFileSearchChangePolicyContextCacheEntries = 4096

type fileSearchIndexPolicy struct {
	// Boundary change: the matching implementation lives in a small plugin-owned
	// package so real-index benchmarks can use the same rules without importing
	// the full system plugin and creating a filesearch engine cycle.
	inner                *indexpolicy.Policy
	skipHiddenFiles      atomic.Bool
	changeContextCacheMu sync.RWMutex
	changeContextCache   map[fileSearchChangePolicyContextKey]fileSearchTraversalPolicyContext
}

type fileSearchChangePolicyContextKey struct {
	rootID          string
	rootPath        string
	policyRootPath  string
	scopePath       string
	skipHiddenFiles bool
}

var defaultFileSearchIgnorePatterns = indexpolicy.DefaultIgnorePatterns()

func newFileSearchIndexPolicy() *fileSearchIndexPolicy {
	policy := &fileSearchIndexPolicy{inner: indexpolicy.New()}
	// Feature addition: hidden-file skipping is now a first-class setting instead
	// of being buried inside the editable ignore-pattern table as `.*`. Defaulting
	// the policy to true preserves the previous launcher behavior for new callers.
	policy.skipHiddenFiles.Store(true)
	return policy
}

func (p *fileSearchIndexPolicy) toFilesearchPolicy() filesearch.Policy {
	return filesearch.Policy{
		NewTraversalContext: p.newTraversalContext,
		ShouldProcessChange: p.shouldProcessChange,
	}
}

func (p *fileSearchIndexPolicy) newTraversalContext(root filesearch.RootRecord, scopePath string) filesearch.TraversalPolicyContext {
	if p == nil || p.inner == nil {
		return nil
	}
	context := p.inner.NewTraversalContext(root.Path, root.PolicyRootPath, scopePath)
	if context == nil {
		return nil
	}
	return fileSearchTraversalPolicyContext{
		inner:           context,
		skipHiddenFiles: p.skipHiddenFiles.Load(),
	}
}

func (p *fileSearchIndexPolicy) changeTraversalContext(root filesearch.RootRecord, scopePath string) filesearch.TraversalPolicyContext {
	if p == nil || p.inner == nil {
		return nil
	}
	skipHiddenFiles := p.skipHiddenFiles.Load()
	key := fileSearchChangePolicyContextKey{
		rootID:          root.ID,
		rootPath:        root.Path,
		policyRootPath:  root.PolicyRootPath,
		scopePath:       scopePath,
		skipHiddenFiles: skipHiddenFiles,
	}

	p.changeContextCacheMu.RLock()
	if cached, ok := p.changeContextCache[key]; ok {
		p.changeContextCacheMu.RUnlock()
		return cached
	}
	p.changeContextCacheMu.RUnlock()

	context := p.inner.NewTraversalContext(root.Path, root.PolicyRootPath, scopePath)
	if context == nil {
		return nil
	}
	cached := fileSearchTraversalPolicyContext{
		inner:           context,
		skipHiddenFiles: skipHiddenFiles,
	}

	p.changeContextCacheMu.Lock()
	defer p.changeContextCacheMu.Unlock()
	if existing, ok := p.changeContextCache[key]; ok {
		return existing
	}
	if p.changeContextCache == nil {
		p.changeContextCache = map[fileSearchChangePolicyContextKey]fileSearchTraversalPolicyContext{}
	}
	if len(p.changeContextCache) >= maxFileSearchChangePolicyContextCacheEntries {
		// Optimization: change-signal policy checks are high-frequency but only
		// need a bounded directory-context memo. Resetting the small cache keeps
		// memory predictable while preserving the common burst case where many
		// events arrive from the same directories.
		p.changeContextCache = map[fileSearchChangePolicyContextKey]fileSearchTraversalPolicyContext{}
	}
	p.changeContextCache[key] = cached
	return cached
}

func (p *fileSearchIndexPolicy) clearChangeTraversalContextCache() {
	if p == nil {
		return
	}
	p.changeContextCacheMu.Lock()
	// Cache invalidation: traversal contexts snapshot configured ignore rules,
	// .gitignore ancestor frames, and the hidden-file switch. Any setting or
	// ignore-file change must discard the memo so future watcher events route
	// through current policy state.
	p.changeContextCache = nil
	p.changeContextCacheMu.Unlock()
}

type fileSearchTraversalPolicyContext struct {
	inner           *indexpolicy.TraversalContext
	skipHiddenFiles bool
}

func (c fileSearchTraversalPolicyContext) ShouldIndexPath(path string, isDir bool) bool {
	if c.inner == nil {
		return true
	}
	if c.skipHiddenFiles && isHiddenFileSearchPath(path) {
		return false
	}
	return c.inner.ShouldIndexPath(path, isDir)
}

func (c fileSearchTraversalPolicyContext) Descend(directoryPath string) filesearch.TraversalPolicyContext {
	if c.inner == nil {
		return c
	}
	// Optimization boundary: util/filesearch only knows the generic traversal
	// interface, while plugin/system/file_search owns the real ignore matcher.
	// The adapter keeps that dependency direction intact and still lets the core
	// scanner carry incremental .gitignore/configured-rule state.
	return fileSearchTraversalPolicyContext{
		inner:           c.inner.Descend(directoryPath),
		skipHiddenFiles: c.skipHiddenFiles,
	}
}

func (c fileSearchTraversalPolicyContext) WithDirectoryEntries(directoryPath string, entries []os.DirEntry) filesearch.TraversalPolicyContext {
	if c.inner == nil {
		return c
	}
	// Optimization: the core scanner can now forward a directory's ReadDir
	// result before evaluating its children. Preserve the plugin-owned hidden
	// file setting while letting indexpolicy skip .gitignore reads for directories
	// that the listing already proved do not contain one.
	return fileSearchTraversalPolicyContext{
		inner:           c.inner.WithDirectoryEntries(directoryPath, entries),
		skipHiddenFiles: c.skipHiddenFiles,
	}
}

func (p *fileSearchIndexPolicy) SetIgnorePatterns(patterns []string) {
	if p == nil || p.inner == nil {
		return
	}
	p.inner.SetIgnorePatterns(patterns)
	p.clearChangeTraversalContextCache()
}

func (p *fileSearchIndexPolicy) SetSkipHiddenFiles(enabled bool) {
	if p == nil {
		return
	}
	p.skipHiddenFiles.Store(enabled)
	p.clearChangeTraversalContextCache()
}

func (p *fileSearchIndexPolicy) shouldProcessChange(root filesearch.RootRecord, change filesearch.ChangeSignal) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(change.Path))
	if cleanPath == "" || cleanPath == "." {
		return true
	}
	if cleanPath == filepath.Clean(root.Path) && root.Kind != filesearch.RootKindDynamic {
		// Bug fix: persisted dynamic roots inherit their parent policy, so their
		// own root path must still pass the configured ignore matcher. Returning
		// early here let an old ~/.wox/filesearch dynamic root keep accepting its
		// SQLite DB events even after that path became a mandatory ignore rule.
		return true
	}
	if filepath.Base(cleanPath) == ".gitignore" {
		p.clearChangeTraversalContextCache()
		if p != nil && p.inner != nil {
			p.inner.ClearGitIgnoreCache()
		}
	}

	isDir := change.PathIsDir
	if !change.PathTypeKnown {
		if info, err := os.Lstat(cleanPath); err == nil {
			// Bug fix: policy checks for watcher paths must classify symlinks the
			// same way full scans do. A symlink-to-directory should be matched as the
			// link entry itself, not as a directory target that can pass dir-only
			// ignore semantics.
			isDir = info.IsDir()
		} else if info, err := os.Stat(cleanPath); err == nil {
			isDir = info.IsDir()
		}
	}

	context := p.changeTraversalContext(root, filepath.Dir(cleanPath))
	if context == nil {
		return true
	}
	// Optimization: change-signal filtering still uses the same traversal rules
	// as full indexing, but repeated events from one directory reuse a cached
	// context instead of rebuilding the .gitignore ancestor stack every time.
	return context.ShouldIndexPath(cleanPath, isDir)
}

func isHiddenFileSearchPath(path string) bool {
	name := filepath.Base(filepath.Clean(strings.TrimSpace(path)))
	// Feature behavior: the setting follows the platform convention used by fd/rg
	// defaults and treats dot-prefixed basenames as hidden. The root itself is
	// still accepted before this helper is called, so explicitly configured hidden
	// roots can exist while hidden descendants stay controlled by the checkbox.
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}
