package filesearch

import "path/filepath"

// contentSelection keeps content scope independent of filename discovery. A
// volume-wide directory view must not implicitly authorize content extraction
// outside the configured content roots. The caller owns this single-worker cache.
type contentSelection struct {
	roots    []RootRecord
	policy   *policyState
	contexts map[string]TraversalPolicyContext
}

func newContentSelection(roots []RootRecord, policy Policy, cache bool) *contentSelection {
	selection := &contentSelection{roots: append([]RootRecord(nil), roots...), policy: newPolicyState(policy)}
	if cache {
		selection.contexts = make(map[string]TraversalPolicyContext)
	}
	return selection
}

// allows uses the same traversal policy as discovery, with a bounded parent
// memo during a full crawl. Incremental hooks use fresh contexts so changed
// ignore files are respected without keeping a second invalidation mechanism.
func (s *contentSelection) allows(path string, isDir bool) bool {
	for _, root := range s.roots {
		if !pathWithinScope(root.Path, path) {
			continue
		}
		parent := filepath.Dir(path)
		key := root.ID + "\x00" + root.Path + "\x00" + parent
		policy := s.contexts[key]
		if policy == nil {
			policy = s.policy.newTraversalContext(root, parent)
			if s.contexts != nil {
				if len(s.contexts) >= 4096 {
					clear(s.contexts)
				}
				s.contexts[key] = policy
			}
		}
		if policy.ShouldIndexPath(path, isDir) {
			return true
		}
	}
	return false
}
