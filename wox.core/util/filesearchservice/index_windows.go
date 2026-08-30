//go:build windows

package filesearchservice

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"

	"wox/util"
	"wox/util/fuzzymatch"

	"golang.org/x/sys/windows"
)

const (
	fsctlQueryUSNJournal          = 0x000900f4
	fsctlReadUSNJournal           = 0x000900bb
	usnReadBufferSize             = 1 << 20
	usnReasonFileCreate           = 0x00000100
	usnReasonFileDelete           = 0x00000200
	usnReasonRenameOldName        = 0x00001000
	usnReasonRenameNewName        = 0x00002000
	columnFlagDirectory           = 1
	columnNameFlagNonASCII        = 1
	invalidColumnRow       uint32 = math.MaxUint32
	maxPathDepth                  = 1024
)

type usnJournalData struct {
	JournalID       uint64
	FirstUSN        int64
	NextUSN         int64
	LowestValidUSN  int64
	MaxUSN          int64
	MaximumSize     uint64
	AllocationDelta uint64
}

type readUSNJournalData struct {
	StartUSN          int64
	ReasonMask        uint32
	ReturnOnlyOnClose uint32
	Timeout           uint64
	BytesToWaitFor    uint64
	JournalID         uint64
	MinMajorVersion   uint16
	MaxMajorVersion   uint16
}

// ColumnIndex owns the compact filename index used by the privileged service.
// Rebuilds prepare a replacement off to the side so queries can keep using the
// previous immutable snapshot until the new one is complete.
type ColumnIndex struct {
	ctx         context.Context
	cancel      context.CancelFunc
	snapshot    atomic.Pointer[columnSnapshot]
	snapshotMu  sync.RWMutex
	stateMu     sync.Mutex
	indexDir    *secureIndexDirectory
	paused      bool
	buildCancel context.CancelFunc
	buildDone   chan struct{}
	rebuilding  atomic.Bool
	statsMu     sync.RWMutex
	stats       IndexStats
	startedAt   time.Time
}

type columnSnapshot struct {
	volumes  []*volumeColumn
	cancel   context.CancelFunc
	monitors sync.WaitGroup
	storage  *mappedSnapshotDirectory
}

// volumeColumn stores row relationships and deduplicated names in parallel
// slices. Full paths are deliberately absent and are materialized only for
// rows that survive the bounded name search.
type volumeColumn struct {
	root          string
	references    []uint64
	parentRows    []uint32
	nameIDs       []uint32
	rowFlags      []byte
	nameOffsets   []uint32
	nameBytes     []byte
	nameMasks     []uint64
	nameFlags     []byte
	nameRowStarts []uint32
	nameRows      []uint32
	referenceRows []uint32
	journalID     uint64
	nextUSN       int64
	deltaMu       sync.RWMutex
	delta         map[uint64]deltaNode
	deltaFiles    int64
	deltaDirs     int64
	fileCount     int64
	storage       *mappedColumnFile
}

type deltaNode struct {
	parentReference uint64
	name            string
	isDir           bool
	deleted         bool
}

type volumeBuilder struct {
	root             string
	references       []uint64
	parentReferences []uint64
	nameIDs          []uint32
	rowFlags         []byte
	referenceRows    map[uint64]uint32
	nameLookup       map[string]uint32
	names            []string
	rootReference    uint64
	directoryCount   int64
}

type nameHit struct {
	nameID uint32
	score  int64
}

type nameHitHeap []nameHit

func (h nameHitHeap) Len() int           { return len(h) }
func (h nameHitHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h nameHitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *nameHitHeap) Push(value any)    { *h = append(*h, value.(nameHit)) }
func (h *nameHitHeap) Pop() any {
	old := *h
	last := old[len(old)-1]
	*h = old[:len(old)-1]
	return last
}

// NewColumnIndex starts an empty service-owned index.
func NewColumnIndex(parent context.Context) *ColumnIndex {
	ctx, cancel := context.WithCancel(parent)
	return &ColumnIndex{ctx: ctx, cancel: cancel, paused: true}
}

// Stats returns a cheap snapshot of the current build or published column index.
func (i *ColumnIndex) Stats() IndexStats {
	if i == nil {
		return IndexStats{}
	}
	i.statsMu.RLock()
	stats := i.stats
	startedAt := i.startedAt
	i.statsMu.RUnlock()
	if stats.IsIndexing && !startedAt.IsZero() {
		stats.ElapsedMs = time.Since(startedAt).Milliseconds()
	}
	i.snapshotMu.RLock()
	if snapshot := i.snapshot.Load(); snapshot != nil {
		if !stats.IsIndexing {
			stats.EntryCount, stats.FileCount = snapshot.counts()
		}
		if len(stats.Volumes) == 0 {
			stats.Volumes = snapshotVolumeRoots(snapshot)
		}
	}
	i.snapshotMu.RUnlock()
	return stats
}

// snapshotVolumeRoots lists the drive roots present in a published column snapshot.
func snapshotVolumeRoots(snapshot *columnSnapshot) []string {
	if snapshot == nil {
		return nil
	}
	roots := make([]string, 0, len(snapshot.volumes))
	for _, volume := range snapshot.volumes {
		if volume == nil || strings.TrimSpace(volume.root) == "" {
			continue
		}
		roots = append(roots, volume.root)
	}
	return roots
}

// Close stops journal monitors associated with the current snapshot.
func (i *ColumnIndex) Close() {
	if i == nil {
		return
	}
	i.cancel()
	_ = i.Pause(context.Background())
}

// RebuildAsync schedules one all-volume rebuild and coalesces concurrent requests.
func (i *ColumnIndex) RebuildAsync() bool {
	if i == nil {
		return false
	}
	i.stateMu.Lock()
	if i.paused || i.indexDir == nil || !i.rebuilding.CompareAndSwap(false, true) {
		i.stateMu.Unlock()
		return false
	}
	buildCtx, buildCancel := context.WithCancel(i.ctx)
	buildDone := make(chan struct{})
	indexDir := i.indexDir
	i.buildCancel = buildCancel
	i.buildDone = buildDone
	i.stateMu.Unlock()
	i.statsMu.Lock()
	i.startedAt = time.Now()
	i.stats = IndexStats{IsIndexing: true}
	i.statsMu.Unlock()
	util.Go(buildCtx, "file index service column rebuild", func() {
		err := i.rebuild(buildCtx, indexDir)
		i.rebuilding.Store(false)
		i.finishRebuild(err)
		i.stateMu.Lock()
		if i.buildDone == buildDone {
			i.buildCancel = nil
			i.buildDone = nil
		}
		close(buildDone)
		i.stateMu.Unlock()
		if err != nil && !errors.Is(err, context.Canceled) {
			util.GetLogger().Error(i.ctx, "file index service rebuild failed: "+err.Error())
		}
	})
	return true
}

// Pause cancels an active build, drains readers and monitors, and releases every cache handle.
func (i *ColumnIndex) Pause(ctx context.Context) error {
	if i == nil {
		return nil
	}
	i.stateMu.Lock()
	i.paused = true
	if i.buildCancel != nil {
		i.buildCancel()
	}
	buildDone := i.buildDone
	i.stateMu.Unlock()
	if buildDone != nil {
		select {
		case <-buildDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	i.snapshotMu.Lock()
	snapshot := i.snapshot.Swap(nil)
	i.snapshotMu.Unlock()
	if snapshot != nil {
		snapshot.close()
	}
	i.stateMu.Lock()
	indexDir := i.indexDir
	i.indexDir = nil
	i.stateMu.Unlock()
	if indexDir != nil {
		_ = indexDir.close()
	}
	i.statsMu.Lock()
	i.stats.IsIndexing = false
	i.stats.EntryCount = 0
	i.stats.FileCount = 0
	i.stats.IndexBytes = 0
	i.stats.IndexCapacityBytes = 0
	i.statsMu.Unlock()
	return nil
}

// Resume validates and pins the recreated owner directory before rebuilding.
func (i *ColumnIndex) Resume(ctx context.Context, ownerSID string, indexPath string) error {
	if i == nil {
		return ErrIndexNotReady
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	i.stateMu.Lock()
	paused := i.paused
	i.stateMu.Unlock()
	if paused {
		if err := i.Pause(ctx); err != nil {
			return err
		}
	}
	indexDir, err := openSecureIndexDirectory(ownerSID, indexPath)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = indexDir.close()
		return err
	}
	i.stateMu.Lock()
	if !i.paused && i.indexDir != nil && strings.EqualFold(i.indexDir.path, indexDir.path) {
		i.stateMu.Unlock()
		_ = indexDir.close()
		if i.snapshot.Load() == nil {
			i.RebuildAsync()
		}
		return nil
	}
	previous := i.indexDir
	i.indexDir = indexDir
	i.paused = false
	i.stateMu.Unlock()
	if previous != nil {
		_ = previous.close()
	}
	if !i.RebuildAsync() && i.snapshot.Load() == nil {
		return errors.New("file index service could not start rebuild")
	}
	return nil
}

func (i *ColumnIndex) finishRebuild(err error) {
	i.statsMu.Lock()
	defer i.statsMu.Unlock()
	i.stats.IsIndexing = false
	if !i.startedAt.IsZero() {
		i.stats.LastElapsedMs = time.Since(i.startedAt).Milliseconds()
		i.stats.ElapsedMs = i.stats.LastElapsedMs
	}
	i.stats.CurrentVolume = ""
	if err != nil && !errors.Is(err, context.Canceled) {
		i.stats.Error = err.Error()
	}
}

// rebuild prepares all eligible volumes before atomically publishing a snapshot.
func (i *ColumnIndex) rebuild(ctx context.Context, indexDir *secureIndexDirectory) error {
	started := time.Now()
	roots, err := fixedNTFSVolumeRoots()
	if err != nil {
		return err
	}
	i.updateBuildProgress(0, len(roots), "", 0, 0, 0, 0)
	snapshotDir, err := indexDir.createSnapshotDirectory()
	if err != nil {
		return fmt.Errorf("create file index snapshot directory: %w", err)
	}
	next := &columnSnapshot{storage: snapshotDir}
	published := false
	defer func() {
		if !published {
			next.close()
		}
	}()
	volumes := make([]*volumeColumn, 0, len(roots))
	var completedEntries, completedFiles, completedBytes int64
	for rootIndex, root := range roots {
		volume, buildErr := buildVolumeColumn(ctx, root, func(entryCount, fileCount int64) {
			i.updateBuildProgress(rootIndex+1, len(roots), root, completedEntries+entryCount, completedFiles+fileCount, completedBytes, len(volumes))
		})
		if buildErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("file index service skipped volume %s: %v", root, buildErr))
			continue
		}
		columnFile, fileErr := snapshotDir.createFile(fmt.Sprintf("volume-%d.columns", rootIndex))
		if fileErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("file index service could not create volume file %s: %v", root, fileErr))
			continue
		}
		volume, buildErr = writeMappedVolumeColumnFile(columnFile, volume, "")
		if buildErr != nil {
			columnFile.Close()
			util.GetLogger().Warn(ctx, fmt.Sprintf("file index service could not map volume %s: %v", root, buildErr))
			continue
		}
		volumes = append(volumes, volume)
		completedEntries += int64(len(volume.references))
		completedFiles += volume.fileCount
		completedBytes += volume.indexBytes()
		i.updateBuildProgress(rootIndex+1, len(roots), root, completedEntries, completedFiles, completedBytes, len(volumes))
	}
	if len(volumes) == 0 {
		return errors.New("no local fixed NTFS volume could be indexed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	monitorCtx, monitorCancel := context.WithCancel(i.ctx)
	next.volumes = volumes
	next.cancel = monitorCancel
	i.snapshotMu.Lock()
	previous := i.snapshot.Swap(next)
	for _, volume := range volumes {
		next.monitors.Add(1)
		go func() {
			defer next.monitors.Done()
			i.monitorVolume(monitorCtx, volume)
		}()
	}
	i.snapshotMu.Unlock()
	published = true
	if previous != nil {
		previous.close()
	}

	// Full MFT builds allocate large temporary maps and name strings, then go
	// idle before the runtime has another reason to collect them. Reclaim those
	// pages once at the publish boundary instead of carrying build memory in the
	// long-lived service or adding GC work to search and USN hot paths.
	debug.FreeOSMemory()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	i.statsMu.Lock()
	for _, volume := range volumes {
		i.stats.IndexCapacityBytes += volume.indexCapacityBytes()
	}
	i.stats.HeapAllocBytes = int64(memory.HeapAlloc)
	i.stats.HeapInuseBytes = int64(memory.HeapInuse)
	i.stats.HeapReleasedBytes = int64(memory.HeapReleased)
	i.statsMu.Unlock()
	util.GetLogger().Info(ctx, fmt.Sprintf("file index service column rebuild complete: volumes=%d entries=%d elapsed=%s", len(volumes), completedEntries, time.Since(started)))
	return nil
}

func (i *ColumnIndex) updateBuildProgress(volumeIndex, volumeTotal int, currentVolume string, entryCount, fileCount, indexBytes int64, volumeCount int) {
	i.statsMu.Lock()
	i.stats.VolumeIndex = volumeIndex
	i.stats.VolumeTotal = volumeTotal
	i.stats.CurrentVolume = currentVolume
	i.stats.EntryCount = entryCount
	i.stats.FileCount = fileCount
	i.stats.IndexBytes = indexBytes
	i.stats.VolumeCount = volumeCount
	i.stats.Error = ""
	i.statsMu.Unlock()
}

func (s *columnSnapshot) counts() (entryCount, fileCount int64) {
	for _, volume := range s.volumes {
		volume.deltaMu.RLock()
		entryCount += int64(len(volume.references)) + volume.deltaFiles + volume.deltaDirs
		fileCount += volume.fileCount + volume.deltaFiles
		volume.deltaMu.RUnlock()
	}
	return entryCount, fileCount
}

func (s *columnSnapshot) close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.monitors.Wait()
	for _, volume := range s.volumes {
		if volume.storage != nil {
			_ = volume.storage.close()
		}
	}
	if s.storage != nil {
		_ = s.storage.close()
	}
}

func (v *volumeColumn) indexBytes() int64 {
	return int64(len(v.references))*8 +
		int64(len(v.parentRows))*4 +
		int64(len(v.nameIDs))*4 +
		int64(len(v.rowFlags)) +
		int64(len(v.nameOffsets))*4 +
		int64(len(v.nameBytes)) +
		int64(len(v.nameMasks))*8 +
		int64(len(v.nameFlags)) +
		int64(len(v.nameRowStarts))*4 +
		int64(len(v.nameRows))*4 +
		int64(len(v.referenceRows))*4
}

func (v *volumeColumn) indexCapacityBytes() int64 {
	return int64(cap(v.references))*8 +
		int64(cap(v.parentRows))*4 +
		int64(cap(v.nameIDs))*4 +
		int64(cap(v.rowFlags)) +
		int64(cap(v.nameOffsets))*4 +
		int64(cap(v.nameBytes)) +
		int64(cap(v.nameMasks))*8 +
		int64(cap(v.nameFlags)) +
		int64(cap(v.nameRowStarts))*4 +
		int64(cap(v.nameRows))*4 +
		int64(cap(v.referenceRows))*4
}

// Search returns bounded filename matches across every indexed NTFS volume.
func (i *ColumnIndex) Search(ctx context.Context, query string, limit int, usePinyin bool) ([]SnapshotEntry, error) {
	if i == nil {
		return nil, ErrIndexNotReady
	}
	i.snapshotMu.RLock()
	defer i.snapshotMu.RUnlock()
	snapshot := i.snapshot.Load()
	if snapshot == nil {
		return nil, ErrIndexNotReady
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	results := make([]SnapshotEntry, 0, limit*len(snapshot.volumes))
	for _, volume := range snapshot.volumes {
		matches, err := volume.search(ctx, query, limit, usePinyin)
		if err != nil {
			return nil, err
		}
		results = append(results, matches...)
	}
	sort.Slice(results, func(left, right int) bool {
		if results[left].Score != results[right].Score {
			return results[left].Score > results[right].Score
		}
		if results[left].IsDir != results[right].IsDir {
			return results[left].IsDir
		}
		return strings.ToLower(results[left].Path) < strings.ToLower(results[right].Path)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// search scores each unique name once, then expands only the best name IDs to rows.
func (v *volumeColumn) search(ctx context.Context, query string, limit int, usePinyin bool) ([]SnapshotEntry, error) {
	queryMask := asciiCharacterMask(query)
	nameLimit := limit * 4
	if nameLimit < 512 {
		nameLimit = 512
	}
	if nameLimit > 4096 {
		nameLimit = 4096
	}
	hits := make(nameHitHeap, 0, nameLimit)
	heap.Init(&hits)

	v.deltaMu.RLock()
	defer v.deltaMu.RUnlock()
	for nameID := 1; nameID+1 < len(v.nameOffsets); nameID++ {
		if nameID&4095 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if queryMask != 0 && v.nameFlags[nameID]&columnNameFlagNonASCII == 0 && v.nameMasks[nameID]&queryMask != queryMask {
			continue
		}
		match := fuzzymatch.FuzzyMatch(v.name(uint32(nameID)), query, usePinyin)
		if !match.IsMatch {
			continue
		}
		hit := nameHit{nameID: uint32(nameID), score: match.Score}
		if len(hits) < nameLimit {
			heap.Push(&hits, hit)
		} else if hit.score > hits[0].score {
			hits[0] = hit
			heap.Fix(&hits, 0)
		}
	}
	sort.Slice(hits, func(left, right int) bool { return hits[left].score > hits[right].score })

	results := make([]SnapshotEntry, 0, limit)
	baseCandidateLimit := limit * 2
	baseCandidates := 0
	baseDone := false
	for _, hit := range hits {
		start, end := v.nameRowStarts[hit.nameID], v.nameRowStarts[hit.nameID+1]
		for _, row := range v.nameRows[start:end] {
			reference := v.references[row]
			if _, changed := v.delta[reference]; changed {
				continue
			}
			path, ok := v.resolvePath(reference)
			if !ok {
				continue
			}
			results = append(results, SnapshotEntry{Path: path, IsDir: v.rowFlags[row]&columnFlagDirectory != 0, Score: hit.score})
			baseCandidates++
			if baseCandidates == baseCandidateLimit {
				baseDone = true
				break
			}
		}
		if baseDone {
			break
		}
	}

	for reference, node := range v.delta {
		if node.deleted || node.name == "" {
			continue
		}
		match := fuzzymatch.FuzzyMatch(node.name, query, usePinyin)
		if !match.IsMatch {
			continue
		}
		path, ok := v.resolvePath(reference)
		if !ok {
			continue
		}
		results = append(results, SnapshotEntry{Path: path, IsDir: node.isDir, Score: match.Score})
	}
	sort.Slice(results, func(left, right int) bool { return results[left].Score > results[right].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (v *volumeColumn) name(nameID uint32) string {
	start, end := v.nameOffsets[nameID], v.nameOffsets[nameID+1]
	if start == end {
		return ""
	}
	// The backing blob is immutable for the lifetime of the snapshot, so a
	// zero-copy string view avoids allocating once per filename on every query.
	return unsafe.String(&v.nameBytes[start], int(end-start))
}

// resolvePath follows parent references through the mutable delta and immutable base.
func (v *volumeColumn) resolvePath(reference uint64) (string, bool) {
	segments := make([]string, 0, 16)
	for depth := 0; depth < maxPathDepth; depth++ {
		if node, changed := v.delta[reference]; changed {
			if node.deleted {
				return "", false
			}
			segments = append(segments, node.name)
			reference = node.parentReference
			continue
		}
		row, ok := v.rowForReference(reference)
		if !ok {
			return "", false
		}
		if row == 0 {
			path := v.root
			for index := len(segments) - 1; index >= 0; index-- {
				path = filepath.Join(path, segments[index])
			}
			return path, true
		}
		segments = append(segments, v.name(v.nameIDs[row]))
		parentRow := v.parentRows[row]
		if parentRow == invalidColumnRow {
			return "", false
		}
		reference = v.references[parentRow]
	}
	return "", false
}

func (v *volumeColumn) rowForReference(reference uint64) (uint32, bool) {
	index := sort.Search(len(v.referenceRows), func(index int) bool {
		return v.references[v.referenceRows[index]] >= reference
	})
	if index >= len(v.referenceRows) {
		return 0, false
	}
	row := v.referenceRows[index]
	return row, v.references[row] == reference
}

// buildVolumeColumn streams MFT records directly into compact parallel columns.
func buildVolumeColumn(ctx context.Context, root string, onProgress func(entryCount, fileCount int64)) (*volumeColumn, error) {
	handle, err := openVolume(root)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)

	journal, err := queryUSNJournal(handle, root)
	if err != nil {
		return nil, err
	}
	builder := newVolumeBuilder(root)
	query := mftEnumData{HighUSN: math.MaxInt64}
	buffer := make([]byte, mftReadBufferSize)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var returned uint32
		err := windows.DeviceIoControl(handle, fsctlEnumUSNData, (*byte)(unsafe.Pointer(&query)), uint32(unsafe.Sizeof(query)), &buffer[0], uint32(len(buffer)), &returned, nil)
		if errors.Is(err, windows.ERROR_HANDLE_EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("enumerate NTFS MFT for %s: %w", root, err)
		}
		next, nodes, err := parseUSNRecords(buffer[:returned])
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			builder.add(node)
		}
		if onProgress != nil {
			onProgress(int64(len(builder.references)), int64(len(builder.references))-builder.directoryCount)
		}
		if next <= query.StartFileReferenceNumber {
			break
		}
		query.StartFileReferenceNumber = next
	}
	return builder.finish(journal)
}

func newVolumeBuilder(root string) *volumeBuilder {
	return &volumeBuilder{
		root:             filepath.Clean(root),
		references:       []uint64{0},
		parentReferences: []uint64{0},
		nameIDs:          []uint32{0},
		rowFlags:         []byte{columnFlagDirectory},
		referenceRows:    make(map[uint64]uint32, 64*1024),
		nameLookup:       map[string]uint32{"": 0},
		names:            []string{""},
		directoryCount:   1,
	}
}

// add records one MFT node without retaining a full path or per-row name string.
func (b *volumeBuilder) add(node mftNode) {
	if node.Reference&0x0000FFFFFFFFFFFF == ntfsRootRecord {
		b.setRootReference(node.Reference)
		return
	}
	if node.ParentReference&0x0000FFFFFFFFFFFF == ntfsRootRecord {
		b.setRootReference(node.ParentReference)
	}
	if _, exists := b.referenceRows[node.Reference]; exists {
		return
	}
	nameID, ok := b.nameLookup[node.Name]
	if !ok {
		nameID = uint32(len(b.names))
		b.nameLookup[node.Name] = nameID
		b.names = append(b.names, node.Name)
	}
	row := uint32(len(b.references))
	b.referenceRows[node.Reference] = row
	b.references = append(b.references, node.Reference)
	b.parentReferences = append(b.parentReferences, node.ParentReference)
	b.nameIDs = append(b.nameIDs, nameID)
	flag := byte(0)
	if node.IsDir {
		flag = columnFlagDirectory
		b.directoryCount++
	}
	b.rowFlags = append(b.rowFlags, flag)
}

func (b *volumeBuilder) setRootReference(reference uint64) {
	if b.rootReference == 0 {
		b.rootReference = reference
		b.references[0] = reference
	}
	b.referenceRows[reference] = 0
}

// finish resolves parent rows and builds the name-to-row fan-out arrays.
func (b *volumeBuilder) finish(journal usnJournalData) (*volumeColumn, error) {
	if b.rootReference == 0 {
		return nil, errors.New("NTFS root record is missing")
	}
	parentRows := make([]uint32, len(b.references))
	for row := 1; row < len(parentRows); row++ {
		parent, ok := b.referenceRows[b.parentReferences[row]]
		if !ok {
			parent = invalidColumnRow
		}
		parentRows[row] = parent
	}

	nameOffsets := make([]uint32, len(b.names)+1)
	nameMasks := make([]uint64, len(b.names))
	nameFlags := make([]byte, len(b.names))
	nameSize := 0
	for nameID, name := range b.names {
		nameOffsets[nameID] = uint32(nameSize)
		nameSize += len(name)
		nameMasks[nameID] = asciiCharacterMask(name)
		if !utf8.ValidString(name) || len(name) != utf8.RuneCountInString(name) {
			for index := 0; index < len(name); index++ {
				if name[index] >= utf8.RuneSelf {
					nameFlags[nameID] |= columnNameFlagNonASCII
					break
				}
			}
		}
	}
	nameOffsets[len(b.names)] = uint32(nameSize)
	nameBytes := make([]byte, 0, nameSize)
	for _, name := range b.names {
		nameBytes = append(nameBytes, name...)
	}

	nameRowStarts := make([]uint32, len(b.names)+1)
	for row := 1; row < len(b.nameIDs); row++ {
		nameRowStarts[b.nameIDs[row]+1]++
	}
	for index := 1; index < len(nameRowStarts); index++ {
		nameRowStarts[index] += nameRowStarts[index-1]
	}
	nameRows := make([]uint32, len(b.references)-1)
	cursors := append([]uint32(nil), nameRowStarts[:len(b.names)]...)
	for row := 1; row < len(b.nameIDs); row++ {
		nameID := b.nameIDs[row]
		nameRows[cursors[nameID]] = uint32(row)
		cursors[nameID]++
	}

	referenceRows := make([]uint32, len(b.references))
	for row := range referenceRows {
		referenceRows[row] = uint32(row)
	}
	sort.Slice(referenceRows, func(left, right int) bool {
		return b.references[referenceRows[left]] < b.references[referenceRows[right]]
	})

	return &volumeColumn{
		root:          b.root,
		references:    b.references,
		parentRows:    parentRows,
		nameIDs:       b.nameIDs,
		rowFlags:      b.rowFlags,
		nameOffsets:   nameOffsets,
		nameBytes:     nameBytes,
		nameMasks:     nameMasks,
		nameFlags:     nameFlags,
		nameRowStarts: nameRowStarts,
		nameRows:      nameRows,
		referenceRows: referenceRows,
		journalID:     journal.JournalID,
		nextUSN:       journal.NextUSN,
		delta:         make(map[uint64]deltaNode),
		fileCount:     int64(len(b.references)) - b.directoryCount,
	}, nil
}

func asciiCharacterMask(value string) uint64 {
	var mask uint64
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char >= 'A' && char <= 'Z' {
			char += 'a' - 'A'
		}
		switch {
		case char >= 'a' && char <= 'z':
			mask |= uint64(1) << (char - 'a')
		case char >= '0' && char <= '9':
			mask |= uint64(1) << (26 + char - '0')
		}
	}
	return mask
}

// fixedNTFSVolumeRoots returns the local fixed NTFS volumes owned by service mode.
func fixedNTFSVolumeRoots() ([]string, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, 4)
	for drive := 0; drive < 26; drive++ {
		if mask&(uint32(1)<<drive) == 0 {
			continue
		}
		root := fmt.Sprintf("%c:\\", 'A'+drive)
		rootPtr, err := windows.UTF16PtrFromString(root)
		if err != nil || windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		fileSystem := make([]uint16, 32)
		if err := windows.GetVolumeInformation(rootPtr, nil, 0, nil, nil, nil, &fileSystem[0], uint32(len(fileSystem))); err != nil {
			continue
		}
		if strings.EqualFold(windows.UTF16ToString(fileSystem), "NTFS") {
			roots = append(roots, root)
		}
	}
	return roots, nil
}

// openVolume opens the raw NTFS volume behind a drive root.
func openVolume(root string) (windows.Handle, error) {
	volume := strings.TrimRight(filepath.VolumeName(root), `\`)
	path, err := windows.UTF16PtrFromString(`\\.\` + volume)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, fmt.Errorf("open NTFS volume %s: %w", volume, err)
	}
	return handle, nil
}

// queryUSNJournal captures the replay boundary used after the MFT scan.
func queryUSNJournal(handle windows.Handle, root string) (usnJournalData, error) {
	var journal usnJournalData
	var returned uint32
	if err := windows.DeviceIoControl(handle, fsctlQueryUSNJournal, nil, 0, (*byte)(unsafe.Pointer(&journal)), uint32(unsafe.Sizeof(journal)), &returned, nil); err != nil {
		return usnJournalData{}, fmt.Errorf("query USN journal for %s: %w", root, err)
	}
	return journal, nil
}

// monitorVolume keeps one volume's bounded delta current from the USN journal.
func (i *ColumnIndex) monitorVolume(ctx context.Context, volume *volumeColumn) {
	handle, err := openVolume(volume.root)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("file index service monitor unavailable for %s: %v", volume.root, err))
		return
	}
	defer windows.CloseHandle(handle)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	buffer := make([]byte, usnReadBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := i.readVolumeChanges(ctx, handle, volume, buffer); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("file index service USN catch-up failed for %s: %v", volume.root, err))
				i.RebuildAsync()
				return
			}
		}
	}
}

// readVolumeChanges catches up to the current journal tail and updates the delta.
func (i *ColumnIndex) readVolumeChanges(ctx context.Context, handle windows.Handle, volume *volumeColumn, buffer []byte) error {
	for {
		input := readUSNJournalData{StartUSN: volume.nextUSN, ReasonMask: math.MaxUint32, JournalID: volume.journalID, MinMajorVersion: 2, MaxMajorVersion: 2}
		var returned uint32
		if err := windows.DeviceIoControl(handle, fsctlReadUSNJournal, (*byte)(unsafe.Pointer(&input)), uint32(unsafe.Sizeof(input)), &buffer[0], uint32(len(buffer)), &returned, nil); err != nil {
			return err
		}
		nextUSN, nodes, err := parseUSNRecords(buffer[:returned])
		if err != nil {
			return err
		}
		volume.nextUSN = int64(nextUSN)
		if len(nodes) == 0 {
			return nil
		}
		volume.deltaMu.Lock()
		for _, node := range nodes {
			switch {
			case node.Reason&usnReasonFileDelete != 0 || node.Reason&usnReasonRenameOldName != 0:
				volume.applyDelta(node.Reference, deltaNode{deleted: true})
			case node.Reason&(usnReasonFileCreate|usnReasonRenameNewName) != 0:
				volume.applyDelta(node.Reference, deltaNode{parentReference: node.ParentReference, name: node.Name, isDir: node.IsDir})
			}
		}
		deltaCount := len(volume.delta)
		volume.deltaMu.Unlock()
		threshold := len(volume.references) / 100
		if threshold < 20_000 {
			threshold = 20_000
		}
		if deltaCount >= threshold {
			i.RebuildAsync()
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
}

// applyDelta updates the live entry counters while deltaMu is held.
func (v *volumeColumn) applyDelta(reference uint64, next deltaNode) {
	previousExists, previousIsDir := v.effectiveDeltaNode(reference)
	nextExists := !next.deleted
	if previousExists {
		if previousIsDir {
			v.deltaDirs--
		} else {
			v.deltaFiles--
		}
	}
	if nextExists {
		if next.isDir {
			v.deltaDirs++
		} else {
			v.deltaFiles++
		}
	}
	v.delta[reference] = next
}

func (v *volumeColumn) effectiveDeltaNode(reference uint64) (bool, bool) {
	if node, ok := v.delta[reference]; ok {
		return !node.deleted, node.isDir
	}
	row, ok := v.rowForReference(reference)
	return ok, ok && v.rowFlags[row]&columnFlagDirectory != 0
}
