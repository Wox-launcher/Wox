package filesearch

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type DirtyQueueConfig struct {
	DebounceWindow            time.Duration
	MaxPendingWaitWindow      time.Duration
	MaxDebounceWindow         time.Duration
	BackpressurePathThreshold int
	BackpressureRootThreshold int
}

type DirtyQueue struct {
	config  DirtyQueueConfig
	mu      sync.Mutex
	pending map[string][]DirtySignal
	// pathRefCount keeps PendingCounts cheap on the watcher enqueue path. The
	// queue still stores full signals for reconcile planning, but status updates
	// should not rescan that slice for every file-system event.
	pathRefCount map[string]int
}

type DirtyQueueStats struct {
	RootCount int
	// Bug fix: keep root-level signals separate from roots that only contain
	// ordinary path changes, so debounce backpressure can treat them differently.
	RootSignalCount int
	PathCount       int
	LatestSignal    time.Time
	EarliestSignal  time.Time
}

func NewDirtyQueue(config DirtyQueueConfig) *DirtyQueue {
	return &DirtyQueue{
		config:       config,
		pending:      map[string][]DirtySignal{},
		pathRefCount: map[string]int{},
	}
}

func (q *DirtyQueue) Push(signal DirtySignal) {
	normalized, ok := normalizeDirtySignal(signal)
	if !ok {
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.pending == nil {
		q.pending = map[string][]DirtySignal{}
	}
	q.pending[normalized.RootID] = append(q.pending[normalized.RootID], normalized)
	q.trackSignalPathLocked(normalized)
}

func (q *DirtyQueue) FlushReady(now time.Time) []ReconcileBatch {
	return q.FlushReadyWithDebounce(now, q.debounceWindow())
}

func (q *DirtyQueue) FlushReadyWithDebounce(now time.Time, debounceWindow time.Duration) []ReconcileBatch {
	q.mu.Lock()
	defer q.mu.Unlock()

	rootIDs := make([]string, 0, len(q.pending))
	for rootID, signals := range q.pending {
		if len(signals) == 0 {
			continue
		}
		if !dirtySignalsReady(now, signals, debounceWindow, q.maxPendingWaitWindow()) {
			continue
		}
		rootIDs = append(rootIDs, rootID)
	}
	sort.Strings(rootIDs)

	batches := make([]ReconcileBatch, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		signals := q.pending[rootID]
		q.untrackSignalsLocked(signals)
		delete(q.pending, rootID)
		batch := buildReconcileBatch(rootID, signals)
		for _, signal := range signals {
			batch.Cursor = newerFSEventsCursor(batch.Cursor, signal.Cursor)
		}
		batches = append(batches, batch)
	}

	return batches
}

// PendingCounts returns exact queued root/path counts without scanning every
// signal. This exists because Scanner refreshes transient pending counts when
// file-watch events arrive, and that path needs predictable O(1) work.
func (q *DirtyQueue) PendingCounts() (int, int) {
	if q == nil {
		return 0, 0
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.pending), len(q.pathRefCount)
}

func (q *DirtyQueue) debounceWindow() time.Duration {
	return q.config.DebounceWindow
}

func (q *DirtyQueue) maxPendingWaitWindow() time.Duration {
	return q.config.MaxPendingWaitWindow
}

func (q *DirtyQueue) Stats() DirtyQueueStats {
	if q == nil {
		return DirtyQueueStats{}
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	return q.statsLocked()
}

func (q *DirtyQueue) statsLocked() DirtyQueueStats {
	stats := DirtyQueueStats{}
	for _, signals := range q.pending {
		if len(signals) == 0 {
			continue
		}

		stats.RootCount++
		hasRootSignal := false
		for _, signal := range signals {
			if stats.LatestSignal.IsZero() || signal.At.After(stats.LatestSignal) {
				stats.LatestSignal = signal.At
			}
			if stats.EarliestSignal.IsZero() || signal.At.Before(stats.EarliestSignal) {
				stats.EarliestSignal = signal.At
			}
			if signal.Kind == DirtySignalKindRoot {
				hasRootSignal = true
				continue
			}
		}
		if hasRootSignal {
			stats.RootSignalCount++
		}
	}

	stats.PathCount = len(q.pathRefCount)
	return stats
}

func (q *DirtyQueue) trackSignalPathLocked(signal DirtySignal) {
	if signal.Kind != DirtySignalKindPath || signal.Path == "" {
		return
	}
	if q.pathRefCount == nil {
		q.pathRefCount = map[string]int{}
	}
	// Optimization: status refresh runs on the watcher enqueue path. Keeping an
	// exact reference count avoids scanning every queued signal just to know
	// whether the UI should show pending dirty work during large FSEvents bursts.
	q.pathRefCount[signal.Path]++
}

func (q *DirtyQueue) untrackSignalsLocked(signals []DirtySignal) {
	if len(signals) == 0 || len(q.pathRefCount) == 0 {
		return
	}
	for _, signal := range signals {
		if signal.Kind != DirtySignalKindPath || signal.Path == "" {
			continue
		}
		count := q.pathRefCount[signal.Path]
		if count <= 1 {
			delete(q.pathRefCount, signal.Path)
			continue
		}
		q.pathRefCount[signal.Path] = count - 1
	}
}

func buildReconcileBatch(rootID string, signals []DirtySignal) ReconcileBatch {
	if len(signals) == 0 {
		return ReconcileBatch{RootID: rootID}
	}

	latestSignal := latestDirtySignal(signals)

	for _, signal := range signals {
		if signal.Kind == DirtySignalKindRoot {
			return ReconcileBatch{
				RootID:         rootID,
				TraceID:        latestSignal.TraceID,
				Mode:           ReconcileModeRoot,
				DirtyPathCount: len(signals),
			}
		}
	}

	directDeltas, directorySignals, subtreeSignals := splitDirtySignals(signals)
	tree := newDirtyPathTree()
	for _, dirtyPath := range uniqueDirtyPaths(subtreeSignals) {
		tree.add(dirtyPath)
	}
	paths := tree.reduce()
	// A recursive rescan already relists every directory beneath it.
	directoryPaths := make([]string, 0, len(directorySignals))
	for _, dirtyPath := range uniqueDirtyPaths(directorySignals) {
		if !tree.covers(dirtyPath) {
			directoryPaths = append(directoryPaths, dirtyPath)
		}
	}

	mode := ReconcileModeSubtree
	if len(paths) == 0 && len(directoryPaths) == 0 && len(directDeltas) > 0 {
		mode = ReconcileModeDirectDelta
	}
	return ReconcileBatch{
		RootID:         rootID,
		TraceID:        latestSignal.TraceID,
		Mode:           mode,
		Paths:          paths,
		DirectoryPaths: directoryPaths,
		DirectDeltas:   directDeltas,
		DirtyPathCount: len(signals),
	}
}

// splitDirtySignals sorts signals into exact file deltas, one-level directory
// relists, and recursive subtree rescans.
func splitDirtySignals(signals []DirtySignal) ([]PathDelta, []DirtySignal, []DirtySignal) {
	deltaByPath := map[string]PathDelta{}
	deltaOrder := make([]string, 0, len(signals))
	directorySignals := make([]DirtySignal, 0)
	subtreeSignals := make([]DirtySignal, 0, len(signals))

	for _, signal := range signals {
		if shouldRelistDirectory(signal) {
			directorySignals = append(directorySignals, signal)
			continue
		}
		if shouldUseDirectDelta(signal) {
			path := cleanDirtyQueuePath(signal.Path)
			if path == "" {
				continue
			}
			if _, exists := deltaByPath[path]; !exists {
				deltaOrder = append(deltaOrder, path)
			}
			// Bug fix: file watcher events were previously collapsed to the parent
			// directory before planning, which made a single file write rescan and
			// diff every sibling. Keep the latest per-file semantic so the planner
			// can build an exact direct-delta job.
			deltaByPath[path] = PathDelta{
				Path:          path,
				SemanticKind:  signal.SemanticKind,
				PathIsDir:     signal.PathIsDir,
				PathTypeKnown: signal.PathTypeKnown,
			}
			continue
		}
		subtreeSignals = append(subtreeSignals, signal)
	}

	sort.Strings(deltaOrder)
	deltas := make([]PathDelta, 0, len(deltaOrder))
	for _, path := range deltaOrder {
		deltas = append(deltas, deltaByPath[path])
	}
	return deltas, directorySignals, subtreeSignals
}

// shouldRelistDirectory selects directory events that describe the directory
// itself (attributes, ownership, xattrs, mtime). Every feed reports a child's
// creation, removal and rename as that child's own event, so relisting one
// level is enough; this is how fsearch handles a folder event. Creates,
// removes, renames and unknown changes stay recursive: a tree moved into place
// emits no events for its descendants.
func shouldRelistDirectory(signal DirtySignal) bool {
	if signal.Kind != DirtySignalKindPath || !signal.PathTypeKnown || !signal.PathIsDir {
		return false
	}
	return signal.SemanticKind == ChangeSemanticKindModify || signal.SemanticKind == ChangeSemanticKindMetadata
}

func shouldUseDirectDelta(signal DirtySignal) bool {
	if signal.Kind != DirtySignalKindPath {
		return false
	}
	if !signal.PathTypeKnown {
		// A path that vanished before its type could be read is deleted by path.
		// The column delete drops a file row or a directory row alike, and
		// applyEntries also deletes a directory's stored descendants. Scanning
		// the parent instead could not prune a vanished child of the root, which
		// used to force a whole-root reconcile.
		return signal.SemanticKind == ChangeSemanticKindRemove || signal.SemanticKind == ChangeSemanticKindRename
	}
	if signal.PathIsDir {
		return false
	}
	switch signal.SemanticKind {
	case ChangeSemanticKindCreate, ChangeSemanticKindModify, ChangeSemanticKindMetadata, ChangeSemanticKindRemove, ChangeSemanticKindRename, ChangeSemanticKindUnknown, "":
		return true
	default:
		return false
	}
}

func normalizeDirtySignal(signal DirtySignal) (DirtySignal, bool) {
	signal.Path = cleanDirtyQueuePath(signal.Path)
	if signal.RootID == "" {
		return DirtySignal{}, false
	}

	switch signal.Kind {
	case DirtySignalKindRoot:
		if signal.Path == "" {
			// Root dirties are valid without a path.
		}
	case DirtySignalKindPath:
		if signal.Path == "" {
			return DirtySignal{}, false
		}
	default:
		if signal.Path == "" {
			return DirtySignal{}, false
		}
		signal.Kind = DirtySignalKindPath
	}

	if signal.At.IsZero() {
		signal.At = time.Now()
	}

	return signal, true
}

func uniqueDirtyPaths(signals []DirtySignal) []string {
	seen := map[string]struct{}{}
	paths := make([]string, 0, len(signals))
	for _, signal := range signals {
		dirtyPath, ok := dirtyPathForSignal(signal)
		if !ok {
			continue
		}
		if _, exists := seen[dirtyPath]; exists {
			continue
		}
		seen[dirtyPath] = struct{}{}
		paths = append(paths, dirtyPath)
	}

	sort.Strings(paths)
	return paths
}

func dirtyPathForSignal(signal DirtySignal) (string, bool) {
	switch signal.Kind {
	case DirtySignalKindRoot:
		return cleanDirtyQueuePath(signal.Path), true
	case DirtySignalKindPath:
		return scopePathForDirtySignal(signal.Path, signal.PathIsDir, signal.PathTypeKnown)
	default:
		return scopePathForDirtySignal(signal.Path, signal.PathIsDir, signal.PathTypeKnown)
	}
}

func scopePathForDirtySignal(path string, pathIsDir bool, pathTypeKnown bool) (string, bool) {
	path = cleanDirtyQueuePath(path)
	if path == "" {
		return "", false
	}

	if pathTypeKnown && pathIsDir {
		return path, true
	}

	parent := filepath.Dir(path)
	if parent == "." || parent == path || parent == string(filepath.Separator) && path == parent {
		return path, true
	}

	return parent, true
}

// dirtyPathTree drops dirty scopes already covered by a dirty ancestor. Sibling
// scopes are never merged into their parent: fsearch never widens a refresh,
// and merging eight dirty folders used to rescan every other child of the
// parent as well.
type dirtyPathTree struct {
	root *dirtyPathNode
}

type dirtyPathNode struct {
	path     string
	dirty    bool
	children map[string]*dirtyPathNode
}

func newDirtyPathTree() *dirtyPathTree {
	return &dirtyPathTree{
		root: &dirtyPathNode{
			path:     "",
			children: map[string]*dirtyPathNode{},
		},
	}
}

func (t *dirtyPathTree) add(path string) {
	if path == "" {
		return
	}

	node := t.root
	rootPath, segments := splitDirtyPath(path)
	if rootPath != "" {
		if node.children == nil {
			node.children = map[string]*dirtyPathNode{}
		}
		child := node.children[rootPath]
		if child == nil {
			child = &dirtyPathNode{
				path:     rootPath,
				children: map[string]*dirtyPathNode{},
			}
			node.children[rootPath] = child
		}
		node = child
	}

	if len(segments) == 0 {
		node.dirty = true
		return
	}

	for _, segment := range segments {
		if node.children == nil {
			node.children = map[string]*dirtyPathNode{}
		}
		child := node.children[segment]
		if child == nil {
			child = &dirtyPathNode{
				path:     joinDirtyPath(node.path, segment),
				children: map[string]*dirtyPathNode{},
			}
			node.children[segment] = child
		}
		node = child
	}
	node.dirty = true
}

func (t *dirtyPathTree) reduce() []string {
	return reduceDirtyPathNode(t.root)
}

// covers reports whether path equals or lies beneath a dirty scope.
func (t *dirtyPathTree) covers(path string) bool {
	rootPath, segments := splitDirtyPath(path)
	node := t.root
	if rootPath != "" {
		node = node.children[rootPath]
	}
	for _, segment := range segments {
		if node == nil || node.dirty {
			break
		}
		node = node.children[segment]
	}
	return node != nil && node.dirty
}

func reduceDirtyPathNode(node *dirtyPathNode) []string {
	if node == nil {
		return nil
	}
	if node.dirty {
		if node.path == "" {
			return nil
		}
		return []string{node.path}
	}

	childKeys := make([]string, 0, len(node.children))
	for key := range node.children {
		childKeys = append(childKeys, key)
	}
	sort.Strings(childKeys)

	paths := make([]string, 0, len(node.children))
	for _, key := range childKeys {
		paths = append(paths, reduceDirtyPathNode(node.children[key])...)
	}
	return paths
}

func dirtySignalsReady(now time.Time, signals []DirtySignal, debounceWindow time.Duration, maxPendingWaitWindow time.Duration) bool {
	if len(signals) == 0 {
		return false
	}

	latest := latestDirtySignalAt(signals)
	if debounceWindow <= 0 || now.Sub(latest) >= debounceWindow {
		return true
	}

	if maxPendingWaitWindow <= 0 {
		return false
	}

	earliest := earliestDirtySignalAt(signals)
	// Bug fix: a queue that only waits for the latest event can starve forever
	// when FSEvents keeps reporting unrelated background writes. The max-pending
	// window keeps burst coalescing but guarantees an old pending root eventually
	// gets planned even if the quiet window has not been reached.
	return now.Sub(earliest) >= maxPendingWaitWindow
}

func latestDirtySignalAt(signals []DirtySignal) time.Time {
	latest := signals[0].At
	for i := 1; i < len(signals); i++ {
		if signals[i].At.After(latest) {
			latest = signals[i].At
		}
	}
	return latest
}

func earliestDirtySignalAt(signals []DirtySignal) time.Time {
	earliest := signals[0].At
	for i := 1; i < len(signals); i++ {
		if signals[i].At.Before(earliest) {
			earliest = signals[i].At
		}
	}
	return earliest
}

func latestDirtySignal(signals []DirtySignal) DirtySignal {
	latest := signals[0]
	for i := 1; i < len(signals); i++ {
		if signals[i].At.After(latest.At) {
			latest = signals[i]
		}
	}
	return latest
}

func cleanDirtyQueuePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func splitDirtyPath(path string) (string, []string) {
	root := filepath.VolumeName(path)
	trimmed := path
	if root != "" {
		trimmed = strings.TrimPrefix(trimmed, root)
	}
	if strings.HasPrefix(trimmed, string(filepath.Separator)) {
		root += string(filepath.Separator)
		trimmed = strings.TrimPrefix(trimmed, string(filepath.Separator))
	}

	parts := strings.Split(trimmed, string(filepath.Separator))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		out = append(out, part)
	}
	return root, out
}

func joinDirtyPath(parent, segment string) string {
	if parent == "" {
		return segment
	}
	return filepath.Join(parent, segment)
}
