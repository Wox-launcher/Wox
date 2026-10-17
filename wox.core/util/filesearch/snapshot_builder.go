package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"
	"wox/util/filesearch/directoryread"
)

type SnapshotBuilder struct {
	policy                      *policyState
	directFileBatchSize         int
	subtreeTraversalWorkerCount int
}

// Diagnostic addition: the real-index artifact needs the dominant streaming
// traversal split into filesystem read, entry type, policy, and metadata costs.
// Streaming traversal now uses bounded workers, so counters are atomic and the
// unreadable examples are sampled under a tiny lock instead of logging every
// protected macOS directory independently.
type subtreeStreamDiagnostics struct {
	readDirNanos      atomic.Int64
	readDirCount      atomic.Int64
	dirEntryTypeNanos atomic.Int64
	dirEntryTypeCount atomic.Int64
	policyCheckNanos  atomic.Int64
	policyCheckCount  atomic.Int64
	dirEntryInfoNanos atomic.Int64
	dirEntryInfoCount atomic.Int64

	unreadableCount    atomic.Int64
	unreadableExamples []string
	unreadableMu       sync.Mutex
}

func (d *subtreeStreamDiagnostics) recordReadDir(elapsed time.Duration) {
	d.readDirNanos.Add(elapsed.Nanoseconds())
	d.readDirCount.Add(1)
}

func (d *subtreeStreamDiagnostics) recordDirEntryType(elapsed time.Duration) {
	d.dirEntryTypeNanos.Add(elapsed.Nanoseconds())
	d.dirEntryTypeCount.Add(1)
}

func (d *subtreeStreamDiagnostics) recordPolicyCheck(elapsed time.Duration) {
	d.policyCheckNanos.Add(elapsed.Nanoseconds())
	d.policyCheckCount.Add(1)
}

func (d *subtreeStreamDiagnostics) recordDirEntryInfo(elapsed time.Duration) {
	d.dirEntryInfoNanos.Add(elapsed.Nanoseconds())
	d.dirEntryInfoCount.Add(1)
}

func (d *subtreeStreamDiagnostics) recordUnreadable(path string, err error) {
	if d == nil || err == nil {
		return
	}
	d.unreadableCount.Add(1)

	d.unreadableMu.Lock()
	defer d.unreadableMu.Unlock()
	if len(d.unreadableExamples) >= maxUnreadableTraversalExamples {
		return
	}
	example := strings.ReplaceAll(strings.TrimSpace(path+": "+err.Error()), "\n", " ")
	if example == "" {
		return
	}
	d.unreadableExamples = append(d.unreadableExamples, example)
}

func (d *subtreeStreamDiagnostics) unreadableSnapshot() (int64, []string) {
	if d == nil {
		return 0, nil
	}
	count := d.unreadableCount.Load()
	d.unreadableMu.Lock()
	examples := append([]string(nil), d.unreadableExamples...)
	d.unreadableMu.Unlock()
	return count, examples
}

func (d *subtreeStreamDiagnostics) log(ctx context.Context, scope string) {
	if d == nil {
		return
	}
	// Bug fix: this must read the accumulator at function exit. A value receiver
	// on a deferred method copied the zero-value counters at defer time, which
	// made the real-index artifact show scan timings with work_count=0.
	logFilesearchScanDiagnostic(ctx, "subtree_stream_readdir", scope, scanDiagnosticMillis(d.readDirNanos.Load()), int(d.readDirCount.Load()))
	logFilesearchScanDiagnostic(ctx, "subtree_stream_direntry_type", scope, scanDiagnosticMillis(d.dirEntryTypeNanos.Load()), int(d.dirEntryTypeCount.Load()))
	logFilesearchScanDiagnostic(ctx, "subtree_stream_policy_check", scope, scanDiagnosticMillis(d.policyCheckNanos.Load()), int(d.policyCheckCount.Load()))
	logFilesearchScanDiagnostic(ctx, "subtree_stream_direntry_info", scope, scanDiagnosticMillis(d.dirEntryInfoNanos.Load()), int(d.dirEntryInfoCount.Load()))
	unreadableCount, unreadableExamples := d.unreadableSnapshot()
	logFilesearchUnreadableSummary(ctx, scope, unreadableCount, unreadableExamples)
}

const maxUnreadableTraversalExamples = 12

func scanDiagnosticMillis(nanos int64) int64 {
	if nanos <= 0 {
		return 0
	}
	return (nanos + int64(time.Millisecond) - 1) / int64(time.Millisecond)
}

func NewSnapshotBuilder(policy *policyState) *SnapshotBuilder {
	if policy == nil {
		policy = newPolicyState(Policy{})
	}
	return &SnapshotBuilder{
		policy:                      policy,
		directFileBatchSize:         defaultSplitBudget().DirectFileBatchSize,
		subtreeTraversalWorkerCount: defaultSubtreeTraversalWorkerCount(),
	}
}

func defaultSubtreeTraversalWorkerCount() int {
	count := runtime.NumCPU()
	if count <= 0 {
		return 1
	}
	return count
}

func (b *SnapshotBuilder) SetDirectFileBatchSize(size int) {
	if b == nil || size <= 0 {
		return
	}
	b.directFileBatchSize = size
}

func (b *SnapshotBuilder) BuildRootEntries(ctx context.Context, root RootRecord) ([]EntryRecord, error) {
	snapshot, err := b.BuildSubtreeJobSnapshot(ctx, root, Job{
		RootID:    root.ID,
		RootPath:  root.Path,
		ScopePath: root.Path,
		Kind:      JobKindSubtree,
	})
	if err != nil {
		return nil, err
	}

	return snapshot.Entries, nil
}

// BuildDirectFilesJobSnapshot materializes the full direct-files scope owned by
// one sealed job. The planner no longer splits one directory into many direct-
// files jobs because that older shape made stale-file pruning ambiguous. This
// helper remains for tests and small direct-files paths; runtime execution now
// prefers StreamDirectFilesJobBatches to keep SQLite staging bounded in memory.
func (b *SnapshotBuilder) BuildDirectFilesJobSnapshot(ctx context.Context, root RootRecord, job Job) (SubtreeSnapshotBatch, error) {
	scopePath := filepath.Clean(job.ScopePath)
	batch := SubtreeSnapshotBatch{
		RootID:    root.ID,
		ScopePath: scopePath,
	}

	info, err := b.validateScopePath(root, scopePath)
	if err != nil || info == nil {
		return batch, err
	}
	if !info.IsDir() {
		return batch, fmt.Errorf("direct-files scope %q is not a directory", scopePath)
	}

	dirEntries, err := directoryread.ReadDir(ctx, scopePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Dirty scopes may disappear after validation in temp/build folders.
			// Returning the empty owned scope lets the DB prune stale rows without
			// promoting a vanished child path into a root-wide retry.
			return batch, nil
		}
		return batch, fmt.Errorf("failed to read direct-files scope %s: %w", scopePath, err)
	}

	policyContext := b.policy.newTraversalContext(root, scopePath)
	directFiles := make([]EntryRecord, 0, len(dirEntries))
	for _, dirEntry := range dirEntries {
		select {
		case <-ctx.Done():
			return batch, ctx.Err()
		default:
		}

		childPath := filepath.Join(scopePath, dirEntry.Name())
		isDir, childInfo, infoErr := strictDirEntryType(scopePath, dirEntry)
		if infoErr != nil {
			return batch, infoErr
		}
		if shouldSkipSystemPathForRoot(root, childPath, isDir) {
			continue
		}
		if !policyContext.ShouldIndexPath(childPath, isDir) {
			continue
		}
		if isDir {
			continue
		}
		if childInfo == nil {
			// The earlier eager Info() call paid metadata I/O even for entries that
			// policy or system-path filtering would skip. We only load FileInfo once
			// the file is confirmed indexable because newEntryRecord needs the full
			// stat payload for persisted metadata.
			childInfo, infoErr = strictDirEntryInfo(scopePath, dirEntry)
			if infoErr != nil {
				return batch, infoErr
			}
		}
		directFiles = append(directFiles, newEntryRecord(root, childPath, childInfo))
	}

	sort.Slice(directFiles, func(left int, right int) bool {
		return directFiles[left].Path < directFiles[right].Path
	})

	batch.Directories = append(batch.Directories, DirectoryRecord{
		Path:       scopePath,
		RootID:     root.ID,
		ParentPath: filepath.Dir(scopePath),
	})
	batch.Entries = append(batch.Entries, newEntryRecord(root, scopePath, info))
	batch.Entries = append(batch.Entries, directFiles...)
	return batch, nil
}

// StreamDirectFilesJobBatches emits one directory-owned direct-files job as
// bounded staging batches. The older planner solved memory pressure by turning
// one directory into many jobs, but that split stale-file ownership and made
// direct-file pruning ambiguous. Keeping one job per directory and streaming
// its files in small batches keeps ownership correct without rebuilding the
// original whole-directory memory spike.
func (b *SnapshotBuilder) StreamDirectFilesJobBatches(ctx context.Context, root RootRecord, job Job, onBatch func(SubtreeSnapshotBatch) error) error {
	if onBatch == nil {
		return fmt.Errorf("direct-files batch callback is required")
	}

	scopePath := filepath.Clean(job.ScopePath)
	info, err := b.validateScopePath(root, scopePath)
	if err != nil || info == nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("direct-files scope %q is not a directory", scopePath)
	}

	dirEntries, err := directoryread.ReadDir(ctx, scopePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Streaming direct-files jobs still own the directory scope. If the
			// directory vanishes after validation, an empty stream is enough for the
			// caller to prune staged direct-file rows at the same narrow boundary.
			return nil
		}
		return fmt.Errorf("failed to read direct-files scope %s: %w", scopePath, err)
	}

	scopeID := directoryEntryID(scopePath)
	newBatch := func(includeScope bool) SubtreeSnapshotBatch {
		batch := SubtreeSnapshotBatch{
			RootID:    root.ID,
			ScopePath: scopePath,
		}
		if includeScope {
			batch.Records = append(batch.Records, observedInfoRecord(scopeID, directoryEntryID(filepath.Dir(scopePath)), filepath.Base(scopePath), info))
		}
		return batch
	}
	flushBatch := func(batch *SubtreeSnapshotBatch) error {
		if batch == nil {
			return nil
		}
		if len(batch.Records) == 0 && len(batch.UnreadableScopes) == 0 {
			return nil
		}
		return onBatch(*batch)
	}

	batch := newBatch(true)
	filesInBatch := 0
	maxFilesPerBatch := b.maxRecordsPerBatch()

	policyContext := b.policy.newTraversalContext(root, scopePath)
	for _, dirEntry := range dirEntries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		childPath := filepath.Join(scopePath, dirEntry.Name())
		isDir, childInfo, infoErr := strictDirEntryType(scopePath, dirEntry)
		if infoErr != nil {
			return infoErr
		}
		if shouldSkipSystemPathForRoot(root, childPath, isDir) {
			continue
		}
		if !policyContext.ShouldIndexPath(childPath, isDir) {
			continue
		}
		if isDir {
			continue
		}
		if childInfo == nil {
			// Streaming direct-files batches now delays Info() until a child file
			// survives skip/policy checks, because the old eager stat path repeated
			// metadata work for entries that never reached SQLite staging.
			childInfo, infoErr = strictDirEntryInfo(scopePath, dirEntry)
			if infoErr != nil {
				return infoErr
			}
		}

		batch.Records = append(batch.Records, observedInfoRecord(directoryEntryID(childPath), scopeID, dirEntry.Name(), childInfo))
		filesInBatch++
		if filesInBatch < maxFilesPerBatch {
			continue
		}
		if err := flushBatch(&batch); err != nil {
			return err
		}
		batch = newBatch(false)
		filesInBatch = 0
	}

	return flushBatch(&batch)
}

func (b *SnapshotBuilder) maxRecordsPerBatch() int {
	if b.directFileBatchSize > 0 {
		return b.directFileBatchSize
	}
	if size := defaultSplitBudget().DirectFileBatchSize; size > 0 {
		return size
	}
	return 1024
}

// StreamDirectoryJobBatches relists one directory: its own record and every
// immediate child. Child directories the index already holds (knownDirectory)
// are emitted as rows but not descended into, since their contents report
// their own changes. Child directories new to the index are walked fully,
// because a tree moved or created in place may never report its descendants.
func (b *SnapshotBuilder) StreamDirectoryJobBatches(ctx context.Context, root RootRecord, job Job, knownDirectory func(id uint64) bool, onBatch func(SubtreeSnapshotBatch) error) error {
	if job.Kind != JobKindDirectory {
		return fmt.Errorf("stream directory requires kind %q, got %q", JobKindDirectory, job.Kind)
	}
	scopePath := filepath.Clean(job.ScopePath)
	scopeID := directoryEntryID(scopePath)
	batch := SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scopePath}

	info, err := b.validateScopePath(root, scopePath)
	if err != nil {
		return err
	}
	if info == nil || !info.IsDir() {
		// The writer deletes whatever this observation no longer contains: the
		// whole stored tree for a vanished or ignored path, or only the stored
		// descendants when a file replaced the directory.
		if info != nil {
			batch.Records = append(batch.Records, observedInfoRecord(scopeID, directoryEntryID(filepath.Dir(scopePath)), filepath.Base(scopePath), info))
		}
		return onBatch(batch)
	}

	dirEntries, err := directoryread.ReadDir(ctx, scopePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return onBatch(batch)
		}
		return fmt.Errorf("failed to read directory scope %s: %w", scopePath, err)
	}

	maxRecordsPerBatch := b.maxRecordsPerBatch()
	policyContext := traversalPolicyWithDirectoryEntries(b.policy.newTraversalContext(root, scopePath), scopePath, dirEntries)
	batch.Records = append(batch.Records, observedInfoRecord(scopeID, directoryEntryID(filepath.Dir(scopePath)), filepath.Base(scopePath), info))
	var newDirectories []subtreeStreamQueueItem
	for _, dirEntry := range dirEntries {
		if err := ctx.Err(); err != nil {
			return err
		}
		childPath := filepath.Join(scopePath, dirEntry.Name())
		isDir, childInfo, infoErr := strictDirEntryType(scopePath, dirEntry)
		if infoErr == nil {
			if shouldSkipSystemPathForRoot(root, childPath, isDir) || !policyContext.ShouldIndexPath(childPath, isDir) {
				continue
			}
			if childInfo == nil {
				childInfo, infoErr = strictDirEntryInfo(scopePath, dirEntry)
			}
		}
		if infoErr != nil {
			if !errors.Is(infoErr, os.ErrNotExist) {
				batch.UnreadableScopes = append(batch.UnreadableScopes, childPath)
			}
			continue
		}

		childID := directoryEntryID(childPath)
		if isDir && !knownDirectory(childID) {
			newDirectories = append(newDirectories, subtreeStreamQueueItem{
				path:     childPath,
				name:     dirEntry.Name(),
				id:       childID,
				parentID: scopeID,
				info:     childInfo,
				policy:   policyContext.Descend(childPath),
			})
			continue
		}
		batch.Records = append(batch.Records, observedInfoRecord(childID, scopeID, dirEntry.Name(), childInfo))
		if len(batch.Records)+len(batch.UnreadableScopes) >= maxRecordsPerBatch {
			if err := onBatch(batch); err != nil {
				return err
			}
			batch = SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scopePath}
		}
	}
	if len(batch.Records) > 0 || len(batch.UnreadableScopes) > 0 {
		if err := onBatch(batch); err != nil {
			return err
		}
	}
	if len(newDirectories) == 0 {
		return nil
	}
	return b.streamDirectoryTrees(ctx, root, scopePath, newDirectories, onBatch)
}

// BuildSubtreeJobSnapshot materializes only the subtree owned by one planned
// job. The previous whole-root accumulation forced execution to hold an entire
// root snapshot in memory even when the planner had already split that root
// into bounded jobs, which is what drove the earlier indexing-time memory spike.
func (b *SnapshotBuilder) BuildSubtreeJobSnapshot(ctx context.Context, root RootRecord, job Job) (SubtreeSnapshotBatch, error) {
	return b.BuildSubtreeSnapshot(ctx, root, job.ScopePath)
}

func (b *SnapshotBuilder) BuildSubtreeSnapshot(ctx context.Context, root RootRecord, scopePath string) (SubtreeSnapshotBatch, error) {
	scopePath = filepath.Clean(scopePath)
	batch := SubtreeSnapshotBatch{
		RootID:    root.ID,
		ScopePath: scopePath,
	}

	info, err := b.validateScopePath(root, scopePath)
	if err != nil {
		return batch, err
	}
	if info == nil {
		return batch, nil
	}

	if !info.IsDir() {
		batch.Entries = append(batch.Entries, newEntryRecord(root, scopePath, info))
		return batch, nil
	}

	type queueItem struct {
		path   string
		info   os.FileInfo
		policy TraversalPolicyContext
	}

	queue := []queueItem{{
		path:   scopePath,
		info:   info,
		policy: b.policy.newTraversalContext(root, scopePath),
	}}

	for len(queue) > 0 {
		select {
		case <-ctx.Done():
			return batch, ctx.Err()
		default:
		}

		current := queue[0]
		queue = queue[1:]

		dirEntries, readErr := directoryread.ReadDir(ctx, current.path)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				// The snapshot builder used to record the directory as existing
				// before ReadDir. A temp/build directory can disappear in that tiny
				// window, so missing scopes now produce an empty owned snapshot that
				// prunes stale rows without retrying the whole root.
				if current.path == scopePath {
					return SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scopePath}, nil
				}
				continue
			}
			batch.Directories = append(batch.Directories, DirectoryRecord{
				Path:       current.path,
				RootID:     root.ID,
				ParentPath: filepath.Dir(current.path),
			})
			batch.Entries = append(batch.Entries, newEntryRecord(root, current.path, current.info))
			if current.path == scopePath {
				return batch, fmt.Errorf("failed to read scope directory %s: %w", current.path, readErr)
			}
			batch.UnreadableScopes = append(batch.UnreadableScopes, current.path)
			util.GetLogger().Warn(ctx, "filesearch skipped unreadable directory "+current.path+": "+readErr.Error())
			continue
		}
		currentPolicy := traversalPolicyWithDirectoryEntries(current.policy, current.path, dirEntries)
		batch.Directories = append(batch.Directories, DirectoryRecord{
			Path:       current.path,
			RootID:     root.ID,
			ParentPath: filepath.Dir(current.path),
		})
		batch.Entries = append(batch.Entries, newEntryRecord(root, current.path, current.info))

		for _, dirEntry := range dirEntries {
			childPath := filepath.Join(current.path, dirEntry.Name())
			isDir, info, infoErr := strictDirEntryType(current.path, dirEntry)
			if infoErr != nil {
				if !errors.Is(infoErr, os.ErrNotExist) {
					batch.UnreadableScopes = append(batch.UnreadableScopes, childPath)
				}
				continue
			}

			if shouldSkipSystemPathForRoot(root, childPath, isDir) {
				continue
			}
			if !currentPolicy.ShouldIndexPath(childPath, isDir) {
				continue
			}
			if info == nil {
				// Recursive subtree snapshots must still persist real mtime/size data,
				// but loading FileInfo after skip/policy checks avoids extra stat calls
				// for entries that would never be indexed.
				info, infoErr = strictDirEntryInfo(current.path, dirEntry)
				if infoErr != nil {
					if !errors.Is(infoErr, os.ErrNotExist) {
						batch.UnreadableScopes = append(batch.UnreadableScopes, childPath)
					}
					continue
				}
			}

			if !isDir {
				batch.Entries = append(batch.Entries, newEntryRecord(root, childPath, info))
				continue
			}

			// Optimization: recursive snapshots now carry the same traversal policy
			// context as the streaming path. The previous per-path callback rebuilt
			// ignore ancestors for every child, while the queued context keeps
			// .gitignore/configured-rule state aligned with the accepted directory.
			queue = append(queue, queueItem{
				path:   childPath,
				info:   info,
				policy: currentPolicy.Descend(childPath),
			})
		}
	}

	return batch, nil
}

// subtreeStreamQueueItem is one directory waiting to be listed. Its own and its
// parent's column IDs travel with it, so each directory ID is hashed once by
// whoever listed it and children inherit it as their ParentID.
type subtreeStreamQueueItem struct {
	path     string
	name     string
	id       uint64
	parentID uint64
	info     os.FileInfo
	policy   TraversalPolicyContext
}

type subtreeStreamQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []subtreeStreamQueueItem
	active int
	err    error
}

func newSubtreeStreamQueue(initial []subtreeStreamQueueItem) *subtreeStreamQueue {
	queue := &subtreeStreamQueue{
		items: initial,
	}
	queue.cond = sync.NewCond(&queue.mu)
	return queue
}

func (q *subtreeStreamQueue) pop() (subtreeStreamQueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && q.active > 0 && q.err == nil {
		q.cond.Wait()
	}
	if q.err != nil || len(q.items) == 0 {
		return subtreeStreamQueueItem{}, false
	}

	item := q.items[0]
	var zero subtreeStreamQueueItem
	q.items[0] = zero
	q.items = q.items[1:]
	q.active++
	return item, true
}

func (q *subtreeStreamQueue) push(item subtreeStreamQueueItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.err != nil {
		return false
	}
	q.items = append(q.items, item)
	q.cond.Signal()
	return true
}

func (q *subtreeStreamQueue) done() {
	q.mu.Lock()
	q.active--
	if q.active == 0 && len(q.items) == 0 {
		q.cond.Broadcast()
	}
	q.mu.Unlock()
}

func (q *subtreeStreamQueue) fail(err error) {
	if err == nil {
		return
	}
	q.mu.Lock()
	if q.err == nil {
		q.err = err
	}
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *subtreeStreamQueue) currentErr() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.err
}

// StreamSubtreeJobBatches walks one recursive subtree once and emits bounded
// batches to the caller. Full indexing no longer performs an exact recursive count, so
// this streaming path is the primary large-root traversal instead of a second
// copy of the planner's earlier walk.
func (b *SnapshotBuilder) StreamSubtreeJobBatches(ctx context.Context, root RootRecord, job Job, onBatch func(SubtreeSnapshotBatch) error) error {
	if onBatch == nil {
		return fmt.Errorf("subtree batch callback is required")
	}
	if job.Kind != JobKindSubtree {
		return fmt.Errorf("stream subtree requires kind %q, got %q", JobKindSubtree, job.Kind)
	}

	scopePath := filepath.Clean(job.ScopePath)
	info, err := b.validateScopePath(root, scopePath)
	if err != nil {
		return err
	}
	if info == nil {
		return onBatch(SubtreeSnapshotBatch{RootID: root.ID, ScopePath: scopePath})
	}
	if !info.IsDir() {
		return onBatch(SubtreeSnapshotBatch{
			RootID:    root.ID,
			ScopePath: scopePath,
			Entries:   []EntryRecord{newEntryRecord(root, scopePath, info)},
		})
	}

	return b.streamDirectoryTrees(ctx, root, scopePath, []subtreeStreamQueueItem{{
		path:     scopePath,
		name:     filepath.Base(scopePath),
		id:       directoryEntryID(scopePath),
		parentID: directoryEntryID(filepath.Dir(scopePath)),
		info:     info,
		policy:   b.policy.newTraversalContext(root, scopePath),
	}}, onBatch)
}

// streamDirectoryTrees walks every seed directory recursively with the bounded
// worker pool and emits batches owned by scopePath. Only a failure to read
// scopePath itself is fatal; other unreadable directories are reported as
// unreadable scopes so their prior facts are preserved.
func (b *SnapshotBuilder) streamDirectoryTrees(ctx context.Context, root RootRecord, scopePath string, seeds []subtreeStreamQueueItem, onBatch func(SubtreeSnapshotBatch) error) error {
	diagnostics := &subtreeStreamDiagnostics{}
	defer diagnostics.log(ctx, scopePath)

	maxRecordsPerBatch := b.maxRecordsPerBatch()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopCancelWatcher := make(chan struct{})
	workQueue := newSubtreeStreamQueue(seeds)
	go func() {
		select {
		case <-streamCtx.Done():
			workQueue.fail(streamCtx.Err())
		case <-stopCancelWatcher:
		}
	}()
	defer close(stopCancelWatcher)

	workerCount := b.subtreeTraversalWorkerCount
	if workerCount <= 0 {
		workerCount = defaultSubtreeTraversalWorkerCount()
	}
	batchCh := make(chan SubtreeSnapshotBatch, workerCount*2)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := 0; index < workerCount; index++ {
		go func() {
			defer workers.Done()
			if err := b.streamSubtreeWorker(streamCtx, root, scopePath, maxRecordsPerBatch, workQueue, batchCh, diagnostics); err != nil {
				workQueue.fail(err)
				cancel()
			}
		}()
	}
	go func() {
		workers.Wait()
		close(batchCh)
	}()

	// Traversal, stat and record resolution run in bounded workers, while this
	// goroutine remains the only caller of onBatch, so the column builder keeps
	// a single writer and only inserts already-resolved rows.
	var callbackErr error
	for batch := range batchCh {
		if callbackErr != nil {
			continue
		}
		if err := onBatch(batch); err != nil {
			callbackErr = err
			workQueue.fail(err)
			cancel()
		}
	}
	if callbackErr != nil {
		return callbackErr
	}
	if err := workQueue.currentErr(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

func (b *SnapshotBuilder) streamSubtreeWorker(ctx context.Context, root RootRecord, scopePath string, maxRecordsPerBatch int, workQueue *subtreeStreamQueue, batchCh chan<- SubtreeSnapshotBatch, diagnostics *subtreeStreamDiagnostics) error {
	newBatch := func() SubtreeSnapshotBatch {
		return SubtreeSnapshotBatch{
			RootID:    root.ID,
			ScopePath: scopePath,
		}
	}
	shouldFlush := func(batch SubtreeSnapshotBatch) bool {
		return len(batch.Records)+len(batch.UnreadableScopes) >= maxRecordsPerBatch
	}
	flushBatch := func(batch *SubtreeSnapshotBatch) error {
		if batch == nil {
			return nil
		}
		if len(batch.Records) == 0 && len(batch.UnreadableScopes) == 0 {
			return nil
		}
		select {
		case batchCh <- *batch:
			*batch = newBatch()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	batch := newBatch()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		current, ok := workQueue.pop()
		if !ok {
			return flushBatch(&batch)
		}
		if err := func() error {
			defer workQueue.done()

			readStartedAt := time.Now()
			dirEntries, readErr := directoryread.ReadDir(ctx, current.path)
			diagnostics.recordReadDir(time.Since(readStartedAt))
			if readErr != nil {
				if errors.Is(readErr, os.ErrNotExist) {
					return nil
				}
				if current.path == scopePath {
					return fmt.Errorf("failed to read scope directory %s: %w", current.path, readErr)
				}
				diagnostics.recordUnreadable(current.path, readErr)
				batch.UnreadableScopes = append(batch.UnreadableScopes, current.path)
				batch.Records = append(batch.Records, observedInfoRecord(current.id, current.parentID, current.name, current.info))
				if shouldFlush(batch) {
					return flushBatch(&batch)
				}
				return nil
			}
			currentPolicy := traversalPolicyWithDirectoryEntries(current.policy, current.path, dirEntries)

			batch.Records = append(batch.Records, observedInfoRecord(current.id, current.parentID, current.name, current.info))
			if shouldFlush(batch) {
				if err := flushBatch(&batch); err != nil {
					return err
				}
			}

			for _, dirEntry := range dirEntries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				childPath := filepath.Join(current.path, dirEntry.Name())
				typeStartedAt := time.Now()
				isDir, childInfo, infoErr := strictDirEntryType(current.path, dirEntry)
				diagnostics.recordDirEntryType(time.Since(typeStartedAt))
				if infoErr != nil {
					if !errors.Is(infoErr, os.ErrNotExist) {
						diagnostics.recordUnreadable(childPath, infoErr)
						batch.UnreadableScopes = append(batch.UnreadableScopes, childPath)
					}
					continue
				}
				if shouldSkipSystemPathForRoot(root, childPath, isDir) {
					continue
				}
				policyStartedAt := time.Now()
				shouldIndex := currentPolicy.ShouldIndexPath(childPath, isDir)
				diagnostics.recordPolicyCheck(time.Since(policyStartedAt))
				if !shouldIndex {
					continue
				}
				if childInfo == nil {
					infoStartedAt := time.Now()
					childInfo, infoErr = strictDirEntryInfo(current.path, dirEntry)
					diagnostics.recordDirEntryInfo(time.Since(infoStartedAt))
					if infoErr != nil {
						if !errors.Is(infoErr, os.ErrNotExist) {
							diagnostics.recordUnreadable(childPath, infoErr)
							batch.UnreadableScopes = append(batch.UnreadableScopes, childPath)
						}
						continue
					}
				}
				childID := directoryEntryID(childPath)
				if isDir {
					if !workQueue.push(subtreeStreamQueueItem{
						path:     childPath,
						name:     dirEntry.Name(),
						id:       childID,
						parentID: current.id,
						info:     childInfo,
						policy:   currentPolicy.Descend(childPath),
					}) {
						if err := workQueue.currentErr(); err != nil {
							return err
						}
						return ctx.Err()
					}
					continue
				}

				batch.Records = append(batch.Records, observedInfoRecord(childID, current.id, dirEntry.Name(), childInfo))
				if shouldFlush(batch) {
					if err := flushBatch(&batch); err != nil {
						return err
					}
				}
			}
			return nil
		}(); err != nil {
			return err
		}
	}
}

func (b *SnapshotBuilder) validateScopePath(root RootRecord, scopePath string) (os.FileInfo, error) {
	if root.ID == "" {
		return nil, fmt.Errorf("root id is required")
	}
	if root.Path == "" {
		return nil, fmt.Errorf("root path is required")
	}
	if scopePath == "" || !filepath.IsAbs(scopePath) {
		return nil, fmt.Errorf("scope path %q is invalid", scopePath)
	}
	if !pathWithinScope(root.Path, scopePath) {
		return nil, fmt.Errorf("scope path %q is outside root path %q", scopePath, root.Path)
	}

	var info os.FileInfo
	var err error
	if scopePath == filepath.Clean(root.Path) {
		info, err = os.Stat(scopePath)
	} else {
		// Bug fix: incremental dirty scopes should not be able to follow a
		// symlink-to-directory when full scans do not follow symlink children. Keep
		// explicit symlink roots working via Stat above, but classify nested scopes
		// with Lstat so the link itself is indexed without traversing the target.
		info, err = os.Lstat(scopePath)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	if scopePath != filepath.Clean(root.Path) && !b.shouldIndexScopePath(root, scopePath, info.IsDir()) {
		return nil, nil
	}

	return info, nil
}

func (b *SnapshotBuilder) shouldIndexScopePath(root RootRecord, path string, isDir bool) bool {
	if shouldSkipSystemPathForRoot(root, path, isDir) {
		return false
	}
	if b == nil || b.policy == nil {
		return true
	}
	cleanPath := filepath.Clean(path)
	context := b.policy.newTraversalContext(root, filepath.Dir(cleanPath))
	// Bug fix: dirty-scope validation now uses the same traversal policy as the
	// subtree scanner. Keeping scope checks on the removed per-path callback would
	// let ignored dirty paths enter execution through a different matcher.
	return context.ShouldIndexPath(cleanPath, isDir)
}

func traversalPolicyWithDirectoryEntries(policy TraversalPolicyContext, directoryPath string, entries []os.DirEntry) TraversalPolicyContext {
	if policy == nil {
		return allowAllTraversalPolicyContext{}
	}
	entryAware, ok := policy.(DirectoryEntryAwareTraversalPolicyContext)
	if !ok {
		return policy
	}
	// Optimization: scanners already have the ReadDir payload for the directory
	// being evaluated. Passing it through the optional policy hook lets rich
	// policies load directory-local ignore files only when they are present,
	// while simpler policies continue to use the base interface unchanged.
	updated := entryAware.WithDirectoryEntries(directoryPath, entries)
	if updated == nil {
		return policy
	}
	return updated
}
