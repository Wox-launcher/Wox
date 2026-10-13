//go:build windows

package filesearchservice

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"wox/util/filesearchservice/protocol"
)

// FactSink consumes borrowed payloads before returning. Backpressure propagates to
// the native reader instead of retaining a second, service-owned filename index.
type FactSink func(protocol.Kind, []byte) error

// EnumerateFacts emits MFT records with the journal boundary captured before scanning.
// End captures a catch-up target: replay from Begin through End before publication.
func EnumerateFacts(ctx context.Context, root string, emit FactSink) error {
	root, err := validateFactVolume(root)
	if err != nil {
		return err
	}
	handle, err := openFactVolume(root)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	journal, err := readFactJournal(ctx, handle)
	if err != nil {
		return err
	}
	rootID, err := factRootID(root)
	if err != nil {
		return err
	}
	boundary := protocol.Boundary{Volume: root, RootID: rootID, JournalID: journal.JournalID, NextUSN: journal.NextUSN}
	payload, err := protocol.EncodeBoundary(boundary)
	if err != nil {
		return err
	}
	if err := emit(protocol.Begin, payload); err != nil {
		return err
	}
	buffer := make([]byte, mftReadBufferSize)
	batch := make([]byte, 0, protocol.TargetBatchSize)
	query := mftEnumData{HighUSN: math.MaxInt64}
	for {
		returned, err := factControl(ctx, handle, fsctlEnumUSNData, unsafe.Pointer(&query), uint32(unsafe.Sizeof(query)), buffer)
		if errors.Is(err, windows.ERROR_HANDLE_EOF) {
			break
		}
		if err != nil {
			return err
		}
		next, nodes, err := parseUSNRecords(buffer[:returned])
		if err != nil {
			return err
		}
		for _, node := range nodes {
			if node.Reference == rootID {
				continue
			}
			batch, err = appendFact(batch, node, false, emit)
			if err != nil {
				return err
			}
		}
		if next <= query.StartFileReferenceNumber {
			return fmt.Errorf("MFT enumeration did not advance")
		}
		query.StartFileReferenceNumber = next
	}
	if len(batch) != 0 {
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
	}
	after, err := readFactJournal(ctx, handle)
	if err != nil {
		return err
	}
	if after.JournalID != boundary.JournalID || after.FirstUSN > boundary.NextUSN || after.LowestValidUSN > boundary.NextUSN {
		return emit(protocol.ResetRequired, []byte("USN history changed during enumeration"))
	}
	boundary.NextUSN = after.NextUSN
	payload, err = protocol.EncodeBoundary(boundary)
	if err != nil {
		return err
	}
	return emit(protocol.End, payload)
}

// ReadFacts replays one bounded read, optionally waiting for a new journal record.
// A checkpoint is emitted only after all record frames belonging to that read.
func ReadFacts(ctx context.Context, boundary protocol.Boundary, wait bool, emit FactSink) error {
	root, err := validateFactVolume(boundary.Volume)
	if err != nil {
		return err
	}
	handle, err := openFactVolume(root)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	journal, err := readFactJournal(ctx, handle)
	if err != nil {
		return err
	}
	if journal.JournalID != boundary.JournalID || boundary.NextUSN < journal.FirstUSN || boundary.NextUSN < journal.LowestValidUSN || boundary.NextUSN > journal.NextUSN {
		return emit(protocol.ResetRequired, []byte("USN history is no longer available"))
	}
	input := readUSNJournalData{StartUSN: boundary.NextUSN, ReasonMask: math.MaxUint32, JournalID: boundary.JournalID, MinMajorVersion: 2, MaxMajorVersion: 2}
	if wait {
		input.BytesToWaitFor = 1
	}
	buffer := make([]byte, usnReadBufferSize)
	returned, err := factControl(ctx, handle, fsctlReadUSNJournal, unsafe.Pointer(&input), uint32(unsafe.Sizeof(input)), buffer)
	if errors.Is(err, windows.ERROR_JOURNAL_DELETE_IN_PROGRESS) || errors.Is(err, windows.ERROR_JOURNAL_NOT_ACTIVE) || errors.Is(err, windows.ERROR_JOURNAL_ENTRY_DELETED) {
		return emit(protocol.ResetRequired, []byte("USN history changed during replay"))
	}
	if err != nil {
		return err
	}
	next, nodes, err := parseUSNRecords(buffer[:returned])
	if err != nil {
		return err
	}
	if int64(next) < boundary.NextUSN {
		return fmt.Errorf("USN cursor moved backwards")
	}
	batch := make([]byte, 0, protocol.TargetBatchSize)
	for _, node := range nodes {
		if node.Reference == boundary.RootID {
			continue
		}
		batch, err = appendFact(batch, node, true, emit)
		if err != nil {
			return err
		}
	}
	if len(batch) != 0 {
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
	}
	boundary.NextUSN = int64(next)
	payload, err := protocol.EncodeBoundary(boundary)
	if err != nil {
		return err
	}
	return emit(protocol.Checkpoint, payload)
}

// appendFact batches native records without resolving paths or retaining volume-sized state.
func appendFact(batch []byte, node mftNode, live bool, emit FactSink) ([]byte, error) {
	if len(batch) > 0 && len(batch)+len(node.Name)+56 > protocol.TargetBatchSize {
		if err := emit(protocol.Records, batch); err != nil {
			return batch, err
		}
		batch = batch[:0]
	}
	entry := protocol.Entry{ID: node.Reference, ParentID: node.ParentReference, USN: node.USN, Reason: node.Reason, Name: node.Name}
	if node.IsDir {
		entry.Flags |= protocol.Directory
	}
	if live && node.Reason&(usnReasonFileDelete|usnReasonRenameOldName) != 0 {
		entry.Flags |= protocol.Deleted
	}
	return protocol.AppendEntry(batch, entry)
}

// validateFactVolume excludes paths, UNC devices, and arbitrary privileged file access.
func validateFactVolume(root string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	volume := filepath.VolumeName(root)
	if len(volume) != 2 || volume[1] != ':' || !((volume[0] >= 'a' && volume[0] <= 'z') || (volume[0] >= 'A' && volume[0] <= 'Z')) || root != volume+`\` {
		return "", fmt.Errorf("file fact requests require a drive root")
	}
	return strings.ToUpper(volume) + `\`, nil
}

func openFactVolume(root string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(`\\.\` + filepath.VolumeName(root))
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
}

// factRootID uses the actual root reference, including NTFS's sequence bits.
func factRootID(root string) (uint64, error) {
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return 0, err
	}
	return uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow), nil
}

func readFactJournal(ctx context.Context, handle windows.Handle) (usnJournalData, error) {
	var journal usnJournalData
	buffer := unsafe.Slice((*byte)(unsafe.Pointer(&journal)), int(unsafe.Sizeof(journal)))
	returned, err := factControl(ctx, handle, fsctlQueryUSNJournal, nil, 0, buffer)
	if err == nil && returned < uint32(len(buffer)) {
		err = fmt.Errorf("truncated USN journal information")
	}
	return journal, err
}

// factControl uses cancellable overlapped I/O so an idle subscription can wait in
// the kernel without polling, and shutdown never leaves a native volume read blocked.
func factControl(ctx context.Context, handle windows.Handle, code uint32, input unsafe.Pointer, inputSize uint32, output []byte) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(event)
	overlapped := windows.Overlapped{HEvent: event}
	var returned uint32
	err = windows.DeviceIoControl(handle, code, (*byte)(input), inputSize, &output[0], uint32(len(output)), &returned, &overlapped)
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		return returned, err
	}
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = windows.CancelIoEx(handle, &overlapped); close(cancelled) })
	err = windows.GetOverlappedResult(handle, &overlapped, &returned, true)
	if !stop() {
		<-cancelled
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	return returned, err
}
