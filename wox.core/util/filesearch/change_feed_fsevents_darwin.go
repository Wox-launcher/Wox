//go:build darwin

package filesearch

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <CoreServices/CoreServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include <stdint.h>

extern void woxFSEventsCallback(uintptr_t handle, size_t numEvents, char **eventPaths, FSEventStreamEventFlags *eventFlags, FSEventStreamEventId *eventIds);
void woxFSEventsBridge(ConstFSEventStreamRef streamRef, void *clientCallBackInfo, size_t numEvents, void *eventPaths, const FSEventStreamEventFlags eventFlags[], const FSEventStreamEventId eventIds[]);
*/
import "C"

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime/cgo"
	"strings"
	"sync"
	"time"
	"unsafe"
	"wox/util"
)

const (
	fseventsLatency = time.Second
	// FSEvents can replay thousands of changes after startup, wake, or a large
	// directory edit. Keep the callback non-blocking, but give normal bursts
	// enough room so user-visible file edits are not dropped under load.
	fseventsSignalBufferSize         = 8192
	fseventsDroppedSignalLogInterval = 5 * time.Second
	fseventsSummaryLogInterval       = 5 * time.Second
)

type FSEventsChangeFeed struct {
	mu               sync.RWMutex
	stream           C.FSEventStreamRef
	queue            C.dispatch_queue_t
	streamGeneration uint64
	roots            []RootRecord
	rootMatcher      rootPathMatcher
	signals          chan ChangeSignal
	handle           cgo.Handle
	handlePtr        *C.uintptr_t
	closed           bool

	callbackSummaryMu             sync.Mutex
	callbackEventCount            int
	callbackMatchedRootCount      int
	callbackUnmatchedEventCount   int
	callbackSignalCount           int
	callbackLastEventID           uint64
	lastCallbackSummaryLog        time.Time
	signalSummaryMu               sync.Mutex
	emittedSignalCount            int
	droppedSignalCount            int
	lastSignalSummaryLog          time.Time
	channelFullDroppedSignalMu    sync.Mutex
	channelFullDroppedSignalCount int
	lastDroppedSignalLog          time.Time
}

func NewFSEventsChangeFeed() *FSEventsChangeFeed {
	feed := &FSEventsChangeFeed{
		signals: make(chan ChangeSignal, fseventsSignalBufferSize),
	}
	feed.handle = cgo.NewHandle(feed)
	feed.handlePtr = (*C.uintptr_t)(C.malloc(C.size_t(unsafe.Sizeof(C.uintptr_t(0)))))
	*feed.handlePtr = C.uintptr_t(feed.handle)
	return feed
}

func (f *FSEventsChangeFeed) Mode() string {
	return "fsevents"
}

func (f *FSEventsChangeFeed) Signals() <-chan ChangeSignal {
	return f.signals
}

func (f *FSEventsChangeFeed) Refresh(ctx context.Context, roots []RootRecord) error {
	_ = ctx
	prepared := prepareFSEventsRefresh(roots, time.Now(), defaultFeedCursorSafeWindow)
	for _, signal := range prepared.signals {
		f.emit(signal)
	}

	f.stopCurrentStream("refresh")

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.roots = append([]RootRecord(nil), prepared.watchRoots...)
	// Optimization: FSEvents batches can be very large, so root ownership must
	// not repeat filepath.Rel work for every event/root pair. Refresh builds the
	// immutable matcher once beside the root snapshot used for fallback scans.
	f.rootMatcher = newRootPathMatcher(prepared.watchRoots)
	f.mu.Unlock()

	if len(prepared.watchRoots) == 0 {
		return nil
	}

	stream, err := f.createStream(prepared.watchRoots, prepared.sinceEventID)
	if err != nil {
		for _, root := range prepared.watchRoots {
			f.emit(ChangeSignal{
				Kind:          ChangeSignalKindFeedUnavailable,
				RootID:        root.ID,
				FeedType:      RootFeedTypeFSEvents,
				Path:          root.Path,
				PathIsDir:     true,
				PathTypeKnown: true,
				Reason:        err.Error(),
				At:            time.Now(),
			})
		}
		return err
	}
	queue, err := f.createQueue()
	if err != nil {
		C.FSEventStreamRelease(stream)
		return err
	}

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		C.FSEventStreamRelease(stream)
		return nil
	}
	generation := f.streamGeneration + 1
	f.streamGeneration = generation
	f.stream = stream
	f.queue = queue
	f.mu.Unlock()

	C.FSEventStreamSetDispatchQueue(stream, queue)
	if C.FSEventStreamStart(stream) == 0 {
		for _, root := range prepared.watchRoots {
			f.emit(ChangeSignal{
				Kind:          ChangeSignalKindFeedUnavailable,
				RootID:        root.ID,
				FeedType:      RootFeedTypeFSEvents,
				Path:          root.Path,
				PathIsDir:     true,
				PathTypeKnown: true,
				Reason:        "start fsevents stream",
				At:            time.Now(),
			})
		}
		f.stopCurrentStream("start_failed")
		return fmt.Errorf("start fsevents stream")
	}

	// Bug fix: the active stream must outlive the Refresh caller context. Some
	// refreshes run from short indexing or dynamic-root tasks; tying the stream
	// to those contexts leaves roots marked ready while the native watcher has
	// already been stopped.
	f.logStreamStarted(generation, len(prepared.watchRoots), prepared.sinceEventID, prepared.watchRoots)

	return nil
}

func (f *FSEventsChangeFeed) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	f.closed = true
	f.mu.Unlock()

	f.stopCurrentStream("close")
	f.handle.Delete()
	if f.handlePtr != nil {
		C.free(unsafe.Pointer(f.handlePtr))
		f.handlePtr = nil
	}
	return nil
}

func (f *FSEventsChangeFeed) SnapshotRootFeed(ctx context.Context, root RootRecord) (RootFeedSnapshot, error) {
	_ = ctx
	cursorText, err := encodeFeedCursor(FeedCursor{
		FeedType:  RootFeedTypeFSEvents,
		UpdatedAt: time.Now().UnixMilli(),
		FSEventID: uint64(C.FSEventsGetCurrentEventId()),
	})
	if err != nil {
		return RootFeedSnapshot{}, err
	}

	return RootFeedSnapshot{
		FeedType:   RootFeedTypeFSEvents,
		FeedCursor: cursorText,
		FeedState:  RootFeedStateReady,
	}, nil
}

func (f *FSEventsChangeFeed) createQueue() (C.dispatch_queue_t, error) {
	label := C.CString("wox.filesearch.fsevents")
	defer C.free(unsafe.Pointer(label))

	queue := C.dispatch_queue_create(label, nil)
	if queue == nil {
		return nil, fmt.Errorf("create fsevents dispatch queue")
	}
	return queue, nil
}

func (f *FSEventsChangeFeed) createStream(roots []RootRecord, sinceEventID uint64) (C.FSEventStreamRef, error) {
	pathArray := C.CFArrayCreateMutable(C.CFAllocatorRef(0), 0, &C.kCFTypeArrayCallBacks)
	if pathArray == 0 {
		return nil, fmt.Errorf("create fsevents path array")
	}
	defer C.CFRelease(C.CFTypeRef(pathArray))

	for _, root := range roots {
		cPath := C.CString(root.Path)
		cfPath := C.CFStringCreateWithCString(C.CFAllocatorRef(0), cPath, C.kCFStringEncodingUTF8)
		C.free(unsafe.Pointer(cPath))
		if cfPath == 0 {
			return nil, fmt.Errorf("create fsevents path string for %q", root.Path)
		}
		C.CFArrayAppendValue(pathArray, unsafe.Pointer(cfPath))
		C.CFRelease(C.CFTypeRef(cfPath))
	}

	context := C.FSEventStreamContext{}
	context.info = unsafe.Pointer(f.handlePtr)
	flags := C.FSEventStreamCreateFlags(
		C.kFSEventStreamCreateFlagFileEvents |
			C.kFSEventStreamCreateFlagWatchRoot |
			C.kFSEventStreamCreateFlagNoDefer,
	)

	stream := C.FSEventStreamCreate(
		C.CFAllocatorRef(0),
		(C.FSEventStreamCallback)(C.woxFSEventsBridge),
		&context,
		C.CFArrayRef(pathArray),
		C.FSEventStreamEventId(sinceEventID),
		C.CFTimeInterval(fseventsLatency.Seconds()),
		flags,
	)
	if stream == nil {
		return nil, fmt.Errorf("create fsevents stream")
	}

	return stream, nil
}

func (f *FSEventsChangeFeed) stopCurrentStream(reason string) {
	f.mu.Lock()
	stream := f.stream
	generation := f.streamGeneration
	f.stream = nil
	f.queue = nil
	f.mu.Unlock()

	if stream != nil {
		C.FSEventStreamStop(stream)
		C.FSEventStreamInvalidate(stream)
		C.FSEventStreamRelease(stream)
		util.GetLogger().Info(context.Background(), fmt.Sprintf(
			"filesearch fsevents stream stopped: generation=%d reason=%s",
			generation,
			reason,
		))
	}
}

func (f *FSEventsChangeFeed) copyRootSnapshot() ([]RootRecord, rootPathMatcher) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return append([]RootRecord(nil), f.roots...), f.rootMatcher
}

func (f *FSEventsChangeFeed) emit(signal ChangeSignal) {
	if signal.RootID == "" {
		return
	}
	if signal.At.IsZero() {
		signal.At = time.Now()
	}

	f.mu.RLock()
	closed := f.closed
	f.mu.RUnlock()
	if closed {
		return
	}

	select {
	case f.signals <- signal:
		if fileSearchDiagnosticLoggingEnabled {
			f.logSignalSummary(signal, false)
		}
	default:
		if fileSearchDiagnosticLoggingEnabled {
			f.logSignalSummary(signal, true)
		}
		f.logDroppedSignal(signal)
	}
}

func (f *FSEventsChangeFeed) logDroppedSignal(signal ChangeSignal) {
	f.channelFullDroppedSignalMu.Lock()
	defer f.channelFullDroppedSignalMu.Unlock()

	f.channelFullDroppedSignalCount++
	now := time.Now()
	if !f.lastDroppedSignalLog.IsZero() && now.Sub(f.lastDroppedSignalLog) < fseventsDroppedSignalLogInterval {
		return
	}

	// Diagnostic logging: a full signal channel means the watcher has lost a
	// concrete file event. Aggregate the warning so a filesystem burst still
	// points at the failure boundary without flooding the log.
	dropped := f.channelFullDroppedSignalCount
	f.channelFullDroppedSignalCount = 0
	f.lastDroppedSignalLog = now
	util.GetLogger().Warn(context.Background(), fmt.Sprintf(
		"filesearch fsevents signal dropped: channel_full=true dropped_since_last=%d kind=%s semantic=%s root=%s path=%s",
		dropped,
		signal.Kind,
		signal.SemanticKind,
		signal.RootID,
		summarizeLogPath(signal.Path),
	))
}

func (f *FSEventsChangeFeed) logStreamStarted(generation uint64, rootCount int, sinceEventID uint64, roots []RootRecord) {
	util.GetLogger().Info(context.Background(), fmt.Sprintf(
		"filesearch fsevents stream started: generation=%d roots=%d since_event_id=%d root_paths=%s",
		generation,
		rootCount,
		sinceEventID,
		summarizeFSEventsRootPaths(roots),
	))
}

func (f *FSEventsChangeFeed) logCallbackSummary(events int, matchedRoots int, unmatchedEvents int, signals int, lastEventID uint64) {
	if !fileSearchDiagnosticLoggingEnabled || events <= 0 {
		return
	}

	f.callbackSummaryMu.Lock()
	defer f.callbackSummaryMu.Unlock()

	f.callbackEventCount += events
	f.callbackMatchedRootCount += matchedRoots
	f.callbackUnmatchedEventCount += unmatchedEvents
	f.callbackSignalCount += signals
	if lastEventID > f.callbackLastEventID {
		f.callbackLastEventID = lastEventID
	}

	now := time.Now()
	if !f.lastCallbackSummaryLog.IsZero() && now.Sub(f.lastCallbackSummaryLog) < fseventsSummaryLogInterval {
		return
	}

	// Diagnostic logging: the callback summary distinguishes "macOS never
	// called us" from later root matching, emit, and consumer bottlenecks without
	// logging every filesystem event.
	eventCount := f.callbackEventCount
	matchedRootCount := f.callbackMatchedRootCount
	unmatchedEventCount := f.callbackUnmatchedEventCount
	signalCount := f.callbackSignalCount
	callbackLastEventID := f.callbackLastEventID
	f.callbackEventCount = 0
	f.callbackMatchedRootCount = 0
	f.callbackUnmatchedEventCount = 0
	f.callbackSignalCount = 0
	f.callbackLastEventID = 0
	f.lastCallbackSummaryLog = now

	util.GetLogger().Info(context.Background(), fmt.Sprintf(
		"filesearch fsevents callback summary: events=%d matched_roots=%d unmatched_events=%d emitted_candidates=%d last_event_id=%d",
		eventCount,
		matchedRootCount,
		unmatchedEventCount,
		signalCount,
		callbackLastEventID,
	))
}

func (f *FSEventsChangeFeed) logSignalSummary(signal ChangeSignal, dropped bool) {
	if !fileSearchDiagnosticLoggingEnabled {
		return
	}

	f.signalSummaryMu.Lock()
	defer f.signalSummaryMu.Unlock()

	if dropped {
		f.droppedSignalCount++
	} else {
		f.emittedSignalCount++
	}

	now := time.Now()
	if !f.lastSignalSummaryLog.IsZero() && now.Sub(f.lastSignalSummaryLog) < fseventsSummaryLogInterval {
		return
	}

	// Diagnostic logging: emit summary confirms that callback output reached the
	// Go signal channel and shows whether the consumer is falling behind.
	emitted := f.emittedSignalCount
	droppedCount := f.droppedSignalCount
	f.emittedSignalCount = 0
	f.droppedSignalCount = 0
	f.lastSignalSummaryLog = now

	util.GetLogger().Info(context.Background(), fmt.Sprintf(
		"filesearch fsevents emit summary: emitted=%d dropped=%d channel_len=%d channel_cap=%d kind=%s semantic=%s root=%s path=%s",
		emitted,
		droppedCount,
		len(f.signals),
		cap(f.signals),
		signal.Kind,
		signal.SemanticKind,
		signal.RootID,
		summarizeLogPath(signal.Path),
	))
}

func (f *FSEventsChangeFeed) onEvents(paths []string, flags []uint64, ids []uint64) {
	roots, matcher := f.copyRootSnapshot()
	if len(roots) == 0 {
		return
	}

	diagnosticLoggingEnabled := fileSearchDiagnosticLoggingEnabled
	now := time.Now()
	matchedRootCount := 0
	unmatchedEventCount := 0
	signalCount := 0
	lastEventID := uint64(0)
	for index := range paths {
		eventPath := filepath.Clean(paths[index])
		if diagnosticLoggingEnabled && ids[index] > lastEventID {
			lastEventID = ids[index]
		}
		matchedRoots := make([]RootRecord, 0, len(roots))
		if root, ok := matcher.findClean(eventPath); ok {
			pathIsDir := flags[index]&fseventFlagItemIsDir != 0
			if shouldSkipSystemPathForRoot(root, eventPath, pathIsDir) {
				continue
			}
			// FSEvents delivers a concrete path for normal events. Route that path
			// only to the longest matching root so a promoted dynamic root does not
			// also wake its parent and lose the intended narrow reconcile boundary.
			matchedRoots = append(matchedRoots, root)
		}
		if len(matchedRoots) == 0 && fseventRequiresRootReconcile(flags[index]) {
			matchedRoots = roots
		}
		if diagnosticLoggingEnabled && len(matchedRoots) == 0 {
			unmatchedEventCount++
		}
		if diagnosticLoggingEnabled {
			matchedRootCount += len(matchedRoots)
		}

		for _, root := range matchedRoots {
			for _, signal := range translateFSEvent(root, eventPath, flags[index], ids[index], now) {
				if diagnosticLoggingEnabled {
					signalCount++
				}
				f.emit(signal)
			}
		}
	}
	if diagnosticLoggingEnabled {
		// Optimization: callback summaries are diagnostic-only and sit directly on
		// the FSEvents hot path. Keep all counters and formatting behind the same
		// dev switch used by File Search scan and SQLite diagnostics.
		f.logCallbackSummary(len(paths), matchedRootCount, unmatchedEventCount, signalCount, lastEventID)
	}
}

func summarizeFSEventsRootPaths(roots []RootRecord) string {
	if len(roots) == 0 {
		return "<none>"
	}
	const maxRootPathSamples = 4
	sampleCount := len(roots)
	if sampleCount > maxRootPathSamples {
		sampleCount = maxRootPathSamples
	}
	parts := make([]string, 0, sampleCount)
	for index, root := range roots {
		if index >= maxRootPathSamples {
			break
		}
		parts = append(parts, summarizeLogPath(root.Path))
	}
	if len(roots) > maxRootPathSamples {
		parts = append(parts, fmt.Sprintf("+%d more", len(roots)-maxRootPathSamples))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
}

//export woxFSEventsCallback
func woxFSEventsCallback(handle C.uintptr_t, numEvents C.size_t, eventPaths **C.char, eventFlags *C.FSEventStreamEventFlags, eventIds *C.FSEventStreamEventId) {
	feed, ok := cgo.Handle(handle).Value().(*FSEventsChangeFeed)
	if !ok || feed == nil {
		return
	}

	count := int(numEvents)
	pathSlice := unsafe.Slice(eventPaths, count)
	flagSlice := unsafe.Slice(eventFlags, count)
	idSlice := unsafe.Slice(eventIds, count)

	paths := make([]string, 0, count)
	flags := make([]uint64, 0, count)
	ids := make([]uint64, 0, count)
	for index := 0; index < count; index++ {
		paths = append(paths, C.GoString(pathSlice[index]))
		flags = append(flags, uint64(flagSlice[index]))
		ids = append(ids, uint64(idSlice[index]))
	}

	feed.onEvents(paths, flags, ids)
}
