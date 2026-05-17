package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"wox/util"

	"github.com/google/uuid"
)

type DynamicRootConfig struct {
	Enabled                    bool
	Window                     time.Duration
	MinChangeCount             int
	MinFlushCount              int
	MinDepthBelowRoot          int
	IdleDemotionAfter          time.Duration
	MaxDynamicRootsPerUserRoot int
	MaxDynamicRootsGlobal      int
}

type dynamicRootHeatTracker struct {
	mu     sync.Mutex
	states map[string]*dynamicRootHeatState
}

type dynamicRootHeatState struct {
	rootID  string
	path    string
	count   int
	firstAt time.Time
	lastAt  time.Time
	flushes map[int]struct{}
}

type dynamicRootHeatCandidate struct {
	rootID string
	path   string
	count  int
	lastAt time.Time
}

func defaultDynamicRootConfig() DynamicRootConfig {
	return DynamicRootConfig{
		Enabled: true,
		Window:  15 * time.Minute,
		// Hot directories should split quickly enough to stop repeated parent
		// rescans during active development. Five changes still requires two
		// successful dirty flushes, so one noisy save cannot promote by itself.
		MinChangeCount:             5,
		MinFlushCount:              2,
		MinDepthBelowRoot:          2,
		IdleDemotionAfter:          24 * time.Hour,
		MaxDynamicRootsPerUserRoot: 16,
		MaxDynamicRootsGlobal:      64,
	}
}

func normalizeDynamicRootConfig(config DynamicRootConfig) DynamicRootConfig {
	if !config.Enabled {
		return DynamicRootConfig{Enabled: false}
	}

	defaults := defaultDynamicRootConfig()
	if config.Window <= 0 {
		config.Window = defaults.Window
	}
	if config.MinChangeCount <= 0 {
		config.MinChangeCount = defaults.MinChangeCount
	}
	if config.MinFlushCount <= 0 {
		config.MinFlushCount = defaults.MinFlushCount
	}
	if config.MinDepthBelowRoot <= 0 {
		config.MinDepthBelowRoot = defaults.MinDepthBelowRoot
	}
	if config.IdleDemotionAfter <= 0 {
		config.IdleDemotionAfter = defaults.IdleDemotionAfter
	}
	if config.MaxDynamicRootsPerUserRoot <= 0 {
		config.MaxDynamicRootsPerUserRoot = defaults.MaxDynamicRootsPerUserRoot
	}
	if config.MaxDynamicRootsGlobal <= 0 {
		config.MaxDynamicRootsGlobal = defaults.MaxDynamicRootsGlobal
	}
	return config
}

func newDynamicRootHeatTracker() *dynamicRootHeatTracker {
	return &dynamicRootHeatTracker{states: map[string]*dynamicRootHeatState{}}
}

func (h *dynamicRootHeatTracker) record(rootID string, hotPath string, at time.Time, config DynamicRootConfig) {
	if h == nil || rootID == "" || hotPath == "" {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	key := dynamicHeatKey(rootID, hotPath)
	state := h.states[key]
	if state == nil || at.Sub(state.firstAt) > config.Window {
		state = &dynamicRootHeatState{
			rootID: rootID,
			// Optimization: heat state is compared against each flushed delta and
			// scope path later. Store the canonical path once so markSuccessfulFlush
			// only cleans the varying batch inputs inside their own loops.
			path:    filepath.Clean(hotPath),
			firstAt: at,
			flushes: map[int]struct{}{},
		}
		h.states[key] = state
	}
	state.count++
	state.lastAt = at
}

func (h *dynamicRootHeatTracker) markSuccessfulFlush(batches []ReconcileBatch, generation int) {
	if h == nil || generation <= 0 || len(batches) == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for _, state := range h.states {
		for _, batch := range batches {
			if batch.RootID != state.rootID {
				continue
			}
			if batch.Mode == ReconcileModeRoot {
				state.flushes[generation] = struct{}{}
				break
			}
			// Feature addition: file-level deltas have no subtree scope, so they
			// must be matched by their exact paths instead of being treated like a
			// root flush. Otherwise one tiny file update would satisfy heat checks
			// for every dynamic child under the same configured root.
			for _, delta := range batch.DirectDeltas {
				cleanDeltaPath := filepath.Clean(delta.Path)
				if cleanPathsOverlap(cleanDeltaPath, state.path) {
					state.flushes[generation] = struct{}{}
					break
				}
			}
			if _, ok := state.flushes[generation]; ok {
				break
			}
			if len(batch.Paths) == 0 {
				continue
			}
			for _, scopePath := range batch.Paths {
				cleanScopePath := filepath.Clean(scopePath)
				if cleanPathsOverlap(cleanScopePath, state.path) {
					state.flushes[generation] = struct{}{}
					break
				}
			}
			if _, ok := state.flushes[generation]; ok {
				break
			}
		}
	}
}

func (h *dynamicRootHeatTracker) readyCandidates(config DynamicRootConfig, now time.Time) []dynamicRootHeatCandidate {
	if h == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	candidates := make([]dynamicRootHeatCandidate, 0)
	for key, state := range h.states {
		if now.Sub(state.firstAt) > config.Window {
			delete(h.states, key)
			continue
		}
		if state.count < config.MinChangeCount || len(state.flushes) < config.MinFlushCount {
			continue
		}
		candidates = append(candidates, dynamicRootHeatCandidate{
			rootID: state.rootID,
			path:   state.path,
			count:  state.count,
			lastAt: state.lastAt,
		})
	}
	sort.Slice(candidates, func(left int, right int) bool {
		if candidates[left].count == candidates[right].count {
			return candidates[left].path < candidates[right].path
		}
		return candidates[left].count > candidates[right].count
	})
	return candidates
}

func (h *dynamicRootHeatTracker) clear(rootID string, hotPath string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.states, dynamicHeatKey(rootID, hotPath))
}

func dynamicHeatKey(rootID string, hotPath string) string {
	return rootID + "\x00" + filepath.Clean(hotPath)
}

func (s *Scanner) recordDynamicRootHeat(root RootRecord, signal ChangeSignal) {
	config := normalizeDynamicRootConfig(s.dynamicRootConfig)
	if !config.Enabled || signal.Kind != ChangeSignalKindDirtyPath || root.Kind == RootKindDynamic {
		return
	}
	hotPath := dynamicHotDirectoryForSignal(signal)
	if hotPath == "" || !pathWithinScope(root.Path, hotPath) || filepath.Clean(root.Path) == hotPath {
		return
	}
	if pathDepthBelowRoot(root.Path, hotPath) < config.MinDepthBelowRoot {
		return
	}
	if s.dynamicHeat == nil {
		s.dynamicHeat = newDynamicRootHeatTracker()
	}
	// Signal-time counting preserves the real hot directory before DirtyQueue
	// merges paths into a root-level batch. Promotion is still gated later by a
	// successful flush generation so failed reconciles do not create roots.
	s.dynamicHeat.record(root.ID, hotPath, signal.At, config)
}

func dynamicHotDirectoryForSignal(signal ChangeSignal) string {
	if strings.TrimSpace(signal.Path) == "" {
		return ""
	}
	cleanPath := filepath.Clean(signal.Path)
	if signal.PathTypeKnown && signal.PathIsDir {
		return cleanPath
	}
	return filepath.Dir(cleanPath)
}

func pathDepthBelowRoot(rootPath string, path string) int {
	rel, err := filepath.Rel(filepath.Clean(rootPath), filepath.Clean(path))
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return 0
	}
	depth := 0
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		if segment != "" && segment != "." {
			depth++
		}
	}
	return depth
}

func (s *Scanner) handleSuccessfulDirtyFlush(ctx context.Context, batches []ReconcileBatch, now time.Time) error {
	config := normalizeDynamicRootConfig(s.dynamicRootConfig)
	if !config.Enabled {
		return nil
	}
	if s.dynamicHeat == nil {
		s.dynamicHeat = newDynamicRootHeatTracker()
	}

	s.dynamicFlushGeneration++
	s.dynamicHeat.markSuccessfulFlush(batches, s.dynamicFlushGeneration)
	if err := s.updateDynamicRootHotTimes(ctx, batches, now); err != nil {
		return err
	}
	if err := s.promoteReadyDynamicRoots(ctx, now); err != nil {
		return err
	}
	return s.demoteIdleDynamicRoots(ctx, now)
}

func (s *Scanner) updateDynamicRootHotTimes(ctx context.Context, batches []ReconcileBatch, now time.Time) error {
	if len(batches) == 0 {
		return nil
	}
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		return err
	}
	rootByID := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		rootByID[root.ID] = root
	}

	seen := map[string]struct{}{}
	for _, batch := range batches {
		if _, ok := seen[batch.RootID]; ok {
			continue
		}
		seen[batch.RootID] = struct{}{}
		root, ok := rootByID[batch.RootID]
		if !ok || root.Kind != RootKindDynamic {
			continue
		}
		// Dynamic roots cannot promote nested children in v1, but their own
		// successful dirty flush should refresh LastHotAt so idle demotion only
		// reclaims genuinely cold splits.
		root.LastHotAt = now.UnixMilli()
		root.UpdatedAt = util.GetSystemTimestamp()
		if err := s.updateRootStateAndCache(ctx, root); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) promoteReadyDynamicRoots(ctx context.Context, now time.Time) error {
	config := normalizeDynamicRootConfig(s.dynamicRootConfig)
	if !config.Enabled || s.dynamicHeat == nil {
		return nil
	}

	candidates := s.dynamicHeat.readyCandidates(config, now)
	for _, candidate := range candidates {
		if err := s.promoteDynamicRootCandidate(ctx, candidate, config, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) promoteDynamicRootCandidate(ctx context.Context, candidate dynamicRootHeatCandidate, config DynamicRootConfig, now time.Time) error {
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		return err
	}
	parentRoot, ok := rootByID(roots, candidate.rootID)
	if !ok || parentRoot.Kind == RootKindDynamic {
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}
	candidatePath := filepath.Clean(candidate.path)
	if !pathWithinScope(parentRoot.Path, candidatePath) || candidatePath == filepath.Clean(parentRoot.Path) {
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}
	info, err := os.Stat(candidatePath)
	if err != nil || !info.IsDir() {
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}
	if rootAtPathExists(roots, candidatePath) || dynamicRootNestingConflict(roots, candidatePath) {
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}
	if !s.shouldProcessChange(parentRoot, ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		RootID:        parentRoot.ID,
		Path:          candidatePath,
		PathIsDir:     true,
		PathTypeKnown: true,
	}) {
		// Bug fix: hot-path promotion must honor the same ignore rules as the
		// scanner. Otherwise an ignored noisy subtree can become its own dynamic
		// root and keep receiving watcher events even though the parent policy would
		// have excluded it.
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}
	if !dynamicRootCapsAllow(roots, parentRoot.ID, config) {
		if fileSearchDiagnosticLoggingEnabled {
			// Diagnostic logging: cap skips are expected during noisy home-root
			// watcher bursts. Keep the reason available for investigations without
			// making every rejected hot directory write an info log by default.
			util.GetLogger().Info(ctx, fmt.Sprintf(
				"filesearch dynamic root promotion skipped by cap: parent=%s path=%s",
				parentRoot.ID,
				summarizeLogPath(candidatePath),
			))
		}
		s.dynamicHeat.clear(candidate.rootID, candidate.path)
		return nil
	}

	policyRootPath := strings.TrimSpace(parentRoot.PolicyRootPath)
	if policyRootPath == "" {
		policyRootPath = parentRoot.Path
	}
	dynamicRoot := RootRecord{
		ID:                  uuid.NewString(),
		Path:                candidatePath,
		Kind:                RootKindDynamic,
		Status:              RootStatusIdle,
		FeedType:            parentRoot.FeedType,
		FeedState:           RootFeedStateReady,
		DynamicParentRootID: parentRoot.ID,
		PolicyRootPath:      policyRootPath,
		PromotedAt:          now.UnixMilli(),
		LastHotAt:           candidate.lastAt.UnixMilli(),
		CreatedAt:           now.UnixMilli(),
		UpdatedAt:           now.UnixMilli(),
	}
	if err := s.db.PromoteDynamicRoot(ctx, parentRoot, dynamicRoot); err != nil {
		return err
	}
	s.dynamicHeat.clear(candidate.rootID, candidate.path)
	// A freshly promoted root needs its own reconcile so subsequent searches use
	// the same path rows but with dynamic-root ownership and inherited policy.
	s.enqueueDirtyWithContext(ctx, DirtySignal{
		Kind:          DirtySignalKindRoot,
		RootID:        dynamicRoot.ID,
		Path:          dynamicRoot.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		At:            now,
	})
	s.refreshChangeFeed(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch dynamic root promoted: parent=%s dynamic=%s path=%s changes=%d",
		parentRoot.ID,
		dynamicRoot.ID,
		summarizeLogPath(dynamicRoot.Path),
		candidate.count,
	))
	return nil
}

func (s *Scanner) demoteIdleDynamicRoots(ctx context.Context, now time.Time) error {
	config := normalizeDynamicRootConfig(s.dynamicRootConfig)
	if !config.Enabled || config.IdleDemotionAfter <= 0 {
		return nil
	}
	roots, err := s.db.ListRoots(ctx)
	if err != nil {
		return err
	}
	for _, root := range roots {
		if root.Kind != RootKindDynamic {
			continue
		}
		lastHotAt := root.LastHotAt
		if lastHotAt <= 0 {
			lastHotAt = root.PromotedAt
		}
		if lastHotAt <= 0 || now.Sub(time.UnixMilli(lastHotAt)) < config.IdleDemotionAfter {
			continue
		}
		parentRoot, ok := rootByID(roots, root.DynamicParentRootID)
		if !ok {
			continue
		}
		if err := s.db.DemoteDynamicRoot(ctx, parentRoot, root); err != nil {
			return err
		}
		s.enqueueDirtyWithContext(ctx, DirtySignal{
			Kind:          DirtySignalKindPath,
			RootID:        parentRoot.ID,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			At:            now,
		})
		s.refreshChangeFeed(ctx)
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"filesearch dynamic root demoted: parent=%s dynamic=%s path=%s",
			parentRoot.ID,
			root.ID,
			summarizeLogPath(root.Path),
		))
	}
	return nil
}

func rootByID(roots []RootRecord, rootID string) (RootRecord, bool) {
	for _, root := range roots {
		if root.ID == rootID {
			return root, true
		}
	}
	return RootRecord{}, false
}

func rootAtPathExists(roots []RootRecord, path string) bool {
	cleanPath := filepath.Clean(path)
	for _, root := range roots {
		if filepath.Clean(root.Path) == cleanPath {
			return true
		}
	}
	return false
}

func dynamicRootNestingConflict(roots []RootRecord, path string) bool {
	cleanPath := filepath.Clean(path)
	for _, root := range roots {
		if root.Kind != RootKindDynamic {
			continue
		}
		cleanRootPath := filepath.Clean(root.Path)
		if pathWithinScope(cleanRootPath, cleanPath) || pathWithinScope(cleanPath, cleanRootPath) {
			return true
		}
	}
	return false
}

func dynamicRootCapsAllow(roots []RootRecord, parentRootID string, config DynamicRootConfig) bool {
	globalCount := 0
	parentCount := 0
	for _, root := range roots {
		if root.Kind != RootKindDynamic {
			continue
		}
		globalCount++
		if root.DynamicParentRootID == parentRootID {
			parentCount++
		}
	}
	return globalCount < config.MaxDynamicRootsGlobal && parentCount < config.MaxDynamicRootsPerUserRoot
}
