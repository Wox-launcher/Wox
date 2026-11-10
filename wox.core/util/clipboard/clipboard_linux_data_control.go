//go:build linux && cgo

package clipboard

/*
#cgo pkg-config: wayland-client
#cgo CFLAGS: -D_GNU_SOURCE
#include <stdlib.h>
#include "clipboard_linux_data_control.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"
	"wox/util"
)

// dataControlSelection is a complete clipboard payload captured from one Wayland selection.
type dataControlSelection struct {
	mimeType string
	data     []byte
}

// writeDataControlSelection owns a background clipboard source until the compositor replaces it.
// Retaining a private payload lets callers release or reuse their buffers as soon as this returns.
func writeDataControlSelection(mimeType string, payload []byte) error {
	payload = bytes.Clone(payload)
	nativeMime := C.CString(mimeType)
	defer C.free(unsafe.Pointer(nativeMime))
	var result C.WoxDataControlReadResult
	state := C.wox_data_control_write_start(nativeMime, &result)
	defer C.wox_data_control_read_result_free(&result)
	if state == nil {
		if result.error != nil {
			return errors.New(C.GoString(result.error))
		}
		return errors.New("failed to publish Wayland clipboard selection")
	}
	go func() {
		defer C.wox_data_control_watch_close(state)
		for {
			fd := C.wox_data_control_write_next(state)
			if fd < 0 {
				return
			}
			// Each reader has its own pipe. Serve concurrently so a slow reader
			// cannot delay another paste or the source's cancellation event.
			go serveDataControlSelection(int(fd), payload)
		}
	}()
	return nil
}

// serveDataControlSelection bounds pipe lifetime when a receiver stops reading midway through a paste.
func serveDataControlSelection(fd int, payload []byte) {
	file := os.NewFile(uintptr(fd), "wayland-clipboard-write")
	defer file.Close()
	err := file.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err == nil {
		_, err = file.Write(payload)
	}
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("clipboard: Wayland selection transfer failed: %v", err))
	}
}

// readDataControlSelection reads the regular clipboard through ext-data-control-v1.
func readDataControlSelection() (dataControlSelection, error) {
	var result C.WoxDataControlReadResult
	status := C.wox_data_control_read(&result)
	defer C.wox_data_control_read_result_free(&result)

	if status == 1 {
		return dataControlSelection{}, noDataErr
	}
	if status != 0 {
		if result.error != nil {
			return dataControlSelection{}, errors.New(C.GoString(result.error))
		}
		return dataControlSelection{}, errors.New("ext-data-control-v1 clipboard read failed")
	}
	if result.mime_type == nil || result.size == 0 {
		return dataControlSelection{}, noDataErr
	}

	return dataControlSelection{
		mimeType: C.GoString(result.mime_type),
		data:     C.GoBytes(unsafe.Pointer(result.data), C.int(result.size)),
	}, nil
}

// CaptureNextWaylandCopy observes a new clipboard offer before invoking trigger.
// It distinguishes repeated copies of identical content from the old clipboard.
func CaptureNextWaylandCopy(trigger func() error) (Data, error) {
	var result C.WoxDataControlReadResult
	state := C.wox_data_control_watch_start(&result)
	if state == nil {
		defer C.wox_data_control_read_result_free(&result)
		if result.error != nil {
			return nil, errors.New(C.GoString(result.error))
		}
		return nil, errors.New("failed to start Wayland clipboard observer")
	}
	defer C.wox_data_control_watch_close(state)
	defer C.wox_data_control_read_result_free(&result)
	if err := trigger(); err != nil {
		return nil, err
	}
	if status := C.wox_data_control_watch_next(state, &result); status != 0 || result.mime_type == nil {
		return nil, errors.New("no new readable Wayland clipboard selection")
	}
	payload := dataControlSelection{mimeType: C.GoString(result.mime_type), data: C.GoBytes(unsafe.Pointer(result.data), C.int(result.size))}
	switch dataControlSelectionContentType(payload) {
	case ClipboardTypeText:
		if text := strings.TrimRight(string(payload.data), "\x00"); text != "" {
			return &TextData{Text: text}, nil
		}
	case ClipboardTypeFile:
		if paths := parsePortalURIList(string(payload.data)); len(paths) != 0 {
			return &FilePathData{FilePaths: paths}, nil
		}
	}
	return nil, noDataErr
}
