//go:build windows

package supervisor

import (
	"errors"
	"io"
	"net"
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

func defaultControlAddress() string {
	return `\\.\pipe\WoxSupervisor`
}

func dialControl() (io.ReadWriteCloser, error) {
	name, err := windows.UTF16PtrFromString(controlAddress())
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), "WoxSupervisor"), nil
}

type controlListener interface {
	Accept() (io.ReadWriteCloser, error)
	Close() error
}

type pipeListener struct {
	name   *uint16
	mu     sync.Mutex
	active windows.Handle
	closed bool
}

func listenControl() (controlListener, error) {
	name, err := windows.UTF16PtrFromString(controlAddress())
	if err != nil {
		return nil, err
	}
	return &pipeListener{name: name}, nil
}

// Accept uses cancellable connection I/O so closing an idle listener cannot block process exit.
func (l *pipeListener) Accept() (io.ReadWriteCloser, error) {
	handle, err := windows.CreateNamedPipe(
		l.name,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
		windows.PIPE_UNLIMITED_INSTANCES,
		1<<20,
		1<<20,
		1000,
		nil,
	)
	if err != nil {
		return nil, err
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	defer windows.CloseHandle(event)
	overlapped := windows.Overlapped{HEvent: event}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = windows.CloseHandle(handle)
		return nil, net.ErrClosed
	}
	l.active = handle
	// Start the operation under the lock so Close cannot cancel before it is submitted.
	connectErr := windows.ConnectNamedPipe(handle, &overlapped)
	l.mu.Unlock()

	if errors.Is(connectErr, windows.ERROR_IO_PENDING) {
		var transferred uint32
		connectErr = windows.GetOverlappedResult(handle, &overlapped, &transferred, true)
	}
	l.mu.Lock()
	closed := l.closed
	l.active = 0
	l.mu.Unlock()
	if closed {
		_ = windows.CloseHandle(handle)
		return nil, net.ErrClosed
	}
	if connectErr != nil && !errors.Is(connectErr, windows.ERROR_PIPE_CONNECTED) {
		_ = windows.CloseHandle(handle)
		return nil, connectErr
	}
	return os.NewFile(uintptr(handle), "WoxSupervisor"), nil
}

func (l *pipeListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.active != 0 {
		// Accept owns the handle until cancellation completes. Closing a handle
		// with synchronous ConnectNamedPipe pending can block indefinitely.
		_ = windows.CancelIoEx(l.active, nil)
	}
	return nil
}
