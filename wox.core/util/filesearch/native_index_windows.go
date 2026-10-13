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
	started  time.Time
	elapsed  int64
	err      string
}

// ntfsNameIndex shares one build/compaction slot across volumes. Idle USN reads
// run independently so a busy drive cannot delay changes on another drive.
type ntfsNameIndex struct {
	directory *directoryIndex
	cancel    context.CancelFunc
	workers   sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
	work      chan struct{}
	mu        sync.Mutex
	states    map[string]nativeVolumeState
	notify    func()
	callbacks nativeIndexCallbacks
}

// openNativeNameIndex restores mappings immediately, then catches up each
// source in the background. It never starts or elevates the service itself.
func openNativeNameIndex(ctx context.Context, dir string, callbacks nativeIndexCallbacks) (nativeNameIndex, error) {
	if !filesearchservice.IsRunning() {
		return nil, nil
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
		index.states[id] = nativeVolumeState{}
	}
	for id, shard := range index.directory.shards {
		index.workers.Add(1)
		go index.runVolume(workerCtx, id, &ntfsSource{root: shard.root.Path, store: shard.store, stream: stream, notify: callbacks.content, wantsNotifications: callbacks.wantsContent})
	}
	return index, nil
}

// runVolume retries transport failures without discarding a searchable base.
// Missing or expired journals require a fresh build through the same work slot.
func (i *ntfsNameIndex) runVolume(ctx context.Context, id string, source *ntfsSource) {
	defer i.workers.Done()
	needsBuild := errors.Is(source.store.Read(func(column.View) error { return nil }), column.ErrNotReady)
	for ctx.Err() == nil {
		var err error
		if needsBuild || source.store.NeedsCompaction() {
			select {
			case i.work <- struct{}{}:
			case <-ctx.Done():
				return
			}
			if needsBuild {
				i.setState(id, true, nil)
				err = source.build(ctx)
				i.setState(id, false, err)
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
			i.clearError(id)
			continue
		}
		if errors.Is(err, filesearchservice.ErrFactsReset) || errors.Is(err, column.ErrNotReady) {
			needsBuild = true
		}
		i.setState(id, false, err)
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

// setState publishes only lifecycle transitions, never one UI event per record.
func (i *ntfsNameIndex) setState(id string, building bool, err error) {
	i.mu.Lock()
	state := i.states[id]
	if building && !state.building {
		state.started = time.Now()
	}
	if !building && state.building {
		state.elapsed = time.Since(state.started).Milliseconds()
	}
	state.building = building
	state.err = ""
	if err != nil {
		state.err = err.Error()
	}
	i.states[id] = state
	i.mu.Unlock()
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

// Status combines cheap per-revision counts with the ordinary worker's state;
// the privileged service contributes no indexing or query statistics.
func (i *ntfsNameIndex) Status(ctx context.Context) (StatusSnapshot, IndexStatsSnapshot, error) {
	status := StatusSnapshot{}
	stats := IndexStatsSnapshot{}
	i.mu.Lock()
	status.RootCount = len(i.states)
	for root, state := range i.states {
		stats.LastElapsedMs += state.elapsed
		if state.building {
			status.IsIndexing = true
			status.ActiveRootPath = root
			status.ActiveRootStatus = RootStatusScanning
			status.ActiveRunStatus = RunStatusExecuting
			status.ActiveRunKind = RunKindFull
			status.ActiveStage = RunStageExecuting
			status.ActiveRunElapsedMs = time.Since(state.started).Milliseconds()
		}
		if state.err != "" {
			status.ErrorRootCount++
			status.ErrorRootPath, status.LastError = root, state.err
		}
	}
	i.mu.Unlock()
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
	status.ActiveRunEntryCount, status.ActiveRunFileCount = stats.EntryCount, stats.FileCount
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
