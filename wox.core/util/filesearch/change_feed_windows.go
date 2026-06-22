//go:build windows

package filesearch

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	usnPollInterval              = time.Second
	usnReadBufferSize            = 1 << 20
	fsctlQueryUSNJournal  uint32 = 0x000900f4
	fsctlReadUSNJournal   uint32 = 0x000900bb
	usnReasonMaskAll      uint32 = 0xffffffff
	usnFileIDType         uint32 = 0
	usnExtendedFileIDType uint32 = 2
)

var procOpenFileByID = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenFileById")

type WindowsChangeFeed struct {
	fallback  *FallbackChangeFeed
	usn       *usnWatcherSet
	signals   chan ChangeSignal
	done      chan struct{}
	closeOnce sync.Once
}

type usnVolumeConfig struct {
	journal     usnJournalState
	roots       []RootRecord
	rootMatcher rootPathMatcher
	startUSN    int64
}

type usnVolumeWatcher struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type usnWatcherSet struct {
	mu       sync.Mutex
	emit     func(ChangeSignal)
	watchers []*usnVolumeWatcher
	closed   bool
}

type usnJournalDataV0 struct {
	UsnJournalID    uint64
	FirstUSN        int64
	NextUSN         int64
	LowestValidUSN  int64
	MaxUSN          int64
	MaximumSize     uint64
	AllocationDelta uint64
}

type readUSNJournalDataV1 struct {
	StartUSN          int64
	ReasonMask        uint32
	ReturnOnlyOnClose uint32
	Timeout           uint64
	BytesToWaitFor    uint64
	UsnJournalID      uint64
	MinMajorVersion   uint16
	MaxMajorVersion   uint16
}

type usnFileIDDescriptor struct {
	Size   uint32
	Type   uint32
	FileID [16]byte
}

type usnFileID struct {
	extended bool
	value    [16]byte
}

type usnRawRecord struct {
	fileID       usnFileID
	parentFileID usnFileID
	usn          int64
	reason       uint32
	name         string
	fileAttrs    uint32
}

type usnResolvedRecord struct {
	Path      string
	PathKnown bool
	PathIsDir bool
	USN       int64
	Reason    uint32
}

func NewWindowsChangeFeed() *WindowsChangeFeed {
	feed := &WindowsChangeFeed{
		fallback: NewFallbackChangeFeed(),
		signals:  make(chan ChangeSignal, 256),
		done:     make(chan struct{}),
	}
	feed.usn = newUSNWatcherSet(feed.emit)

	go feed.forwardFallbackSignals()

	return feed
}

func (f *WindowsChangeFeed) Mode() string {
	return "usn+fallback"
}

func (f *WindowsChangeFeed) Signals() <-chan ChangeSignal {
	return f.signals
}

func (f *WindowsChangeFeed) Refresh(ctx context.Context, roots []RootRecord) error {
	now := time.Now()
	volumeConfigsByPath := map[string]*usnVolumeConfig{}
	fallbackRoots := make([]RootRecord, 0, len(roots))

	for _, root := range roots {
		journal, ok := resolveWindowsUSNJournal(root.Path)
		if !ok {
			fallbackRoots = append(fallbackRoots, root)
			continue
		}

		config := volumeConfigsByPath[journal.Volume]
		if config == nil {
			config = &usnVolumeConfig{
				journal: journal,
			}
			volumeConfigsByPath[journal.Volume] = config
		}
		config.roots = append(config.roots, root)
	}

	volumeConfigs := make([]usnVolumeConfig, 0, len(volumeConfigsByPath))
	for _, config := range volumeConfigsByPath {
		prepared := prepareUSNVolumeRefresh(config.roots, config.journal, now, defaultFeedCursorSafeWindow)
		config.roots = prepared.roots
		// Optimization: USN polling can resolve many paths per tick. Build the
		// matcher once per refreshed volume so resolved records avoid repeated
		// root scans with filepath.Rel in the hot emit loop.
		config.rootMatcher = newRootPathMatcher(prepared.roots)
		config.startUSN = prepared.startUSN
		for _, signal := range prepared.signals {
			f.emit(signal)
		}
		volumeConfigs = append(volumeConfigs, *config)
	}

	f.usn.Refresh(volumeConfigs)
	return f.fallback.Refresh(ctx, fallbackRoots)
}

func (f *WindowsChangeFeed) Close() error {
	var closeErr error
	f.closeOnce.Do(func() {
		close(f.done)
		if err := f.usn.Close(); err != nil {
			closeErr = err
		}
		if err := f.fallback.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func (f *WindowsChangeFeed) SnapshotRootFeed(ctx context.Context, root RootRecord) (RootFeedSnapshot, error) {
	_ = ctx
	return snapshotWindowsRootFeed(root)
}

func (f *WindowsChangeFeed) forwardFallbackSignals() {
	for {
		select {
		case <-f.done:
			return
		case signal := <-f.fallback.Signals():
			f.emit(signal)
		}
	}
}

func (f *WindowsChangeFeed) emit(signal ChangeSignal) {
	if signal.RootID == "" {
		return
	}
	if signal.At.IsZero() {
		signal.At = time.Now()
	}

	select {
	case f.signals <- signal:
	default:
	}
}

func newUSNWatcherSet(emit func(ChangeSignal)) *usnWatcherSet {
	return &usnWatcherSet{
		emit: emit,
	}
}

func (u *usnWatcherSet) Refresh(configs []usnVolumeConfig) {
	watchers := u.replaceWatchers(configs)
	for _, watcher := range watchers {
		watcher.cancel()
		<-watcher.done
	}
}

func (u *usnWatcherSet) Close() error {
	u.mu.Lock()
	if u.closed {
		u.mu.Unlock()
		return nil
	}
	u.closed = true
	watchers := u.watchers
	u.watchers = nil
	u.mu.Unlock()

	for _, watcher := range watchers {
		watcher.cancel()
		<-watcher.done
	}

	return nil
}

func (u *usnWatcherSet) replaceWatchers(configs []usnVolumeConfig) []*usnVolumeWatcher {
	u.mu.Lock()
	defer u.mu.Unlock()

	previous := u.watchers
	u.watchers = nil
	if u.closed {
		return previous
	}

	for _, config := range configs {
		watchCtx, cancel := context.WithCancel(context.Background())
		watcher := &usnVolumeWatcher{
			cancel: cancel,
			done:   make(chan struct{}),
		}
		u.watchers = append(u.watchers, watcher)

		go u.runVolumeLoop(watchCtx, config, watcher.done)
	}

	return previous
}

func (u *usnWatcherSet) runVolumeLoop(ctx context.Context, config usnVolumeConfig, done chan struct{}) {
	defer close(done)

	currentJournal := config.journal
	currentUSN := config.startUSN
	unavailable := false

	poll := func() {
		journal, ok := resolveWindowsUSNJournal(config.journal.Volume)
		if !ok {
			if !unavailable {
				u.emitUnavailable(config.roots, "usn journal unavailable")
			}
			unavailable = true
			return
		}

		if unavailable {
			for _, root := range config.roots {
				u.emit(newUSNRecoverySignal(root, "usn journal recovered", time.Now()))
			}
			currentJournal = journal
			currentUSN = journal.NextUSN
			unavailable = false
			return
		}

		if currentUSN > 0 && currentJournal.JournalID != 0 && journal.JournalID != currentJournal.JournalID {
			for _, root := range config.roots {
				u.emit(newUSNRecoverySignal(root, "usn journal id changed", time.Now()))
			}
			currentJournal = journal
			currentUSN = journal.NextUSN
			return
		}
		if currentUSN > 0 && journal.FirstUSN > 0 && currentUSN < journal.FirstUSN {
			for _, root := range config.roots {
				u.emit(newUSNRecoverySignal(root, "usn cursor fell behind journal retention", time.Now()))
			}
			currentJournal = journal
			currentUSN = journal.NextUSN
			return
		}

		nextUSN, records, err := readUSNJournalChanges(journal, currentUSN)
		if err != nil {
			if isUSNReconcileError(err) {
				for _, root := range config.roots {
					u.emit(newUSNRecoverySignal(root, err.Error(), time.Now()))
				}
				currentJournal = journal
				currentUSN = journal.NextUSN
				return
			}

			if !unavailable {
				u.emitUnavailable(config.roots, err.Error())
			}
			unavailable = true
			return
		}

		currentJournal = journal
		currentUSN = nextUSN
		u.emitResolvedRecords(config.roots, config.rootMatcher, journal, records)
	}

	poll()

	ticker := time.NewTicker(usnPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

func (u *usnWatcherSet) emitUnavailable(roots []RootRecord, reason string) {
	now := time.Now()
	for _, root := range roots {
		u.emit(ChangeSignal{
			Kind:          ChangeSignalKindFeedUnavailable,
			RootID:        root.ID,
			FeedType:      RootFeedTypeUSN,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			Reason:        reason,
			At:            now,
		})
	}
}

func (u *usnWatcherSet) emitResolvedRecords(roots []RootRecord, matcher rootPathMatcher, journal usnJournalState, records []usnResolvedRecord) {
	now := time.Now()
	for _, record := range records {
		if record.PathKnown {
			cleanPath := filepath.Clean(record.Path)
			if root, ok := matcher.findClean(cleanPath); ok {
				if shouldSkipSystemPathForRoot(root, cleanPath, record.PathIsDir) {
					continue
				}
				// Known USN paths must be routed to the longest matching root. The
				// previous broadcast woke both a dynamic root and its parent, which
				// defeated the ownership split and forced unnecessary parent rescans.
				u.emit(translateUSNDelta(root, journal, cleanPath, record.PathIsDir, record.PathKnown, record.USN, record.Reason, now))
			}
			continue
		}

		cursorText, err := encodeFeedCursor(FeedCursor{
			FeedType:  RootFeedTypeUSN,
			UpdatedAt: now.UnixMilli(),
			JournalID: journal.JournalID,
			USN:       record.USN,
			Volume:    journal.Volume,
		})
		if err != nil {
			cursorText = ""
		}
		for _, root := range roots {
			u.emit(ChangeSignal{
				Kind:          ChangeSignalKindDirtyRoot,
				RootID:        root.ID,
				FeedType:      RootFeedTypeUSN,
				Path:          root.Path,
				PathIsDir:     true,
				PathTypeKnown: true,
				Cursor:        cursorText,
				At:            now,
			})
		}
	}
}

func snapshotWindowsRootFeed(root RootRecord) (RootFeedSnapshot, error) {
	journal, ok := resolveWindowsUSNJournal(root.Path)
	if !ok {
		return RootFeedSnapshot{
			FeedType:   RootFeedTypeFallback,
			FeedCursor: "",
			FeedState:  RootFeedStateReady,
		}, nil
	}

	cursorText, err := encodeFeedCursor(FeedCursor{
		FeedType:  RootFeedTypeUSN,
		UpdatedAt: time.Now().UnixMilli(),
		JournalID: journal.JournalID,
		USN:       journal.NextUSN,
		Volume:    journal.Volume,
	})
	if err != nil {
		return RootFeedSnapshot{}, err
	}

	return RootFeedSnapshot{
		FeedType:   RootFeedTypeUSN,
		FeedCursor: cursorText,
		FeedState:  RootFeedStateReady,
	}, nil
}

func resolveWindowsUSNJournal(rootPath string) (usnJournalState, bool) {
	volumePath, err := windowsVolumePath(rootPath)
	if err != nil {
		return usnJournalState{}, false
	}

	journal, err := queryUSNJournal(volumePath)
	if err != nil {
		return usnJournalState{}, false
	}

	return journal, true
}

func windowsVolumePath(rootPath string) (string, error) {
	if rootPath == "" {
		return "", fmt.Errorf("empty root path")
	}

	buffer := make([]uint16, 1024)
	if err := windows.GetVolumePathName(windows.StringToUTF16Ptr(filepath.Clean(rootPath)), &buffer[0], uint32(len(buffer))); err != nil {
		return "", err
	}

	return normalizeWindowsPath(windows.UTF16ToString(buffer)), nil
}

func queryUSNJournal(volumePath string) (usnJournalState, error) {
	handle, err := openUSNVolumeHandle(volumePath)
	if err != nil {
		return usnJournalState{}, err
	}
	defer windows.CloseHandle(handle)

	var data usnJournalDataV0
	var bytesReturned uint32
	if err := windows.DeviceIoControl(
		handle,
		fsctlQueryUSNJournal,
		nil,
		0,
		(*byte)(unsafe.Pointer(&data)),
		uint32(unsafe.Sizeof(data)),
		&bytesReturned,
		nil,
	); err != nil {
		return usnJournalState{}, err
	}

	return usnJournalState{
		Volume:    normalizeWindowsPath(volumePath),
		JournalID: data.UsnJournalID,
		FirstUSN:  data.FirstUSN,
		NextUSN:   data.NextUSN,
	}, nil
}

func readUSNJournalChanges(journal usnJournalState, startUSN int64) (int64, []usnResolvedRecord, error) {
	volumeHandle, err := openUSNVolumeHandle(journal.Volume)
	if err != nil {
		return startUSN, nil, err
	}
	defer windows.CloseHandle(volumeHandle)

	rootHandle, err := openWindowsRootHandle(journal.Volume)
	if err != nil {
		return startUSN, nil, err
	}
	defer windows.CloseHandle(rootHandle)

	input := readUSNJournalDataV1{
		StartUSN:        startUSN,
		ReasonMask:      usnReasonMaskAll,
		UsnJournalID:    journal.JournalID,
		MinMajorVersion: 2,
		MaxMajorVersion: 3,
	}

	buffer := make([]byte, usnReadBufferSize)
	var bytesReturned uint32
	if err := windows.DeviceIoControl(
		volumeHandle,
		fsctlReadUSNJournal,
		(*byte)(unsafe.Pointer(&input)),
		uint32(unsafe.Sizeof(input)),
		&buffer[0],
		uint32(len(buffer)),
		&bytesReturned,
		nil,
	); err != nil {
		return startUSN, nil, err
	}
	if bytesReturned < 8 {
		return startUSN, nil, nil
	}

	nextUSN := int64(binary.LittleEndian.Uint64(buffer[:8]))
	records, err := parseUSNResolvedRecords(rootHandle, journal.Volume, buffer[8:bytesReturned])
	if err != nil {
		return startUSN, nil, err
	}

	return nextUSN, records, nil
}

func parseUSNResolvedRecords(rootHandle windows.Handle, volumePath string, buffer []byte) ([]usnResolvedRecord, error) {
	records := make([]usnResolvedRecord, 0, 32)
	for offset := 0; offset+8 <= len(buffer); {
		recordLength := int(binary.LittleEndian.Uint32(buffer[offset : offset+4]))
		if recordLength <= 0 || offset+recordLength > len(buffer) {
			return nil, fmt.Errorf("invalid usn record length")
		}

		rawRecord, err := parseUSNRawRecord(buffer[offset : offset+recordLength])
		if err != nil {
			return nil, err
		}
		records = append(records, resolveUSNRecord(rootHandle, volumePath, rawRecord))

		offset += recordLength
	}

	return records, nil
}

func parseUSNRawRecord(buffer []byte) (usnRawRecord, error) {
	if len(buffer) < 8 {
		return usnRawRecord{}, fmt.Errorf("short usn record")
	}

	switch binary.LittleEndian.Uint16(buffer[4:6]) {
	case 2:
		if len(buffer) < 60 {
			return usnRawRecord{}, fmt.Errorf("short usn v2 record")
		}

		fileNameLength := int(binary.LittleEndian.Uint16(buffer[56:58]))
		fileNameOffset := int(binary.LittleEndian.Uint16(buffer[58:60]))
		if fileNameOffset < 0 || fileNameLength < 0 || fileNameOffset+fileNameLength > len(buffer) {
			return usnRawRecord{}, fmt.Errorf("invalid usn v2 file name bounds")
		}

		fileID := usnFileID{}
		parentFileID := usnFileID{}
		copy(fileID.value[:8], buffer[8:16])
		copy(parentFileID.value[:8], buffer[16:24])

		return usnRawRecord{
			fileID:       fileID,
			parentFileID: parentFileID,
			usn:          int64(binary.LittleEndian.Uint64(buffer[24:32])),
			reason:       binary.LittleEndian.Uint32(buffer[40:44]),
			name:         decodeUSNFileName(buffer[fileNameOffset : fileNameOffset+fileNameLength]),
			fileAttrs:    binary.LittleEndian.Uint32(buffer[52:56]),
		}, nil
	case 3:
		if len(buffer) < 76 {
			return usnRawRecord{}, fmt.Errorf("short usn v3 record")
		}

		fileNameLength := int(binary.LittleEndian.Uint16(buffer[72:74]))
		fileNameOffset := int(binary.LittleEndian.Uint16(buffer[74:76]))
		if fileNameOffset < 0 || fileNameLength < 0 || fileNameOffset+fileNameLength > len(buffer) {
			return usnRawRecord{}, fmt.Errorf("invalid usn v3 file name bounds")
		}

		fileID := usnFileID{extended: true}
		parentFileID := usnFileID{extended: true}
		copy(fileID.value[:], buffer[8:24])
		copy(parentFileID.value[:], buffer[24:40])

		return usnRawRecord{
			fileID:       fileID,
			parentFileID: parentFileID,
			usn:          int64(binary.LittleEndian.Uint64(buffer[40:48])),
			reason:       binary.LittleEndian.Uint32(buffer[56:60]),
			name:         decodeUSNFileName(buffer[fileNameOffset : fileNameOffset+fileNameLength]),
			fileAttrs:    binary.LittleEndian.Uint32(buffer[68:72]),
		}, nil
	default:
		return usnRawRecord{}, fmt.Errorf("unsupported usn record version")
	}
}

func resolveUSNRecord(rootHandle windows.Handle, volumePath string, rawRecord usnRawRecord) usnResolvedRecord {
	record := usnResolvedRecord{
		PathIsDir: rawRecord.fileAttrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0,
		USN:       rawRecord.usn,
		Reason:    rawRecord.reason,
	}

	path, err := openUSNPathByID(rootHandle, rawRecord.fileID, record.PathIsDir)
	if err == nil && path != "" {
		record.Path = normalizeWindowsPath(path)
		record.PathKnown = true
		return record
	}

	parentPath, parentErr := openUSNPathByID(rootHandle, rawRecord.parentFileID, true)
	if parentErr == nil && parentPath != "" {
		record.Path = normalizeWindowsPath(filepath.Join(parentPath, rawRecord.name))
		record.PathKnown = true
		return record
	}

	record.Path = normalizeWindowsPath(volumePath)
	record.PathKnown = false
	return record
}

func openUSNPathByID(rootHandle windows.Handle, fileID usnFileID, isDir bool) (string, error) {
	handle, err := openFileByID(rootHandle, fileID, isDir)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	return finalPathByHandle(handle)
}

func openFileByID(rootHandle windows.Handle, fileID usnFileID, isDir bool) (windows.Handle, error) {
	descriptor := usnFileIDDescriptor{
		Size: uint32(unsafe.Sizeof(usnFileIDDescriptor{})),
		Type: usnFileIDType,
	}
	if fileID.extended {
		descriptor.Type = usnExtendedFileIDType
	}
	descriptor.FileID = fileID.value

	flags := uint32(0)
	if isDir {
		flags = windows.FILE_FLAG_BACKUP_SEMANTICS
	}

	r0, _, e1 := procOpenFileByID.Call(
		uintptr(rootHandle),
		uintptr(unsafe.Pointer(&descriptor)),
		0,
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		0,
		uintptr(flags),
	)
	handle := windows.Handle(r0)
	if handle == windows.InvalidHandle {
		if e1 != nil && e1 != windows.ERROR_SUCCESS {
			return 0, error(e1)
		}
		return 0, windows.ERROR_INVALID_HANDLE
	}
	return handle, nil
}

func finalPathByHandle(handle windows.Handle) (string, error) {
	buffer := make([]uint16, 1024)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
		if err != nil {
			return "", err
		}
		if int(n) < len(buffer) {
			return normalizeWindowsPath(windows.UTF16ToString(buffer[:n])), nil
		}
		buffer = make([]uint16, n+1)
	}
}

func openUSNVolumeHandle(volumePath string) (windows.Handle, error) {
	devicePath := `\\.\` + strings.TrimRight(normalizeWindowsPath(volumePath), `\`)
	return windows.CreateFile(
		windows.StringToUTF16Ptr(devicePath),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
}

func openWindowsRootHandle(volumePath string) (windows.Handle, error) {
	return windows.CreateFile(
		windows.StringToUTF16Ptr(normalizeWindowsPath(volumePath)),
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
}

func decodeUSNFileName(buffer []byte) string {
	if len(buffer)%2 != 0 {
		return ""
	}
	words := make([]uint16, 0, len(buffer)/2)
	for index := 0; index < len(buffer); index += 2 {
		words = append(words, binary.LittleEndian.Uint16(buffer[index:index+2]))
	}
	return windows.UTF16ToString(words)
}

func normalizeWindowsPath(path string) string {
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else if strings.HasPrefix(path, `\\?\`) {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	return filepath.Clean(path)
}

func isUSNReconcileError(err error) bool {
	return errors.Is(err, windows.ERROR_JOURNAL_ENTRY_DELETED) ||
		errors.Is(err, windows.ERROR_JOURNAL_DELETE_IN_PROGRESS) ||
		strings.Contains(strings.ToLower(err.Error()), "unsupported usn record version")
}
