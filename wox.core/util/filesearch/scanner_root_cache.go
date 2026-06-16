package filesearch

import (
	"context"
	"errors"
	"sort"
)

// replaceRootCache installs a complete scanner-owned root snapshot. Change-feed
// signal handling is an event hot path, so resolved root state must come from an
// immutable in-memory view instead of paying a SQLite round trip per signal.
func (s *Scanner) replaceRootCache(roots []RootRecord) {
	if s == nil {
		return
	}

	next := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		if root.ID == "" {
			continue
		}
		next[root.ID] = root
	}

	s.rootCacheMu.Lock()
	s.rootCacheByID = next
	s.rootCacheLoaded = true
	s.rootCacheMu.Unlock()
}

// upsertRootCache updates one root only after a DB write has succeeded. It
// intentionally no-ops before a full snapshot is loaded so partial state writes
// cannot make an unknown root look definitively absent.
func (s *Scanner) upsertRootCache(root RootRecord) {
	if s == nil || root.ID == "" {
		return
	}

	s.rootCacheMu.Lock()
	defer s.rootCacheMu.Unlock()
	if !s.rootCacheLoaded {
		return
	}
	if s.rootCacheByID == nil {
		s.rootCacheByID = map[string]RootRecord{}
	}
	s.rootCacheByID[root.ID] = root
}

// seedRootCacheLookup stores a DB lookup result as a partial cache entry while
// leaving rootCacheLoaded=false. The hot path still treats this as a cold cache
// and falls back to SQLite on later lookups, which avoids routing with stale
// partial state before refreshChangeFeedWithRoots installs a complete snapshot.
func (s *Scanner) seedRootCacheLookup(root RootRecord) {
	if s == nil || root.ID == "" {
		return
	}

	s.rootCacheMu.Lock()
	defer s.rootCacheMu.Unlock()
	if s.rootCacheByID == nil {
		s.rootCacheByID = map[string]RootRecord{}
	}
	s.rootCacheByID[root.ID] = root
}

// invalidateRootCache clears both entries and the completeness marker. Root
// collection changes alter the set of valid IDs, so the next signal must reload
// from DB instead of routing with a complete-but-stale snapshot.
func (s *Scanner) invalidateRootCache() {
	if s == nil {
		return
	}

	s.rootCacheMu.Lock()
	s.rootCacheByID = nil
	s.rootCacheLoaded = false
	s.rootCacheMu.Unlock()
}

// cachedRootByID returns both hit state and whether the cache represents a full
// root snapshot. Callers use the loaded flag to distinguish a definitive miss
// from a cold cache that should still consult SQLite.
func (s *Scanner) cachedRootByID(rootID string) (RootRecord, bool, bool) {
	if s == nil || rootID == "" {
		return RootRecord{}, false, false
	}

	s.rootCacheMu.RLock()
	defer s.rootCacheMu.RUnlock()
	root, found := s.rootCacheByID[rootID]
	return root, found, s.rootCacheLoaded
}

func (s *Scanner) cachedRootSnapshot() ([]RootRecord, bool) {
	if s == nil {
		return nil, false
	}

	s.rootCacheMu.RLock()
	defer s.rootCacheMu.RUnlock()
	if !s.rootCacheLoaded {
		return nil, false
	}

	roots := make([]RootRecord, 0, len(s.rootCacheByID))
	for _, root := range s.rootCacheByID {
		roots = append(roots, root)
	}
	sort.Slice(roots, func(i int, j int) bool {
		if roots[i].Path == roots[j].Path {
			return roots[i].ID < roots[j].ID
		}
		return roots[i].Path < roots[j].Path
	})
	// Optimization: status notifications are emitted from the watcher enqueue
	// path. Returning a deterministic copy lets Engine.GetStatus avoid ListRoots
	// during signal bursts while preserving the caller-owned slice semantics of
	// the previous DB-backed implementation.
	return roots, true
}

func (s *Scanner) updateRootStateAndCache(ctx context.Context, root RootRecord) error {
	if s == nil || s.db == nil {
		return errors.New("filesearch scanner database is not open")
	}
	if err := s.db.UpdateRootState(ctx, root); err != nil {
		return err
	}
	// Optimization: keep the hot-path root cache coherent with Scanner-owned
	// state writes. Failed DB writes deliberately do not update memory so routing
	// cannot observe state that was never persisted.
	s.upsertRootCache(root)
	return nil
}
