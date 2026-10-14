package filesearch

import (
	"context"
	"path/filepath"

	"wox/util"
)

// nativeNameIndex owns ordinary-user columns whose facts require a platform
// broker. Query and storage policy remain identical to configured-root indexes.
type nativeNameIndex interface {
	Search(context.Context, SearchQuery, int) ([]SearchResult, error)
	Status(context.Context) (StatusSnapshot, IndexStatsSnapshot, error)
	Progress() StatusSnapshot
	Close() error
	directoryView() *directoryIndex
}

type nativeIndexCallbacks struct {
	status       func()
	content      func(context.Context, []ContentHookNotification)
	wantsContent func() bool
	loadElapsed  func(context.Context, string) (int64, error)
	saveElapsed  func(context.Context, string, int64) error
}

// RefreshNativeNameIndex reconciles the source after a service install, update,
// or removal. Lifecycle calls serialize with reset and application shutdown.
func (e *Engine) RefreshNativeNameIndex(ctx context.Context) error {
	e.resetMu.Lock()
	defer e.resetMu.Unlock()
	return e.refreshNativeNameIndex(ctx)
}

// refreshNativeNameIndex requires resetMu and closes readers before reopening
// their directory, preventing two source loops from writing the same journal.
func (e *Engine) refreshNativeNameIndex(ctx context.Context) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	previous := e.nativeIndex
	db := e.db
	e.closeContentHookLocked()
	e.nativeIndex = nil
	e.mu.Unlock()
	if previous != nil {
		if err := previous.Close(); err != nil {
			return err
		}
	}
	callbacks := nativeIndexCallbacks{
		status:       func() { e.notifyStatusChanged(util.NewTraceContext()) },
		content:      e.notifyNativeContent,
		wantsContent: func() bool { e.mu.RLock(); defer e.mu.RUnlock(); return e.contentHook != nil },
	}
	if db != nil {
		callbacks.loadElapsed = db.lastNativeFullIndexElapsedMs
		callbacks.saveElapsed = db.setNativeFullIndexElapsedMs
	}
	index, err := openNativeNameIndex(ctx, filepath.Join(util.GetLocation().GetFileSearchDirectory(), "native-columns"), callbacks)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.nativeIndex = index
	e.mu.Unlock()
	return nil
}

// notifyNativeContent runs only after the directory mutation is visible. The
// hook owns filtering, bounded coalescing and extraction off the USN reader.
func (e *Engine) notifyNativeContent(ctx context.Context, notifications []ContentHookNotification) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.contentHook == nil {
		return
	}
	for _, notification := range notifications {
		e.contentHook.Notify(ctx, notification)
	}
}

// stopNativeNameIndex requires resetMu, but releases the engine lock before
// joining workers because their final status notification also reads the engine.
func (e *Engine) stopNativeNameIndex() error {
	e.mu.Lock()
	index := e.nativeIndex
	e.nativeIndex = nil
	e.mu.Unlock()
	if index != nil {
		return index.Close()
	}
	return nil
}
