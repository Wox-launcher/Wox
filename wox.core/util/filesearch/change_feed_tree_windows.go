package filesearch

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// windowsTreeChangeFeed uses ReadDirectoryChangesW's subtree flag: a root owns
// one 64 KiB buffer regardless of its descendant count. Kernel overflow still
// reports history loss, so consumers repair gaps instead of silently missing files.
type windowsTreeChangeFeed struct {
	mu      sync.Mutex
	watches map[string]*windowsTreeWatch
	queue   *changeSignalQueue
	closed  bool
}

type windowsTreeWatch struct {
	mu     sync.Mutex
	root   RootRecord
	handle windows.Handle
	cancel context.CancelFunc
	done   chan struct{}
}

func newWindowsTreeChangeFeed() *windowsTreeChangeFeed {
	return &windowsTreeChangeFeed{watches: map[string]*windowsTreeWatch{}, queue: newChangeSignalQueue(256)}
}

func (f *windowsTreeChangeFeed) Mode() string                 { return "windows-subtree" }
func (f *windowsTreeChangeFeed) Signals() <-chan ChangeSignal { return f.queue.Signals() }

// Refresh joins removed watches before reusing their paths or releasing memory.
func (f *windowsTreeChangeFeed) Refresh(_ context.Context, roots []RootRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	wanted := map[string]RootRecord{}
	for _, root := range roots {
		wanted[root.ID] = root
	}
	for id, watch := range f.watches {
		root, exists := wanted[id]
		if !exists || root.Path != watch.root.Path {
			watch.stop()
			delete(f.watches, id)
		}
	}
	for id, root := range wanted {
		if f.watches[id] != nil {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		watch := &windowsTreeWatch{root: root, handle: windows.InvalidHandle, cancel: cancel, done: make(chan struct{})}
		f.watches[id] = watch
		go f.run(ctx, watch)
	}
	return nil
}

// stop cancels an outstanding overlapped read without racing a subsequent read.
func (w *windowsTreeWatch) stop() {
	w.mu.Lock()
	w.cancel()
	if w.handle != windows.InvalidHandle {
		_ = windows.CancelIoEx(w.handle, nil)
	}
	w.mu.Unlock()
	<-w.done
}

func (f *windowsTreeChangeFeed) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	for _, watch := range f.watches {
		watch.stop()
	}
	f.watches = nil
	f.queue.close()
	return nil
}

// run retries missing or temporarily inaccessible roots, including root moves.
func (f *windowsTreeChangeFeed) run(ctx context.Context, watch *windowsTreeWatch) {
	defer close(watch.done)
	for ctx.Err() == nil {
		err := f.readTree(ctx, watch)
		if ctx.Err() != nil {
			return
		}
		f.report(watch.root, ChangeSignalKindFeedUnavailable, fmt.Sprint(err))
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// readTree keeps the event and buffer alive until every overlapped read completes.
func (f *windowsTreeChangeFeed) readTree(ctx context.Context, watch *windowsTreeWatch) error {
	path, err := windows.UTF16PtrFromString(watch.root.Path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event)
	watch.mu.Lock()
	watch.handle = handle
	watch.mu.Unlock()
	defer func() { watch.mu.Lock(); watch.handle = windows.InvalidHandle; watch.mu.Unlock() }()
	originalPath, err := finalPathByHandle(handle)
	if err != nil {
		return err
	}
	buffer := make([]byte, 64*1024)
	registered := false
	for {
		overlap := windows.Overlapped{HEvent: event}
		watch.mu.Lock()
		if ctx.Err() != nil {
			watch.mu.Unlock()
			return ctx.Err()
		}
		_ = windows.ResetEvent(event)
		err = windows.ReadDirectoryChanges(handle, &buffer[0], uint32(len(buffer)), true, windows.FILE_NOTIFY_CHANGE_FILE_NAME|windows.FILE_NOTIFY_CHANGE_DIR_NAME|windows.FILE_NOTIFY_CHANGE_LAST_WRITE|windows.FILE_NOTIFY_CHANGE_ATTRIBUTES, nil, &overlap, 0)
		watch.mu.Unlock()
		if err != nil && !errors.Is(err, windows.ERROR_IO_PENDING) {
			return err
		}
		if !registered {
			f.report(watch.root, ChangeSignalKindHistoryLost, "subtree watch registered")
			registered = true
		}
		var count uint32
		// The API watches descendants, not the root's own name. Check its
		// identity while idle so a moved/deleted root is reopened at its path.
		for {
			status, waitErr := windows.WaitForSingleObject(event, 5000)
			if waitErr != nil || status != uint32(windows.WAIT_TIMEOUT) {
				break
			}
			currentPath, pathErr := finalPathByHandle(handle)
			_, statErr := os.Stat(watch.root.Path)
			if pathErr != nil || statErr != nil || currentPath != originalPath {
				_ = windows.CancelIoEx(handle, &overlap)
				_ = windows.GetOverlappedResult(handle, &overlap, &count, true)
				return errors.New("watched root moved or disappeared")
			}
		}
		err = windows.GetOverlappedResult(handle, &overlap, &count, true)
		if errors.Is(err, windows.ERROR_NOTIFY_ENUM_DIR) || err == nil && count == 0 {
			f.report(watch.root, ChangeSignalKindHistoryLost, "subtree notification overflow")
			continue
		}
		if err != nil {
			return err
		}
		if err := f.decode(watch.root, buffer[:count]); err != nil {
			f.report(watch.root, ChangeSignalKindHistoryLost, err.Error())
		}
	}
}

// decode validates the variable-length Windows records before accessing names.
func (f *windowsTreeChangeFeed) decode(root RootRecord, data []byte) error {
	for {
		if len(data) < 12 {
			return errors.New("truncated directory notification")
		}
		next, action, size := binary.LittleEndian.Uint32(data), binary.LittleEndian.Uint32(data[4:]), binary.LittleEndian.Uint32(data[8:])
		if size%2 != 0 || uint64(size)+12 > uint64(len(data)) {
			return errors.New("invalid directory notification name")
		}
		name := make([]uint16, int(size)/2)
		for i := range name {
			name[i] = binary.LittleEndian.Uint16(data[12+i*2:])
		}
		path := filepath.Join(root.Path, windows.UTF16ToString(name))
		if !pathWithinScope(root.Path, path) {
			return errors.New("directory notification outside root")
		}
		semantic := ChangeSemanticKindModify
		switch action {
		case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_RENAMED_NEW_NAME:
			semantic = ChangeSemanticKindCreate
		case windows.FILE_ACTION_REMOVED, windows.FILE_ACTION_RENAMED_OLD_NAME:
			semantic = ChangeSemanticKindRemove
		}
		isDir, known := statPathType(path)
		if !shouldSkipSystemPathForRoot(root, path, isDir) {
			f.queue.offer(ChangeSignal{Kind: ChangeSignalKindDirtyPath, SemanticKind: semantic, RootID: root.ID, Path: path, PathIsDir: isDir, PathTypeKnown: known, FeedType: RootFeedTypeFallback, At: time.Now()})
		}
		if next == 0 {
			return nil
		}
		if next < 12+size || int(next) >= len(data) {
			return errors.New("invalid directory notification offset")
		}
		data = data[next:]
	}
}

func (f *windowsTreeChangeFeed) report(root RootRecord, kind ChangeSignalKind, reason string) {
	f.queue.offer(ChangeSignal{Kind: kind, RootID: root.ID, Path: root.Path, PathIsDir: true, PathTypeKnown: true, FeedType: RootFeedTypeFallback, Reason: reason, At: time.Now()})
}
