//go:build windows

package filesearchservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const ipcMessageLimit = 64 * 1024
const snapshotFrameBatchSize = 512

// Serve accepts local authenticated users and handles bounded service requests.
func Serve(ctx context.Context, ownerSID string, handler RequestHandler) error {
	return servePipe(ctx, PipeName, ownerSID, handler)
}

// ServeWithStats also includes the service-owned index state in health responses.
func ServeWithStats(ctx context.Context, ownerSID string, stats func() IndexStats, handler RequestHandler) error {
	return servePipe(ctx, PipeName, ownerSID, handler, stats)
}

func servePipe(ctx context.Context, pipeName string, ownerSID string, handler RequestHandler, statsProviders ...func() IndexStats) error {
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return err
	}
	if _, err := windows.StringToSid(ownerSID); err != nil {
		return fmt.Errorf("invalid service owner SID: %w", err)
	}
	securityDescriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;" + ownerSID + ")")
	if err != nil {
		return err
	}
	securityAttributes := windows.SecurityAttributes{
		Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: securityDescriptor,
	}
	for ctx.Err() == nil {
		handle, err := windows.CreateNamedPipe(
			name,
			windows.PIPE_ACCESS_DUPLEX,
			windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|windows.PIPE_REJECT_REMOTE_CLIENTS,
			windows.PIPE_UNLIMITED_INSTANCES, ipcMessageLimit, ipcMessageLimit, 1000, &securityAttributes,
		)
		if err != nil {
			return fmt.Errorf("create service pipe: %w", err)
		}
		connectErr := windows.ConnectNamedPipe(handle, nil)
		if connectErr != nil && !errors.Is(connectErr, windows.ERROR_PIPE_CONNECTED) {
			windows.CloseHandle(handle)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("connect service pipe: %w", connectErr)
		}
		file := os.NewFile(uintptr(handle), pipeName)
		go func() {
			handleRequest(file, handler, statsProviders...)
			file.Close()
		}()
	}
	return ctx.Err()
}

func handleRequest(file *os.File, handler RequestHandler, statsProviders ...func() IndexStats) {
	var request Request
	decoder := json.NewDecoder(io.LimitReader(file, ipcMessageLimit))
	if err := decoder.Decode(&request); err != nil {
		_ = json.NewEncoder(file).Encode(Response{OK: false, Error: err.Error(), Protocol: ProtocolVersion, Version: EmbeddedVersion})
		return
	}
	encoder := json.NewEncoder(file)
	response := Response{OK: true, Protocol: ProtocolVersion, Version: EmbeddedVersion}
	if request.Protocol != ProtocolVersion {
		response.OK = false
		response.Error = fmt.Sprintf("unsupported protocol %d", request.Protocol)
	} else if request.Method == "health" {
		if len(statsProviders) > 0 && statsProviders[0] != nil {
			stats := statsProviders[0]()
			response.IndexStats = &stats
		}
		_ = encoder.Encode(response)
		return
	} else if handler != nil {
		if err := encoder.Encode(response); err != nil {
			return
		}
		entries := make([]SnapshotEntry, 0, snapshotFrameBatchSize)
		flushEntries := func() error {
			if len(entries) == 0 {
				return nil
			}
			if err := encoder.Encode(snapshotFrame{Entries: entries}); err != nil {
				return err
			}
			entries = make([]SnapshotEntry, 0, snapshotFrameBatchSize)
			return nil
		}
		afterResponse, handlerErr := handler(request, func(entry SnapshotEntry) error {
			entries = append(entries, entry)
			if len(entries) == cap(entries) {
				return flushEntries()
			}
			return nil
		})
		if handlerErr == nil {
			handlerErr = flushEntries()
		}
		frame := snapshotFrame{Done: true}
		if handlerErr != nil {
			frame.Error = handlerErr.Error()
		}
		_ = encoder.Encode(frame)
		if afterResponse != nil {
			afterResponse()
		}
		return
	} else {
		response.OK = false
		response.Error = fmt.Sprintf("unsupported method %q", request.Method)
	}
	_ = encoder.Encode(response)
}

// Health verifies that the installed service is responsive and protocol-compatible.
func Health(ctx context.Context) (Response, error) {
	response, err := healthPipe(ctx, PipeName)
	if err != nil {
		markStopped()
		return response, err
	}
	markRunning()
	if response.IndexStats != nil {
		rememberIndexedVolumeRoots(response.IndexStats.Volumes)
	}
	return response, nil
}

// GetIndexStats loads the current service-owned column index state.
func GetIndexStats(ctx context.Context) (IndexStats, error) {
	response, err := Health(ctx)
	if err != nil {
		return IndexStats{}, err
	}
	if response.IndexStats == nil {
		return IndexStats{}, errors.New("file index service does not expose index stats")
	}
	return *response.IndexStats, nil
}

func healthPipe(ctx context.Context, pipeName string) (Response, error) {
	return callPipe(ctx, pipeName, Request{Protocol: ProtocolVersion, Method: "health"})
}

func call(ctx context.Context, request Request) (Response, error) {
	return callPipe(ctx, PipeName, request)
}

func callPipe(ctx context.Context, pipeName string, request Request) (Response, error) {
	file, err := openPipe(ctx, pipeName)
	if err != nil {
		return Response{}, err
	}
	defer file.Close()
	request.Protocol = ProtocolVersion
	if err := json.NewEncoder(file).Encode(request); err != nil {
		return Response{}, err
	}
	return decodeResponse(file)
}

// Snapshot streams one MFT-backed root snapshot from the installed service.
func Snapshot(ctx context.Context, rootPath string, onEntry func(SnapshotEntry) error) error {
	return SnapshotRoots(ctx, []string{rootPath}, onEntry)
}

// SnapshotRoots streams one MFT read filtered to multiple roots on the same volume.
func SnapshotRoots(ctx context.Context, rootPaths []string, onEntry func(SnapshotEntry) error) error {
	return snapshotRootsPipe(ctx, PipeName, rootPaths, onEntry)
}

// Search sends one filename query and receives only the service's bounded matches.
func Search(ctx context.Context, query string, limit int, usePinyin bool) ([]SnapshotEntry, error) {
	if !IsRunning() {
		return nil, ErrIndexNotReady
	}
	results := make([]SnapshotEntry, 0, limit)
	err := streamRequest(ctx, PipeName, Request{Method: "search", Query: query, Limit: limit, UsePinyin: usePinyin}, func(entry SnapshotEntry) error {
		results = append(results, entry)
		return nil
	})
	if err != nil {
		if err.Error() == ErrIndexNotReady.Error() {
			return nil, ErrIndexNotReady
		}
		return nil, err
	}
	return results, nil
}

// Rebuild asks the service to refresh its all-volume column index in the background.
func Rebuild(ctx context.Context) error {
	return streamRequest(ctx, PipeName, Request{Method: "rebuild"}, nil)
}

// Pause stops journal monitoring, waits for active readers, and unmaps the index.
func Pause(ctx context.Context) error {
	return streamRequest(ctx, PipeName, Request{Method: "pause"}, nil)
}

// Resume opens the recreated Wox-owned directory and starts an asynchronous rebuild.
func Resume(ctx context.Context, indexPath string) error {
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return err
	}
	return streamRequest(ctx, PipeName, Request{Method: "resume", IndexPath: indexPath}, nil)
}

func snapshotPipe(ctx context.Context, pipeName string, rootPath string, onEntry func(SnapshotEntry) error) error {
	return snapshotRootsPipe(ctx, pipeName, []string{rootPath}, onEntry)
}

func snapshotRootsPipe(ctx context.Context, pipeName string, rootPaths []string, onEntry func(SnapshotEntry) error) error {
	request := Request{Method: "snapshot", RootPaths: rootPaths}
	if len(rootPaths) > 0 {
		request.RootPath = rootPaths[0]
	}
	return streamRequest(ctx, pipeName, request, onEntry)
}

// Update asks the running service to install a newer signed helper without UAC.
func Update(ctx context.Context, candidatePath string, version string, sha256 string) error {
	return streamRequest(ctx, PipeName, Request{Method: "update", CandidatePath: candidatePath, Version: version, SHA256: sha256}, nil)
}

func streamRequest(ctx context.Context, pipeName string, request Request, onEntry func(SnapshotEntry) error) error {
	file, err := openPipe(ctx, pipeName)
	if err != nil {
		return err
	}
	defer file.Close()
	request.Protocol = ProtocolVersion
	if err := json.NewEncoder(file).Encode(request); err != nil {
		return err
	}
	decoder := json.NewDecoder(io.LimitReader(file, 1<<63-1))
	if _, err := decodeResponseWithDecoder(decoder); err != nil {
		return err
	}
	for {
		var frame snapshotFrame
		if err := decoder.Decode(&frame); err != nil {
			return err
		}
		if frame.Error != "" {
			return errors.New(frame.Error)
		}
		if frame.Done {
			return nil
		}
		if frame.Entry != nil && onEntry != nil {
			if err := onEntry(*frame.Entry); err != nil {
				return err
			}
		}
		if onEntry != nil {
			for _, entry := range frame.Entries {
				if err := onEntry(entry); err != nil {
					return err
				}
			}
		}
	}
}

func openPipe(ctx context.Context, pipeName string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(pipeName)
	if err != nil {
		return nil, err
	}
	var handle windows.Handle
	for {
		handle, err = windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.SECURITY_SQOS_PRESENT|windows.SECURITY_IDENTIFICATION, 0)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_PIPE_BUSY) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return os.NewFile(uintptr(handle), pipeName), nil
}

func decodeResponse(file io.Reader) (Response, error) {
	return decodeResponseWithDecoder(json.NewDecoder(io.LimitReader(file, ipcMessageLimit)))
}

func decodeResponseWithDecoder(decoder *json.Decoder) (Response, error) {
	var response Response
	if err := decoder.Decode(&response); err != nil {
		return Response{}, err
	}
	if !response.OK {
		return response, errors.New(response.Error)
	}
	return response, nil
}
