//go:build linux && cgo

package clipboard

/*
#cgo pkg-config: wayland-client
#cgo CFLAGS: -D_GNU_SOURCE
#include "clipboard_linux_data_control.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// dataControlSelection is a complete clipboard payload captured from one Wayland selection.
type dataControlSelection struct {
	mimeType string
	data     []byte
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
