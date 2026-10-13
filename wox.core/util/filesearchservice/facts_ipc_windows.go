//go:build windows

package filesearchservice

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"wox/util/filesearchservice/protocol"
)

const factsPipeName = `\\.\pipe\WoxFileIndexServiceFactsV3`

var ErrFactsReset = errors.New("file fact history requires a new snapshot")

type factRequestHandler func(context.Context, protocol.Kind, protocol.Boundary, FactSink) error

// ServeFacts keeps privileged data transfer separate from the management endpoint.
// Each connection owns one bounded replay or one enumeration, never an index.
func ServeFacts(ctx context.Context, ownerSID string) error {
	return serveFactsPipe(ctx, factsPipeName, ownerSID, func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit FactSink) error {
		root, err := validateFactVolume(boundary.Volume)
		if err != nil {
			return err
		}
		roots, err := fixedNTFSVolumeRoots()
		if err != nil {
			return err
		}
		allowed := false
		for _, candidate := range roots {
			allowed = allowed || candidate == root
		}
		if !allowed {
			return errors.New("file fact volume is not a fixed NTFS drive")
		}
		boundary.Volume = root
		if kind == protocol.Enumerate {
			return EnumerateFacts(ctx, root, emit)
		}
		return ReadFacts(ctx, boundary, kind == protocol.Subscribe, emit)
	})
}

// serveFactsPipe bounds open streams and concurrent scans independently. A slow
// consumer blocks its own native reader; it cannot grow an application queue.
func serveFactsPipe(ctx context.Context, pipeName, ownerSID string, handler factRequestHandler) error {
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return err
	}
	if _, err := windows.StringToSid(ownerSID); err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;" + ownerSID + ")")
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	slots := make(chan struct{}, 32)
	scan := make(chan struct{}, 1)
	for {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		handle, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
			windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
			33, protocol.TargetBatchSize, 4096, 1000, &attributes)
		if err != nil {
			return err
		}
		_, err = factPipeIO(ctx, handle, func(overlapped *windows.Overlapped, _ *uint32) error {
			connectErr := windows.ConnectNamedPipe(handle, overlapped)
			if errors.Is(connectErr, windows.ERROR_PIPE_CONNECTED) {
				return nil
			}
			return connectErr
		})
		if err != nil {
			windows.CloseHandle(handle)
			return err
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { windows.CloseHandle(handle); <-slots }()
			handleFactsConnection(ctx, handle, handler, scan)
		}()
	}
}

// handleFactsConnection watches disconnects while the native journal is idle.
// The second read accepts no commands: closing the connection is cancellation.
func handleFactsConnection(ctx context.Context, handle windows.Handle, handler factRequestHandler, scan chan struct{}) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	requestCtx, stopRequest := context.WithTimeout(ctx, 5*time.Second)
	request, err := protocol.NewReader(factPipe{requestCtx, handle}).Read()
	stopRequest()
	if err != nil || request.Sequence != 0 || request.Session == 0 || (request.Kind != protocol.Enumerate && request.Kind != protocol.Subscribe && request.Kind != protocol.Replay) {
		return
	}
	boundary, err := protocol.DecodeBoundary(request.Payload)
	if err != nil {
		return
	}
	watchDone := make(chan struct{})
	go func() {
		var extra [1]byte
		_, _ = (factPipe{ctx, handle}).Read(extra[:])
		cancel()
		close(watchDone)
	}()
	defer func() { cancel(); <-watchDone }()
	sequence := uint64(0)
	emit := func(kind protocol.Kind, data []byte) error {
		sequence++
		writeCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		return protocol.Write(factPipe{writeCtx, handle}, protocol.Frame{Kind: kind, Session: request.Session, Sequence: sequence, Payload: data})
	}
	if request.Kind == protocol.Enumerate {
		select {
		case scan <- struct{}{}:
			defer func() { <-scan }()
		default:
			_ = emit(protocol.Failure, []byte("another volume enumeration is active"))
			return
		}
	}
	if err := handler(ctx, request.Kind, boundary, emit); err != nil && ctx.Err() == nil {
		message := []byte(err.Error())
		if len(message) > 4096 {
			message = message[:4096]
		}
		_ = emit(protocol.Failure, message)
	}
}

// StreamFacts delivers borrowed frames synchronously and verifies session ordering.
// Replay never waits for changes; Subscribe waits for one read then checkpoints.
func StreamFacts(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, receive FactSink) error {
	return streamFactsPipe(ctx, factsPipeName, kind, boundary, receive)
}

func streamFactsPipe(ctx context.Context, name string, kind protocol.Kind, boundary protocol.Boundary, receive FactSink) error {
	if kind != protocol.Enumerate && kind != protocol.Replay && kind != protocol.Subscribe {
		return errors.New("invalid file fact operation")
	}
	payload, err := protocol.EncodeBoundary(boundary)
	if err != nil {
		return err
	}
	pipeName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var handle windows.Handle
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Second)
	defer cancelConnect()
	for {
		if err := connectCtx.Err(); err != nil {
			return err
		}
		handle, err = windows.CreateFile(pipeName, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-connectCtx.Done():
			timer.Stop()
			return connectCtx.Err()
		case <-timer.C:
		}
	}
	cancelConnect()
	defer windows.CloseHandle(handle)
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	session := binary.LittleEndian.Uint64(nonce[:]) | 1
	pipe := factPipe{ctx, handle}
	if err := protocol.Write(pipe, protocol.Frame{Kind: kind, Session: session, Payload: payload}); err != nil {
		return err
	}
	reader := protocol.NewReader(pipe)
	begun := false
	for sequence := uint64(1); ; sequence++ {
		frame, err := reader.Read()
		if err != nil {
			return err
		}
		if frame.Session != session || frame.Sequence != sequence {
			return errors.New("file fact stream sequence mismatch")
		}
		switch frame.Kind {
		case protocol.Failure:
			return fmt.Errorf("file fact service: %s", frame.Payload)
		case protocol.ResetRequired:
			return ErrFactsReset
		case protocol.Begin:
			if kind != protocol.Enumerate || begun || sequence != 1 {
				return errors.New("unexpected file fact begin")
			}
			begun = true
		case protocol.Records:
			if kind == protocol.Enumerate && !begun {
				return errors.New("file facts arrived before begin")
			}
		case protocol.End:
			if kind != protocol.Enumerate || !begun {
				return errors.New("unexpected file fact end")
			}
		case protocol.Checkpoint:
			if kind == protocol.Enumerate {
				return errors.New("unexpected file fact checkpoint")
			}
		default:
			return errors.New("unexpected file fact response")
		}
		if receive != nil {
			if err := receive(frame.Kind, frame.Payload); err != nil {
				return err
			}
		}
		if frame.Kind == protocol.End || frame.Kind == protocol.Checkpoint {
			return nil
		}
	}
}

type factPipe struct {
	ctx    context.Context
	handle windows.Handle
}

func (p factPipe) Read(data []byte) (int, error) {
	n, err := factPipeIO(p.ctx, p.handle, func(o *windows.Overlapped, n *uint32) error { return windows.ReadFile(p.handle, data, n, o) })
	if errors.Is(err, windows.ERROR_BROKEN_PIPE) || err == nil && n == 0 && len(data) != 0 {
		err = io.EOF
	}
	return int(n), err
}

func (p factPipe) Write(data []byte) (int, error) {
	n, err := factPipeIO(p.ctx, p.handle, func(o *windows.Overlapped, n *uint32) error { return windows.WriteFile(p.handle, data, n, o) })
	return int(n), err
}

// factPipeIO drains an overlapped operation before releasing its Go buffers or
// event, including the race between successful completion and cancellation.
func factPipeIO(ctx context.Context, handle windows.Handle, start func(*windows.Overlapped, *uint32) error) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(event)
	overlapped := windows.Overlapped{HEvent: event}
	var n uint32
	err = start(&overlapped, &n)
	if !errors.Is(err, windows.ERROR_IO_PENDING) {
		return n, err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = windows.CancelIoEx(handle, &overlapped); close(done) })
	err = windows.GetOverlappedResult(handle, &overlapped, &n, true)
	if !stop() {
		<-done
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	return n, err
}
