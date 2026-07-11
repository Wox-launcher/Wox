package filesearch

import (
	"context"
	"errors"
	"os"
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
	contentDB    *ContentSearchDB
	nameDB       *FileSearchDB
	extensions   map[string]bool
	maxReadBytes int64

	ctx         context.Context
	cancel      context.CancelFunc
	wakeCh      chan struct{}
	pending     map[string]ContentHookNotification
	pendingMu   sync.Mutex
	lifecycleMu sync.Mutex
	closed      bool
	stopCh      chan struct{}
	stopped     sync.Once
	wg          sync.WaitGroup
}

// NewContentIndexHook creates a running content hook. Call Close to stop the
// background worker. Notifications arrive only after the authoritative name
// index mutation, so the hook does not repeat traversal-policy evaluation.
func NewContentIndexHook(contentDB *ContentSearchDB, nameDB *FileSearchDB, extensions map[string]bool, maxReadBytes int64) *ContentIndexHook {
	hookCtx, cancel := context.WithCancel(context.Background())
	h := &ContentIndexHook{
		contentDB:    contentDB,
		nameDB:       nameDB,
		extensions:   extensions,
		maxReadBytes: maxReadBytes,
		ctx:          hookCtx,
		cancel:       cancel,
		wakeCh:       make(chan struct{}, 1),
		pending:      make(map[string]ContentHookNotification, 256),
		stopCh:       make(chan struct{}),
	}
	h.wg.Add(1)
	util.Go(hookCtx, "content index hook worker", h.worker)
	return h
}

// Notify coalesces the latest state for each path or scope and wakes the background worker.
func (h *ContentIndexHook) Notify(ctx context.Context, notification ContentHookNotification) {
	if h == nil {
		return
	}
	select {
	case <-h.done():
		return
	default:
	}

	key := notification.Path
	if notification.Kind == ContentHookKindScopeReplaced {
		key = string(notification.Kind) + "\x00" + notification.RootID + "\x00" + notification.ScopePath
	}
	if key == "" {
		return
	}
	h.pendingMu.Lock()
	h.pending[key] = notification
	h.pendingMu.Unlock()

	select {
	case <-h.done():
		return
	case <-h.stopCh:
		return
	case h.wakeCh <- struct{}{}:
	default:
		// A queued wake-up already covers the newly coalesced pending state.
	}
}

// Close stops the background worker and waits for it to drain.
func (h *ContentIndexHook) Close() {
	if h == nil {
		return
	}
	h.stopped.Do(func() {
		h.lifecycleMu.Lock()
		h.closed = true
		if h.cancel != nil {
			h.cancel()
		}
		close(h.stopCh)
		h.lifecycleMu.Unlock()
	})
	h.wg.Wait()
}

func (h *ContentIndexHook) worker() {
	defer h.wg.Done()
	for {
		select {
		case <-h.done():
			return
		case <-h.stopCh:
			return
		case <-h.wakeCh:
			for {
				notification, ok := h.takePending()
				if !ok {
					break
				}
				h.process(notification)
			}
		}
	}
}

func (h *ContentIndexHook) takePending() (ContentHookNotification, bool) {
	h.pendingMu.Lock()
	defer h.pendingMu.Unlock()
	for key, notification := range h.pending {
		delete(h.pending, key)
		return notification, true
	}
	return ContentHookNotification{}, false
}

func (h *ContentIndexHook) process(notification ContentHookNotification) {
	ctx := h.hookContext()
	if ctx.Err() != nil {
		return
	}

	switch notification.Kind {
	case ContentHookKindUpsert:
		h.processUpsert(ctx, notification.Path)
	case ContentHookKindDelete:
		h.processDelete(ctx, notification.Path)
	case ContentHookKindScopeReplaced:
		h.processScopeReplaced(ctx, notification.RootID, notification.ScopePath)
	}
}

func (h *ContentIndexHook) processUpsert(ctx context.Context, path string) {
	if h.contentDB == nil {
		return
	}
	if !IsContentSearchableExtension(path, h.extensions) {
		return
	}

	info, err := os.Lstat(path)
	if err != nil {
		// File may have been deleted between the scanner event and hook
		// processing. Treat as delete to keep content index consistent.
		_ = h.contentDB.DeleteContent(ctx, path)
		return
	}
	if info.IsDir() {
		return
	}

	readBytes := contentExtractionMaxBytes(path, info.Size(), h.maxReadBytes)
	text, err := extractContentText(path, readBytes)
	if err != nil {
		return
	}
	ext := contentNormalizeExtension(path)
	_, _ = h.contentDB.IndexContent(ctx, path, info.ModTime().UnixMilli(), info.Size(), ext, text)
}

func (h *ContentIndexHook) processDelete(ctx context.Context, path string) {
	if h.contentDB == nil {
		return
	}
	_ = h.contentDB.DeleteContent(ctx, path)
}

// processScopeReplaced reconciles the content index for a directory scope that
// was fully replaced by a scanner run. It deletes content entries for paths
// that no longer exist in the name index, and queues content indexing for
// newly searchable files that are now on disk.
func (h *ContentIndexHook) processScopeReplaced(ctx context.Context, rootID, scopePath string) {
	if h.contentDB == nil || h.nameDB == nil || scopePath == "" {
		return
	}
	reconcileCtx := h.hookContext()
	if reconcileCtx.Err() != nil {
		return
	}

	h.reconcileScope(reconcileCtx, rootID, scopePath)
}

// reconcileScope walks the content_entries table under scopePath and deletes
// rows whose paths are no longer in the name index or no longer exist on disk.
// New files that are now searchable are queued for content indexing through
// the normal upsert path.
func (h *ContentIndexHook) reconcileScope(ctx context.Context, rootID, scopePath string) {
	if h.contentDB == nil || h.nameDB == nil {
		return
	}

	contentPaths, err := h.contentDB.ListContentEntryPathsUnderScope(ctx, scopePath)
	if err != nil {
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			return
		}
		util.GetLogger().Warn(ctx, "content hook reconcile: failed to list content paths under "+scopePath+": "+err.Error())
		return
	}
	if ctx.Err() != nil {
		return
	}

	namePaths, err := h.nameDB.ListEntryPathsUnderScope(ctx, rootID, scopePath)
	if err != nil {
		if ctx.Err() == nil {
			util.GetLogger().Warn(ctx, "content hook reconcile: failed to list name paths under "+scopePath+": "+err.Error())
		}
		return
	}
	namePathSet := make(map[string]struct{}, len(namePaths))
	for _, path := range namePaths {
		namePathSet[path] = struct{}{}
	}
	stalePaths := make([]string, 0)
	for _, path := range contentPaths {
		if _, exists := namePathSet[path]; !exists {
			stalePaths = append(stalePaths, path)
		}
	}
	if err := h.contentDB.DeleteContentBatch(ctx, stalePaths); err != nil && ctx.Err() == nil {
		util.GetLogger().Warn(ctx, "content hook reconcile: failed to delete stale paths under "+scopePath+": "+err.Error())
	}
}

func (h *ContentIndexHook) done() <-chan struct{} {
	if h == nil || h.ctx == nil {
		return nil
	}
	return h.ctx.Done()
}

func (h *ContentIndexHook) hookContext() context.Context {
	if h == nil || h.ctx == nil {
		return context.Background()
	}
	return h.ctx
}
