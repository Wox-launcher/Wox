package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"

	"github.com/google/uuid"
)

type Engine struct {
	mu              sync.RWMutex
	resetMu         sync.Mutex
	closed          bool
	db              *FileSearchDB
	contentDB       *ContentSearchDB
	searchProvider  *SQLiteSearchProvider
	scanner         *Scanner
	policy          *policyState
	statusListeners *util.HashMap[string, func(StatusSnapshot)]
	contentHook     ContentHook
}

func NewEngine(ctx context.Context) (*Engine, error) {
	return NewEngineWithOptions(ctx, DefaultEngineOptions())
}

func NewEngineWithOptions(ctx context.Context, options EngineOptions) (*Engine, error) {
	db, err := NewFileSearchDB(ctx)
	if err != nil {
		return nil, err
	}

	policyState := newPolicyState(options.Policy)
	engine := &Engine{
		db:              db,
		searchProvider:  NewSQLiteSearchProvider(db),
		policy:          policyState,
		statusListeners: util.NewHashMap[string, func(StatusSnapshot)](),
	}

	engine.scanner = newScannerWithPolicyState(db, policyState)
	engine.scanner.SetStateChangeHandler(engine.notifyStatusChanged)

	// Keep the built-in file engine focused on the persisted SQLite search index.
	// The previous runtime mirrored every entry into a second in-memory provider,
	// which doubled storage responsibilities and made memory usage scale with the
	// number of indexed roots.
	if db.NeedsSearchArtifactRebuild() {
		engine.startScannerAfterSearchArtifactRebuild(ctx)
	} else {
		engine.scanner.Start(util.NewTraceContext())
	}
	util.GetLogger().Info(ctx, "filesearch engine initialized: indexed_provider=sqlite-search")
	engine.logInitSnapshotAsync(ctx)

	return engine, nil
}

// startScannerAfterSearchArtifactRebuild keeps schema migration work off the plugin init path.
func (e *Engine) startScannerAfterSearchArtifactRebuild(ctx context.Context) {
	util.Go(ctx, "filesearch search artifact rebuild", func() {
		rebuildCtx := util.NewTraceContext()

		e.resetMu.Lock()
		defer e.resetMu.Unlock()

		e.mu.RLock()
		if e.closed || e.db == nil || e.scanner == nil {
			e.mu.RUnlock()
			return
		}
		db := e.db
		scanner := e.scanner
		e.mu.RUnlock()

		startedAt := util.GetSystemTimestamp()
		util.GetLogger().Info(rebuildCtx, "filesearch search artifact rebuild started")
		if err := db.RebuildSearchArtifacts(rebuildCtx); err != nil {
			util.GetLogger().Warn(rebuildCtx, "filesearch search artifact rebuild failed: "+err.Error())
		} else {
			util.GetLogger().Info(rebuildCtx, fmt.Sprintf("filesearch search artifact rebuild finished, cost %d ms", util.GetSystemTimestamp()-startedAt))
		}

		e.mu.RLock()
		shouldStartScanner := !e.closed && e.scanner == scanner
		e.mu.RUnlock()
		if shouldStartScanner {
			scanner.Start(util.NewTraceContext())
		}
	})
}

func (e *Engine) logInitSnapshotAsync(ctx context.Context) {
	if e == nil || e.db == nil {
		return
	}
	if !shouldCollectFileSearchDiagnosticSnapshot() {
		// Optimization: init snapshots are logging-only and can scan FTS vocab
		// tables, so skip the goroutine entirely outside dev diagnostics.
		return
	}

	// Capture the heavy SQLite snapshot after engine init returns because the
	// previous synchronous fts5vocab sampling blocked plugin initialization on
	// large databases. That prevented the file plugin instance from registering,
	// so `f ` stopped entering file-plugin query mode even though the engine was
	// otherwise healthy.
	util.Go(ctx, "filesearch init sqlite snapshot", func() {
		snapshotCtx, cancel := context.WithTimeout(util.NewTraceContext(), 30*time.Second)
		defer cancel()

		e.mu.RLock()
		if e.closed || e.db == nil {
			e.mu.RUnlock()
			return
		}
		snapshot, err := e.db.SearchIndexSnapshot(snapshotCtx)
		e.mu.RUnlock()
		if err != nil {
			util.GetLogger().Warn(snapshotCtx, "filesearch failed to capture sqlite snapshot during init: "+err.Error())
			return
		}
		logSQLiteIndexSnapshot(snapshotCtx, "engine_initialized", snapshot, true)
	})
}

func (e *Engine) UpdatePolicy(policy Policy) {
	if e == nil {
		return
	}
	if e.policy != nil {
		e.policy.Set(policy)
	}

	e.mu.RLock()
	scanner := e.scanner
	e.mu.RUnlock()
	if scanner != nil {
		scanner.RequestRescan(util.NewTraceContext())
	}
}

func (e *Engine) ResetIndex(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.mu.RLock()
	if e.closed || e.db == nil {
		e.mu.RUnlock()
		return nil
	}
	scanner := e.scanner
	db := e.db
	e.mu.RUnlock()

	if scanner != nil {
		return scanner.RequestResetRescan(ctx)
	}
	if db != nil {
		return db.ResetIndex(ctx)
	}
	return nil
}

func (e *Engine) RebuildIndex(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.resetMu.Lock()
	defer e.resetMu.Unlock()

	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return fmt.Errorf("filesearch engine closed")
	}
	oldScanner := e.scanner
	e.mu.RUnlock()

	if oldScanner != nil {
		oldScanner.StopAndWait()
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return fmt.Errorf("filesearch engine closed")
	}

	oldDB := e.db
	oldContentDB := e.contentDB
	e.closeContentHookLocked()
	e.db = nil
	e.contentDB = nil
	e.searchProvider = nil
	e.scanner = nil
	var closeErr error
	if oldDB != nil {
		if err := oldDB.Close(); err != nil {
			closeErr = fmt.Errorf("close old filesearch database: %w", err)
		}
	}
	if oldContentDB != nil {
		if err := oldContentDB.Close(); err != nil {
			if closeErr == nil {
				closeErr = fmt.Errorf("close old content search database: %w", err)
			}
		}
	}
	if closeErr != nil {
		e.mu.Unlock()
		return closeErr
	}

	fileSearchDir := util.GetLocation().GetFileSearchDirectory()
	// Feature addition: "Index Files" is now a true rebuild. The previous reset
	// deleted rows inside the live SQLite database, which left WAL/SHM files,
	// old pragmas, and any corrupted side tables in place. Close SQLite first,
	// remove the whole storage directory, then open a fresh database before
	// writing configured roots and starting the scan.
	if err := os.RemoveAll(fileSearchDir); err != nil {
		e.mu.Unlock()
		return fmt.Errorf("remove filesearch directory: %w", err)
	}

	newDB, err := NewFileSearchDB(ctx)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	newScanner := newScannerWithPolicyState(newDB, e.policy)
	newScanner.SetStateChangeHandler(e.notifyStatusChanged)
	newProvider := NewSQLiteSearchProvider(newDB)

	e.db = newDB
	e.searchProvider = newProvider
	e.scanner = newScanner
	e.mu.Unlock()

	newScanner.Start(util.NewTraceContext())
	util.GetLogger().Info(ctx, fmt.Sprintf("filesearch storage rebuilt: directory=%s", fileSearchDir))
	return nil
}

func (e *Engine) Close() error {
	if e == nil {
		return nil
	}

	e.resetMu.Lock()
	defer e.resetMu.Unlock()

	e.mu.RLock()
	scanner := e.scanner
	e.mu.RUnlock()
	if scanner != nil {
		scanner.StopAndWait()
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
	if e.contentHook != nil {
		e.contentHook.Close()
		e.contentHook = nil
	}
	if e.db != nil {
		err := e.db.Close()
		e.db = nil
		e.searchProvider = nil
		e.scanner = nil
		if e.contentDB != nil {
			contentErr := e.contentDB.Close()
			e.contentDB = nil
			if err == nil {
				err = contentErr
			}
		}
		return err
	}
	if e.contentDB != nil {
		err := e.contentDB.Close()
		e.contentDB = nil
		return err
	}
	return nil
}

func (e *Engine) AddRoot(ctx context.Context, rootPath string) error {
	if e == nil {
		return nil
	}
	cleaned := filepath.Clean(rootPath)
	info, err := os.Stat(cleaned)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("filesearch root is not a directory: %s", cleaned)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.db == nil {
		return fmt.Errorf("filesearch engine closed")
	}

	existing, err := e.db.FindRootByPath(ctx, cleaned)
	if err != nil {
		return err
	}
	now := util.GetSystemTimestamp()
	if existing != nil {
		existing.Kind = RootKindUser
		existing.DynamicParentRootID = ""
		existing.PolicyRootPath = ""
		existing.PromotedAt = 0
		existing.LastHotAt = 0
		existing.UpdatedAt = now
		existing.Status = RootStatusPreparing
		// A user-added path can collide with a hidden dynamic root. Clearing the
		// lifecycle fields here makes that path a real user root instead of
		// leaving stale parent-policy metadata attached to the reused row.
		if err := e.db.UpsertRoot(ctx, *existing); err != nil {
			return err
		}
	} else {
		root := RootRecord{
			ID:        uuid.NewString(),
			Path:      cleaned,
			Kind:      RootKindUser,
			Status:    RootStatusPreparing,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := e.db.UpsertRoot(ctx, root); err != nil {
			return err
		}
	}

	if e.scanner != nil {
		// Root membership changed in SQLite; invalidate before the rescan request
		// so watcher signals arriving during the scheduling window cannot route
		// against the previous complete root snapshot.
		e.scanner.invalidateRootCache()
		e.scanner.RequestRescan(ctx)
	}
	return nil
}

func (e *Engine) RemoveRoot(ctx context.Context, rootPath string) error {
	if e == nil {
		return nil
	}
	cleaned := filepath.Clean(rootPath)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.db == nil {
		return fmt.Errorf("filesearch engine closed")
	}

	root, err := e.db.FindRootByPath(ctx, cleaned)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}

	if err := e.db.DeleteRoot(ctx, root.ID); err != nil {
		return err
	}

	if e.scanner != nil {
		// Root membership changed in SQLite; invalidate before the rescan request
		// so watcher signals arriving during the scheduling window cannot route
		// against the previous complete root snapshot.
		e.scanner.invalidateRootCache()
		e.scanner.RequestRescan(ctx)
	}
	return nil
}

func (e *Engine) ListRoots(ctx context.Context) ([]RootRecord, error) {
	if e == nil {
		return nil, nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.db == nil {
		return nil, fmt.Errorf("filesearch engine closed")
	}

	roots, err := e.db.ListRoots(ctx)
	if err != nil {
		return nil, err
	}
	return userVisibleRoots(roots), nil
}

func (e *Engine) GetStatus(ctx context.Context) (StatusSnapshot, error) {
	if e == nil {
		return StatusSnapshot{}, nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed {
		return StatusSnapshot{}, fmt.Errorf("filesearch engine closed")
	}

	allRoots, err := e.statusRoots(ctx)
	if err != nil {
		return StatusSnapshot{}, err
	}
	// Dynamic roots are internal scan ownership boundaries. Status counters keep
	// reporting the user-configured root set while the scanner still uses all
	// roots for execution and diagnostics.
	roots := userVisibleRoots(allRoots)

	var transientScanState TransientRootState
	hasTransientScanState := false
	var transientSyncState TransientSyncState
	hasTransientSyncState := false

	if e.scanner != nil {
		if activeState, ok := e.scanner.GetTransientRootState(); ok {
			transientScanState = activeState
			hasTransientScanState = true
			mergeTransientRootState(roots, activeState.Root)
		}
		if activeState, ok := e.scanner.GetTransientSyncState(); ok {
			transientSyncState = activeState
			hasTransientSyncState = true
			if activeState.Root.ID != "" {
				mergeTransientRootState(roots, activeState.Root)
			}
		}
	}

	status := StatusSnapshot{
		RootCount: len(roots),
	}
	if hasTransientSyncState {
		status.PendingDirtyRootCount = transientSyncState.PendingRootCount
		status.PendingDirtyPathCount = transientSyncState.PendingPathCount
	}

	for _, root := range roots {
		progressCurrent, progressTotal := normalizeRootProgress(root)
		status.ProgressCurrent += progressCurrent
		status.ProgressTotal += progressTotal

		switch root.Status {
		case RootStatusPreparing:
			status.PreparingRootCount++
		case RootStatusScanning:
			status.ScanningRootCount++
		case RootStatusSyncing:
			status.SyncingRootCount++
		case RootStatusWriting:
			status.WritingRootCount++
		case RootStatusFinalizing:
			status.FinalizingRootCount++
		case RootStatusError:
			status.ErrorRootCount++
			if status.LastError == "" && root.LastError != nil {
				status.LastError = strings.TrimSpace(*root.LastError)
			}
			if status.ErrorRootPath == "" {
				status.ErrorRootPath = root.Path
			}
		}

		if isActiveRootStatus(root.Status) && activeRootStatusPriority(root.Status) >= activeRootStatusPriority(status.ActiveRootStatus) {
			status.ActiveRootStatus = root.Status
			status.ActiveProgressCurrent = root.ProgressCurrent
			status.ActiveProgressTotal = root.ProgressTotal
			switch {
			case hasTransientSyncState && transientSyncState.Root.ID == root.ID:
				status.ActiveRootIndex = transientSyncState.RootIndex
				status.ActiveRootTotal = transientSyncState.RootTotal
				status.ActiveDiscoveredCount = 0
				status.ActiveDirectoryIndex = transientSyncState.ScopeCount
				status.ActiveDirectoryTotal = transientSyncState.ScopeCount
				status.ActiveItemCurrent = 0
				status.ActiveItemTotal = int64(transientSyncState.DirtyPathCount)
			case hasTransientScanState && transientScanState.Root.ID == root.ID:
				status.ActiveRootIndex = transientScanState.RootIndex
				status.ActiveRootTotal = transientScanState.RootTotal
				status.ActiveDiscoveredCount = transientScanState.DiscoveredCount
				status.ActiveDirectoryIndex = transientScanState.DirectoryIndex
				status.ActiveDirectoryTotal = transientScanState.DirectoryTotal
				status.ActiveItemCurrent = transientScanState.ItemCurrent
				status.ActiveItemTotal = transientScanState.ItemTotal
			}
		}
	}

	var activeRun StatusSnapshot
	hasActiveRun := false
	if e.scanner != nil {
		if currentRun, ok := e.scanner.GetTransientRunState(); ok {
			activeRun = currentRun
			hasActiveRun = true
			mergeTransientRunStatus(&status, activeRun)
		}
	}

	// Run preparation/execution owns the live indexing state. The previous code
	// merged the active run and then immediately overwrote IsIndexing from the
	// persisted root counters, which made the toolbar treat a live planning pass
	// as "not indexing" whenever another root was already in error.
	if hasActiveRun {
		status.IsInitialIndexing = activeRun.IsIndexing &&
			activeRun.ActiveStage == RunStagePlanning &&
			activeRun.ActiveProgressCurrent == 0
		status.IsIndexing = activeRun.IsIndexing
		return status, nil
	}

	status.IsInitialIndexing = status.RootCount > 0 && (status.ActiveRootStatus == RootStatusPreparing || status.ActiveRootStatus == RootStatusScanning) && status.ActiveProgressCurrent == 0 && (status.PreparingRootCount > 0 || status.ScanningRootCount > 0)
	status.IsIndexing = status.PreparingRootCount > 0 || status.ScanningRootCount > 0 || status.SyncingRootCount > 0 || status.WritingRootCount > 0 || status.FinalizingRootCount > 0 || status.IsInitialIndexing
	return status, nil
}

func (e *Engine) statusRoots(ctx context.Context) ([]RootRecord, error) {
	if e.scanner != nil {
		if roots, ok := e.scanner.cachedRootSnapshot(); ok {
			// Optimization: status changes are often emitted while watcher signals
			// are being enqueued. The scanner root cache is kept coherent with root
			// membership/state writes, so a complete snapshot avoids a repeated
			// SQLite ListRoots round trip on that hot notification path.
			return roots, nil
		}
	}
	if e.db == nil {
		return nil, fmt.Errorf("filesearch engine closed")
	}
	return e.db.ListRoots(ctx)
}

func mergeTransientRunStatus(status *StatusSnapshot, activeRun StatusSnapshot) {
	if status == nil {
		return
	}

	// Run-scoped progress now owns the user-facing denominator because one
	// logical root can expand into many jobs. The legacy root counters remain in
	// the snapshot as diagnostics, but active status/progress should prefer the
	// sealed run state whenever a preparation/execution run is in flight.
	status.ProgressCurrent = activeRun.ProgressCurrent
	status.ProgressTotal = activeRun.ProgressTotal
	status.ActiveRootStatus = activeRun.ActiveRootStatus
	status.ActiveProgressCurrent = activeRun.ActiveProgressCurrent
	status.ActiveProgressTotal = activeRun.ActiveProgressTotal
	status.ActiveRootIndex = activeRun.ActiveRootIndex
	status.ActiveRootTotal = activeRun.ActiveRootTotal
	status.ActiveDiscoveredCount = activeRun.ActiveDiscoveredCount
	status.ActiveDirectoryIndex = activeRun.ActiveDirectoryIndex
	status.ActiveDirectoryTotal = activeRun.ActiveDirectoryTotal
	status.ActiveItemCurrent = activeRun.ActiveItemCurrent
	status.ActiveItemTotal = activeRun.ActiveItemTotal
	status.ActiveRootPath = activeRun.ActiveRootPath
	status.ActiveRunStatus = activeRun.ActiveRunStatus
	status.ActiveRunKind = activeRun.ActiveRunKind
	status.ActiveJobKind = activeRun.ActiveJobKind
	status.ActiveScopePath = activeRun.ActiveScopePath
	status.ActiveStage = activeRun.ActiveStage
	status.RunProgressCurrent = activeRun.RunProgressCurrent
	status.RunProgressTotal = activeRun.RunProgressTotal
	status.ActiveRunFileCount = activeRun.ActiveRunFileCount
	status.ActiveRunEntryCount = activeRun.ActiveRunEntryCount
	status.ActiveRunElapsedMs = activeRun.ActiveRunElapsedMs
	status.IsIndexing = activeRun.IsIndexing
	if strings.TrimSpace(activeRun.LastError) != "" {
		status.LastError = activeRun.LastError
	}
}

func mergeTransientRootState(roots []RootRecord, transientRoot RootRecord) {
	for index := range roots {
		if roots[index].ID == transientRoot.ID {
			roots[index] = transientRoot
			return
		}
	}
}

func userVisibleRoots(roots []RootRecord) []RootRecord {
	visible := make([]RootRecord, 0, len(roots))
	for _, root := range roots {
		if root.Kind == RootKindDynamic {
			continue
		}
		visible = append(visible, root)
	}
	return visible
}

func (e *Engine) OnStatusChanged(callback func(StatusSnapshot)) func() {
	if callback == nil {
		return func() {}
	}

	listenerId := uuid.NewString()
	e.statusListeners.Store(listenerId, callback)
	return func() {
		e.statusListeners.Delete(listenerId)
	}
}

func (e *Engine) notifyStatusChanged(ctx context.Context) {
	status, err := e.GetStatus(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "filesearch failed to emit status changed event: "+err.Error())
		return
	}

	e.statusListeners.Range(func(_ string, callback func(StatusSnapshot)) bool {
		callback(status)
		return true
	})
}

func normalizeRootProgress(root RootRecord) (int64, int64) {
	switch root.Status {
	case RootStatusPreparing:
		return 0, RootProgressScale
	case RootStatusScanning, RootStatusSyncing, RootStatusWriting:
		total := root.ProgressTotal
		if total <= 0 || total > RootProgressScale {
			total = RootProgressScale
		}

		current := root.ProgressCurrent
		if current < 0 {
			current = 0
		}
		if current > total {
			current = total
		}

		return current, total
	case RootStatusFinalizing:
		if root.ProgressTotal > 0 {
			total := root.ProgressTotal
			if total > RootProgressScale {
				total = RootProgressScale
			}
			current := root.ProgressCurrent
			if current < 0 {
				current = 0
			}
			if current > total {
				current = total
			}
			return current, total
		}
		return RootProgressScale, RootProgressScale
	case RootStatusIdle:
		if root.ProgressTotal > 0 {
			return RootProgressScale, RootProgressScale
		}
		return 0, RootProgressScale
	case RootStatusError:
		return 0, 0
	default:
		return 0, RootProgressScale
	}
}

func isActiveRootStatus(status RootStatus) bool {
	switch status {
	case RootStatusPreparing, RootStatusScanning, RootStatusSyncing, RootStatusWriting, RootStatusFinalizing:
		return true
	default:
		return false
	}
}

func activeRootStatusPriority(status RootStatus) int {
	switch status {
	case RootStatusFinalizing:
		return 5
	case RootStatusWriting:
		return 4
	case RootStatusSyncing:
		return 3
	case RootStatusScanning:
		return 2
	case RootStatusPreparing:
		return 1
	default:
		return 0
	}
}

func (e *Engine) SyncUserRoots(ctx context.Context, rootPaths []string) error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.db == nil {
		return fmt.Errorf("filesearch engine closed")
	}

	_, err := syncUserRootsToDB(ctx, e.db, e.scanner, rootPaths, true)
	return err
}

// NormalizeUserRootPaths returns the concrete user roots that should participate
// in indexing. Exact duplicates are redundant, and nested roots are actively
// harmful when the parent scan can write the child's paths into the unique
// entries table. Keeping only the parent root prevents accidental settings like
// "$HOME" plus "$HOME/Projects" from making full runs fail with duplicate entry
// paths while preserving explicit child roots that the parent scan prunes.
func NormalizeUserRootPaths(ctx context.Context, rootPaths []string) []string {
	candidates := make([]string, 0, len(rootPaths))
	seen := map[string]struct{}{}
	for _, rootPath := range rootPaths {
		cleaned := strings.TrimSpace(rootPath)
		if cleaned == "" {
			continue
		}

		cleaned = filepath.Clean(cleaned)
		if cleaned == "." {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		info, err := os.Stat(cleaned)
		if err != nil {
			util.GetLogger().Warn(ctx, "filesearch skipped missing root "+cleaned+": "+err.Error())
			continue
		}
		if !info.IsDir() {
			util.GetLogger().Warn(ctx, "filesearch skipped non-directory root "+cleaned)
			continue
		}

		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	normalized := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		parent := ""
		for _, other := range candidates {
			if other == candidate || !parentRootCoversChildRoot(other, candidate) {
				continue
			}
			if parent == "" || len(other) > len(parent) {
				parent = other
			}
		}
		if parent != "" {
			util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch skipped overlapping child root: child=%s parent=%s", candidate, parent))
			continue
		}
		normalized = append(normalized, candidate)
	}

	return normalized
}

// parentRootCoversChildRoot reports whether indexing parentRoot can produce the
// childRoot entries, so the child root is redundant.
func parentRootCoversChildRoot(parentRoot string, childRoot string) bool {
	if !pathWithinScope(parentRoot, childRoot) {
		return false
	}
	return !shouldSkipSystemPathForRoot(RootRecord{Path: parentRoot}, childRoot, true)
}

func syncUserRootsToDB(ctx context.Context, db *FileSearchDB, scanner *Scanner, rootPaths []string, requestRescan bool) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("filesearch database is not open")
	}

	desiredRoots := map[string]struct{}{}
	for _, rootPath := range NormalizeUserRootPaths(ctx, rootPaths) {
		desiredRoots[rootPath] = struct{}{}
	}

	roots, err := db.ListRoots(ctx)
	if err != nil {
		return false, err
	}

	existingUserRoots := map[string]RootRecord{}
	for _, root := range roots {
		if root.Kind != RootKindUser {
			continue
		}
		existingUserRoots[filepath.Clean(root.Path)] = root
	}

	changed := false
	addedCount := 0
	removedCount := 0
	for existingPath, root := range existingUserRoots {
		if _, ok := desiredRoots[existingPath]; ok {
			continue
		}
		if err := db.DeleteRoot(ctx, root.ID); err != nil {
			return false, err
		}
		changed = true
		removedCount++
	}

	now := util.GetSystemTimestamp()
	for desiredPath := range desiredRoots {
		if _, ok := existingUserRoots[desiredPath]; ok {
			continue
		}

		root := RootRecord{
			ID:        uuid.NewString(),
			Path:      desiredPath,
			Kind:      RootKindUser,
			Status:    RootStatusPreparing,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.UpsertRoot(ctx, root); err != nil {
			return false, err
		}
		changed = true
		addedCount++
	}

	util.GetLogger().Info(ctx, fmt.Sprintf(
		"filesearch sync user roots: desired=%d existing=%d added=%d removed=%d changed=%v",
		len(desiredRoots),
		len(existingUserRoots),
		addedCount,
		removedCount,
		changed,
	))
	if changed && scanner != nil {
		// Bulk root sync can add and remove many rows at once. Clear the Scanner
		// cache before any optional rescan so the change-feed goroutine never sees
		// a complete-but-stale user-root snapshot.
		scanner.invalidateRootCache()
		if requestRescan {
			scanner.RequestRescan(ctx)
		}
	}

	return changed, nil
}

func (e *Engine) Search(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, error) {
	if e == nil {
		return nil, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.searchProvider == nil {
		return nil, fmt.Errorf("filesearch engine closed")
	}

	// Filesearch now has one SQLite-backed provider, so the engine stays as a
	// thin owner of lifecycle/policy state and returns the provider result
	// directly instead of preserving the old stream/aggregation wrapper.
	return e.searchProvider.Search(ctx, query, limit)
}

func (e *Engine) IndexSnapshotSummary() string {
	if e == nil {
		return formatSQLiteIndexSnapshotSummary("manual", sqliteIndexSnapshot{})
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.db == nil {
		return formatSQLiteIndexSnapshotSummary("manual", sqliteIndexSnapshot{})
	}
	snapshot, err := e.db.SearchIndexSnapshot(context.Background())
	if err != nil {
		return fmt.Sprintf("filesearch sqlite snapshot: stage=manual error=%s", err.Error())
	}
	return formatSQLiteIndexSnapshotSummary("manual", snapshot)
}

func (e *Engine) IndexTopRootsSummary() string {
	if e == nil {
		return ""
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.db == nil {
		return ""
	}
	snapshot, err := e.db.SearchIndexSnapshot(context.Background())
	if err != nil {
		return ""
	}
	return formatSQLiteIndexTopRoots("manual", snapshot)
}

// SearchContent queries the content index for files matching the query terms.
// Returns results sorted by FTS5 relevance. Only returns results when the
// content crawl is complete (avoids slow queries during indexing).
// This method does NOT take the engine RLock to avoid contending with the
// filesearch scanner — the content tables are independent and SQLite handles
// concurrent reads via WAL.
func (e *Engine) SearchContent(ctx context.Context, query string, limit int) ([]ContentSearchResult, error) {
	if e == nil {
		return nil, nil
	}
	e.mu.RLock()
	if e.closed || e.contentDB == nil {
		e.mu.RUnlock()
		return nil, nil
	}
	contentDB := e.contentDB
	e.mu.RUnlock()

	// Skip content search if crawl is not complete.
	crawlState, _ := contentDB.GetContentCrawlState(ctx)
	if crawlState != "complete" {
		return nil, nil
	}

	return contentDB.SearchContent(ctx, query, limit)
}

// ContentStats returns statistics about the content index.
func (e *Engine) ContentStats(ctx context.Context) (ContentStats, error) {
	if e == nil {
		return ContentStats{}, nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.contentDB == nil {
		return ContentStats{}, nil
	}
	return e.contentDB.ContentStats(ctx)
}

// ResetContentIndex removes the standalone content index database.
func (e *Engine) ResetContentIndex(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closeContentHookLocked()
	contentDB := e.contentDB
	e.contentDB = nil
	e.mu.Unlock()

	var closeErr error
	if contentDB != nil {
		closeErr = contentDB.Close()
	}
	if err := removeContentSearchDBFiles(); err != nil {
		if closeErr != nil {
			return fmt.Errorf("close content search database: %w; %v", closeErr, err)
		}
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close content search database: %w", closeErr)
	}
	return nil
}

// StartContentCrawl launches a content crawl in a background goroutine.
func (e *Engine) StartContentCrawl(ctx context.Context, roots []RootRecord, policy Policy, extensions map[string]bool, maxReadBytes int64, progressCB func(ContentCrawlProgress)) <-chan error {
	done := make(chan error, 1)
	if e == nil {
		close(done)
		return done
	}
	if ctx == nil {
		ctx = context.Background()
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		close(done)
		return done
	}
	contentDB, err := e.ensureContentDBLocked(ctx)
	e.mu.Unlock()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("open content search database for crawl failed: %v", err))
		done <- err
		close(done)
		return done
	}

	crawler := NewContentCrawler(contentDB, roots, policy, extensions, maxReadBytes, progressCB)
	go func() {
		err := crawler.Run(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			util.GetLogger().Error(ctx, fmt.Sprintf("content crawl failed: %v", err))
		}
		done <- err
		close(done)
	}()
	return done
}

// StartContentHook installs the incremental content index hook on the scanner.
// After this call, file changes detected by the scanner's change feed (USN on
// Windows, FSEvents on macOS, fsnotify elsewhere) will incrementally update
// the content FTS index without waiting for the next full crawl. The hook
// reuses the same extension whitelist and max-read-bytes as the full crawler.
// Call StopContentHook before installing a new hook or when content search is
// disabled.
func (e *Engine) StartContentHook(ctx context.Context, extensions map[string]bool, maxReadBytes int64) {
	if e == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.db == nil {
		return
	}
	contentDB, err := e.ensureContentDBLocked(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("open content search database for hook failed: %v", err))
		return
	}

	// Stop any existing hook before installing a new one so extension changes
	// take effect cleanly.
	e.closeContentHookLocked()

	hook := NewContentIndexHook(contentDB, e.db, extensions, maxReadBytes, e.policy)
	e.contentHook = hook

	scanner := e.scanner
	if scanner != nil {
		scanner.SetContentHook(hook)
	}
}

// StopContentHook removes the incremental content index hook and stops its
// background worker. Called when content search is disabled or the engine is
// being rebuilt.
func (e *Engine) StopContentHook() {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closeContentHookLocked()
}

// closeContentHookLocked detaches and closes the incremental content hook while e.mu is held.
func (e *Engine) closeContentHookLocked() {
	hook := e.contentHook
	e.contentHook = nil
	scanner := e.scanner
	if scanner != nil {
		scanner.SetContentHook(nil)
	}
	if hook != nil {
		hook.Close()
	}
}

// ensureContentDBLocked opens the optional content database on first use while e.mu is held.
func (e *Engine) ensureContentDBLocked(ctx context.Context) (*ContentSearchDB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e.contentDB != nil {
		return e.contentDB, nil
	}
	contentDB, err := NewContentSearchDB(ctx)
	if err != nil {
		return nil, err
	}
	e.contentDB = contentDB
	return contentDB, nil
}

// NeedsSearchArtifactRebuild reports whether the FTS search artifacts are
// still being rebuilt. The content crawl waits for this to return false
// before starting, to avoid "database is locked" errors.
func (e *Engine) NeedsSearchArtifactRebuild() bool {
	if e == nil {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.db != nil && e.db.NeedsSearchArtifactRebuild()
}

// GetContentCrawlState returns the content crawl state from the DB meta table.
func (e *Engine) GetContentCrawlState(ctx context.Context) (string, error) {
	if e == nil {
		return "", nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.closed || e.contentDB == nil {
		return "", nil
	}
	return e.contentDB.GetContentCrawlState(ctx)
}
