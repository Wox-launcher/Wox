//go:build windows

package filesearch

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"wox/util"
	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
)

type nativeVolumeState struct {
	building bool
	pending  bool // Includes volumes queued for the shared slot and failed builds awaiting retry.
	ready    bool // A published generation can remain searchable while a replacement is built.
	progress nativeBuildProgress
	started  time.Time
	elapsed  int64
	err      string
}

// ntfsNameIndex shares one build/compaction slot across volumes. Idle USN reads
// run independently so a busy drive cannot delay changes on another drive.
type ntfsNameIndex struct {
	directory  *directoryIndex
	cancel     context.CancelFunc
	workers    sync.WaitGroup
	closeOnce  sync.Once
	closeErr   error
	work       chan struct{}
	mu         sync.Mutex
	states     map[string]nativeVolumeState
	notify     func()
	changes    chan struct{}
	callbacks  nativeIndexCallbacks
	runStarted time.Time
}

// openNativeNameIndex restores mappings immediately, then catches up each
// source in the background. It never starts or elevates the service itself.
func openNativeNameIndex(ctx context.Context, dir string, callbacks nativeIndexCallbacks) (nativeNameIndex, error) {
	if !filesearchservice.IsRunning() {
		return nil, nil
	}
	// Reuse the old/new path resolution already needed by content indexing.
	// Subscribers receive changes even when content extraction is disabled.
	content, wants := callbacks.content, callbacks.wantsContent
	callbacks.availability = sharedNativeChanges.setAvailable
	callbacks.wantsContent = func() bool {
		sharedNativeChanges.mu.Lock()
		subscribed := len(sharedNativeChanges.feeds) != 0
		sharedNativeChanges.mu.Unlock()
		return subscribed || content != nil && (wants == nil || wants())
	}
	callbacks.content = func(ctx context.Context, notifications []ContentHookNotification) {
		if content != nil && (wants == nil || wants()) {
			content(ctx, notifications)
		}
		for _, notification := range notifications {
			signal := ChangeSignal{Kind: ChangeSignalKindDirtyPath, FeedType: RootFeedTypeUSN, Path: notification.Path, PathTypeKnown: true, SemanticKind: ChangeSemanticKindModify, At: time.Now()}
			switch notification.Kind {
			case ContentHookKindDelete:
				signal.SemanticKind = ChangeSemanticKindRemove
			case ContentHookKindScopeReplaced:
				signal.Path, signal.PathIsDir = notification.ScopePath, true
			}
			sharedNativeChanges.publish(signal)
		}
	}
	return newNTFSNameIndex(ctx, dir, filesearchservice.IndexedVolumeRoots(), filesearchservice.StreamFacts, callbacks)
}

// newNTFSNameIndex accepts the fact transport so lifecycle tests use real
// durable columns without requiring a privileged service or a physical volume.
func newNTFSNameIndex(ctx context.Context, dir string, roots []string, stream factStream, callbacks nativeIndexCallbacks) (*ntfsNameIndex, error) {
	if len(roots) == 0 {
		return nil, errors.New("no fixed NTFS volumes available")
	}
	workerCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	index := &ntfsNameIndex{directory: newDirectoryIndex(dir), cancel: cancel, work: make(chan struct{}, 1), states: make(map[string]nativeVolumeState), notify: callbacks.status, callbacks: callbacks}
	for _, root := range roots {
		id := normalizePath(root)
		if _, exists := index.states[id]; exists {
			continue
		}
		if err := index.directory.attach(RootRecord{ID: id, Path: root}); err != nil {
			_ = index.Close()
			return nil, err
		}
		state := nativeVolumeState{}
		state.pending = errors.Is(index.directory.shards[id].store.Read(func(column.View) error { return nil }), column.ErrNotReady)
		state.ready = !state.pending
		if callbacks.loadElapsed != nil {
			elapsed, err := callbacks.loadElapsed(ctx, id)
			if err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch failed to restore NTFS build duration for %s: %v", root, err))
			} else {
				state.elapsed = max(0, elapsed)
			}
		}
		index.states[id] = state
	}
	if index.notify != nil {
		index.changes = make(chan struct{}, 1)
		index.workers.Add(1)
		go index.publishStatusChanges(workerCtx)
	}
	for id, shard := range index.directory.shards {
		index.workers.Add(1)
		go index.runVolume(workerCtx, id, &ntfsSource{
			root: shard.root.Path, store: shard.store, stream: stream, notify: callbacks.content, wantsNotifications: callbacks.wantsContent,
			progress: func(progress nativeBuildProgress) { index.updateProgress(id, progress) },
			changed:  index.queueStatusChange,
		})
	}
	return index, nil
}

// queueStatusChange never makes USN ingestion wait for toolbar or stats consumers.
func (i *ntfsNameIndex) queueStatusChange() {
	select {
	case i.changes <- struct{}{}:
	default:
	}
}

// publishStatusChanges coalesces volume changes in a fixed window. Continuous
// writes still get updates, while an idle index has no running polling timer.
func (i *ntfsNameIndex) publishStatusChanges(ctx context.Context) {
	defer i.workers.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-i.changes:
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		// Consume the merged signal before publishing so a change arriving
		// during the callback remains queued for the next update.
		select {
		case <-i.changes:
		default:
		}
		if ctx.Err() != nil {
			return
		}
		i.notify()
	}
}

// runVolume retries transport failures without discarding a searchable base.
// Missing or expired journals require a fresh build through the same work slot.
func (i *ntfsNameIndex) runVolume(ctx context.Context, id string, source *ntfsSource) {
	defer i.workers.Done()
	setAvailable := func(available bool) {
		if i.callbacks.availability != nil {
			i.callbacks.availability(source.root, available)
		}
	}
	defer setAvailable(false)
	needsBuild := errors.Is(source.store.Read(func(column.View) error { return nil }), column.ErrNotReady)
	for ctx.Err() == nil {
		var err error
		if needsBuild || source.store.NeedsCompaction() {
			if needsBuild {
				setAvailable(false)
			}
			select {
			case i.work <- struct{}{}:
			case <-ctx.Done():
				return
			}
			if needsBuild {
				err = i.buildVolume(ctx, id, source)
				if err == nil {
					needsBuild = false
					if i.callbacks.content != nil {
						i.callbacks.content(ctx, []ContentHookNotification{{Kind: ContentHookKindScopeReplaced, ScopePath: source.root}})
					}
				}
			} else {
				err = source.store.Compact(ctx)
			}
			<-i.work
		} else {
			err = source.replay(ctx, true)
		}
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			setAvailable(true)
			i.clearError(id)
			continue
		}
		setAvailable(false)
		if errors.Is(err, filesearchservice.ErrFactsReset) || errors.Is(err, column.ErrNotReady) {
			needsBuild = true
		}
		i.setState(ctx, id, false, err)
		util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch NTFS source %s retrying: %v", source.root, err))
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// buildVolume emits a bounded heartbeat even while enumeration is waiting or
// columns are being finalized. Its ticker is joined before the worker goes idle.
func (i *ntfsNameIndex) buildVolume(ctx context.Context, id string, source *ntfsSource) error {
	i.setState(ctx, id, true, nil)
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				if i.notify != nil {
					i.notify()
				}
			}
		}
	}()
	err := source.build(ctx)
	close(done)
	<-stopped
	i.setState(ctx, id, false, err)
	return err
}

// updateProgress consumes frame-level counters without issuing per-record UI calls.
func (i *ntfsNameIndex) updateProgress(id string, progress nativeBuildProgress) {
	i.mu.Lock()
	state := i.states[id]
	state.progress = progress
	i.states[id] = state
	i.mu.Unlock()
}

// setState publishes only lifecycle transitions, never one UI event per record.
func (i *ntfsNameIndex) setState(ctx context.Context, id string, building bool, err error) {
	i.mu.Lock()
	state := i.states[id]
	if building && !state.building {
		// Pending volumes share this rebuild's clock and completed counts. Start
		// a fresh clock only after all work from the previous rebuild has ended.
		active := false
		for _, current := range i.states {
			active = active || current.pending || current.building
		}
		if i.runStarted.IsZero() || !active {
			i.runStarted = time.Now()
			for root, current := range i.states {
				current.progress = nativeBuildProgress{}
				i.states[root] = current
			}
		}
		state.started = time.Now()
		state.pending = true
		state.progress = nativeBuildProgress{}
	}
	completed := !building && state.building && err == nil
	if completed {
		state.elapsed = max(1, time.Since(state.started).Milliseconds())
		state.pending, state.ready = false, true
	}
	state.building = building
	state.err = ""
	if err != nil {
		state.err = err.Error()
	}
	i.states[id] = state
	i.mu.Unlock()
	if completed && i.callbacks.saveElapsed != nil {
		// Only successful full builds replace the durable statistic. Completion
		// may race shutdown, so give this small write a bounded uncancelled budget.
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if err := i.callbacks.saveElapsed(persistCtx, id, state.elapsed); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("filesearch failed to persist NTFS build duration for %s: %v", id, err))
		}
		cancel()
	}
	if i.notify != nil {
		i.notify()
	}
}

func (i *ntfsNameIndex) clearError(id string) {
	i.mu.Lock()
	state := i.states[id]
	changed := state.err != ""
	state.err = ""
	i.states[id] = state
	i.mu.Unlock()
	if changed && i.notify != nil {
		i.notify()
	}
}

func (i *ntfsNameIndex) Search(ctx context.Context, query SearchQuery, limit int) ([]SearchResult, error) {
	return i.directory.search(ctx, query, limit)
}

func (i *ntfsNameIndex) directoryView() *directoryIndex { return i.directory }

// Progress borrows only small worker state; frequent UI notifications must not
// walk disk files or recount millions of published rows during a build.
func (i *ntfsNameIndex) Progress() StatusSnapshot {
	status, _ := i.progressSnapshot()
	return status
}

// progressSnapshot keeps live build counters separate from published index stats.
func (i *ntfsNameIndex) progressSnapshot() (StatusSnapshot, int64) {
	status := StatusSnapshot{}
	var elapsed int64
	ready := false
	i.mu.Lock()
	defer i.mu.Unlock()
	status.RootCount = len(i.states)
	for root, state := range i.states {
		ready = ready || state.ready
		status.ActiveRunFileCount += state.progress.files
		status.ActiveRunEntryCount += state.progress.entries
		if state.pending {
			status.IsIndexing = true
			status.ActiveRunKind = RunKindFull
			if status.ActiveRunStatus == "" {
				status.ActiveRunStatus = RunStatusPlanning
			}
		}
		if state.building {
			status.IsIndexing = true
			status.ActiveRootPath = root
			status.ActiveRootStatus = RootStatusScanning
			status.ActiveRunStatus = RunStatusExecuting
			status.ActiveRunKind = RunKindFull
			status.ActiveStage = RunStageExecuting
			if state.progress.finalizing {
				status.ActiveRootStatus = RootStatusFinalizing
				status.ActiveRunStatus = RunStatusFinalizing
				status.ActiveStage = RunStageFinalizing
			}
			elapsed += max(1, time.Since(state.started).Milliseconds())
		} else {
			elapsed += state.elapsed
		}
		if state.err != "" {
			status.ErrorRootCount++
			status.ErrorRootPath, status.LastError = root, state.err
		}
	}
	if status.IsIndexing && !i.runStarted.IsZero() {
		status.ActiveRunElapsedMs = time.Since(i.runStarted).Milliseconds()
	}
	status.IsInitialIndexing = status.IsIndexing && !ready
	return status, elapsed
}

// Status combines published counts with worker progress for Settings and diagnostics.
func (i *ntfsNameIndex) Status(ctx context.Context) (StatusSnapshot, IndexStatsSnapshot, error) {
	status, elapsed := i.progressSnapshot()
	stats := IndexStatsSnapshot{LastElapsedMs: elapsed}
	i.directory.mu.RLock()
	defer i.directory.mu.RUnlock()
	for _, shard := range i.directory.shards {
		files, entries, err := shard.store.Counts(ctx)
		if errors.Is(err, column.ErrNotReady) {
			status.IsIndexing = true
		}
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return status, stats, err
		}
		stats.FileCount += files
		stats.EntryCount += entries
	}
	_ = filepath.WalkDir(i.directory.dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				stats.DiskBytes += info.Size()
			}
		}
		return nil
	})
	stats.RootCount, stats.IsIndexing, stats.Error = status.RootCount, status.IsIndexing, status.LastError
	status.IsInitialIndexing = status.IsIndexing && stats.EntryCount == 0
	if !status.IsIndexing {
		status.ActiveRunEntryCount, status.ActiveRunFileCount = stats.EntryCount, stats.FileCount
	}
	return status, stats, nil
}

// Close cancels overlapped pipe reads before joining workers and unmapping files.
func (i *ntfsNameIndex) Close() error {
	i.closeOnce.Do(func() {
		i.cancel()
		i.workers.Wait()
		i.closeErr = i.directory.close()
	})
	return i.closeErr
}
