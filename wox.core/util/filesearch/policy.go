package filesearch

import (
	"context"
	"fmt"
	"os"
	"sync"
	"wox/util"
)

type Policy struct {
	NewTraversalContext func(root RootRecord, scopePath string) TraversalPolicyContext
	ShouldProcessChange func(root RootRecord, change ChangeSignal) bool
}

// TraversalPolicyContext lets recursive scanners carry policy state for the
// directory they are reading. Policy evaluation intentionally goes through this
// interface only, so large indexes cannot accidentally fall back to rebuilding
// the ancestor ignore chain for every child entry.
type TraversalPolicyContext interface {
	ShouldIndexPath(path string, isDir bool) bool
	Descend(directoryPath string) TraversalPolicyContext
}

// DirectoryEntryAwareTraversalPolicyContext lets a scanner pass the directory
// listing it already paid for into policy code. The base traversal contract stays
// small for other callers, while filesearch can avoid probing every directory
// for optional policy files such as .gitignore when ReadDir already proved they
// are absent.
type DirectoryEntryAwareTraversalPolicyContext interface {
	TraversalPolicyContext
	WithDirectoryEntries(directoryPath string, entries []os.DirEntry) TraversalPolicyContext
}

type EngineOptions struct {
	Policy Policy
}

const runPlannerSplitPolicyVersionV1 = 1

// splitBudget keeps the version-1 run planner thresholds in code.
// The previous root-centric planner had no internal job sizing, so one huge
// root could stay as one opaque workload. Version 1 fixes that with constants
// first so progress semantics stay predictable before we consider any user
// settings or adaptive tuning.
type splitBudget struct {
	LeafEntryBudget  int64
	LeafWriteBudget  int64
	LeafMemoryBudget int64
	// Version 1 keeps one direct-files job per directory so delete ownership
	// stays simple. The older planner split one directory into many jobs, which
	// made stale direct-file pruning ambiguous. The same limit now only caps the
	// internal staging batch size inside that single job.
	DirectFileBatchSize int
}

func defaultSplitBudget() splitBudget {
	return splitBudget{
		LeafEntryBudget:     4096,
		LeafWriteBudget:     4096,
		LeafMemoryBudget:    8 << 20,
		DirectFileBatchSize: 2048,
	}
}

func DefaultEngineOptions() EngineOptions {
	return EngineOptions{}
}

type policyState struct {
	mu     sync.RWMutex
	policy Policy
}

func newPolicyState(policy Policy) *policyState {
	return &policyState{policy: policy}
}

func (s *policyState) Set(policy Policy) {
	if s == nil {
		return
	}

	s.mu.Lock()
	s.policy = policy
	s.mu.Unlock()
}

func (s *policyState) newTraversalContext(root RootRecord, scopePath string) TraversalPolicyContext {
	if s == nil {
		return allowAllTraversalPolicyContext{}
	}

	s.mu.RLock()
	callback := s.policy.NewTraversalContext
	s.mu.RUnlock()
	if callback == nil {
		return allowAllTraversalPolicyContext{}
	}

	var policyContext TraversalPolicyContext
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("filesearch policy NewTraversalContext panic recovered: %v", recovered))
				policyContext = nil
			}
		}()
		policyContext = callback(root, scopePath)
	}()
	if policyContext == nil {
		return allowAllTraversalPolicyContext{}
	}

	return safeTraversalPolicyContext{
		inner: policyContext,
	}
}

type allowAllTraversalPolicyContext struct{}

func (c allowAllTraversalPolicyContext) ShouldIndexPath(path string, isDir bool) bool {
	return true
}

func (c allowAllTraversalPolicyContext) Descend(directoryPath string) TraversalPolicyContext {
	// No configured traversal policy means the engine should stay policy-neutral
	// and accept every path. This is the only fallback now; there is no legacy
	// per-path matcher left for full indexing to call by accident.
	return c
}

func (c allowAllTraversalPolicyContext) WithDirectoryEntries(directoryPath string, entries []os.DirEntry) TraversalPolicyContext {
	return c
}

type safeTraversalPolicyContext struct {
	inner TraversalPolicyContext
}

func (c safeTraversalPolicyContext) ShouldIndexPath(path string, isDir bool) bool {
	if c.inner == nil {
		return true
	}
	return runPolicyCallback("TraversalContext.ShouldIndexPath", func() bool {
		return c.inner.ShouldIndexPath(path, isDir)
	})
}

func (c safeTraversalPolicyContext) Descend(directoryPath string) TraversalPolicyContext {
	if c.inner == nil {
		return allowAllTraversalPolicyContext{}
	}

	var child TraversalPolicyContext
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("filesearch policy TraversalContext.Descend panic recovered: %v", recovered))
				child = nil
			}
		}()
		child = c.inner.Descend(directoryPath)
	}()
	if child == nil {
		return allowAllTraversalPolicyContext{}
	}
	return safeTraversalPolicyContext{
		inner: child,
	}
}

func (c safeTraversalPolicyContext) WithDirectoryEntries(directoryPath string, entries []os.DirEntry) TraversalPolicyContext {
	if c.inner == nil {
		return allowAllTraversalPolicyContext{}
	}

	entryAware, ok := c.inner.(DirectoryEntryAwareTraversalPolicyContext)
	if !ok {
		return c
	}

	var updated TraversalPolicyContext
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("filesearch policy TraversalContext.WithDirectoryEntries panic recovered: %v", recovered))
				updated = nil
			}
		}()
		updated = entryAware.WithDirectoryEntries(directoryPath, entries)
	}()
	if updated == nil {
		return c
	}
	return safeTraversalPolicyContext{
		inner: updated,
	}
}

func (s *policyState) shouldProcessChange(root RootRecord, change ChangeSignal) bool {
	if s == nil {
		return true
	}

	s.mu.RLock()
	callback := s.policy.ShouldProcessChange
	s.mu.RUnlock()
	if callback == nil {
		return true
	}

	return runPolicyCallback("ShouldProcessChange", func() bool {
		return callback(root, change)
	})
}

func runPolicyCallback(name string, callback func() bool) (allowed bool) {
	allowed = true
	defer func() {
		if recovered := recover(); recovered != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("filesearch policy %s panic recovered: %v", name, recovered))
			allowed = true
		}
	}()

	return callback()
}

func statPathType(path string) (bool, bool) {
	if path == "" {
		return false, false
	}

	if info, err := os.Lstat(path); err == nil {
		// Bug fix: change feeds and manual dirty routing must match full scans,
		// which index symlink entries but do not recurse into their targets. Lstat
		// keeps a symlink-to-directory from becoming a subtree dirty scope by
		// accident; Stat remains only as a fallback for unusual filesystems where
		// Lstat cannot classify an existing path.
		return info.IsDir(), true
	}
	if info, err := os.Stat(path); err == nil {
		return info.IsDir(), true
	}

	return false, false
}
