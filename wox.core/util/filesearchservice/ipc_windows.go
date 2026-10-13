//go:build windows

package filesearchservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const ipcMessageLimit = 64 * 1024

// Serve runs the small management endpoint independently of binary fact streams.
func Serve(ctx context.Context, ownerSID string, handler RequestHandler) error {
	return servePipe(ctx, PipeName, ownerSID, handler)
}

// servePipe bounds management clients and cancels pending connects/reads during
// shutdown. A blocked request cannot consume the binary endpoint's stream slots.
func servePipe(ctx context.Context, pipeName, ownerSID string, handler RequestHandler) error {
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return err
	}
	if _, err := windows.StringToSid(ownerSID); err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;" + ownerSID + ")")
	if err != nil {
		return err
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}
	ctx, cancel := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	slots := make(chan struct{}, 16)
	for {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		handle, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
			windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
			17, ipcMessageLimit, ipcMessageLimit, 1000, &attributes)
		if err != nil {
			return err
		}
		_, err = factPipeIO(ctx, handle, func(o *windows.Overlapped, _ *uint32) error {
			err := windows.ConnectNamedPipe(handle, o)
			if errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
				return nil
			}
			return err
		})
		if err != nil {
			windows.CloseHandle(handle)
			return err
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { windows.CloseHandle(handle); <-slots }()
			requestCtx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			handleRequest(factPipe{requestCtx, handle}, handler)
		}()
	}
}

// handleRequest returns one bounded response; no filename records or query
// results are accepted on this endpoint.
func handleRequest(pipe io.ReadWriter, handler RequestHandler) {
	response := Response{OK: true, Protocol: ProtocolVersion, Version: EmbeddedVersion}
	var request Request
	var after func()
	err := json.NewDecoder(io.LimitReader(pipe, ipcMessageLimit)).Decode(&request)
	if err == nil {
		if request.Protocol != ProtocolVersion {
			err = fmt.Errorf("unsupported protocol %d", request.Protocol)
		} else if request.Method != "health" {
			if request.Method != "update" || handler == nil {
				err = fmt.Errorf("unsupported method %q", request.Method)
			} else {
				after, err = handler(request)
			}
		}
	}
	if err != nil {
		response.OK, response.Error = false, err.Error()
	}
	// An accepted update must finish even if the client disappears after staging.
	_ = json.NewEncoder(pipe).Encode(response)
	if err == nil && after != nil {
		after()
	}
}

// Health verifies that the installed service is responsive and protocol-compatible.
func Health(ctx context.Context) (Response, error) {
	response, err := healthPipe(ctx, PipeName)
	if err != nil {
		markStopped()
	} else {
		markRunning()
	}
	return response, err
}

func healthPipe(ctx context.Context, pipeName string) (Response, error) {
	return callPipe(ctx, pipeName, Request{Method: "health"})
}

// callPipe keeps both connection retry and all pipe IO bounded by cancellation.
func callPipe(ctx context.Context, pipeName string, request Request) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return Response{}, err
	}
	var handle windows.Handle
	for {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		handle, err = windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return Response{}, err
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	defer windows.CloseHandle(handle)
	pipe := factPipe{ctx, handle}
	request.Protocol = ProtocolVersion
	if err := json.NewEncoder(pipe).Encode(request); err != nil {
		return Response{}, err
	}
	return decodeResponse(pipe)
}

// Update asks the service to stage a validated replacement and stop after acknowledgment.
func Update(ctx context.Context, candidatePath, version, sha256 string) error {
	_, err := callPipe(ctx, PipeName, Request{Method: "update", CandidatePath: candidatePath, Version: version, SHA256: sha256})
	return err
}

func decodeResponse(reader io.Reader) (Response, error) {
	var response Response
	if err := json.NewDecoder(io.LimitReader(reader, ipcMessageLimit)).Decode(&response); err != nil {
		return Response{}, err
	}
	if response.Protocol != ProtocolVersion {
		return response, ErrProtocolMismatch
	}
	if !response.OK {
		return response, errors.New(response.Error)
	}
	return response, nil
}
