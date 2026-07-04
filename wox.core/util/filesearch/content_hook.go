package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"wox/util"
)

// ContentHookKind classifies a content index notification so the hook can apply
// the right operation without re-inferring it from the scanner job shape.
type ContentHookKind string

const (
	// ContentHookKindUpsert means the file at Path was created or modified and
	// its content should be (re)indexed if the extension is searchable.
	ContentHookKindUpsert ContentHookKind = "upsert"
	// ContentHookKindDelete means the file at Path was removed from the name
	// index and its content entry (if any) should be deleted.
	ContentHookKindDelete ContentHookKind = "delete"
	// ContentHookKindScopeReplaced means a directory scope was fully replaced
	// by a scanner run. The hook reconciles content entries under ScopePath:
	// deletes content for paths no longer in the entries table, and queues
	// content indexing for newly searchable files.
	ContentHookKindScopeReplaced ContentHookKind = "scope_replaced"
)

// ContentHookNotification is one notification delivered to the content hook.
type ContentHookNotification struct {
	Kind      ContentHookKind
	Path      string
	ScopePath string
	RootID    string
}

// ContentHook is the contract the scanner uses to keep the content index in
// sync with incremental name-index changes. The hook implementation owns
// deduplication, throttling, and background execution so the scanner hot path
// never blocks on file reads.
type ContentHook interface {
	Notify(ctx context.Context, notification ContentHookNotification)
	Close()
}

// ContentIndexHook is the default ContentHook implementation. It runs a
// background goroutine that drains a deduplicated notification queue and
// applies content index mutations (IndexContent / DeleteContent / scope
// reconcile) off the scanner's critical path.
type ContentIndexHook struct {
	db           *FileSearchDB
	extensions   map[string]bool
	maxReadBytes int64
	policy       *policyState

	queue    chan ContentHookNotification
	dedupe   map[string]ContentHookKind
	dedupeMu sync.Mutex
	stopCh   chan struct{}
	stopped  sync.Once
	wg       sync.WaitGroup
}

// NewContentIndexHook creates a running content hook. Call Close to stop the
// background worker. The policy is used to filter paths the same way the
// scanner does, so content indexing respects ignore rules and hidden-file
// settings without re-implementing them.
func NewContentIndexHook(db *FileSearchDB, extensions map[string]bool, maxReadBytes int64, policy *policyState) *ContentIndexHook {
	h := &ContentIndexHook{
		db:           db,
		extensions:   extensions,
		maxReadBytes: maxReadBytes,
		policy:       policy,
		queue:        make(chan ContentHookNotification, 1024),
		dedupe:       make(map[string]ContentHookKind, 256),
		stopCh:       make(chan struct{}),
	}
	h.wg.Add(1)
	util.Go(context.Background(), "content index hook worker", h.worker)
	return h
}

// Notify enqueues a content index notification. This is safe to call from the
// scanner goroutine and never blocks for longer than the queue capacity. If
// the queue is full the notification is dropped with a debug log — the next
// full crawl will recover the missing entries.
func (h *ContentIndexHook) Notify(ctx context.Context, notification ContentHookNotification) {
	if h == nil {
		return
	}

	// Dedupe by path. For the same path, a delete cancels a pending upsert and
	// an upsert cancels a pending delete. Scope-replaced notifications are
	// always delivered because they carry a scope path, not a file path.
	if notification.Kind != ContentHookKindScopeReplaced {
		h.dedupeMu.Lock()
		if existing, ok := h.dedupe[notification.Path]; ok {
			if existing == notification.Kind {
				h.dedupeMu.Unlock()
				return
			}
		}
		h.dedupe[notification.Path] = notification.Kind
		h.dedupeMu.Unlock()
	}

	select {
	case h.queue <- notification:
	default:
		// Queue full — drop the notification. The periodic full crawl will
		// recover. This keeps the scanner unblocked during large bursts.
		util.GetLogger().Debug(ctx, "content index hook queue full, dropping notification for "+notification.Path)
	}
}

// Close stops the background worker and waits for it to drain.
func (h *ContentIndexHook) Close() {
	if h == nil {
		return
	}
	h.stopped.Do(func() {
		close(h.stopCh)
	})
	h.wg.Wait()
}

func (h *ContentIndexHook) worker() {
	defer h.wg.Done()
	for {
		select {
		case <-h.stopCh:
			return
		case notification := <-h.queue:
			h.process(notification)
		}
	}
}

func (h *ContentIndexHook) process(notification ContentHookNotification) {
	ctx := util.NewTraceContext()

	switch notification.Kind {
	case ContentHookKindUpsert:
		h.dedupeMu.Lock()
		delete(h.dedupe, notification.Path)
		h.dedupeMu.Unlock()
		h.processUpsert(ctx, notification.Path)
	case ContentHookKindDelete:
		h.dedupeMu.Lock()
		delete(h.dedupe, notification.Path)
		h.dedupeMu.Unlock()
		h.processDelete(ctx, notification.Path)
	case ContentHookKindScopeReplaced:
		h.processScopeReplaced(ctx, notification.RootID, notification.ScopePath)
	}
}

func (h *ContentIndexHook) processUpsert(ctx context.Context, path string) {
	if h.db == nil {
		return
	}
	if !IsContentSearchableExtension(path, h.extensions) {
		return
	}

	info, err := os.Lstat(path)
	if err != nil {
		// File may have been deleted between the scanner event and hook
		// processing. Treat as delete to keep content index consistent.
		_ = h.db.DeleteContent(ctx, path)
		return
	}
	if info.IsDir() {
		return
	}

	// Apply the same ignore/hidden policy the scanner uses so content
	// indexing stays consistent with the name index.
	if h.policy != nil {
		traversalCtx := h.policy.newTraversalContext(RootRecord{Path: filepath.Dir(path)}, filepath.Dir(path))
		if !traversalCtx.ShouldIndexPath(path, false) {
			_ = h.db.DeleteContent(ctx, path)
			return
		}
	}

	readBytes := h.maxReadBytes
	if info.Size() < readBytes {
		readBytes = info.Size()
	}
	text, err := readContentFile(path, readBytes)
	if err != nil {
		return
	}
	ext := contentNormalizeExtension(path)
	_, _ = h.db.IndexContent(ctx, path, info.ModTime().UnixMilli(), info.Size(), ext, text)
}

func (h *ContentIndexHook) processDelete(ctx context.Context, path string) {
	if h.db == nil {
		return
	}
	_ = h.db.DeleteContent(ctx, path)
}

// processScopeReplaced reconciles the content index for a directory scope that
// was fully replaced by a scanner run. It deletes content entries for paths
// that no longer exist in the name index, and queues content indexing for
// newly searchable files that are now on disk.
func (h *ContentIndexHook) processScopeReplaced(ctx context.Context, rootID, scopePath string) {
	if h.db == nil || scopePath == "" {
		return
	}

	// Reconcile in a background goroutine to avoid blocking the hook worker
	// for large scopes. This is the only notification kind that spawns
	// additional work, because scope replacements are relatively rare (full
	// scans, dynamic root promotions) and can cover thousands of files.
	util.Go(ctx, "content index scope reconcile", func() {
		reconcileCtx := util.NewTraceContext()
		h.reconcileScope(reconcileCtx, rootID, scopePath)
	})
}

// reconcileScope walks the content_entries table under scopePath and deletes
// rows whose paths are no longer in the name index or no longer exist on disk.
// New files that are now searchable are queued for content indexing through
// the normal upsert path.
func (h *ContentIndexHook) reconcileScope(ctx context.Context, rootID, scopePath string) {
	if h.db == nil {
		return
	}

	contentPaths, err := h.db.ListContentEntryPathsUnderScope(ctx, scopePath)
	if err != nil {
		util.GetLogger().Warn(ctx, "content hook reconcile: failed to list content paths under "+scopePath+": "+err.Error())
		return
	}

	// Delete content entries for paths no longer in the name index.
	for _, p := range contentPaths {
		exists, err := h.db.EntryPathExists(ctx, p)
		if err != nil {
			continue
		}
		if !exists {
			_ = h.db.DeleteContent(ctx, p)
		}
	}

	// Walk the scope directory and queue content indexing for searchable
	// files that are in the name index but not yet in the content index (or
	// whose content hash may have changed). This reuses the same extension
	// filter and policy traversal as the initial crawl.
	if h.policy == nil {
		return
	}
	// Determine the root path for traversal context. For dynamic roots the
	// scopePath may not be a root path, but the traversal context only needs
	// the directory to start descending from, so scopePath works as the
	// starting point.
	traversalCtx := h.policy.newTraversalContext(RootRecord{Path: scopePath}, scopePath)

	_ = filepath.WalkDir(scopePath, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if !traversalCtx.ShouldIndexPath(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			traversalCtx = traversalCtx.Descend(path)
			return nil
		}
		if !IsContentSearchableExtension(path, h.extensions) {
			return nil
		}
		// Queue an upsert for this file. The worker will read and index it.
		h.Notify(ctx, ContentHookNotification{
			Kind: ContentHookKindUpsert,
			Path: path,
		})
		return nil
	})
}
